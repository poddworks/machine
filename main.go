package main

import (
	"github.com/jeffjen/machine/driver/aws"
	"github.com/jeffjen/machine/driver/generic"

	"github.com/codegangsta/cli"

	"os"
)

const (
	DEFAULT_CERT_PATH = "~/.machine"

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
		cli.StringFlag{Name: "port", EnvVar: "MACHINE_PORT", Value: DEFAULT_MACHINE_PORT, Usage: "Private key to use in Authentication"},
		cli.StringFlag{Name: "certpath", Value: DEFAULT_CERT_PATH, Usage: "Certificate path"},
		cli.StringFlag{Name: "organization", Value: DEFAULT_ORGANIZATION_PLACEMENT_NAME, Usage: "Organization for CA"},
	}
	app.Before = nil
	app.Action = nil
	app.Run(os.Args)
}
