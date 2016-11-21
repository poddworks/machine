package main

import (
	config "github.com/poddworks/machine/config"
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/poddworks/machine/driver/aws"
	"github.com/poddworks/machine/driver/generic"
	"github.com/poddworks/machine/driver/swarm"
	"github.com/poddworks/machine/lib/cert"

	"github.com/urfave/cli"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	path "path/filepath"
	"regexp"
	"text/template"
)

func ListInstanceCommand() cli.Command {
	return cli.Command{
		Name:  "ls",
		Usage: "List cached Docker Engine instance info",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "quiet, q", Usage: "List instances without fancy tabs"},
			cli.StringSliceFlag{Name: "filter, f", Usage: "Filter by instance name prefix"},
		},
		Action: func(c *cli.Context) error {
			var (
				quiet = c.Bool("quiet")

				filters = c.StringSlice("filter")
			)

			var matchers []*regexp.Regexp
			for _, filter := range filters {
				matchers = append(matchers, regexp.MustCompile(fmt.Sprintf("%s*", filter)))
			}

			if quiet {
				listQuiet(matchers)
			} else {
				listTable(matchers)
			}
			return nil
		},
	}
}

func CreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "Create instances",
		Flags: []cli.Flag{},
		Subcommands: []cli.Command{
			aws.NewCreateCommand(),
			generic.NewCreateCommand(),
			swarm.NewCreateCommand(),
		},
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}

func InstanceCommand(cmd, act string) cli.Command {
	return cli.Command{
		Name:            cmd,
		Usage:           fmt.Sprintf("%s instances", act),
		SkipFlagParsing: true,
		Action: func(c *cli.Context) error {
			var (
				args = c.Args()

				lastArg = len(args) - 1
			)

			if args.Get(lastArg) == "--generate-bash-completion" {
				for name, _ := range mach.InstList {
					fmt.Fprint(c.App.Writer, name, " ")
				}
				return nil
			}

			for _, name := range c.Args() {
				info, ok := mach.InstList[name]
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
		Action: func(c *cli.Context) error {
			var name = c.Args().First()

			if name == "" {
				// Search for MACHINE_NAME for enabled/active instance
				name = os.Getenv("MACHINE_NAME")
			}

			instMeta, ok := mach.InstList[name]
			if !ok {
				return cli.NewExitError("error/instance-not-found", 1)
			}
			if instMeta.DockerHost == nil {
				return cli.NewExitError("error/instance-not-available", 1)
			} else {
				fmt.Println(instMeta.Host)
			}

			return nil
		},
		BashComplete: func(c *cli.Context) {
			for name, _ := range mach.InstList {
				fmt.Fprint(c.App.Writer, name, " ")
			}
		},
	}
}

func EnvCommand() cli.Command {
	return cli.Command{
		Name:  "env",
		Usage: "Apply Docker Engine environment for target",
		Action: func(c *cli.Context) error {
			var (
				name = c.Args().First()
			)

			if name == "" {
				return cli.NewExitError("error/required-instances-missing", 1)
			}

			if name == "swarm" {
				fmt.Printf("export DOCKER_TLS_VERIFY=1\n")
				fmt.Printf("export DOCKER_CERT_PATH=%s\n", config.Config.Certpath)
				fmt.Printf("export DOCKER_HOST=%s://%s\n", "tcp", "localhost:2376")
				fmt.Printf("export MACHINE_NAME=\n")
				fmt.Printf("# eval $(machine env %s)\n", name)
			} else {
				instMeta, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("error/instance-not-found", 1)
				}
				fmt.Printf("export DOCKER_TLS_VERIFY=1\n")
				fmt.Printf("export DOCKER_CERT_PATH=%s\n", config.Config.Certpath)
				fmt.Printf("export DOCKER_HOST=%s\n", instMeta.DockerHostName())
				fmt.Printf("export MACHINE_NAME=%s\n", name)
				fmt.Printf("# eval $(machine env %s)\n", name)
			}

			return nil
		},
		Subcommands: []cli.Command{
			{
				Name:  "clear",
				Usage: "Clear Docker Engine environment",
				Action: func(c *cli.Context) error {
					fmt.Println("unset DOCKER_TLS_VERIFY DOCKER_CERT_PATH DOCKER_HOST MACHINE_NAME")
					fmt.Println("# eval $(machine env clear)")
					return nil
				},
			},
			{
				Name:  "display",
				Usage: "Present configured Docker Engine environment",
				Action: func(c *cli.Context) error {
					fmt.Printf("DOCKER_TLS_VERIFY=%s\n", os.Getenv("DOCKER_TLS_VERIFY"))
					fmt.Printf("DOCKER_CERT_PATH=%s\n", os.Getenv("DOCKER_CERT_PATH"))
					fmt.Printf("DOCKER_HOST=%s\n", os.Getenv("DOCKER_HOST"))
					fmt.Printf("MACHINE_NAME=%s\n", os.Getenv("MACHINE_NAME"))
					return nil
				},
			},
		},
		BashComplete: func(c *cli.Context) {
			fmt.Fprint(c.App.Writer, "swarm", " ")
			for name, _ := range mach.InstList {
				fmt.Fprint(c.App.Writer, name, " ")
			}
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
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}

func SSHCommand() cli.Command {
	return cli.Command{
		Name:        "ssh",
		Usage:       "Login to remote machine with SSH",
		Subcommands: []cli.Command{},
		Action: func(c *cli.Context) error {
			var (
				name = c.Args().First()
			)

			if name == "" {
				// Search for MACHINE_NAME for enabled/active instance
				name = os.Getenv("MACHINE_NAME")
			}

			info, ok := mach.InstList[name]
			if !ok {
				return cli.NewExitError("error/instance-not-found", 1)
			}

			inst := mach.NewHost()
			if err := inst.Shell(info.Host); err != nil {
				return cli.NewExitError("error/failed-to-login", 1)
			} else {
				return nil
			}
		},
		BashComplete: func(c *cli.Context) {
			for name, _ := range mach.InstList {
				fmt.Fprint(c.App.Writer, name, " ")
			}
		},
	}
}

func TlsCommand() cli.Command {
	return cli.Command{
		Name:  "tls",
		Usage: "Generate certificate for TLS",
		Subcommands: []cli.Command{
			{
				Name:  "bootstrap",
				Usage: "Generate certificate for TLS",
				Action: func(c *cli.Context) error {
					if cert.GenerateCACertificate(config.Config.Org, config.Config.Certpath) != nil {
						return cli.NewExitError("error/failed-to-bootstrap-ca", 1)
					}
					_, Cert, Key, err := cert.GenerateClientCertificate(config.Config.Certpath, config.Config.Org)
					if err != nil {
						return cli.NewExitError("error/failed-to-bootstrap-client", 1)
					}
					Cert.Name = path.Join(config.Config.Certpath, Cert.Name)
					if err = ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						return cli.NewExitError("error/failed-to-write-client-cert", 1)
					}
					Key.Name = path.Join(config.Config.Certpath, Key.Name)
					if err = ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError("error/failed-to-write-client-key", 1)
					}
					return nil
				},
			},
			{
				Name:  "gen-client",
				Usage: "Generate client certificate with self-signed CA",
				Action: func(c *cli.Context) error {
					CA, Cert, Key, err := cert.GenerateClientCertificate(config.Config.Certpath, config.Config.Org)
					if err != nil {
						return cli.NewExitError("error/failed-to-bootstrap-client", 1)
					}

					if err = ioutil.WriteFile(CA.Name, CA.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError("error/failed-to-write-ca-cert", 1)
					}
					if err = ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						return cli.NewExitError("error/failed-to-write-client-cert", 1)
					}
					if err = ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError("error/failed-to-write-client-key", 1)
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
					var hosts = make([]string, 0)
					if hostname := c.String("host"); hostname == "" {
						return cli.NewExitError("error/required-hostnames-missing", 1)
					} else {
						hosts = append(hosts, hostname)
					}
					hosts = append(hosts, c.StringSlice("altname")...)

					CA, Cert, Key, err := cert.GenerateServerCertificate(config.Config.Certpath, config.Config.Org, hosts)
					if err != nil {
						err = cli.NewExitError("error/failed-to-bootstrap-server", 1)
					}
					if err = ioutil.WriteFile(CA.Name, CA.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError("error/failed-to-write-ca-cert", 1)
					}
					if err = ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						return cli.NewExitError("error/failed-to-write-server-cert", 1)
					}
					if err = ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError("error/failed-to-write-server-key", 1)
					}
					return nil
				},
			},
			{
				Name:  "gen-cert-install",
				Usage: "Generate and install certificate on target",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "is-new", Usage: "Bootstrap certificate on new Machine instance"},
				},
				Action: func(c *cli.Context) error {
					var (
						isNew = c.Bool("is-new")
					)

					if len(c.Args()) == 0 {
						return cli.NewExitError("error/required-instances-missing", 1)
					}
					defer mach.InstList.Dump()

					for _, name := range c.Args() {
						if name == "" {
							return cli.NewExitError("error/required-instances-missing", 1)
						}

						info, ok := mach.InstList[name]
						if !ok {
							return cli.NewExitError("error/instances-not-found", 1)
						}
						if info.DockerHost == nil {
							return cli.NewExitError("error/instances-not-available", 1)
						}

						// Tell host provisioner whether to reuse old Docker Daemon config
						inst := mach.NewDockerHost()
						inst.SetProvision(isNew)

						if err := inst.InstallDockerEngineCertificate(info.Host, info.AltHost...); err != nil {
							return cli.NewExitError("error/failed-to-install-docker-cert", 1)
						}

						// Force set instance running state
						info.State = "running"
					}

					return nil
				},
				BashComplete: func(c *cli.Context) {
					for name, _ := range mach.InstList {
						fmt.Fprint(c.App.Writer, name, " ")
					}
				},
			},
		},
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}

func RecipeCommand() cli.Command {
	return cli.Command{
		Name:  "recipe",
		Usage: "Generate recipe for provision/management",
		Subcommands: []cli.Command{
			{
				Name:  "get-provision",
				Usage: "Generate recipe for Docker Engine configuration to use by exec playbook",
				Action: func(c *cli.Context) error {
					compose, err := os.Create("compose.yml")
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}
					defer compose.Close()
					_, err = compose.WriteString(mach.COMPOSE)
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}

					installPkg, err := os.Create("00-install-pkg")
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}
					defer installPkg.Close()
					_, err = installPkg.WriteString(mach.INSTALL_PKG)
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}

					installDockerEngine, err := os.Create("01-install-docker-engine")
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}
					defer installDockerEngine.Close()
					_, err = installDockerEngine.WriteString(mach.INSTALL_DOCKER_ENGINE)
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}

					configSystem, err := os.Create("02-config-system")
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}
					defer configSystem.Close()
					_, err = configSystem.WriteString(mach.CONFIGURE_SYSTEM)
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}

					defaultDockerDaemon, err := os.Create("docker.daemon.json")
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}
					defer defaultDockerDaemon.Close()
					_, err = defaultDockerDaemon.WriteString(mach.DOCKER_DAEMON_CONFIG)
					if err != nil {
						return cli.NewExitError("error/failed-to-create-provision-file", 1)
					}

					return nil
				},
			},
			{
				Name:  "get-swarm-compose",
				Usage: "Generate Docker Engine Swarm docker-compose style",
				Action: func(c *cli.Context) error {
					type swarmParam struct {
						Certpath string
						Nodes    []string
					}

					var hosts = []string{"127.0.0.1", "localhost"}
					_, Cert, Key, err := cert.GenerateServerCertificate(config.Config.Certpath, config.Config.Org, hosts)
					if err != nil {
						return cli.NewExitError("error/failed-to-bootstrap-server", 1)
					}
					if err = ioutil.WriteFile(path.Join(config.Config.Certpath, Cert.Name), Cert.Buf.Bytes(), 0644); err != nil {
						return cli.NewExitError("error/failed-to-write-server-cert", 1)
					}
					if err = ioutil.WriteFile(path.Join(config.Config.Certpath, Key.Name), Key.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError("error/failed-to-write-server-key", 1)
					}

					swarm, err := os.Create("swarm.yml")
					if err != nil {
						return cli.NewExitError("error/failed-to-create-swarm-file", 1)
					}
					defer swarm.Close()

					var info = swarmParam{
						Nodes:    make([]string, 0),
						Certpath: config.Config.Certpath,
					}
					if len(c.Args()) > 0 {
						for _, name := range c.Args() {
							node, ok := mach.InstList[name]
							if !ok {
								return cli.NewExitError("error/instance-not-found", 1)
							}
							info.Nodes = append(info.Nodes, node.DockerHost.String())
						}
					} else {
						for _, inst := range mach.InstList {
							info.Nodes = append(info.Nodes, inst.DockerHost.String())
						}
					}

					var tmpl = template.Must(template.New("swarm").Parse(mach.SWARM_MASTER))
					if err := tmpl.Execute(swarm, info); err != nil {
						return cli.NewExitError("error/failed-to-create-swarm-file", 1)
					}

					return nil
				},
				BashComplete: func(c *cli.Context) {
					for name, _ := range mach.InstList {
						fmt.Fprint(c.App.Writer, name, " ")
					}
				},
			},
		},
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}

func DnstoolCommand() cli.Command {
	return cli.Command{
		Name:  "dns",
		Usage: "Query DNS record",
		Subcommands: []cli.Command{
			{
				Name:  "lookup-srv",
				Usage: "Lookup SRV record",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "proto", Value: "tcp", Usage: "Service Protocol [tcp|udp]"},
					cli.BoolFlag{Name: "verbose", Usage: "Print more info"},
				},
				Action: func(c *cli.Context) error {
					var (
						verbose = c.Bool("verbose")

						proto = c.String("proto")

						srv, zone = c.Args().Get(0), c.Args().Get(1)
					)

					records, err := mach.LookupSRV(srv, proto, zone)
					if err != nil {
						return cli.NewExitError("error/failed-to-get-srv-record", 1)
					} else {
						for _, r := range records {
							r.Target = r.Target[0 : len(r.Target)-1]
						}
					}

					if verbose {
						text, _ := json.MarshalIndent(records, "", "  ")
						fmt.Fprintf(os.Stdout, "%s\n", text)
					} else {
						for _, r := range records {
							fmt.Fprintf(os.Stdout, "%s ", r.Target)
						}
					}

					return nil
				},
			},
		},
		BashComplete: func(c *cli.Context) {
			for _, cmd := range c.App.Commands {
				fmt.Fprint(c.App.Writer, " ", cmd.Name)
			}
		},
	}
}
