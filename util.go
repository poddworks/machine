package main

import (
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"
	"gopkg.in/yaml.v2"

	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	path "path/filepath"
	"strings"
)

func parseArgs(c *cli.Context) (user, key string, hosts []string) {
	return c.String("user"), c.String("cert"), c.StringSlice("host")
}

func runCmd(c *cli.Context) {
	var (
		cmd              = strings.Join(c.Args(), " ")
		collect          = make(chan error)
		user, key, hosts = parseArgs(c.Parent())
	)
	for _, host := range hosts {
		go func(host string) {
			sshctx := ssh.New(ssh.Config{User: user, Server: host, Key: key, Port: "22"})
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
		scripts          = c.Args()
		collect          = make(chan error)
		sudo             = c.Bool("sudo")
		user, key, hosts = parseArgs(c.Parent())
	)
	for _, host := range hosts {
		go func(host string) {
			cmdr := ssh.New(ssh.Config{User: user, Server: host, Key: key, Port: "22"})
			if sudo {
				cmdr.Sudo()
			}
			for _, script := range scripts {
				dst := path.Join("/tmp", path.Base(script))
				if err := cmdr.CopyFile(script, dst, 0644); err != nil {
					fmt.Println(err.Error())
					collect <- err
					return
				}
				fmt.Println(host, "- sent script", script, "->", dst)
				respStream, err := cmdr.Stream("bash " + dst)
				if err != nil {
					fmt.Println(err.Error())
					collect <- err
					return
				}
				var text string
				for output := range respStream {
					text, err = output.Data()
					if err != nil {
						fmt.Println(host, "-", err.Error())
						// steam will end because error state delivers last
					} else {
						fmt.Println(host, "-", text)
					}
				}
				// abort if script execution failed
				if err != nil {
					collect <- err
					return
				}
			}
			collect <- nil // mark end of script run
		}(host)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func runPlaybook(c *cli.Context) {
	var (
		collect          = make(chan error)
		user, key, hosts = parseArgs(c.Parent())
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

	var playbook ssh.Recipe
	err = yaml.Unmarshal(content, &playbook)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	for _, host := range hosts {
		go func(host string) {
			cmdr := ssh.New(ssh.Config{User: user, Server: host, Key: key, Port: "22"})
			fmt.Println(host, "-", "sending archive to remote")
			if err := playbook.Archive.Send(cmdr); err != nil {
				fmt.Fprintln(os.Stderr, host, "-", err.Error())
				collect <- err
				return
			}
			for _, p := range playbook.Provision {
				fmt.Println(host, "-", "playbook section", "-", p.Name)
				for _, a := range p.Action {
					respStream, err := a.Act(cmdr)
					if err != nil {
						fmt.Fprintln(os.Stderr, host, "-", err.Error())
						collect <- err
						return
					}
					var text string
					for output := range respStream {
						text, err = output.Data()
						if err != nil {
							fmt.Fprintln(os.Stderr, host, "-", err.Error())
							// steam will end because error state delivers last
						} else {
							fmt.Println(host, "-", text)
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
		}(host)
	}
	for chk := 0; chk < len(hosts); chk++ {
		<-collect
	}
}

func parseCertArgs(c *cli.Context) (org, certpath string, err error) {
	usr, err := user.Current()
	if err != nil {
		return // Unable to determine user
	}
	org = c.Parent().String("organization")
	certpath = c.Parent().String("certpath")
	certpath = strings.Replace(certpath, "~", usr.HomeDir, 1)
	certpath, err = path.Abs(certpath)
	if err != nil {
		return
	}
	err = os.MkdirAll(certpath, 0700)
	return
}

func generateServerCertificate(c *cli.Context) (CA, Cert, Key *cert.PemBlock) {
	var hosts = make([]string, 0)
	if hostname := c.String("host"); hostname == "" {
		fmt.Println("You must provide hostname to create Certificate for")
		os.Exit(1)
	} else {
		hosts = append(hosts, hostname)
	}
	hosts = append(hosts, c.StringSlice("altname")...)
	org, certpath, err := parseCertArgs(c)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	CA, Cert, Key, err = cert.GenerateServerCertificate(certpath, org, hosts)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return
}
