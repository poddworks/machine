package ssh

import (
	"io"
	"os"
)

type Commander interface {
	// Report host this Commander connects to
	Host() (host, port string)

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
	Run(cmd string) (string, error)

	// Run command and stay quiet
	RunQuiet(cmd string) error

	// Run command and stream combined output
	Stream(cmd string) (<-chan Response, error)

	// Elevate commander role and return a Deferr Target
	Sudo() SudoSession

	// Close Connection and cleanup
	Close() error
}

type SudoSession interface {
	// Step down from sudo status
	StepDown()
}
