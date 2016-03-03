package ssh

import (
	"errors"
	"fmt"
	"os"
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
