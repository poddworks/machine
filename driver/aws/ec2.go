package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"math/rand"
	"os"
	"strings"
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

func ec2_tagInstance(tags []string, instances []*ec2.Instance) *ec2.CreateTagsInput {
	tagparam := &ec2.CreateTagsInput{
		Tags:      make([]*ec2.Tag, 0, len(tags)),
		Resources: make([]*string, 0, len(instances)),
	}
	for _, inst := range instances {
		fmt.Println(*inst.InstanceId)
		tagparam.Resources = append(tagparam.Resources, inst.InstanceId)
	}
	for _, tag := range tags {
		var parts = strings.SplitN(tag, "=", 2)
		tagparam.Tags = append(tagparam.Tags, &ec2.Tag{
			Key:   aws.String(parts[0]),
			Value: aws.String(parts[1]),
		})
	}
	return tagparam
}

func newEC2Inst(c *cli.Context, profile *Profile) {
	var (
		amiId            = c.String("instance-ami-id")
		num2Launch       = c.Int("instance-count")
		iamProfile       = c.String("instance-profile")
		instTags         = c.StringSlice("instance-tag")
		instType         = c.String("instance-type")
		isPrivate        = c.Bool("subnet-private")
		subnetId         = c.String("subnet-id")
		networkACLGroups = c.StringSlice("security-group")
	)
	ec2param := &ec2.RunInstancesInput{
		InstanceType:     aws.String(instType),
		MaxCount:         aws.Int64(int64(num2Launch)),
		MinCount:         aws.Int64(1),
		SecurityGroupIds: ec2_findSecurityGroup(&profile.VPC, networkACLGroups...),
	}
	if amiId != "" {
		ec2param.ImageId = aws.String(amiId)
	} else {
		ec2param.ImageId = profile.Ami.Id
	}
	if strings.HasPrefix(iamProfile, "arn:aws:iam") {
		ec2param.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Arn: aws.String(iamProfile),
		}
	} else if iamProfile != "" {
		ec2param.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Name: aws.String(iamProfile),
		}
	}
	if subnetId != "" {
		ec2param.SubnetId = aws.String(subnetId)
	} else {
		ec2param.SubnetId = ec2_getSubnet(&profile.VPC, !isPrivate)
	}
	resp, err := svc.RunInstances(ec2param)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	_, err = svc.CreateTags(ec2_tagInstance(instTags, resp.Instances))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
