package ssh

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"

	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
)

type SSHCommander struct {
	ssh_config  *ssh.ClientConfig
	sshAuthSock net.Conn
	addr        string
	sudo        bool
}

func (sshCmd *SSHCommander) connect() (*ssh.Session, error) {
	cli, err := ssh.Dial("tcp", sshCmd.addr, sshCmd.ssh_config)
	if err != nil {
		return nil, err
	} else {
		return cli.NewSession()
	}
}

func (sshCmd *SSHCommander) Host() (host, port string) {
	host, port, _ = net.SplitHostPort(sshCmd.addr)
	return
}

func (sshCmd *SSHCommander) Sudo() SudoSession {
	sshCmd.sudo = true
	return sshCmd
}

func (sshCmd *SSHCommander) StepDown() {
	sshCmd.sudo = false
}

func (sshCmd *SSHCommander) Load(target string, here io.Writer) error {
	session, err := sshCmd.connect()
	if err != nil {
		return err
	}
	r, _ := session.StdoutPipe()
	var ret = make(chan error)
	go func() {
		defer session.Close()
		var cmd = fmt.Sprint("cat ", target)
		if sshCmd.sudo {
			cmd = fmt.Sprintf("sudo -s %s", cmd)
		}
		ret <- session.Run(cmd)
	}()
	io.Copy(here, r)
	return <-ret
}

func (sshCmd *SSHCommander) LoadFile(target, here string, mode os.FileMode) error {
	buf := new(bytes.Buffer)
	err := sshCmd.Load(target, buf)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(here, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, buf)
	return err
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

func (sshCmd *SSHCommander) RunQuiet(cmd string) (err error) {
	session, err := sshCmd.connect()
	if err != nil {
		return
	}
	session.Stdout = nil
	session.Stderr = nil
	defer session.Close()
	if sshCmd.sudo {
		cmd = fmt.Sprintf("sudo -s %s", cmd)
	}
	err = session.Run(cmd)
	return
}

func (sshCmd *SSHCommander) Shell() (err error) {
	var (
		termWidth, termHeight int
	)

	session, err := sshCmd.connect()
	if err != nil {
		return
	}
	defer session.Close()

	// Attach host input, output
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}
	fd := os.Stdin.Fd()

	if terminal.IsTerminal(int(fd)) {
		oldState, err := terminal.MakeRaw(int(fd))
		if err != nil {
			return err
		}
		defer terminal.Restore(int(fd), oldState)

		termWidth, termHeight, err = terminal.GetSize(int(fd))
		if err != nil {
			termWidth = 80
			termHeight = 24
		}
	}

	err = session.RequestPty("xterm", termHeight, termWidth, modes)
	if err != nil {
		return
	}

	err = session.Shell()
	if err != nil {
		return
	}

	return session.Wait()
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

func (sshCmd *SSHCommander) Close() error {
	if sshCmd.sshAuthSock != nil {
		return sshCmd.sshAuthSock.Close()
	} else {
		return nil
	}
}

func New(cfg Config) Commander {
	var (
		auths = []ssh.AuthMethod{}

		sshAuthSock net.Conn
	)
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
	}
	if conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		sshAuthSock = conn
	}
	if pubkey, err := cfg.GetKeyFile(); err == nil {
		auths = append(auths, ssh.PublicKeys(pubkey))
	}
	return &SSHCommander{
		ssh_config:  &ssh.ClientConfig{User: cfg.User, Auth: auths},
		sshAuthSock: sshAuthSock,
		addr:        cfg.Server + ":" + cfg.Port,
	}
}
