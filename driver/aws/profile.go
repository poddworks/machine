package aws

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	AWS_PROFILE_CONFIG_FILE = ".aws-profile.json"
)

type SubnetProfile struct {
	Az        *string `json:"availability_zone"`
	Cidr      *string `json:"cidr"`
	DefaultAz *bool   `json:"default_for_Az"`
	Id        *string `json:"id"`
	Public    *bool   `json:"public"`
}

type SecurityGroup struct {
	Id   *string `json:"id"`
	Desc *string `json:"description"`
	Name *string `json:"name"`
}

type VPCProfile struct {
	Cidr          *string         `json:"cidr"`
	Id            *string         `json:"id"`
	Subnet        []SubnetProfile `json:"subnet"`
	SecurityGroup []SecurityGroup `json:"security_group"`
}

type AMIProfile struct {
	Arch *string `json:"arch"`
	Desc *string `json:"description"`
	Id   *string `json:"id"`
	Name *string `json:"name"`
}

type KeyPair struct {
	Digest *string `json:"digest"`
	Name   *string `json:"name"`
}

type Profile struct {
	Name    string       `json:"name"`
	Region  string       `json:"region"`
	VPC     VPCProfile   `json:"vpc"`
	KeyPair []KeyPair    `json:"key_pair"`
	Ami     []AMIProfile `json:"ami"`
}

type RegionProfile map[string]*Profile

type AWSProfile map[string]RegionProfile

func (a AWSProfile) Load() AWSProfile {
	origin, err := os.OpenFile(AWS_PROFILE_CONFIG_FILE, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer origin.Close()
	json.NewDecoder(origin).Decode(&a)
	return a
}

func (a AWSProfile) Dump() {
	origin, err := os.OpenFile(AWS_PROFILE_CONFIG_FILE, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer origin.Close()
	json.NewEncoder(origin).Encode(a)
}
