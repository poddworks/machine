package aws

import (
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

var (
	// AWS EC2 client object for establishing command
	svc *ec2.EC2
)

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "aws",
		Usage: "Create and Manage AWS machine",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "region", EnvVar: "AWS_REGION", Usage: "AWS Region"},
			cli.StringFlag{Name: "key", EnvVar: "AWS_ACCESS_KEY_ID", Usage: "AWS access key"},
			cli.StringFlag{Name: "secret", EnvVar: "AWS_SECRET_ACCESS_KEY", Usage: "AWS secret key"},
			cli.StringFlag{Name: "token", EnvVar: "AWS_SESSION_TOKEN", Usage: "session token for temporary credentials"},
			cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
			cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
		},
		Before: func(c *cli.Context) error {
			// bootstrap EC2 client with command line args
			cfg := aws.NewConfig()
			if region := c.String("region"); region != "" {
				cfg = cfg.WithRegion(region)
			}
			if id, secret, token := c.String("key"), c.String("secret"), c.String("token"); id != "" && secret != "" {
				cfg = cfg.WithCredentials(credentials.NewStaticCredentials(id, secret, token))
			}
			svc = ec2.New(session.New(cfg))
			return nil
		},
		Subcommands: []cli.Command{
			newConfigCommand(),
			newCreateCommand(),
			newImageCommand(),
			newRmCommand(),
		},
	}
}

func newRmCommand() cli.Command {
	return cli.Command{
		Name:  "rm",
		Usage: "Remove and Terminate instance",
		Action: func(c *cli.Context) error {
			_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: []*string{
					aws.String(c.Args().First()),
				},
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return nil
		},
	}
}

func newConfigCommand() cli.Command {
	return cli.Command{
		Name:  "config",
		Usage: "Configure AWS environment",
		Subcommands: []cli.Command{
			syncFromAWS(),
			getFromAWSConfig(),
		},
	}
}

func newCreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "Create a new EC2 instance",
		Flags: []cli.Flag{
			cli.BoolTFlag{Name: "use-docker", Usage: "Opt in to use Docker Engine"},
			cli.StringFlag{Name: "ami-id", Usage: "EC2 instance AMI ID"},
			cli.IntFlag{Name: "count", Value: 1, Usage: "EC2 instances to launch in this request"},
			cli.StringSliceFlag{Name: "group", Usage: "Network security group for user"},
			cli.StringFlag{Name: "iam-role", Usage: "EC2 IAM Role to apply"},
			cli.StringFlag{Name: "profile", Value: "default", Usage: "Name of the profile"},
			cli.IntFlag{Name: "root-size", Value: 8, Usage: "EC2 root volume size"},
			cli.StringFlag{Name: "ssh-key", Usage: "EC2 instance SSH KeyPair"},
			cli.BoolFlag{Name: "subnet-private", Usage: "Launch EC2 instance to internal subnet"},
			cli.StringFlag{Name: "subnet-id", Usage: "Launch EC2 instance to the specified subnet"},
			cli.StringSliceFlag{Name: "tag", Usage: "EC2 instance tag in the form field=value"},
			cli.StringFlag{Name: "type", Value: "t2.micro", Usage: "EC2 instance type"},
			cli.IntSliceFlag{Name: "volume-size", Usage: "EC2 EBS volume size"},
		},
		Action: func(c *cli.Context) error {
			var (
				profile = make(AWSProfile)

				org, certpath, _ = mach.ParseCertArgs(c)

				user = c.GlobalString("user")
				cert = c.GlobalString("cert")

				useDocker = c.Bool("use-docker")

				instList = make(mach.RegisteredInstances)

				inst *mach.Host
			)

			if useDocker {
				inst = mach.NewDockerHost(org, certpath, user, cert)
			} else {
				inst = mach.NewHost(org, certpath, user, cert)
			}

			// Load from AWS configuration from last sync
			profile.Load()

			region, ok := profile[c.GlobalString("region")]
			if !ok {
				fmt.Fprintln(os.Stderr, "Please run sync in the region of choice")
				os.Exit(1)
			}
			p, ok := region[c.String("profile")]
			if !ok {
				fmt.Fprintln(os.Stderr, "Unable to find matching VPC profile")
				os.Exit(1)
			}

			// Load from Instance Roster to register and defer write back
			defer instList.Load().Dump()

			// Invoke EC2 launch procedure
			for state := range newEC2Inst(c, p, inst) {
				if addr, _ := net.ResolveTCPAddr("tcp", *state.PublicIpAddress+":2376"); state.err == nil {
					fmt.Printf("%s - %s - Instance ID: %s\n", *state.PublicIpAddress, *state.PrivateIpAddress, *state.InstanceId)
					if useDocker {
						instList[*state.InstanceId] = &mach.Instance{
							Name:       *state.InstanceId,
							Driver:     "aws",
							DockerHost: addr,
							State:      "running",
						}
					}
				} else {
					fmt.Fprintln(os.Stderr, state.err)
				}
			}

			return nil
		},
	}
}

func newImageCommand() cli.Command {
	return cli.Command{
		Name:  "register-ami",
		Usage: "Register an AMI from specification",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "instance-id", Usage: "EC2 instance ID"},
			cli.StringFlag{Name: "name", Usage: "EC2 AMI Name"},
			cli.StringFlag{Name: "desc", Usage: "EC2 AMI Description"},
		},
		Action: func(c *cli.Context) error {
			var (
				instId = c.String("instance-id")
				name   = c.String("name")
				desc   = c.String("desc")
			)

			resp, err := svc.CreateImage(&ec2.CreateImageInput{
				InstanceId:  aws.String(instId),
				Name:        aws.String(name),
				Description: aws.String(desc),
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			} else {
				fmt.Println(*resp.ImageId)
			}

			return nil
		},
	}
}
