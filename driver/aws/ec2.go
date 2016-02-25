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
		if len(parts) == 2 {
			tagparam.Tags = append(tagparam.Tags, &ec2.Tag{
				Key:   aws.String(parts[0]),
				Value: aws.String(parts[1]),
			})
		} else {
			fmt.Fprint(os.Stderr, "Skipping bad tag spec", tag)
		}
	}
	return tagparam
}

func newEC2Inst(c *cli.Context, profile *Profile) {
	var (
		amiId            = c.String("instance-ami-id")
		keyName          = c.String("instance-key")
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
	if keyName != "" {
		ec2param.KeyName = aws.String(keyName)
	} else if len(profile.VPC.KeyPair) != 0 {
		ec2param.KeyName = profile.VPC.KeyPair[0].Name
	} else {
		fmt.Fprint(os.Stderr, "Cannot proceed without SSH keypair")
		os.Exit(1)
	}
	if amiId != "" {
		ec2param.ImageId = aws.String(amiId)
	} else if len(profile.VPC.Ami) != 0 {
		ec2param.ImageId = profile.VPC.Ami[0].Id
	} else {
		fmt.Fprint(os.Stderr, "Cannot proceed without an AMI")
		os.Exit(1)
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
