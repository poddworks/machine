package aws

import (
	"encoding/json"
	"os"
	"os/user"
	path "path/filepath"
	"strings"
)

const (
	AWS_PROFILE_CONFIG_FILE = "~/.machine/aws-profile.json"
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

func getConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	conf := strings.Replace(AWS_PROFILE_CONFIG_FILE, "~", usr.HomeDir, 1)
	confdir := path.Dir(conf)
	if _, err := os.Stat(confdir); err != nil {
		if os.IsNotExist(err) {
			return conf, os.MkdirAll(confdir, 0700)
		} else {
			return "", err
		}
	} else {
		return conf, nil
	}
}

func (a AWSProfile) Load() error {
	conf, err := getConfigPath()
	if err != nil {
		return err
	}
	origin, err := os.OpenFile(conf, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	return json.NewDecoder(origin).Decode(&a)
}

func (a AWSProfile) Dump() error {
	conf, err := getConfigPath()
	if err != nil {
		return err
	}
	origin, err := os.OpenFile(conf, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	return json.NewEncoder(origin).Encode(a)
}
