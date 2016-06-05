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

	// AWS Profile
	profile = make(AWSProfile)

	// Instance Roster
	instList = make(mach.RegisteredInstances)
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
			newStartCommand(),
			newStopCommand(),
			newRmCommand(),
			newImageCommand(),
		},
	}
}

func newStartCommand() cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "Start instance",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			defer instList.Dump()

			for _, name := range c.Args() {
				info, ok := instList[name]
				if !ok {
					fmt.Fprintln(os.Stderr, "Target machine [", name, "] not found")
					continue
				}

				_, err := svc.StartInstances(&ec2.StartInstancesInput{
					InstanceIds: []*string{
						aws.String(info.Id),
					},
				})
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}

				if state := <-ec2_WaitForReady(&info.Id); state.err != nil {
					fmt.Fprintln(os.Stderr, "Target machine [", name, "] failed to launch")
				} else {
					addr, _ := net.ResolveTCPAddr("tcp", *state.PublicIpAddress+":2376")
					info.DockerHost = addr
					info.State = "running"
				}
			}

			return nil
		},
	}
}

func newStopCommand() cli.Command {
	return cli.Command{
		Name:  "stop",
		Usage: "Stop instance",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			defer instList.Dump()

			for _, name := range c.Args() {
				info, ok := instList[name]
				if !ok {
					fmt.Fprintln(os.Stderr, "Target machine [", name, "] not found")
					continue
				}

				_, err := svc.StopInstances(&ec2.StopInstancesInput{
					InstanceIds: []*string{
						aws.String(info.Id),
					},
				})
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}

				info.DockerHost = nil
				info.State = "stopped"
			}

			return nil
		},
	}
}

func newRmCommand() cli.Command {
	return cli.Command{
		Name:  "rm",
		Usage: "Remove and Terminate instance",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			defer instList.Dump()

			for _, name := range c.Args() {
				info, ok := instList[name]
				if !ok {
					fmt.Fprintln(os.Stderr, "Target machine [", name, "] not found")
					continue
				}

				_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
					InstanceIds: []*string{
						aws.String(info.Id),
					},
				})
				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}

				delete(instList, name)
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
			cli.BoolFlag{Name: "use-docker", Usage: "Opt in to use Docker Engine"},
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
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			defer instList.Dump()

			var (
				profile = make(AWSProfile)

				user = c.GlobalString("user")
				cert = c.GlobalString("cert")

				name = c.Args().First()

				num2Launch = c.Int("count")
				useDocker  = c.Bool("use-docker")

				org, certpath, _ = mach.ParseCertArgs(c)
			)

			// Load from AWS configuration from last sync
			if err := profile.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			if name == "" {
				return cli.NewExitError("Required argument `name` missing", 1)
			} else if _, ok := instList[name]; ok {
				return cli.NewExitError("Machine exist", 1)
			}

			region, ok := profile[c.GlobalString("region")]
			if !ok {
				return cli.NewExitError("Please run sync in the region of choice", 1)
			}
			p, ok := region[c.String("profile")]
			if !ok {
				return cli.NewExitError("Unable to find matching VPC profile", 1)
			}

			instances, err := newEC2Inst(c, p, num2Launch)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			// Invoke EC2 launch procedure
			for state := range deployEC2Inst(user, cert, name, org, certpath, num2Launch, useDocker, instances) {
				if state.err == nil {
					addr, _ := net.ResolveTCPAddr("tcp", *state.PublicIpAddress+":2376")
					fmt.Printf("%s - %s - Instance ID: %s\n", *state.PublicIpAddress, *state.PrivateIpAddress, *state.InstanceId)
					if useDocker {
						instList[state.name] = &mach.Instance{
							Id:         *state.InstanceId,
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
				return cli.NewExitError(err.Error(), 1)
			} else {
				fmt.Println(*resp.ImageId)
			}

			return nil
		},
	}
}
