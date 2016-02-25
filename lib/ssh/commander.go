package ssh

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"bufio"
	"bytes"
	"errors"
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

var (
	ErrCopyNotRegular = errors.New("Can only copy regular file")
)

type Response struct {
	text string
	err  error
}

func (r Response) Data() (string, error) {
	return r.text, r.err
}

func getFileMode(m os.FileMode) (string, error) {
	if !m.IsRegular() {
		return "", ErrCopyNotRegular
	}
	perm := m.Perm() // retrieve permission
	return fmt.Sprintf("C0%d%d%d", perm&0700>>6, perm&0070>>3, perm&0007), nil
}

type Commander interface {
	// Copy file from src to dst
	Copy(src io.Reader, size int64, dst string, mode os.FileMode) error

	// Copy file from src to dst
	CopyFile(src, dst string, mode os.FileMode) error

	// Create full directory
	Mkdir(path string) error

	// Run command and retreive combinded output
	Run(cmd string) (output string, err error)

	// Run command and stream combined output
	Stream(cmd string) (output <-chan Response, err error)

	// Elevate commander role
	Sudo() SudoCommander
}

type SudoCommander interface {
	Commander

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
	sudo       bool
}

func (sshCmd *SSHCommander) connect() (*ssh.Session, error) {
	cli, err := ssh.Dial("tcp", sshCmd.addr, sshCmd.ssh_config)
	if err != nil {
		return nil, err
	} else {
		return cli.NewSession()
	}
}

func (sshCmd *SSHCommander) Sudo() SudoCommander {
	sshCmd.sudo = true
	return sshCmd
}

func (sshCmd *SSHCommander) Copy(src io.Reader, size int64, dst string, mode os.FileMode) error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()

	// setup remote path structure
	if err = sshCmd.Mkdir(filepath.Dir(dst)); err != nil {
		return err
	}

	perm, err := getFileMode(mode)
	if err != nil {
		return err
	}

	w, _ := session.StdinPipe()
	go func() {
		defer w.Close()
		// stream file content
		fmt.Fprintln(w, perm, size, filepath.Base(dst))
		io.Copy(w, src)
		fmt.Fprint(w, "\x00")
	}()

	// initiate scp on remote
	var cmd = fmt.Sprint("scp -t ", dst)
	if sshCmd.sudo {
		cmd = fmt.Sprintf("sudo -s %s", cmd)
	}
	return session.Run(cmd)
}

func (sshCmd *SSHCommander) CopyFile(src, dst string, mode os.FileMode) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return sshCmd.Copy(file, info.Size(), dst, mode)
}

func (sshCmd *SSHCommander) Mkdir(path string) error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	defer session.Close()
	// initiate mkdir on remote
	var cmd = fmt.Sprint("mkdir -p ", path)
	if sshCmd.sudo {
		cmd = fmt.Sprintf("sudo -s %s", cmd)
	}
	return session.Run(cmd)
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
	if sshCmd.sudo {
		cmd = fmt.Sprintf("sudo -s %s", cmd)
	}
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
	if sshCmd.sudo {
		cmd = fmt.Sprintf("sudo -s %s", cmd)
	}
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
	var dockerOpts = []string{
		"-H tcp://0.0.0.0:2375",
		"--tlsverify",
		"--tlscacert /etc/docker/ca.pem",
		"--tlscert /etc/docker/server-cert.pem",
		"--tlskey /etc/docker/server-key.pem",
	}
	var cmd = strings.Join(dockerOpts, " ")
	return session.Run(fmt.Sprintf(`echo 'DOCKER_OPTS="${DOCKER_OPTS} %s"' | sudo tee -a /etc/default/docker`, cmd))
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
