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
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

var (
	// Config reference for AWS API
	sess *session.Session

	// AWS EC2 client object for establishing command
	svc *ec2.EC2

	// AWS command common flags
	flags = []cli.Flag{
		cli.StringFlag{Name: "region", EnvVar: "AWS_REGION", Usage: "AWS Region"},
		cli.StringFlag{Name: "key", EnvVar: "AWS_ACCESS_KEY_ID", Usage: "AWS access key"},
		cli.StringFlag{Name: "secret", EnvVar: "AWS_SECRET_ACCESS_KEY", Usage: "AWS secret key"},
		cli.StringFlag{Name: "token", EnvVar: "AWS_SESSION_TOKEN", Usage: "session token for temporary credentials"},
	}
)

func appendFlag(flag ...cli.Flag) []cli.Flag {
	return append(flags, flag...)
}

func before(c *cli.Context) error {
	// bootstrap EC2 client with command line args
	cfg := aws.NewConfig()
	if region := c.String("region"); region != "" {
		cfg = cfg.WithRegion(region)
	}
	if id, secret, token := c.String("key"), c.String("secret"), c.String("token"); id != "" && secret != "" {
		cfg = cfg.WithCredentials(credentials.NewStaticCredentials(id, secret, token))
	}
	sess = session.New(cfg)
	svc = ec2.New(sess)
	return nil
}

func NewConfigCommand() cli.Command {
	return cli.Command{
		Name:  "aws",
		Usage: "Bootstrap AWS environment",
		Flags: appendFlag(
			cli.StringFlag{Name: "name", Value: "default", Usage: "Name of the profile"},
			cli.StringFlag{Name: "vpc-id", Value: "default", Usage: "AWS VPC identifier"},
		),
		Before: before,
		Action: func(c *cli.Context) {
			var profile = make(AWSProfile)
			defer profile.Load().Dump()
			p := &Profile{Name: c.String("name"), Region: *sess.Config.Region}
			vpcInit(c, &p.VPC)
			amiInit(c, &p.VPC)
			if _, ok := profile[p.Region]; !ok {
				profile[p.Region] = make(RegionProfile)
			}
			profile[p.Region][p.Name] = p
		},
	}
}

func NewCreateCommand(host *mach.Host) cli.Command {
	return cli.Command{
		Name:  "aws",
		Usage: "Create a new EC2 instance",
		Flags: appendFlag(
			cli.StringFlag{Name: "profile", Value: "default", Usage: "Name of the profile"},
			cli.StringFlag{Name: "instance-ami-id", Usage: "EC2 instance AMI ID"},
			cli.IntFlag{Name: "instance-count", Value: 1, Usage: "EC2 instances to launch in this request"},
			cli.StringFlag{Name: "instance-key", Usage: "EC2 instance SSH KeyPair"},
			cli.StringFlag{Name: "instance-profile", Usage: "EC2 IAM Role to apply"},
			cli.StringSliceFlag{Name: "instance-tag", Usage: "EC2 instance tag in the form field=value"},
			cli.IntFlag{Name: "instance-root-size", Value: 8, Usage: "EC2 root volume size"},
			cli.IntSliceFlag{Name: "instance-volume-size", Usage: "EC2 EBS volume size"},
			cli.StringFlag{Name: "instance-type", Value: "t2.micro", Usage: "EC2 instance type"},
			cli.BoolFlag{Name: "subnet-private", Usage: "Launch EC2 instance to internal subnet"},
			cli.StringFlag{Name: "subnet-id", Usage: "Launch EC2 instance to the specified subnet"},
			cli.StringSliceFlag{Name: "security-group", Usage: "Network security group for user"},
		),
		Before: before,
		Action: func(c *cli.Context) {
			var profile = make(AWSProfile)
			profile.Load()
			region, ok := profile[*sess.Config.Region]
			if !ok {
				fmt.Println("Please run sync in the region of choice")
				os.Exit(1)
			}
			if p, ok := region[c.String("profile")]; !ok {
				fmt.Println("Unable to find matching VPC profile")
				os.Exit(1)
			} else {
				newEC2Inst(c, p, host)
			}
		},
	}
}

func NewImageCommand() cli.Command {
	return cli.Command{
		Name:  "aws",
		Usage: "Register an AMI from specification",
		Flags: appendFlag(
			cli.StringFlag{Name: "instance-id", Usage: "EC2 instance ID"},
			cli.StringFlag{Name: "name", Usage: "EC2 AMI Name"},
			cli.StringFlag{Name: "desc", Usage: "EC2 AMI Description"},
		),
		Before: before,
		Action: func(c *cli.Context) {
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
				fmt.Println(err.Error())
				os.Exit(1)
			} else {
				fmt.Println(*resp.ImageId)
			}
		},
	}
}
