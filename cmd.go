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
		Action: func(c *cli.Context) error {
			var (
				// Prepare table render
				table = tablewriter.NewWriter(os.Stdout)
			)

			instList.Load() // Load instance metadata

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

func InstanceCommand(action string) cli.Command {
	return cli.Command{
		Name:            action,
		Usage:           fmt.Sprintf("%s%s instance", strings.ToTitle(action[0:1]), action[1:]),
		SkipFlagParsing: true,
		Action: func(c *cli.Context) error {
			var (
				lastIdx = len(os.Args) - 1

				name = os.Args[lastIdx]

				newArgs []string
			)

			// Load from Instance Roster
			instList.Load()

			info, ok := instList[name]
			if !ok {
				fmt.Fprintln(os.Stderr, "Target machine not found")
				os.Exit(1)
			}

			// Remove the instance by driver
			switch info.Driver {
			case "aws":
				newArgs = append([]string{"machine", "aws"}, os.Args[2:lastIdx]...)
				newArgs = append(newArgs, action, name)
				c.App.Run(newArgs)
				break
			case "generic":
			default:
				// NOOP
				break
			}

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

				name = c.Args().First()
			)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Required argument `name` missing")
				os.Exit(1)
			}

			instList.Load() // Load instance metadata

			instMeta, ok := instList[name]
			if !ok {
				fmt.Fprintln(os.Stderr, "Provided instance [", name, "] not found")
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
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					cert.GenerateCACertificate(org, certpath)
					cert.GenerateClientCertificate(org, certpath)
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
					_, Cert, Key := generateServerCertificate(c)
					if err := ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					if err := ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					return nil
				},
			},
			{
				Name:  "gen-cert-install",
				Usage: "Generate and install certificate on target",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "is-new", Usage: "Installing new Certificate on existing instance"},
					cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
					cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
					cli.StringFlag{Name: "host", Usage: "Host to install Docker Engine Certificate"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
					cli.StringFlag{Name: "name", Usage: "Name to identify Docker Host"},
					cli.StringFlag{Name: "driver", Value: "generic", Usage: "Hint at what type of driver created this instance"},
				},
				Action: func(c *cli.Context) error {
					var (
						org, certpath, _ = mach.ParseCertArgs(c)

						user     = c.String("user")
						cert     = c.String("cert")
						hostname = c.String("host")
						altnames = c.StringSlice("altname")

						name    = c.String("name")
						driver  = c.String("driver")
						addr, _ = net.ResolveTCPAddr("tcp", hostname+":2376")

						inst = mach.NewDockerHost(org, certpath, user, cert)
					)

					if name == "" {
						fmt.Fprintln(os.Stderr, "Required argument `name` missing")
						os.Exit(1)
					}

					// Load from Instance Roster to register and defer write back
					defer instList.Load().Dump()

					// Tell host provisioner whether to reuse old Docker Daemon config
					inst.SetProvision(c.Bool("is-new"))

					if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
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

func generateServerCertificate(c *cli.Context) (CA, Cert, Key *cert.PemBlock) {
	var hosts = make([]string, 0)
	if hostname := c.String("host"); hostname == "" {
		fmt.Fprintln(os.Stderr, "You must provide hostname to create Certificate for")
		os.Exit(1)
	} else {
		hosts = append(hosts, hostname)
	}
	hosts = append(hosts, c.StringSlice("altname")...)
	org, certpath, err := mach.ParseCertArgs(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	CA, Cert, Key, err = cert.GenerateServerCertificate(certpath, org, hosts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return
}
