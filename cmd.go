package main

import (
	"github.com/jeffjen/machine/lib/cert"
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/codegangsta/cli"
	"github.com/olekukonko/tablewriter"

	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"
)

var (
	// Instance roster
	instList = make(mach.RegisteredInstances)
)

func ListInstanceCommand() cli.Command {
	return cli.Command{
		Name:  "ls",
		Usage: "List cached Docker Engine instance info",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				return nil
			}
		},
		Action: func(c *cli.Context) error {
			var (
				// Prepare table render
				table = tablewriter.NewWriter(os.Stdout)
			)

			table.SetHeader([]string{"Name", "DockerHost", "Driver", "State"})
			table.SetBorder(false)
			for name, inst := range instList {
				var dockerhost = inst.DockerHost.String()
				var oneRow = []string{
					name,        // Name
					dockerhost,  // DockerHost
					inst.Driver, // Driver
					inst.State,  // State
				}
				table.Append(oneRow)
			}
			table.Render()

			return nil
		},
	}
}

func InstanceCommand(cmd, act string) cli.Command {
	return cli.Command{
		Name:            cmd,
		Usage:           fmt.Sprintf("%s instance", act),
		SkipFlagParsing: true,
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				return nil
			}
		},
		Action: func(c *cli.Context) error {
			for _, name := range c.Args() {
				info, ok := instList[name]
				if !ok {
					fmt.Fprintln(os.Stderr, "Target machine [", name, "] not found")
					continue
				}
				// Remove the instance by driver
				switch info.Driver {
				case "aws":
					c.App.Run([]string{"machine", "aws", cmd, name})
					break
				case "generic":
				default:
					// NOOP
					break
				}
			}

			return nil
		},
	}
}

func IPCommand() cli.Command {
	return cli.Command{
		Name:  "ip",
		Usage: "Obtain IP address of the Docker Engine instance",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				return nil
			}
		},
		Action: func(c *cli.Context) error {
			var name = c.Args().First()

			instMeta, ok := instList[name]
			if !ok {
				return cli.NewExitError(fmt.Sprintln("Provided instance [", name, "] not found"), 1)
			}
			if instMeta.DockerHost == nil {
				return cli.NewExitError(fmt.Sprintln("Provided instance [", name, "] not running"), 1)
			} else {
				host, _, _ := net.SplitHostPort(instMeta.DockerHost.String())
				fmt.Println(host)
			}

			return nil
		},
	}
}

func EnvCommand() cli.Command {
	return cli.Command{
		Name:  "env",
		Usage: "Apply Docker Engine environment for target",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				return nil
			}
		},
		Action: func(c *cli.Context) error {
			var (
				usr, _   = user.Current()
				certpath = strings.Replace(DEFAULT_CERT_PATH, "~", usr.HomeDir, 1)

				name = c.Args().First()
			)
			if name == "" {
				return cli.NewExitError("Required argument `name` missing", 1)
			}

			instMeta, ok := instList[name]
			if !ok {
				return cli.NewExitError(fmt.Sprintln("Provided instance [", name, "] not found"), 1)
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

func SSHCommand() cli.Command {
	return cli.Command{
		Name:  "ssh",
		Usage: "Login to remote machine or configure ssh",
		Before: func(c *cli.Context) error {
			if err := instList.Load(); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				return nil
			}
		},
		Subcommands: []cli.Command{},
		Action: func(c *cli.Context) error {
			var (
				org, certpath, _ = mach.ParseCertArgs(c)

				user = c.GlobalString("user")
				cert = c.GlobalString("cert")

				name = c.Args().First()

				inst = mach.NewHost(org, certpath, user, cert)
			)

			info, ok := instList[name]
			if !ok {
				return cli.NewExitError("instance name not found", 1)
			}

			host, _, _ := net.SplitHostPort(info.DockerHost.String())
			if err := inst.Shell(host); err != nil {
				return cli.NewExitError(err.Error(), 1)
			} else {
				return nil
			}
		},
	}
}

func TlsCommand() cli.Command {
	return cli.Command{
		Name:  "tls",
		Usage: "Utility for generating certificate for TLS",
		Subcommands: []cli.Command{
			{
				Name:  "bootstrap",
				Usage: "Generate certificate for TLS",
				Action: func(c *cli.Context) error {
					org, certpath, err := mach.ParseCertArgs(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					if cert.GenerateCACertificate(org, certpath) != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					if cert.GenerateClientCertificate(org, certpath) != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					return nil
				},
			},
			{
				Name:  "gen-cert",
				Usage: "Generate server certificate with self-signed CA",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) error {
					_, Cert, Key, err := generateServerCertificate(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					if err = ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					if err = ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					return nil
				},
			},
			{
				Name:  "gen-cert-install",
				Usage: "Generate and install certificate on target",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "is-new", Usage: "Installing new Certificate on existing instance"},
					cli.StringFlag{Name: "host", Usage: "Host to install Docker Engine Certificate"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
					cli.StringFlag{Name: "name", Usage: "Name to identify Docker Host"},
					cli.StringFlag{Name: "driver", Value: "generic", Usage: "Hint at what type of driver created this instance"},
				},
				Before: func(c *cli.Context) error {
					if err := instList.Load(); err != nil {
						return cli.NewExitError(err.Error(), 1)
					} else {
						return nil
					}
				},
				Action: func(c *cli.Context) error {
					defer instList.Dump()

					var (
						org, certpath, _ = mach.ParseCertArgs(c)

						user = c.GlobalString("user")
						cert = c.GlobalString("cert")

						hostname = c.String("host")
						altnames = c.StringSlice("altname")

						name    = c.String("name")
						driver  = c.String("driver")
						addr, _ = net.ResolveTCPAddr("tcp", hostname+":2376")

						inst = mach.NewDockerHost(org, certpath, user, cert)
					)

					if name == "" {
						return cli.NewExitError("Required argument `name` missing", 1)
					}

					// Tell host provisioner whether to reuse old Docker Daemon config
					inst.SetProvision(c.Bool("is-new"))

					if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
						return cli.NewExitError(err.Error(), 1)
					}

					info, ok := instList[name]
					if !ok {
						info = &mach.Instance{Id: name, Driver: driver}
					}
					info.DockerHost = addr
					info.State = "running"

					// Update current records
					instList[name] = info

					return nil
				},
			},
		},
	}
}

func generateServerCertificate(c *cli.Context) (CA, Cert, Key *cert.PemBlock, err error) {
	var hosts = make([]string, 0)
	if hostname := c.String("host"); hostname == "" {
		err = cli.NewExitError("You must provide hostname to create Certificate for", 1)
		return
	} else {
		hosts = append(hosts, hostname)
	}
	hosts = append(hosts, c.StringSlice("altname")...)
	org, certpath, err := mach.ParseCertArgs(c)
	if err != nil {
		err = cli.NewExitError(err.Error(), 1)
	} else {
		CA, Cert, Key, err = cert.GenerateServerCertificate(certpath, org, hosts)
		if err != nil {
			err = cli.NewExitError(err.Error(), 1)
		}
	}
	return
}
