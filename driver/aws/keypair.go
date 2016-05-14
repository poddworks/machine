package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/codegangsta/cli"

	"fmt"
	"os"
)

func keypair_getInfo() (keys []*ec2.KeyPairInfo) {
	if resp, err := svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{}); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	} else {
		keys = resp.KeyPairs
	}
	return
}

func keypairInit(c *cli.Context, profile *Profile) {
	profile.KeyPair = make([]KeyPair, 0)
	for _, key := range keypair_getInfo() {
		profile.KeyPair = append(profile.KeyPair, KeyPair{
			Digest: key.KeyFingerprint,
			Name:   key.KeyName,
		})
	}
}
