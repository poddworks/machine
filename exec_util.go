package main

import (
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"
	"gopkg.in/yaml.v2"

	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func parseArgs(c *cli.Context) (user, key, port string, hosts []string) {
	return c.String("user"), c.String("cert"), c.String("port"), c.StringSlice("host")
}

func runCmd(c *cli.Context) {
	var (
		cmd                    = strings.Join(c.Args(), " ")
		collect                = make(chan error)
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

	for _, host := range hosts {
		sshCfg.Server = host
		go exec(collect, ssh.New(sshCfg), &playbook)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func runScript(c *cli.Context) {
	var (
		scripts                = c.Args()
		collect                = make(chan error)
		sudo                   = c.Bool("sudo")
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

	for _, host := range hosts {
		sshCfg.Server = host
		go exec(collect, ssh.New(sshCfg), &playbook)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func runPlaybook(c *cli.Context) {
	var (
		collect                = make(chan error)
		user, key, port, hosts = parseArgs(c.Parent())

		sshCfg   = ssh.Config{User: user, Key: key, Port: port}
		playbook ssh.Recipe
	)

	if len(c.Args()) == 0 {
		fmt.Println("No playbook specified")
		os.Exit(1)
	}

	content, err := ioutil.ReadFile(c.Args()[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	err = yaml.Unmarshal(content, &playbook)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, host := range hosts {
		sshCfg.Server = host
		go exec(collect, ssh.New(sshCfg), &playbook)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func exec(collect chan<- error, cmdr ssh.Commander, playbook *ssh.Recipe) {
	var (
		// place holder for command output
		text string

		// Remote this commander is sending to
		host, _ = cmdr.Host()
	)

	for _, a := range playbook.Archive {
		fmt.Println(host, "-", "sending", a.Src, "to remote")
		if err := a.Send(cmdr); err != nil {
			fmt.Fprintln(os.Stderr, host, "-", err.Error())
			collect <- err
			return
		}
	}

	for _, p := range playbook.Provision {
		fmt.Println(host, "-", "playbook section", "-", p.Name)
		for _, a := range p.Archive {
			fmt.Println(host, "-", p.Name, "-", "sending", a.Src, "to remote")
			if err := a.Send(cmdr); err != nil {
				fmt.Fprintln(os.Stderr, host, "-", err.Error())
				collect <- err
				return
			}
		}
		for _, a := range p.Action {
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
