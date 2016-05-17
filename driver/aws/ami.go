package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"os"
)

func ami_getInfo() (ami []*ec2.Image) {
	amiparam := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("owner-id"),
				Values: []*string{
					aws.String("099720109477"),
				},
			},
			{
				Name: aws.String("name"),
				Values: []*string{
					aws.String("ubuntu/images/hvm-ssd/ubuntu-trusty-14.04-amd64-server-20160114.5"),
				},
			},
		},
	}
	if resp, err := svc.DescribeImages(amiparam); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	} else {
		ami = resp.Images
	}
	return
}

func amiInit(c *cli.Context, profile *Profile) {
	profile.Ami = make([]AMIProfile, 0)
	for _, ami := range ami_getInfo() {
		profile.Ami = append(profile.Ami, AMIProfile{
			Arch: ami.Architecture,
			Desc: ami.Description,
			Id:   ami.ImageId,
			Name: ami.Name,
		})
	}
	return
}
