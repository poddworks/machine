package main

import (
	"github.com/jeffjen/machine/driver/aws"
	"github.com/jeffjen/machine/driver/generic"
	mach "github.com/jeffjen/machine/lib/machine"

	"github.com/urfave/cli"

	"fmt"
	"os"
)

const (
	DEFAULT_CONFIG_DIR = "~/.machine"

	DEFAULT_ORGANIZATION_PLACEMENT_NAME = "podd.org"

	DEFAULT_MACHINE_PORT = "22"
)

func main() {
	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Name = "machine"
	app.Usage = "Create/Bootstrap machine to use with Docker engine"
	app.EnableBashCompletion = true
	app.Authors = []cli.Author{
		cli.Author{"Yi-Hung Jen", "yihungjen@gmail.com"},
	}
	app.Commands = []cli.Command{
		GenerateRecipeCommand(),
		GenerateSwarmCommand(),
		ListInstanceCommand(),
		InstanceCommand("start", "Start"),
		InstanceCommand("stop", "Stop"),
		InstanceCommand("rm", "Remove And Terminate"),
		InstanceCommand("reboot", "Reboot instanace without start -> stop -> start"),
		IPCommand(),
		EnvCommand(),
		ExecCommand(),
		SSHCommand(),
		TlsCommand(),
		DnstoolCommand(),
		aws.NewCommand(),
		generic.NewCommand(),
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
		cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
		cli.StringFlag{Name: "port", EnvVar: "MACHINE_PORT", Value: DEFAULT_MACHINE_PORT, Usage: "Connected to ssh port"},
		cli.StringFlag{Name: "org", Value: DEFAULT_ORGANIZATION_PLACEMENT_NAME, Usage: "Organization for Self Signed CA"},
		cli.StringFlag{Name: "confdir", Value: DEFAULT_CONFIG_DIR, Usage: "Configuration and Certificate path"},
	}
	app.Before = func(c *cli.Context) error {
		if err := mach.InstList.Load(); err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		return nil
	}
	app.BashComplete = func(c *cli.Context) {
		for _, command := range app.Commands {
			fmt.Fprint(c.App.Writer, command.Name, " ")
		}
	}
	app.Run(os.Args)
}
