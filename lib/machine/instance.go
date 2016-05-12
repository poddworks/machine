package machine

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	path "path/filepath"
	"strings"
)

const (
	INSTANCE_LISTING_FILE = "~/.machine/instance.json"
)

type Instance struct {
	Id         string
	Driver     string
	DockerHost *net.TCPAddr
	State      string
}

type RegisteredInstances map[string]*Instance

func (r RegisteredInstances) Load() RegisteredInstances {
	conf, err := getConfigPath()
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	origin, err := os.OpenFile(conf, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer origin.Close()
	json.NewDecoder(origin).Decode(&r)
	return r
}

func (r RegisteredInstances) Dump() {
	conf, err := getConfigPath()
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	origin, err := os.OpenFile(conf, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer origin.Close()
	json.NewEncoder(origin).Encode(r)
}

func getConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	conf := strings.Replace(INSTANCE_LISTING_FILE, "~", usr.HomeDir, 1)
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
