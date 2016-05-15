package generic

import (
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/codegangsta/cli"

	"fmt"
	"net"
	"os"
)

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "generic",
		Usage: "Setup Machine to use Docker Engine",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
			cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
		},
		Subcommands: []cli.Command{
			newCreateCommand(),
		},
	}
}

func newCreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "Install Docker Engine on target",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "host", Usage: "Host to install Docker Engine"},
			cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
		},
		Action: func(c *cli.Context) error {
			var (
				org, certpath, _ = mach.ParseCertArgs(c)

				user     = c.GlobalString("user")
				cert     = c.GlobalString("cert")
				hostname = c.String("host")
				altnames = c.StringSlice("altname")

				name    = c.Args().First()
				addr, _ = net.ResolveTCPAddr("tcp", hostname+":2376")

				instList = make(mach.RegisteredInstances)

				inst = mach.NewDockerHost(org, certpath, user, cert)
			)

			// Load from Instance Roster to register and defer write back
			defer instList.Load().Dump()

			if name == "" {
				fmt.Fprintln(os.Stderr, "Required argument `name` missing")
				os.Exit(1)
			} else if _, ok := instList[name]; ok {
				fmt.Fprintln(os.Stderr, "Machine exist")
				os.Exit(1)
			}

			if user == "" || cert == "" {
				fmt.Fprintln(os.Stderr, "Missing required remote auth info")
				os.Exit(1)
			}

			if err := inst.InstallDockerEngine(hostname); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			instList[name] = &mach.Instance{
				Id:         name,
				Driver:     "generic",
				DockerHost: addr,
				State:      "running",
			}

			return nil
		},
	}
}
