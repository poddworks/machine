package generic

import (
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/codegangsta/cli"

	"net"
)

var (
	// Instance Roster
	instList = make(mach.RegisteredInstances)
)

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "generic",
		Usage: "Setup Machine to use Docker Engine",
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

				user     = c.GlobalString("user")
				cert     = c.GlobalString("cert")
				hostname = c.String("host")
				altnames = c.StringSlice("altname")

				name    = c.Args().First()
				addr, _ = net.ResolveTCPAddr("tcp", hostname+":2376")

				inst = mach.NewDockerHost(org, certpath, user, cert)
			)

			if name == "" {
				return cli.NewExitError("Required argument `name` missing", 1)
			} else if _, ok := instList[name]; ok {
				return cli.NewExitError("Machine exist", 1)
			}

			if user == "" || cert == "" {
				return cli.NewExitError("Missing required remote auth info", 1)
			}

			if err := inst.InstallDockerEngine(hostname); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
				return cli.NewExitError(err.Error(), 1)
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
