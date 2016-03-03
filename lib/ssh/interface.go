package ssh

import (
	"io"
	"os"
)

type Commander interface {
	// Report host this Commander connects to
	Host() string

	// Load file from target to here
	Load(target string, here io.Writer) error

	// Load file from target to here
	LoadFile(target, here string, mode os.FileMode) error

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
	Sudo() Commander
}
