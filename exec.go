package main

import (
	"github.com/jeffjen/yaml"
	"github.com/poddworks/machine/lib/ssh"

	"github.com/urfave/cli"

	"fmt"
	"io"
	"os"
	"strings"
)

func parseArgs(c *cli.Context) (user, key, port string, hosts []string) {
	return c.GlobalString("user"), c.GlobalString("cert"), c.GlobalString("port"), c.GlobalStringSlice("host")
}

func runCmd(c *cli.Context) error {
	var (
		cmd                    = strings.Join(c.Args(), " ")
		collect                = make(chan error)
		dryrun                 = c.GlobalBool("dryrun")
		user, key, port, hosts = parseArgs(c)

		sshCfg   = ssh.Config{User: user, Key: key, Port: port}
		playbook = ssh.Recipe{}
	)

	playbook.Provision = append(playbook.Provision, ssh.Provision{
		Name:    "Running one command",
		Ok2fail: false,
		Action: []ssh.Action{
			{Cmd: cmd},
		},
	})

	var errCnt = 0
	for _, host := range hosts {
		sshCfg.Server = host
		go exec(collect, dryrun, ssh.New(sshCfg), &playbook)
	}
	for chk := 0; chk < len(hosts); chk++ {
		if e := <-collect; e != nil {
			errCnt++
		}
	}
	if errCnt > 0 {
		return cli.NewExitError("One or more task failed", 1)
	}

	return nil
}

func runScript(c *cli.Context) error {
	var (
		scripts                = c.Args()
		collect                = make(chan error)
		sudo                   = c.Bool("sudo")
		dryrun                 = c.GlobalBool("dryrun")
		user, key, port, hosts = parseArgs(c)

		sshCfg   = ssh.Config{User: user, Key: key, Port: port}
		playbook = ssh.Recipe{}
	)

	for _, script := range scripts {
		playbook.Provision = append(playbook.Provision, ssh.Provision{
			Name:    fmt.Sprintf("Running script %s", script),
			Ok2fail: false,
			Action: []ssh.Action{
				{Script: script, Sudo: sudo},
			},
		})
	}

	var errCnt = 0
	for _, host := range hosts {
		sshCfg.Server = host
		go exec(collect, dryrun, ssh.New(sshCfg), &playbook)
	}
	for chk := 0; chk < len(hosts); chk++ {
		if e := <-collect; e != nil {
			errCnt++
		}
	}
	if errCnt > 0 {
		return cli.NewExitError("One or more task failed", 1)
	}

	return nil
}

func runPlaybook(c *cli.Context) error {
	var (
		collect                = make(chan error)
		dryrun                 = c.GlobalBool("dryrun")
		user, key, port, hosts = parseArgs(c)

		sshCfg = ssh.Config{User: user, Key: key, Port: port}
	)

	if len(c.Args()) == 0 {
		return cli.NewExitError("No playbook specified", 1)
	}

	r, err := os.Open(c.Args()[0])
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	defer r.Close()

	var decoder = yaml.NewDecoder(r)
	defer decoder.Close()
	for {
		playbook := new(ssh.Recipe)
		if err = decoder.Decode(playbook); err != nil {
			if err == io.EOF {
				break
			} else {
				return cli.NewExitError("Deocoding playbook content error", 1)
			}
		}
		var errCnt = 0
		for _, host := range hosts {
			sshCfg.Server = host
			go exec(collect, dryrun, ssh.New(sshCfg), playbook)
		}
		for chk := 0; chk < len(hosts); chk++ {
			if e := <-collect; e != nil {
				errCnt++
			}
		}
		if errCnt > 0 {
			return cli.NewExitError("One or more task failed", 1)
		}
	}

	return nil
}

func exec(collect chan<- error, dryrun bool, cmdr ssh.Commander, playbook *ssh.Recipe) {
	var (
		// place holder for command output
		text string

		// Remote this commander is sending to
		host, _ = cmdr.Host()
	)

	defer cmdr.Close()

	for _, a := range playbook.Archive {
		fmt.Println(host, "-", "sending", "-", a.Source(cmdr), "-", a.Dest())
		if dryrun {
			continue // skip ahead
		}
		if a.Skip {
			continue // skip ahead
		} else {
			if err := a.Send(cmdr); err != nil {
				fmt.Fprintln(os.Stderr, host, "-", err)
				collect <- err
				return
			}
		}
	}

	for _, p := range playbook.Provision {
		fmt.Println(host, "-", "playbook section", "-", p.Name)
		if dryrun {
			continue // skip ahead
		}
		if p.Skip {
			continue // skip ahead
		}
		for _, a := range p.Archive {
			fmt.Println(host, "-", p.Name, "-", "sending", "-", a.Source(cmdr), "-", a.Dest())
			if a.Skip {
				continue // skip ahead
			} else {
				if err := a.Send(cmdr); err != nil {
					fmt.Fprintln(os.Stderr, host, "-", err)
					collect <- err
					return
				}
			}
		}
		for _, a := range p.Action {
			fmt.Println(host, "-", p.Name, "-", a.Command())
			if a.Skip {
				continue // skip ahead
			} else {
				respStream, err := a.Act(cmdr)
				if err != nil {
					fmt.Fprintln(os.Stderr, host, "-", p.Name, "-", err)
					collect <- err
					return
				}
				for output := range respStream {
					text, err = output.Data()
					if err != nil {
						fmt.Fprintln(os.Stderr, host, "-", p.Name, "-", err)
						// steam will end because error state delivers last
					} else {
						fmt.Println(host, "-", p.Name, "-", text)
					}
				}
				// abort if action failed and its not okay to fail
				if err != nil && !p.Ok2fail {
					collect <- err
					return
				}
			}
		}
		// Wipe the slate for this provision block
		p.Clean(cmdr)
	}

	collect <- nil // mark end of playbook
}
