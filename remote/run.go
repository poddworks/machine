package remote

import (
	"github.com/codegangsta/cli"
	"github.com/hypersleep/easyssh"

	"fmt"
	"os"
	"path"
)

func enforceOneArg(c *cli.Context) error {
	if len(c.Args()) != 1 {
		fmt.Println("Expected one argument in exec")
		os.Exit(1)
	}
	return nil
}

func parseArgs(c *cli.Context) (user, key string, hosts []string) {
	return c.String("user"), c.String("cert"), c.StringSlice("host")
}

func runCmd(c *cli.Context) {
	var (
		cmd              = c.Args()[0]
		collect          = make(chan error)
		user, key, hosts = parseArgs(c.Parent())
	)
	for _, host := range hosts {
		go func(host string) {
			sshctx := &easyssh.MakeConfig{User: user, Server: host, Key: key, Port: "22"}
			resp, err := sshctx.Run(cmd)
			if err != nil {
				fmt.Println(err.Error())
				collect <- err
			} else {
				fmt.Println(host, "-", resp)
				collect <- nil
			}
		}(host)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func runScript(c *cli.Context) {
	var (
		script           = c.Args()[0]
		collect          = make(chan error)
		user, key, hosts = parseArgs(c.Parent())
	)
	for _, host := range hosts {
		go func(host string) {
			sshctx := &easyssh.MakeConfig{User: user, Server: host, Key: key, Port: "22"}
			if err := sshctx.Scp(script); err != nil {
				fmt.Println(err.Error())
				collect <- err
				return
			}
			fmt.Println(host, "- script sent")
			output, done, err := sshctx.Stream(fmt.Sprintf("cat %s | sudo bash -", path.Base(script)))
			if err != nil {
				fmt.Println(err.Error())
				collect <- err
				return
			}
			for yes := true; yes; {
				select {
				case o := <-output:
					fmt.Println(host, "-", o)
				case <-done:
					collect <- nil
					yes = false
				}
			}
		}(host)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func NewCommand() cli.Command {
	return cli.Command{
		Name:  "exec",
		Usage: "Invoke command on remote host via SSH",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "user", EnvVar: "MACHINE_USER", Usage: "Run command as user"},
			cli.StringFlag{Name: "cert", EnvVar: "MACHINE_CERT_FILE", Usage: "Private key to use in Authentication"},
			cli.StringSliceFlag{Name: "host", Usage: "Remote host to run command in"},
		},
		Subcommands: []cli.Command{
			{
				Name:   "run",
				Usage:  "Invoke command from argument",
				Before: enforceOneArg,
				Action: runCmd,
			},
			{
				Name:   "script",
				Usage:  "Invoke script from argument",
				Before: enforceOneArg,
				Action: runScript,
			},
		},
	}
}
