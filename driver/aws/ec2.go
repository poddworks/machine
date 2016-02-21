package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"math/rand"
	"os"
)

func ami_getInfo() (ami *ec2.Image) {
	amiparam := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("owner-id"),
				Values: []*string{
					aws.String("564092832996"),
				},
			},
			{
				Name: aws.String("name"),
				Values: []*string{
					aws.String("docker-image-2016-02-21"),
				},
			},
			{
				Name: aws.String("is-public"),
				Values: []*string{
					aws.String("true"),
				},
			},
		},
	}
	if resp, err := svc.DescribeImages(amiparam); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	} else if len(resp.Images) == 0 {
		fmt.Fprint(os.Stderr, "Unexpected state: unable to retrieve AMI for docker engine")
		os.Exit(1)
	} else {
		ami = resp.Images[0] // there should be one only
	}
	return
}

func amiInit(c *cli.Context, profile *AMIProfile) {
	var ami = ami_getInfo()
	profile.Arch = ami.Architecture
	profile.Desc = ami.Description
	profile.Id = ami.ImageId
	profile.Name = ami.Name
}

func ec2_getSubnet(profile *VPCProfile, public bool) (subnetId *string) {
	var collection []*string
	for _, subnet := range profile.Subnet {
		if public && *subnet.Public {
			collection = append(collection, subnet.Id)
		} else if !public && !*subnet.Public {
			collection = append(collection, subnet.Id)
		}
	}
	idx := rand.Intn(len(collection))
	return collection[idx]
}

func ec2_findSecurityGroup(profile *VPCProfile, name ...string) (sgId []*string) {
	sgId = make([]*string, 0)
	for _, grp := range name {
		for _, sgrp := range profile.SecurityGroup {
			if *sgrp.Name == grp {
				sgId = append(sgId, sgrp.Id)
			}
		}
	}
	return
}

func newEC2Inst(c *cli.Context, profile *Profile) {
	var (
		instType         = c.String("type")
		num2Launch       = c.Int("count")
		isPrivate        = c.Bool("private")
		networkACLGroups = c.StringSlice("group")
	)
	ec2param := &ec2.RunInstancesInput{
		ImageId:          profile.Ami.Id,
		InstanceType:     aws.String(instType),
		MaxCount:         aws.Int64(int64(num2Launch)),
		MinCount:         aws.Int64(1),
		SecurityGroupIds: ec2_findSecurityGroup(&profile.VPC, networkACLGroups...),
		SubnetId:         ec2_getSubnet(&profile.VPC, !isPrivate),
	}
	if resp, err := svc.RunInstances(ec2param); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	} else {
		for _, inst := range resp.Instances {
			fmt.Println(*inst.InstanceId)
		}
	}
	return
}
