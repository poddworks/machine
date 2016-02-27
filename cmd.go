package main

import (
	"github.com/jeffjen/machine/driver/aws"
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/machine"
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"

	"fmt"
	"os"
	"sync"
)

var (
	hosts machine.Hosts
)

func CreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "Create and Manage machine",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
			cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
			cli.BoolTFlag{Name: "is-docker-engine", Usage: "Launched instance a Docker Engine"},
		},
		Subcommands: []cli.Command{
			aws.NewCreateCommand(&hosts),
		},
		After: func(c *cli.Context) error {
			const (
				CertPath     = "/home/yihungjen/.machine"
				Organization = "podd.org"
			)

			var (
				useDocker = c.BoolT("is-docker-engine")

				wg sync.WaitGroup
			)

			if !useDocker {
				return nil // not a Docker Engine; abort
			}

			wg.Add(len(hosts.IpAddrs))
			for _, addr := range hosts.IpAddrs {
				go func(addr machine.IpAddr) {
					defer wg.Done()
					subAltNames := []string{addr.Pub, addr.Priv, "localhost", "127.0.0.1"}
					fmt.Println(addr.Pub, "- generate cert for subjects -", subAltNames)
					CA, Cert, Key, err := cert.GenerateServerCertificate(CertPath, Organization, subAltNames)
					if err != nil {
						fmt.Fprintln(os.Stderr, err.Error())
						return
					}

					ssh_config := ssh.Config{
						User:   c.String("user"),
						Server: addr.Pub,
						Key:    c.String("cert"),
						Port:   "22",
					}
					fmt.Println(addr.Pub, "- configure docker engine")
					err = cert.SendEngineCertificate(CA, Cert, Key, ssh_config)
					if err != nil {
						fmt.Fprintln(os.Stderr, err.Error())
						return
					}
				}(addr)
			}
			wg.Wait()

			return nil
		},
	}
}
