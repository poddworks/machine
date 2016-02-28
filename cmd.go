package main

import (
	"github.com/jeffjen/machine/driver/aws"
	"github.com/jeffjen/machine/lib/cert"
	mach "github.com/jeffjen/machine/lib/machine"
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"

	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
)

var (
	host mach.Host
)

const (
	DEFAULT_CERT_PATH = "~/.machine"

	DEFAULT_ORGANIZATION_PLACEMENT_NAME = "podd.org"
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
			aws.NewCreateCommand(&host),
		},
		Before: func(c *cli.Context) error {
			usr, err := user.Current()
			if err == nil {
				host.CertPath = strings.Replace(DEFAULT_CERT_PATH, "~", usr.HomeDir, 1)
				host.Organization = DEFAULT_ORGANIZATION_PLACEMENT_NAME
				host.User = c.String("user")
				host.Cert = c.String("cert")
				host.IsDocker = c.BoolT("is-docker-engine")
			}
			return err
		},
	}
}

func ImageCommand() cli.Command {
	return cli.Command{
		Name:  "register-image",
		Usage: "Register Virtual Machine image",
		Subcommands: []cli.Command{
			aws.NewImageCommand(),
		},
	}
}

func ConfigCommand() cli.Command {
	return cli.Command{
		Name:  "config",
		Usage: "Configure settings pertain to machine management",
		Subcommands: []cli.Command{
			aws.NewConfigCommand(),
		},
	}
}

func ExecCommand() cli.Command {
	return cli.Command{
		Name:  "exec",
		Usage: "Invoke command on remote host via SSH",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
			cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
			cli.StringSliceFlag{Name: "host", Usage: "Remote host to run command in"},
		},
		Subcommands: []cli.Command{
			{
				Name:   "run",
				Usage:  "Invoke command from argument",
				Action: runCmd,
			},
			{
				Name:  "script",
				Usage: "Invoke script from argument",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "sudo", Usage: "Run as sudo for this session"},
				},
				Action: runScript,
			},
		},
	}
}

func TlsCommand() cli.Command {
	return cli.Command{
		Name:  "tls",
		Usage: "Utility for generating certificate for TLS",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "certpath", Value: DEFAULT_CERT_PATH, Usage: "Certificate path"},
			cli.StringFlag{Name: "organization", Value: DEFAULT_ORGANIZATION_PLACEMENT_NAME, Usage: "Organization for CA"},
		},
		Subcommands: []cli.Command{
			{
				Name:  "bootstrap",
				Usage: "Generate certificate for TLS",
				Action: func(c *cli.Context) {
					org, certpath, err := parseCertArgs(c)
					if err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					cert.GenerateCACertificate(org, certpath)
					cert.GenerateClientCertificate(org, certpath)
				},
			},
			{
				Name:  "generate",
				Usage: "Generate server certificate with self-signed CA",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) {
					_, Cert, Key := generateServerCertificate(c)
					if err := ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					if err := ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
				},
			},
			{
				Name:  "install",
				Usage: "Generate and install certificate for Docker Enginea",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
					cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) {
					var (
						user    = c.String("user")
						privKey = c.String("cert")
						host    = c.String("host")

						ssh_config = ssh.Config{
							User:   user,
							Server: host,
							Key:    privKey,
							Port:   "22",
						}
					)

					CA, Cert, Key := generateServerCertificate(c)
					fmt.Println(host, "- generated certificate")

					err := cert.SendEngineCertificate(CA, Cert, Key, ssh_config)
					if err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
				},
			},
		},
	}
}
