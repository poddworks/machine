package ssh

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
)

type Response struct {
	text string
	err  error
}

func (r Response) Data() (string, error) {
	return r.text, r.err
}

type Commander interface {
	// Copy file from src to dst
	Copy(src io.Reader, size int64, dst string) error

	// Copy file from src to dst
	CopyFile(src, dst string) error

	// Create full directory
	MkdirAll(path string) error

	// Remove file, optionally remove contents recursively
	Remove(path string, recursive bool) error

	// Run command and retreive combinded output
	Run(cmd string) (output string, err error)

	// Run command and stream combined output
	Stream(cmd string) (output <-chan Response, err error)

	// Utility with Docker Engine control
	ConfigureDockerTLS() error
	StartDocker() error
	StopDocker() error
}

type Config struct {
	User     string
	Server   string
	Key      string
	Port     string
	Password string
}

func (cfg Config) GetKeyFile() (ssh.Signer, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	keyfile := strings.Replace(cfg.Key, "~", usr.HomeDir, 1)
	keyfile, err = filepath.Abs(keyfile)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}
	pubkey, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}

type SSHCommander struct {
	ssh_config *ssh.ClientConfig
	addr       string
}

func (sshCmd *SSHCommander) connect() (*ssh.Session, error) {
	cli, err := ssh.Dial("tcp", sshCmd.addr, sshCmd.ssh_config)
	if err != nil {
		return nil, err
	} else {
		return cli.NewSession()
	}
}

func (sshCmd *SSHCommander) Copy(src io.Reader, size int64, dst string) error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()

	// setup remote path structure
	if err = sshCmd.MkdirAll(filepath.Dir(dst)); err != nil {
		return err
	}

	w, _ := session.StdinPipe()
	go func() {
		// stream file content
		fmt.Fprintln(w, "C0600", size, filepath.Base(dst))
		io.Copy(w, src)
		fmt.Fprint(w, "\x00")
		w.Close()
	}()

	// initiate scp on remote
	return session.Run(fmt.Sprint("sudo scp -t ", dst))
}

func (sshCmd *SSHCommander) CopyFile(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return sshCmd.Copy(file, info.Size(), dst)
}

func (sshCmd *SSHCommander) MkdirAll(path string) error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run(fmt.Sprint("sudo mkdir -p ", path))
}

func (sshCmd *SSHCommander) Remove(path string, recursive bool) error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()
	if recursive {
		return session.Run(fmt.Sprint("sudo rm -rf ", path))
	} else {
		return session.Run(fmt.Sprint("sudo rm -f ", path))
	}
}

// buffer is a utility object for combined output
type buffer struct {
	sync.Mutex
	buf bytes.Buffer
}

func (b *buffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.buf.Write(p)
}

func (sshCmd *SSHCommander) Run(cmd string) (output string, err error) {
	session, err := sshCmd.connect()
	if err != nil {
		return
	}
	defer session.Close()
	var b buffer
	session.Stdout = &b
	session.Stderr = &b
	err = session.Run(cmd)
	output = b.buf.String()
	return
}

func (sshCmd *SSHCommander) Stream(cmd string) (<-chan Response, error) {
	session, err := sshCmd.connect()
	if err != nil {
		return nil, err
	}
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()
	output := make(chan Response)
	go func() {
		var reader = func(r io.Reader) <-chan string {
			var ch = make(chan string)
			go func() {
				defer close(ch)
				lnr := bufio.NewScanner(r)
				for lnr.Scan() {
					ch <- lnr.Text()
				}
			}()
			return ch
		}
		var ln string
		defer session.Close()
		defer close(output)
		stdOut, stdErr := reader(stdout), reader(stderr)
		for outOk, errOk := true, true; outOk || errOk; {
			select {
			case ln, outOk = <-stdOut:
				if outOk {
					output <- Response{text: ln}
				}
			case ln, errOk = <-stdErr:
				if errOk {
					output <- Response{text: ln}
				}
			}
		}
		output <- Response{err: session.Wait()}
	}()
	if err := session.Start(cmd); err != nil {
		return nil, err
	} else {
		return output, nil
	}
}

func (sshCmd *SSHCommander) ConfigureDockerTLS() error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run(`echo 'DOCKER_OPTS="-H unix:///var/run/docker.sock -H tcp://0.0.0.0:2375 --tlsverify --tlscacert /etc/docker/ca.pem --tlscert /etc/docker/server-cert.pem --tlskey /etc/docker/server-key.pem "' | sudo tee /etc/default/docker`)
}

func (sshCmd *SSHCommander) StartDocker() error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run("sudo service docker start")
}

func (sshCmd *SSHCommander) StopDocker() error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run("sudo service docker stop")
}

func New(cfg Config) Commander {
	auths := []ssh.AuthMethod{}
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
	}
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers))
		defer sshAgent.Close()
	}
	if pubkey, err := cfg.GetKeyFile(); err == nil {
		auths = append(auths, ssh.PublicKeys(pubkey))
	}
	return &SSHCommander{
		ssh_config: &ssh.ClientConfig{User: cfg.User, Auth: auths},
		addr:       cfg.Server + ":" + cfg.Port,
	}
}
