package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/urfave/cli"
)

func keypair_getInfo() (keys []*ec2.KeyPairInfo, err error) {
	if resp, err := svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{}); err != nil {
		return nil, err
	} else {
		keys = resp.KeyPairs
	}
	return
}

func keypairInit(c *cli.Context, profile *Profile) error {
	profile.KeyPair = make([]KeyPair, 0)
	if keys, err := keypair_getInfo(); err != nil {
		return err
	} else {
		for _, key := range keys {
			profile.KeyPair = append(profile.KeyPair, KeyPair{
				Digest: key.KeyFingerprint,
				Name:   key.KeyName,
			})
		}
	}
	return nil
}
