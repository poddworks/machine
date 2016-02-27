package main

import (
	"github.com/jeffjen/machine/lib/cert"
	"github.com/jeffjen/machine/lib/ssh"

	"github.com/codegangsta/cli"

	"fmt"
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
			var (
				respStream <-chan ssh.Response
				err        error

				text string
			)
			sshctx := ssh.New(ssh.Config{User: user, Server: host, Key: key, Port: "22"})
			for _, script := range scripts {
				dst := path.Join("/tmp", path.Base(script))
				if err = sshctx.CopyFile(script, dst, 0644); err != nil {
					fmt.Println(err.Error())
					collect <- err
					return
				}
				fmt.Println(host, "- sent script", script, "->", dst)
				if sudo {
					respStream, err = sshctx.Sudo().Stream("bash " + dst)
				} else {
					respStream, err = sshctx.Stream("bash " + dst)
				}
				if err != nil {
					fmt.Println(err.Error())
					collect <- err
					return
				}
				for output := range respStream {
					text, err = output.Data()
					if err != nil {
						fmt.Println(host, "-", err.Error())
						// steam will end because error state delivers last
					} else {
						fmt.Println(host, "-", text)
					}
				}
				if err != nil { // abort execution if script failed
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
