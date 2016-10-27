package generic

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/urfave/cli"

	"net"
)

func NewCreateCommand() cli.Command {
	return cli.Command{
		Name:  "generic",
		Usage: "Provision Docker Engine on Linux instance",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "no-install", Usage: "Skip Docker Engine Installation"},
			cli.StringFlag{Name: "driver", Value: "generic", Usage: "Assign driver for Docker Engine"},
			cli.StringFlag{Name: "host", Usage: "Host to install Docker Engine"},
			cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
		},
		Action: func(c *cli.Context) error {
			defer mach.InstList.Dump()

			var (
				driver   = c.String("driver")
				hostname = c.String("host")
				altnames = c.StringSlice("altname")

				name    = c.Args().First()
				addr, _ = net.ResolveTCPAddr("tcp", hostname+":2376")

				noInstall = c.Bool("no-install")
			)

			if name == "" {
				return cli.NewExitError("Required argument `name` missing", 1)
			} else if _, ok := mach.InstList[name]; ok {
				return cli.NewExitError("Machine exist", 1)
			}

			inst := mach.NewDockerHost()

			if !noInstall {
				if err := inst.InstallDockerEngine(hostname); err != nil {
					return cli.NewExitError("error/failed-to-install-docker-engine", 1)
				}
			}
			if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
				return cli.NewExitError("error/failed-to-install-docker-cert", 1)
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
