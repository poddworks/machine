package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"os"
)

func vpc_getInfo(id string) (vpc *ec2.Vpc, subs []*ec2.Subnet, grps []*ec2.SecurityGroup) {
	var vparam *ec2.DescribeVpcsInput
	if id == "default" {
		vparam = &ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String("isDefault"),
					Values: []*string{
						aws.String("true"),
					},
				},
			},
		}
	} else {
		vparam = &ec2.DescribeVpcsInput{
			VpcIds: []*string{
				aws.String(id),
			},
		}
	}
	if resp, err := svc.DescribeVpcs(vparam); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	} else if len(resp.Vpcs) == 0 {
		fmt.Fprint(os.Stderr, "Unexpected state: You don't have Default VPC")
		os.Exit(1)
	} else {
		vpc = resp.Vpcs[0]
	}
	subparam := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					vpc.VpcId,
				},
			},
		},
	}
	if resp, err := svc.DescribeSubnets(subparam); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	} else if len(resp.Subnets) == 0 {
		fmt.Fprint(os.Stderr, "Unexpected state: You don't have ANY subnets")
		os.Exit(1)
	} else {
		subs = resp.Subnets
	}
	sgparam := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					vpc.VpcId,
				},
			},
		},
	}
	if resp, err := svc.DescribeSecurityGroups(sgparam); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	} else if len(resp.SecurityGroups) == 0 {
		fmt.Fprint(os.Stderr, "Unexpected state: You don't have ANY security group")
		os.Exit(1)
	} else {
		grps = resp.SecurityGroups
	}
	return
}

func vpcInit(c *cli.Context, profile *VPCProfile) {
	vpc, subs, grps := vpc_getInfo(c.String("vpc-id"))
	profile.Id = vpc.VpcId
	profile.Cidr = vpc.CidrBlock
	for _, subnet := range subs {
		profile.Subnet = append(profile.Subnet, SubnetProfile{
			Az:        subnet.AvailabilityZone,
			Cidr:      subnet.CidrBlock,
			DefaultAz: subnet.DefaultForAz,
			Id:        subnet.SubnetId,
			Public:    subnet.MapPublicIpOnLaunch,
		})
	}
	for _, group := range grps {
		profile.SecurityGroup = append(profile.SecurityGroup, SecurityGroup{
			Id:   group.GroupId,
			Desc: group.Description,
			Name: group.GroupName,
		})
	}
	return
}
