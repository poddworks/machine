package main

import (
	mach "github.com/poddworks/machine/lib/machine"

	"github.com/poddworks/machine/driver/aws"
	"github.com/poddworks/machine/driver/swarm"

	"github.com/urfave/cli"

	"fmt"
	"math/rand"
	"os"
	"time"
)

const (
	DEFAULT_CONFIG_DIR = "~/.machine"

	DEFAULT_ORGANIZATION_PLACEMENT_NAME = "podd.org"

	DEFAULT_MACHINE_PORT = "22"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	app := cli.NewApp()
	app.Version = "1.0.0"
	app.Name = "machine"
	app.Usage = "Swiss Army knife for DevOps"
	app.EnableBashCompletion = true
	app.Authors = []cli.Author{
		cli.Author{"Yi-Hung Jen", "yihungjen@gmail.com"},
	}
	app.Commands = []cli.Command{
		CreateCommand(),
		InstanceCommand("start", "Start"),
		InstanceCommand("stop", "Stop"),
		InstanceCommand("reboot", "Reboot"),
		InstanceCommand("rm", "Remove And Terminate"),
		ListInstanceCommand(),
		IPCommand(),
		EnvCommand(),
		ExecCommand(),
		SSHCommand(),
		TlsCommand(),
		DnstoolCommand(),
		aws.NewCommand(),
		swarm.NewCommand(),
		RecipeCommand(),
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
		// List available commands
		for _, cmd := range app.Commands {
			fmt.Fprint(c.App.Writer, " ", cmd.Name)
		}
	}
	app.Run(os.Args)
}
