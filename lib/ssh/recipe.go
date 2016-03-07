package ssh

import (
	"fmt"
	path "path/filepath"
)

const (
	TMP_REMOTE_DIR = "/tmp/.machine"
)

type Recipe struct {
	Archive   `yaml:"archive"`
	Provision []Provision `yaml:"provision"`
}

type Archive struct {
	RemoteDir string   `yaml:"dir"`
	Local     []string `yaml:"local"`
}

func (a Archive) Send(cmdr Commander) error {
	for _, local := range a.Local {
		dst := path.Join(a.RemoteDir, path.Base(local))
		if err := cmdr.CopyFile(local, dst, 0644); err != nil {
			return err
		}
	}
	return nil
}

type Provision struct {
	Archive `yaml:"archive"`
	Name    string   `yaml:"name"`
	Ok2fail bool     `yaml:"ok2fail"`
	Action  []Action `yaml:"action"`
}

type Action struct {
	Cmd    string `yaml:"cmd"`
	Script string `yaml:"script"`
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
