package main

import (
	"github.com/jeffjen/machine/driver/aws"
	"github.com/jeffjen/machine/driver/generic"
	"github.com/jeffjen/machine/lib/cert"
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"

	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"
	"text/template"
)

func ListInstanceCommand() cli.Command {
	return cli.Command{
		Name:  "ls",
		Usage: "List cached Docker Engine instance info",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "current", Usage: "Show only the enabled Docker Engine"},
		},
		Action: func(c *cli.Context) error {
			var (
				current = c.Bool("current")

				// Prepare table render
				table = tablewriter.NewWriter(os.Stdout)
			)

			table.SetBorder(false)

			if current {
				table.SetHeader([]string{"Name", "DockerHost", "Driver", "State"})
				for name, inst := range mach.InstList {
					var dockerhost = inst.DockerHost.String()
					var oneRow = []string{
						name,        // Name
						dockerhost,  // DockerHost
						inst.Driver, // Driver
						inst.State,  // State
					}
					if strings.Contains(os.Getenv("DOCKER_HOST"), dockerhost) {
						table.Append(oneRow)
					}
				}
			} else {
				table.SetHeader([]string{"", "Name", "DockerHost", "Driver", "State"})
				for name, inst := range mach.InstList {
					var dockerhost = inst.DockerHost.String()
					var oneRow = []string{
						"",          // Current
						name,        // Name
						dockerhost,  // DockerHost
						inst.Driver, // Driver
						inst.State,  // State
					}
					if strings.Contains(os.Getenv("DOCKER_HOST"), dockerhost) {
						oneRow[0] = "*"
					}
					table.Append(oneRow)
				}
			}

			table.Render()

			return nil
		},
	}
}

func CreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "Create Docker Machine",
		Flags: []cli.Flag{},
		Subcommands: []cli.Command{
			aws.NewCreateCommand(),
			generic.NewCreateCommand(),
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
		Usage:           fmt.Sprintf("%s instance", act),
		SkipFlagParsing: true,
		Action: func(c *cli.Context) error {
			if c.Args().First() == "--generate-bash-completion" {
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
		BashComplete: func(c *cli.Context) {
			for name, _ := range mach.InstList {
				fmt.Fprint(c.App.Writer, name, " ")
			}
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
				return cli.NewExitError("Instance not found", 1)
			}
			if instMeta.DockerHost == nil {
				return cli.NewExitError("Instance unreachable", 1)
			} else {
				host, _, _ := net.SplitHostPort(instMeta.DockerHost.String())
				fmt.Println(host)
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
				return cli.NewExitError("Required argument `name` missing", 1)
			}

			_, certpath, err := mach.ParseCertArgs(c)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			if name == "swarm" {
				fmt.Printf("export DOCKER_TLS_VERIFY=1\n")
				fmt.Printf("export DOCKER_CERT_PATH=%s\n", certpath)
				fmt.Printf("export DOCKER_HOST=%s://%s\n", "tcp", "localhost:2376")
				fmt.Printf("export MACHINE_NAME=\n")
				fmt.Printf("# eval $(machine env %s)\n", name)
			} else {
				instMeta, ok := mach.InstList[name]
				if !ok {
					return cli.NewExitError("Instance not found", 1)
				}
				fmt.Printf("export DOCKER_TLS_VERIFY=1\n")
				fmt.Printf("export DOCKER_CERT_PATH=%s\n", certpath)
				fmt.Printf("export DOCKER_HOST=%s://%s\n", instMeta.DockerHost.Network(), instMeta.DockerHost)
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

type swarmParam struct {
	Certpath string
	Nodes    []string
}

func GenerateSwarmCommand() cli.Command {
	return cli.Command{
		Name:  "gen-swarm",
		Usage: "Generate swarm master docker-compose style",
		Action: func(c *cli.Context) error {
			org, certpath, err := mach.ParseCertArgs(c)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			var hosts = []string{"127.0.0.1", "localhost"}
			_, Cert, Key, err := cert.GenerateServerCertificate(certpath, org, hosts)
			if err != nil {
				err = cli.NewExitError(err.Error(), 1)
			}
			if err = ioutil.WriteFile(certpath+"/"+Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			if err = ioutil.WriteFile(certpath+"/"+Key.Name, Key.Buf.Bytes(), 0600); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			swarm, err := os.Create("swarm.yml")
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			defer swarm.Close()

			var info = swarmParam{
				Nodes:    make([]string, 0),
				Certpath: certpath,
			}
			for _, inst := range mach.InstList {
				info.Nodes = append(info.Nodes, inst.Host)
			}
			var tmpl = template.Must(template.New("swarm").Parse(mach.SWARM_MASTER))
			if err := tmpl.Execute(swarm, info); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			return nil
		},
	}
}

func GenerateRecipeCommand() cli.Command {
	return cli.Command{
		Name:  "gen-recipe",
		Usage: "Generate recipe for Docker Engine configuration to use by exec playbook",
		Action: func(c *cli.Context) error {
			compose, err := os.Create("compose.yml")
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			defer compose.Close()
			_, err = compose.WriteString(mach.COMPOSE)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			installPkg, err := os.Create("00-install-pkg")
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			defer installPkg.Close()
			_, err = installPkg.WriteString(mach.INSTALL_PKG)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			installDockerEngine, err := os.Create("01-install-docker-engine")
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			defer installDockerEngine.Close()
			_, err = installDockerEngine.WriteString(mach.INSTALL_DOCKER_ENGINE)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			configSystem, err := os.Create("02-config-system")
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			defer configSystem.Close()
			_, err = configSystem.WriteString(mach.CONFIGURE_SYSTEM)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			defaultDockerDaemon, err := os.Create("docker.daemon.json")
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			defer defaultDockerDaemon.Close()
			_, err = defaultDockerDaemon.WriteString(mach.DOCKER_DAEMON_CONFIG)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

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
		Usage:       "Login to remote machine or configure SSH",
		Subcommands: []cli.Command{},
		Action: func(c *cli.Context) error {
			var (
				user = c.GlobalString("user")
				cert = c.GlobalString("cert")

				name = c.Args().First()
			)

			if name == "" {
				// Search for MACHINE_NAME for enabled/active instance
				name = os.Getenv("MACHINE_NAME")
			}

			org, certpath, err := mach.ParseCertArgs(c)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			info, ok := mach.InstList[name]
			if !ok {
				return cli.NewExitError("Instance not found", 1)
			}

			inst := mach.NewHost(org, certpath, user, cert)

			if err := inst.Shell(info.Host); err != nil {
				return cli.NewExitError(err.Error(), 1)
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
					_, Cert, Key, err := cert.GenerateClientCertificate(certpath, org)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					Cert.Name = path.Join(certpath, Cert.Name)
					if err = ioutil.WriteFile(Cert.Name, Cert.Buf.Bytes(), 0644); err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					Key.Name = path.Join(certpath, Key.Name)
					if err = ioutil.WriteFile(Key.Name, Key.Buf.Bytes(), 0600); err != nil {
						return cli.NewExitError(err.Error(), 1)
					}
					return nil
				},
			},
			{
				Name:  "gen-client",
				Usage: "Generate client certificate with self-signed CA",
				Action: func(c *cli.Context) error {
					org, certpath, err := mach.ParseCertArgs(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}

					CA, Cert, Key, err := cert.GenerateClientCertificate(certpath, org)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}

					if err = ioutil.WriteFile(CA.Name, CA.Buf.Bytes(), 0600); err != nil {
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
				Name:  "gen-cert",
				Usage: "Generate server certificate with self-signed CA",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "Generate certificate for Host"},
					cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
				},
				Action: func(c *cli.Context) error {
					org, certpath, err := mach.ParseCertArgs(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}

					var hosts = make([]string, 0)
					if hostname := c.String("host"); hostname == "" {
						return cli.NewExitError("You must provide hostname to create Certificate for", 1)
					} else {
						hosts = append(hosts, hostname)
					}
					hosts = append(hosts, c.StringSlice("altname")...)

					CA, Cert, Key, err := cert.GenerateServerCertificate(certpath, org, hosts)
					if err != nil {
						err = cli.NewExitError(err.Error(), 1)
					}

					if err = ioutil.WriteFile(CA.Name, CA.Buf.Bytes(), 0600); err != nil {
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
				},
				Action: func(c *cli.Context) error {
					var (
						isNew = c.Bool("is-new")

						user = c.GlobalString("user")
						cert = c.GlobalString("cert")

						name = c.Args().First()
					)

					if name == "" {
						return cli.NewExitError("Required argument `name` missing", 1)
					}

					info, ok := mach.InstList[name]
					if !ok {
						return cli.NewExitError("Instance not found", 1)
					}
					if info.DockerHost == nil {
						return cli.NewExitError("Instance not available", 1)
					}
					defer mach.InstList.Dump()

					org, certpath, err := mach.ParseCertArgs(c)
					if err != nil {
						return cli.NewExitError(err.Error(), 1)
					}

					inst := mach.NewDockerHost(org, certpath, user, cert)

					// Tell host provisioner whether to reuse old Docker Daemon config
					inst.SetProvision(isNew)

					if err := inst.InstallDockerEngineCertificate(info.Host, info.AltHost...); err != nil {
						return cli.NewExitError(err.Error(), 1)
					}

					info.State = "running"
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
