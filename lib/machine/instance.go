package machine

import (
	"encoding/json"
	"io"
	"net"
	"os"
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

var (
	// Instance roster
	InstList = make(RegisteredInstances)
)

func (r RegisteredInstances) Load() error {
	conf, err := getConfigPath()
	if err != nil {
		return err
	}
	origin, err := os.OpenFile(conf, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	if err = json.NewDecoder(origin).Decode(&r); err == io.EOF {
		return nil
	} else {
		return err
	}
}

func (r RegisteredInstances) Dump() error {
	conf, err := getConfigPath()
	if err != nil {
		return err
	}
	origin, err := os.OpenFile(conf, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	return json.NewEncoder(origin).Encode(r)
}

func getConfigPath() (string, error) {
	conf := strings.Replace(INSTANCE_LISTING_FILE, "~", os.Getenv("HOME"), 1)
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
