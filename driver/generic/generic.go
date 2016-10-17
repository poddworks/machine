package generic

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/urfave/cli"

	"net"
)

func NewCreateCommand() cli.Command {
	return cli.Command{
		Name:  "generic",
		Usage: "Setup Machine to use Docker Engine",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "no-install", Usage: "Skip Docker Engine Installation"},
			cli.StringFlag{Name: "driver", Value: "generic", Usage: "Assign driver for Docker Engine"},
			cli.StringFlag{Name: "host", Usage: "Host to install Docker Engine"},
			cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
		},
		Action: func(c *cli.Context) error {
			defer mach.InstList.Dump()

			var (
				user     = c.GlobalString("user")
				cert     = c.GlobalString("cert")
				driver   = c.String("driver")
				hostname = c.String("host")
				altnames = c.StringSlice("altname")

				name    = c.Args().First()
				addr, _ = net.ResolveTCPAddr("tcp", hostname+":2376")

				noInstall = c.Bool("no-install")
			)

			org, certpath, err := mach.ParseCertArgs(c)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}

			if name == "" {
				return cli.NewExitError("Required argument `name` missing", 1)
			} else if _, ok := mach.InstList[name]; ok {
				return cli.NewExitError("Machine exist", 1)
			}

			if user == "" || cert == "" {
				return cli.NewExitError("Missing required remote auth info", 1)
			}

			inst := mach.NewDockerHost(org, certpath, user, cert)

			if !noInstall {
				if err := inst.InstallDockerEngine(hostname); err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
			}
			if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
			mach.InstList[name] = &mach.Instance{
				Id:         name,
				Driver:     driver,
				DockerHost: addr,
				Host:       hostname,
				AltHost:    altnames,
				State:      "running",
			}

			return nil
		},
	}
}
