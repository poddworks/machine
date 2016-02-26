package aws

import (
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
)

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "aws",
		Usage: "Manage machine on AWS",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "region", EnvVar: "AWS_REGION", Usage: "AWS Region"},
			cli.StringFlag{Name: "key", EnvVar: "AWS_ACCESS_KEY_ID", Usage: "AWS access key"},
			cli.StringFlag{Name: "secret", EnvVar: "AWS_SECRET_ACCESS_KEY", Usage: "AWS secret key"},
			cli.StringFlag{Name: "token", EnvVar: "AWS_SESSION_TOKEN", Usage: "session token for temporary credentials"},
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
			sess = session.New(cfg)
			svc = ec2.New(sess)
			return nil
		},
		Subcommands: []cli.Command{
			{
				Name:  "sync",
				Usage: "bootstrap cluster environment",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Value: "default", Usage: "Name of the profile"},
					cli.StringFlag{Name: "vpc-id", Value: "default", Usage: "AWS VPC identifier"},
				},
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
			},
			{
				Name:  "create",
				Usage: "create a new EC2 instance",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "profile", Value: "default", Usage: "Name of the profile"},
					cli.StringFlag{Name: "instance-ami-id", Usage: "EC2 instance AMI ID"},
					cli.IntFlag{Name: "instance-count", Value: 1, Usage: "EC2 instances to launch in this request"},
					cli.StringFlag{Name: "instance-key", Usage: "EC2 instance SSH KeyPair"},
					cli.StringFlag{Name: "instance-profile", Usage: "EC2 IAM Role to apply"},
					cli.StringSliceFlag{Name: "instance-tag", Usage: "EC2 instance tag in the form field=value"},
					cli.IntFlag{Name: "instance-root-size", Value: 8, Usage: "EC2 root volume size"},
					cli.IntSliceFlag{Name: "instance-volume-size", Usage: "EC2 EBS volume size"},
					cli.StringFlag{Name: "instance-type", Value: "t2.micro", Usage: "EC2 instance type"},
					cli.BoolTFlag{Name: "is-docker-engine", Usage: "Launched EC2 instance is a Docker Engine"},
					cli.BoolFlag{Name: "subnet-private", Usage: "Launch EC2 instance to internal subnet"},
					cli.StringFlag{Name: "subnet-id", Usage: "Launch EC2 instance to the specified subnet"},
					cli.StringSliceFlag{Name: "security-group", Usage: "Network security group for user"},
				},
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
						newEC2Inst(c, p)
					}
				},
			},
			{
				Name:  "create-ami",
				Usage: "Create AMI from specification",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "instance-id", Usage: "EC2 instance ID"},
					cli.StringFlag{Name: "name", Usage: "EC2 AMI Name"},
					cli.StringFlag{Name: "desc", Usage: "EC2 AMI Description"},
				},
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
			},
		},
	}
}
