package ssh

import (
	"fmt"
	path "path/filepath"
)

const (
	TMP_REMOTE_DIR = "/tmp/.machine"
)

type Recipe struct {
	Archive   []Archive   `yaml:"archive,omitempty"`
	Provision []Provision `yaml:"provision"`
}

type Archive struct {
	Src  string `yaml:"src"`
	Dst  string `yaml:"dst"`
	Dir  string `yaml:"dir"`
	Sudo bool   `yaml:"sudo:`
}

func (a Archive) Send(cmdr Commander) error {
	if a.Sudo {
		defer cmdr.Sudo().StepDown()
	}
	if a.Dst == "" {
		a.Dst = path.Base(a.Src)
	}
	dst := path.Join(a.Dir, a.Dst)
	return cmdr.CopyFile(a.Src, dst, 0644)
}

type Provision struct {
	Archive []Archive `yaml:"archive,omitempty"`
	Name    string    `yaml:"name"`
	Ok2fail bool      `yaml:"ok2fail"`
	Action  []Action  `yaml:"action"`
}

type Action struct {
	Cmd    string `yaml:"cmd,omitempty"`
	Script string `yaml:"script,omitempty"`
	Sudo   bool   `yaml:"sudo"`
}

func (a Action) Act(cmdr Commander) (output <-chan Response, err error) {
	switch {
	case a.Cmd != "":
		if a.Sudo {
			defer cmdr.Sudo().StepDown()
		}
		output, err = cmdr.Stream(a.Cmd)
		break
	case a.Script != "":
		dst := path.Join(TMP_REMOTE_DIR, path.Base(a.Script))
		err = cmdr.CopyFile(a.Script, dst, 0644)
		if err == nil {
			if a.Sudo {
				defer cmdr.Sudo().StepDown()
			}
			output, err = cmdr.Stream(fmt.Sprintf("bash %s", dst))
		}
		break
	}
	return
}
