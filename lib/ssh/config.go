package ssh

import (
	"golang.org/x/crypto/ssh"

	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	User     string
	Server   string
	Key      string
	Port     string
	Password string
}

func (cfg Config) GetKeyFile() (ssh.Signer, error) {
	keyfile := strings.Replace(cfg.Key, "~", os.Getenv("HOME"), 1)
	keyfile, err := filepath.Abs(keyfile)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}
	pubkey, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}
