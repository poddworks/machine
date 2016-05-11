package main

import (
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"
	"gopkg.in/yaml.v2"

	"fmt"
	"io"
	"os"
	"strings"
)

func parseArgs(c *cli.Context) (user, key, port string, hosts []string) {
	return c.String("user"), c.String("cert"), c.String("port"), c.StringSlice("host")
}

func runCmd(c *cli.Context) error {
	var (
		cmd                    = strings.Join(c.Args(), " ")
		collect                = make(chan error)
		dryrun                 = c.Parent().Bool("dryrun")
		user, key, port, hosts = parseArgs(c.Parent())

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
		fmt.Fprintln(os.Stderr, "One or more task failed")
		os.Exit(1)
	}

	return nil
}

func runScript(c *cli.Context) error {
	var (
		scripts                = c.Args()
		collect                = make(chan error)
		sudo                   = c.Bool("sudo")
		dryrun                 = c.Parent().Bool("dryrun")
		user, key, port, hosts = parseArgs(c.Parent())

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
		fmt.Fprintln(os.Stderr, "One or more task failed")
		os.Exit(1)
	}

	return nil
}

func runPlaybook(c *cli.Context) error {
	var (
		collect                = make(chan error)
		dryrun                 = c.Parent().Bool("dryrun")
		user, key, port, hosts = parseArgs(c.Parent())

		sshCfg = ssh.Config{User: user, Key: key, Port: port}
	)

	if len(c.Args()) == 0 {
		fmt.Println("No playbook specified")
		os.Exit(1)
	}

	r, err := os.Open(c.Args()[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
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
				fmt.Fprintln(os.Stderr, "Deocoding playbook content error")
				os.Exit(1)
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
			fmt.Fprintln(os.Stderr, "One or more task failed")
			os.Exit(1)
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

	for _, a := range playbook.Archive {
		fmt.Println(host, "-", "sending", "-", a.Source(cmdr), "-", a.Dest())
		if dryrun {
			continue // skip ahead
		}
		if err := a.Send(cmdr); err != nil {
			fmt.Fprintln(os.Stderr, host, "-", err.Error())
			collect <- err
			return
		}
	}

	for _, p := range playbook.Provision {
		fmt.Println(host, "-", "playbook section", "-", p.Name)
		for _, a := range p.Archive {
			fmt.Println(host, "-", p.Name, "-", "sending", "-", a.Source(cmdr), "-", a.Dest())
			if dryrun {
				continue // skip ahead
			}
			if err := a.Send(cmdr); err != nil {
				fmt.Fprintln(os.Stderr, host, "-", err.Error())
				collect <- err
				return
			}
		}
		for _, a := range p.Action {
			fmt.Println(host, "-", p.Name, "-", a.Command())
			if dryrun {
				continue // skip ahead
			}
			respStream, err := a.Act(cmdr)
			if err != nil {
				fmt.Fprintln(os.Stderr, host, "-", p.Name, "-", err.Error())
				collect <- err
				return
			}
			for output := range respStream {
				text, err = output.Data()
				if err != nil {
					fmt.Fprintln(os.Stderr, host, "-", p.Name, "-", err.Error())
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

	collect <- nil // mark end of playbook
}
