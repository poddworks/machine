package main

import (
	"github.com/jeffjen/machine/lib/cert"
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/codegangsta/cli"
	"github.com/olekukonko/tablewriter"

	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
)

func ListInstanceCommand() cli.Command {
	return cli.Command{
		Name:  "ls",
		Usage: "List cached Docker Engine instance info",
		Action: func(c *cli.Context) error {
			var (
				instList = make(mach.RegisteredInstances)

				// Prepare table render
				table = tablewriter.NewWriter(os.Stdout)
			)

			instList.Load() // Load instance metadata

			table.SetHeader([]string{"Id", "Name", "DockerHost", "Driver", "State", "Tag"})
			table.SetBorder(false)
			for name, info := range instList {
				var dockerhost = info.DockerHost.String()
				var oneRow = []string{
					info.Id,                     // ID
					name,                        // Name
					dockerhost,                  // DockerHost
					info.Driver,                 // Driver
					info.State,                  // State
					strings.Join(info.Tag, ","), // Tags
				}
				table.Append(oneRow)
			}
			table.Render()

			return nil
		},
	}
}

func EnvCommand() cli.Command {
	return cli.Command{
		Name:  "env",
		Usage: "Apply Docker Engine environment for target",
		Action: func(c *cli.Context) error {
			var (
				usr, _   = user.Current()
				certpath = strings.Replace(DEFAULT_CERT_PATH, "~", usr.HomeDir, 1)

				instList = make(mach.RegisteredInstances)

				name = c.Args().First()
			)
			if name == "" {
				fmt.Println("Required argument `name` missing")
				os.Exit(1)
			}

			instList.Load() // Load instance metadata

			instMeta, ok := instList[name]
			if !ok {
				fmt.Println("Provided instance [", name, "] not found")
				os.Exit(1)
			}

			fmt.Printf("export DOCKER_TLS_VERIFY=1\n")
			fmt.Printf("export DOCKER_CERT_PATH=%s\n", certpath)
			fmt.Printf("export DOCKER_HOST=%s://%s\n", instMeta.DockerHost.Network(), instMeta.DockerHost)
			fmt.Printf("# eval $(machine env %s)\n", name)

			return nil
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
			cli.StringFlag{Name: "port", EnvVar: "MACHINE_PORT", Value: DEFAULT_MACHINE_PORT, Usage: "Private key to use in Authentication"},
			cli.BoolFlag{Name: "dryrun", Usage: "Enable Dry Run"},
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
			{
				Name:   "playbook",
				Usage:  "Go through the playbook",
				Action: runPlaybook,
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
				Action: func(c *cli.Context) error {
					org, certpath, err := mach.ParseCertArgs(c)
					if err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					cert.GenerateCACertificate(org, certpath)
					cert.GenerateClientCertificate(org, certpath)
					return nil
				},
			},
			{
				Name:  "generate",
				Usage: "Generate server certificate with self-signed CA",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) error {
					_, Cert, Key := generateServerCertificate(c)
					if err := ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					if err := ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						fmt.Println(err.Error())
						os.Exit(1)
					}
					return nil
				},
			},
		},
	}
}

func generateServerCertificate(c *cli.Context) (CA, Cert, Key *cert.PemBlock) {
	var hosts = make([]string, 0)
	if hostname := c.String("host"); hostname == "" {
		fmt.Println("You must provide hostname to create Certificate for")
		os.Exit(1)
	} else {
		hosts = append(hosts, hostname)
	}
	hosts = append(hosts, c.StringSlice("altname")...)
	org, certpath, err := mach.ParseCertArgs(c)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	CA, Cert, Key, err = cert.GenerateServerCertificate(certpath, org, hosts)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return
}
