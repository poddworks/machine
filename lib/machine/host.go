package machine

import (
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/ssh"

	"fmt"
)

type Host struct {
	CertPath     string
	Organization string

	User     string
	Cert     string
	IsDocker bool
}

func (h *Host) InstallDockerEngineCertificate(host string, altname ...string) error {
	if !h.IsDocker { // Not processing because not a Docker Engine
		fmt.Println(host, "- skipping Docker Certificate Install")
		return nil
	}

	var subAltNames = []string{
		host,
		"localhost",
		"127.0.0.1",
	}
	subAltNames = append(subAltNames, altname...)
	fmt.Println(host, "- generate cert for subjects -", subAltNames)

	CA, Cert, Key, err := cert.GenerateServerCertificate(h.CertPath, h.Organization, subAltNames)
	if err != nil {
		return err
	}
	ssh_config := ssh.Config{
		User:   h.User,
		Server: host,
		Key:    h.Cert,
		Port:   "22",
	}
	fmt.Println(host, "- configure docker engine")
	return cert.SendEngineCertificate(CA, Cert, Key, ssh_config)
}
