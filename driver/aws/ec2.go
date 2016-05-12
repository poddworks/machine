package aws

import (
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
)

var (
	DEVICE_NAME = []string{"b", "c", "d", "e", "f", "g", "h", "i"}
)

type ec2state struct {
	*ec2.Instance
	err error
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

func ec2_WaitForReady(instId *string) <-chan ec2state {
	out := make(chan ec2state)
	go func() {
		defer close(out)
		param := &ec2.DescribeInstancesInput{InstanceIds: []*string{instId}}
		if err := svc.WaitUntilInstanceRunning(param); err != nil {
			out <- ec2state{nil, fmt.Errorf("%s - %s", *instId, err)}
			return
		}
		resp, err := svc.DescribeInstances(param)
		if err != nil {
			out <- ec2state{nil, fmt.Errorf("%s - %s", *instId, err)}
		} else {
			out <- ec2state{resp.Reservations[0].Instances[0], nil}
		}
		// NOTE: this should end here
	}()
	return out
}

func newEC2Inst(c *cli.Context, profile *Profile, ec2inst *mach.Host) {
	var (
		amiId            = c.String("ami-id")
		num2Launch       = c.Int("count")
		networkACLGroups = c.StringSlice("group")
		iamProfile       = c.String("iam-role")
		instVolRoot      = c.Int("root-size")
		keyName          = c.String("ssh-key")
		isPrivate        = c.Bool("subnet-private")
		subnetId         = c.String("subnet-id")
		instTags         = c.StringSlice("tag")
		instType         = c.String("type")
		instVols         = c.IntSlice("volume-size")
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
	} else if len(profile.Ami) != 0 {
		ec2param.ImageId = profile.Ami[0].Id
	} else {
		fmt.Fprintln(os.Stderr, "Cannot proceed without an AMI")
		os.Exit(1)
	}

	// Step 2: determine keypair to use for remote access
	if keyName != "" {
		ec2param.KeyName = aws.String(keyName)
	} else if len(profile.KeyPair) != 0 {
		ec2param.KeyName = profile.KeyPair[0].Name
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

	var waitForReady = func(instances ...*ec2.Instance) <-chan ec2state {
		var wg sync.WaitGroup
		out := make(chan ec2state)
		go func() {
			defer close(out)
			wg.Add(len(instances))
			for _, inst := range instances {
				go func(ch <-chan ec2state) {
					state := <-ch
					if state.err == nil {
						state.err = ec2inst.InstallDockerEngine(*state.PublicIpAddress)
					}
					if state.err == nil {
						state.err = ec2inst.InstallDockerEngineCertificate(*state.PublicIpAddress, *state.PrivateIpAddress)
					}
					out <- state
					wg.Done()
				}(ec2_WaitForReady(inst.InstanceId))
			}
			wg.Wait()
		}()
		return out
	}
	for state := range waitForReady(resp.Instances...) {
		if state.err == nil {
			fmt.Printf("%s - %s - Instance ID: %s\n", *state.PublicIpAddress, *state.PrivateIpAddress, *state.InstanceId)
		} else {
			fmt.Fprintln(os.Stderr, state.err.Error())
		}
	}
}
