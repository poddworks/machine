package generic

import (
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/codegangsta/cli"

	"fmt"
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
			newRenerateCert(),
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

				inst = mach.NewDockerHost(org, certpath, user, cert)
			)
			if err := inst.InstallDockerEngine(hostname); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			return nil
		},
	}
}

func newRenerateCert() cli.Command {
	return cli.Command{
		Name:  "regnerate-certificate",
		Usage: "Generate and install certificate on target",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "is-new", Usage: "Installing new Certificate on existing instance"},
			cli.StringFlag{Name: "host", Usage: "Host to install Docker Engine Certificate"},
			cli.StringSliceFlag{Name: "altname", Usage: "Alternative name for Host"},
		},
		Action: func(c *cli.Context) error {
			var (
				org, certpath, _ = mach.ParseCertArgs(c)

				user     = c.GlobalString("user")
				cert     = c.GlobalString("cert")
				hostname = c.String("host")
				altnames = c.StringSlice("altname")

				inst = mach.NewDockerHost(org, certpath, user, cert)
			)
			if !c.Bool("is-new") {
				inst.SetProvision(false)
			}
			if err := inst.InstallDockerEngineCertificate(hostname, altnames...); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			return nil
		},
	}
}
