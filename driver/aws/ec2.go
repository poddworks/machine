package aws

import (
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

var (
	DEVICE_NAME = []string{"b", "c", "d", "e", "f", "g", "h", "i"}
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

func ec2_tagInstanceParam(tags []string) (*ec2.CreateTagsInput, error) {
	tagparam := &ec2.CreateTagsInput{
		Tags:      make([]*ec2.Tag, 0),
		Resources: make([]*string, 0),
	}
	for _, tag := range tags {
		var parts = strings.SplitN(tag, "=", 2)
		if len(parts) == 2 {
			tagparam.Tags = append(tagparam.Tags, &ec2.Tag{
				Key:   aws.String(parts[0]),
				Value: aws.String(parts[1]),
			})
		} else {
			return nil, fmt.Errorf("Skipping bad tag spec - %s\n", tag)
		}
	}
	return tagparam, nil
}

func ec2_EbsRoot(size int) (mapping *ec2.BlockDeviceMapping) {
	return &ec2.BlockDeviceMapping{
		DeviceName: aws.String("xvda"),
		Ebs: &ec2.EbsBlockDevice{
			DeleteOnTermination: aws.Bool(true),
			VolumeSize:          aws.Int64(int64(size)),
			VolumeType:          aws.String(ec2.VolumeTypeGp2),
		},
	}
}

func ec2_EbsVols(size ...int) (mapping []*ec2.BlockDeviceMapping) {
	mapping = make([]*ec2.BlockDeviceMapping, 0)
	for i, volSize := range size {
		if volSize <= 0 {
			fmt.Fprintln(os.Stderr, "Skipping bad volume size", volSize)
			continue
		}
		if i >= len(DEVICE_NAME) {
			fmt.Fprintln(os.Stderr, "You had more volumes then AWS allowed")
			os.Exit(1)
		}
		mapping = append(mapping, &ec2.BlockDeviceMapping{
			DeviceName: aws.String("xvd" + DEVICE_NAME[i]),
			Ebs: &ec2.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				VolumeSize:          aws.Int64(int64(volSize)),
				VolumeType:          aws.String(ec2.VolumeTypeGp2),
			},
		})
	}
	return
}

func ec2_ConfigureEngineCert(inst *ec2.Instance) error {
	var (
		CertPath     = "/home/yihungjen/.machine"
		Organization = "podd.org"

		Hosts = []string{
			*inst.PublicIpAddress,
			*inst.PrivateIpAddress,
			"localhost",
			"127.0.0.1",
		}

		ssh_config = ssh.Config{
			User:   "ubuntu",
			Server: *inst.PublicIpAddress,
			Key:    "/home/yihungjen/.ssh/podd.pem",
			Port:   "22",
		}
	)

	fmt.Println(*inst.InstanceId, "- generate cert for subjects -", Hosts)
	CA, Cert, Key, err := cert.GenerateServerCertificate(CertPath, Organization, Hosts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}
	fmt.Println(*inst.InstanceId, "- configure docker engine")

	time.Sleep(10 * time.Second) // FIXME: Wait for SSH to come up fully

	err = cert.SendEngineCertificate(CA, Cert, Key, ssh_config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}
	return nil
}

func ec2_WaitForReady(output chan error, instId *string, next func(*ec2.Instance) error) {
	param := &ec2.DescribeInstancesInput{InstanceIds: []*string{instId}}
	if err := svc.WaitUntilInstanceRunning(param); err != nil {
		fmt.Fprintln(os.Stderr, *instId, "-", err.Error())
		output <- err
		return
	}
	resp, err := svc.DescribeInstances(param)
	if err != nil {
		fmt.Fprintln(os.Stderr, *instId, "-", err.Error())
		output <- err
	} else {
		inst := resp.Reservations[0].Instances[0]
		go func() {
			fmt.Println(*inst.InstanceId, "- ready")
			output <- next(inst)
		}()
	}
	// NOTE: this should end here
}

func newEC2Inst(c *cli.Context, profile *Profile) {
	var (
		amiId            = c.String("instance-ami-id")
		keyName          = c.String("instance-key")
		num2Launch       = c.Int("instance-count")
		iamProfile       = c.String("instance-profile")
		instTags         = c.StringSlice("instance-tag")
		instType         = c.String("instance-type")
		instVolRoot      = c.Int("instance-root-size")
		instVols         = c.IntSlice("instance-volume-size")
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

	// Step 1: determine the Amazone Machine Image ID
	if amiId != "" {
		ec2param.ImageId = aws.String(amiId)
	} else if len(profile.VPC.Ami) != 0 {
		ec2param.ImageId = profile.VPC.Ami[0].Id
	} else {
		fmt.Fprintln(os.Stderr, "Cannot proceed without an AMI")
		os.Exit(1)
	}

	// Step 2: determine keypair to use for remote access
	if keyName != "" {
		ec2param.KeyName = aws.String(keyName)
	} else if len(profile.VPC.KeyPair) != 0 {
		ec2param.KeyName = profile.VPC.KeyPair[0].Name
	} else {
		fmt.Fprintln(os.Stderr, "Cannot proceed without SSH keypair")
		os.Exit(1)
	}

	// Step 3: determine EBS Volume configuration
	ec2param.BlockDeviceMappings = make([]*ec2.BlockDeviceMapping, 0)
	if instVolRoot > 0 {
		ec2param.BlockDeviceMappings = append(ec2param.BlockDeviceMappings, ec2_EbsRoot(instVolRoot))
	}
	if len(instVols) > 0 {
		var mapping = ec2_EbsVols(instVols...)
		ec2param.BlockDeviceMappings = append(ec2param.BlockDeviceMappings, mapping...)
	}

	// Step 4: assign IAM role for the EC2 machine
	if strings.HasPrefix(iamProfile, "arn:aws:iam") {
		ec2param.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Arn: aws.String(iamProfile),
		}
	} else if iamProfile != "" {
		ec2param.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
			Name: aws.String(iamProfile),
		}
	}

	// Step 5: assign accessibility of EC2 instance by subnet
	if subnetId != "" {
		ec2param.SubnetId = aws.String(subnetId)
	} else {
		ec2param.SubnetId = ec2_getSubnet(&profile.VPC, !isPrivate)
	}

	// Step 6: create tags from spec
	tag, err := ec2_tagInstanceParam(instTags)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Last step: launch + tag instances
	resp, err := svc.RunInstances(ec2param)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if len(tag.Tags) > 0 {
		for _, inst := range resp.Instances {
			tag.Resources = append(tag.Resources, inst.InstanceId)
		}
		_, err = svc.CreateTags(tag)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	fmt.Println("Launched instances...")

	var collect = make(chan error)
	for _, inst := range resp.Instances {
		fmt.Println(*inst.InstanceId, "- pending")
		go ec2_WaitForReady(collect, inst.InstanceId, ec2_ConfigureEngineCert)
	}
	for exp := 0; exp < len(resp.Instances); exp++ {
		<-collect
	}
}
