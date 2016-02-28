package machine

import (
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/ssh"

	"fmt"
)

type IpAddr struct {
	Pub  string
	Priv string
}

type Host struct {
	CertPath     string
	Organization string

	User     string
	Cert     string
	IsDocker bool
}

func (h *Host) InstallDockerEngineCertificate(addr IpAddr) error {
	if !h.IsDocker { // Not processing because not a Docker Engine
		return nil
	}

	subAltNames := []string{
		addr.Pub,
		addr.Priv,
		"localhost",
		"127.0.0.1",
	}
	fmt.Println(addr.Pub, "- generate cert for subjects -", subAltNames)

	CA, Cert, Key, err := cert.GenerateServerCertificate(h.CertPath, h.Organization, subAltNames)
	if err != nil {
		return err
	}

	ssh_config := ssh.Config{
		User:   h.User,
		Server: addr.Pub,
		Key:    h.Cert,
		Port:   "22",
	}
	fmt.Println(addr.Pub, "- configure docker engine")
	return cert.SendEngineCertificate(CA, Cert, Key, ssh_config)
}
