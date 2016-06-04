package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
)

func vpc_getInfo(id string) (vpc *ec2.Vpc, subs []*ec2.Subnet, grps []*ec2.SecurityGroup, err error) {
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
		return nil, nil, nil, err
	} else if len(resp.Vpcs) == 0 {
		return nil, nil, nil, fmt.Errorf("Unexpected state: You don't have Default VPC")
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
		return nil, nil, nil, err
	} else if len(resp.Subnets) == 0 {
		return nil, nil, nil, fmt.Errorf("Unexpected state: You don't have ANY subnets")
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
		return nil, nil, nil, err
	} else if len(resp.SecurityGroups) == 0 {
		return nil, nil, nil, fmt.Errorf("Unexpected state: You don't have ANY security group")
	} else {
		grps = resp.SecurityGroups
	}
	return
}

func vpcInit(c *cli.Context, profile *VPCProfile) (account_id string, err error) {
	vpc, subs, grps, err := vpc_getInfo(c.String("vpc-id"))
	if err != nil {
		return "", err
	}
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
		account_id = *group.OwnerId
	}
	return
}
