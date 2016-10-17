package machine

import (
	"github.com/poddworks/machine/lib/cert"
	"github.com/poddworks/machine/lib/docker"
	"github.com/poddworks/machine/lib/ssh"

	"github.com/urfave/cli"

	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var install_docker_stemps = []string{
	`apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D`,
	`apt-get update && apt-get install -y apt-transport-https linux-image-extra-$(uname -r)`,
	`echo "deb https://apt.dockerproject.org/repo ubuntu-trusty main" | tee /etc/apt/sources.list.d/docker.list`,
	`apt-get update && apt-get install -y docker-engine`,
}

type Host struct {
	CertPath     string
	Organization string

	User     string
	Cert     string
	IsDocker bool

	// SSH config for command forwarding
	cmdr ssh.Commander

	// Mark that we are running fresh
	provision bool
}

func ParseCertArgs(c *cli.Context) (org, certpath string, err error) {
	org = c.String("org")
	if org == "" {
		org = c.GlobalString("org")
	}
	if org == "" {
		err = fmt.Errorf("Missing required argument `org`")
		return
	}
	certpath = c.String("confdir")
	if certpath == "" {
		certpath = strings.Replace(c.GlobalString("confdir"), "~", os.Getenv("HOME"), 1)
	}
	if certpath == "" {
		err = fmt.Errorf("Missing required argument `confdir`")
		return
	}
	if _, err = os.Stat(certpath); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(certpath, 0700)
		}
	}
	return
}

func NewDockerHost(org, certpath, user, cert string) *Host {
	return &Host{
		CertPath:     certpath,
		Organization: org,
		User:         user,
		Cert:         cert,
		IsDocker:     true,
		provision:    true,
	}
}

func NewHost(org, certpath, user, cert string) *Host {
	return &Host{
		CertPath:     certpath,
		Organization: org,
		User:         user,
		Cert:         cert,
		IsDocker:     false,
		provision:    true,
	}
}

func (h *Host) SetProvision(provision bool) {
	h.provision = provision
}

func (h *Host) waitSSH() error {
	var (
		status   = make(chan error)
		tick     = time.NewTicker(3 * time.Second)
		attempts = 12

		host, _ = h.cmdr.Host()
	)
	defer tick.Stop()
	go func() {
		defer close(status)
		for ; attempts > 0; attempts-- {
			if err := h.cmdr.RunQuiet("date"); err != nil {
				time.Sleep(5 * time.Second)
			} else {
				break // success
			}
		}
		if attempts == 0 {
			status <- fmt.Errorf("%s - Unable to contact remote", host)
		}
	}()
	var result error
	for yay := true; yay; {
		select {
		case err := <-status:
			result = err
			yay = false
		case <-tick.C:
			fmt.Print(".")
		}
	}
	fmt.Println(".")
	return result
}

func (h *Host) Shell(host string) error {
	ssh_config := ssh.Config{User: h.User, Server: host, Key: h.Cert, Port: "22"}
	h.cmdr = ssh.New(ssh_config)
	defer h.cmdr.Close()
	return h.cmdr.Shell()
}

func (h *Host) exec(cmd string) error {
	var (
		status   = make(chan error)
		tick     = time.NewTicker(3 * time.Second)
		attempts = 3

		host, _ = h.cmdr.Host()
	)
	defer tick.Stop()
	go func() {
		defer close(status)
		for ; attempts > 0; attempts-- {
			if err := h.cmdr.RunQuiet(fmt.Sprintf("bash -c '%s'", cmd)); err != nil {
				fmt.Fprintf(os.Stderr, "%s - %s\n", host, err)
				time.Sleep(1 * time.Second)
			} else {
				break // success!!
			}
		}
		if attempts == 0 {
			status <- fmt.Errorf("exec %s failed", cmd)
		}
	}()
	var result error
	for yay := true; yay; {
		select {
		case err := <-status:
			result = err
			yay = false
		case <-tick.C:
			fmt.Print(".")
		}
	}
	fmt.Println(".")
	return result
}

func (h *Host) sendfile(src io.Reader, size int64, dst string, mode os.FileMode) error {
	var (
		status   = make(chan error)
		tick     = time.NewTicker(3 * time.Second)
		attempts = 3

		host, _ = h.cmdr.Host()
	)
	defer tick.Stop()
	go func() {
		defer close(status)
		for ; attempts > 0; attempts-- {
			if err := h.cmdr.Copy(src, size, dst, mode); err != nil {
				fmt.Fprintf(os.Stderr, "%s - %s\n", host, err)
				time.Sleep(1 * time.Second)
			} else {
				break // success!!
			}
		}
		if attempts == 0 {
			status <- fmt.Errorf("sendfile %s failed", dst)
		}
	}()
	var result error
	for yay := true; yay; {
		select {
		case err := <-status:
			result = err
			yay = false
		case <-tick.C:
			fmt.Print(".")
		}
	}
	fmt.Println(".")
	return result
}

func (h *Host) InstallDockerEngine(host string) error {
	if !h.IsDocker { // Not processing because not a Docker Engine
		fmt.Println(host, "- skipping Docker Engine Install")
		return nil
	}
	ssh_config := ssh.Config{User: h.User, Server: host, Key: h.Cert, Port: "22"}
	h.cmdr = ssh.New(ssh_config)
	defer h.cmdr.Close()

	fmt.Print(host, " - install Docker Engine ")
	if timeout := h.waitSSH(); timeout != nil {
		return timeout
	}
	h.cmdr.Sudo()
	for _, cmd := range install_docker_stemps {
		fmt.Print(host, " - ", cmd, " ")
		if err := h.exec(cmd); err != nil {
			return err // trouble completing command, quit task
		}
	}
	return nil
}

func (h *Host) InstallDockerEngineCertificate(host string, altname ...string) error {
	if !h.IsDocker { // Not processing because not a Docker Engine
		fmt.Println(host, "- skipping Docker Certificate Install")
		return nil
	}
	ssh_config := ssh.Config{User: h.User, Server: host, Key: h.Cert, Port: "22"}
	h.cmdr = ssh.New(ssh_config)
	defer h.cmdr.Close()

	var subAltNames = []string{
		host,
		"localhost",
		"127.0.0.1",
	}
	subAltNames = append(subAltNames, altname...)

	fmt.Println(host, "- generate cert for subjects -", subAltNames)
	CA, Cert, Key, err := cert.GenerateServerCertificate(h.CertPath, h.Organization, subAltNames)
	if err != nil {
		return err
	}

	fmt.Print(host, " - configure docker engine", " ")
	if timeout := h.waitSSH(); timeout != nil {
		return timeout
	} else {
		h.cmdr.Sudo()
		return h.sendEngineCertificate(CA, Cert, Key)
	}
}

func (h *Host) sendEngineCertificate(ca, cert, key *cert.PemBlock) error {
	host, _ := h.cmdr.Host()

	fmt.Print(host, " - Sending Cert", " ")
	if err := h.sendfile(cert.Buf, int64(cert.Buf.Len()), "/etc/docker/"+cert.Name, 0644); err != nil {
		return err
	}

	fmt.Print(host, " - Sending Key", " ")
	if err := h.sendfile(key.Buf, int64(key.Buf.Len()), "/etc/docker/"+key.Name, 0600); err != nil {
		return err
	}

	fmt.Print(host, " - Sending CA", " ")
	if err := h.sendfile(ca.Buf, int64(ca.Buf.Len()), "/etc/docker/"+ca.Name, 0644); err != nil {
		return err
	}

	fmt.Print(host, " - Configuring Docker Engine", " ")
	if err := h.configureDockerTLS(); err != nil {
		return err
	}

	fmt.Print(host, " - Stopping Docker Engine", " ")
	h.stopDocker()

	fmt.Print(host, " - Starting Docker Engine", " ")
	if err := h.startDocker(); err != nil {
		return err
	}

	return nil
}

func (h *Host) configureDockerTLS() error {
	const (
		daemonPath = "/etc/docker/daemon.json"

		CAPem   = "/etc/docker/ca.pem"
		CertPem = "/etc/docker/server-cert.pem"
		KeyPem  = "/etc/docker/server-key.pem"
	)

	var (
		dOpts *docker.DaemonConfig

		buf = new(bytes.Buffer)
	)

	if h.provision {
		dOpts = new(docker.DaemonConfig)
		dOpts.AddHost("unix:///var/run/docker.sock")
	} else {
		err := h.cmdr.Load(daemonPath, buf)
		if err != nil {
			return err
		}
		dOpts, err = docker.LoadDaemonConfig(buf.Bytes())
		if err != nil {
			return err
		}
	}
	dOpts.AddHost("tcp://0.0.0.0:2376")
	dOpts.TlsVerify = true
	dOpts.TlsCACert = CAPem
	dOpts.TlsCert = CertPem
	dOpts.TlsKey = KeyPem
	if r, err := dOpts.Reader(); err != nil {
		return err
	} else {
		return h.sendfile(r, int64(r.Len()), daemonPath, 0600)
	}
}

func (h *Host) startDocker() error {
	return h.exec("service docker start")
}

func (h *Host) stopDocker() error {
	return h.exec("service docker stop")
}
