package config

import (
	"github.com/urfave/cli"

	"fmt"
	"os"
	path "path/filepath"
	"strings"
)

type config struct {
	User       string
	Cert       string
	Org        string
	Certpath   string
	Confdir    string
	Instance   string
	AWSProfile string
}

var (
	Config = new(config)
)

func Parse(c *cli.Context) error {
	org, confdir, user, cert, err := parseArgs(c)
	if err != nil {
		return err
	}
	Config.Org = org
	Config.Certpath = confdir
	Config.User = user
	Config.Cert = cert
	Config.Instance = path.Join(confdir, "instance.json")
	Config.AWSProfile = path.Join(confdir, "aws-profile.json")
	return nil
}

func parseArgs(c *cli.Context) (org, confdir, user, cert string, err error) {
	org = c.String("org")
	if org == "" {
		org = c.GlobalString("org")
	}
	if org == "" {
		err = fmt.Errorf("error/required-org-missing")
		return
	}
	confdir = c.String("confdir")
	if confdir == "" {
		confdir = c.GlobalString("confdir")
	}
	if confdir == "" {
		err = fmt.Errorf("error/required-confidir-missing")
		return
	}
	confdir = strings.Replace(confdir, "~", os.Getenv("HOME"), 1)
	if _, err = os.Stat(confdir); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(confdir, 0700)
		}
	}
	user = c.String("user")
	cert = c.String("cert")
	return
}
