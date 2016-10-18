package machine

import (
	swarm "github.com/docker/docker/api/types/swarm"
	docker "github.com/docker/docker/client"
	tlsconfig "github.com/docker/go-connections/tlsconfig"

	"golang.org/x/net/context"

	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	path "path/filepath"
	"strings"
)

const (
	INSTANCE_LISTING_FILE = "~/.machine/instance.json"
)

type Instance struct {
	Id         string
	Driver     string
	DockerHost *net.TCPAddr
	Host       string
	AltHost    []string
	State      string

	// DO NOT SERIALIZE THIS RUNTIME FIELD
	cli *docker.Client `json:"-"`
}

func (inst *Instance) DockerHostName() string {
	return fmt.Sprintf("%s://%s", inst.DockerHost.Network(), inst.DockerHost)
}

func (inst *Instance) NewDockerClient(certpath string) *docker.Client {
	const dockerAPIVersion = "1.24"

	if inst.DockerHost == nil {
		return nil
	}

	var client *http.Client
	options := tlsconfig.Options{
		CAFile:             path.Join(certpath, "ca.pem"),
		CertFile:           path.Join(certpath, "cert.pem"),
		KeyFile:            path.Join(certpath, "key.pem"),
		InsecureSkipVerify: false,
	}
	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		return nil
	}
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
	}

	// Return docker client
	inst.cli, _ = docker.NewClient(inst.DockerHostName(), dockerAPIVersion, client, nil)
	return inst.cli
}

func (inst *Instance) SwarmInit(certpath string) (addr string, err error) {
	if inst.cli == nil {
		inst.cli = inst.NewDockerClient(certpath)
	}
	clusterInit := swarm.InitRequest{
		ListenAddr:    "0.0.0.0",
		AdvertiseAddr: inst.AltHost[0],
		Spec: swarm.Spec{
			Orchestration: swarm.OrchestrationConfig{
				TaskHistoryRetentionLimit: func(i int64) *int64 { return &i }(1),
			},
		},
	}
	addr = clusterInit.AdvertiseAddr
	_, err = inst.cli.SwarmInit(context.Background(), clusterInit)
	return
}

func (inst *Instance) SwarmToken() (manager, worker string, err error) {
	if inst.cli == nil {
		err = fmt.Errorf("error/require-docker-client-setup")
		return
	}
	info, err := inst.cli.SwarmInspect(context.Background())
	if err != nil {
		return
	}
	manager, worker = info.JoinTokens.Manager, info.JoinTokens.Worker
	return
}

func (inst *Instance) SwarmJoin(token string, targets ...string) (err error) {
	if inst.cli == nil {
		err = fmt.Errorf("error/require-docker-client-setup")
		return
	}
	joinRequest := swarm.JoinRequest{
		ListenAddr:    "0.0.0.0",
		AdvertiseAddr: inst.AltHost[0],
		RemoteAddrs:   targets,
		JoinToken:     token,
	}
	err = inst.cli.SwarmJoin(context.Background(), joinRequest)
	return
}

type RegisteredInstances map[string]*Instance

var (
	// Instance roster
	InstList = make(RegisteredInstances)
)

func (r RegisteredInstances) Load() error {
	conf, err := getConfigPath()
	if err != nil {
		return err
	}
	origin, err := os.OpenFile(conf, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	if err = json.NewDecoder(origin).Decode(&r); err == io.EOF {
		return nil
	} else {
		return err
	}
}

func (r RegisteredInstances) Dump() error {
	conf, err := getConfigPath()
	if err != nil {
		return err
	}
	origin, err := os.OpenFile(conf, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer origin.Close()
	return json.NewEncoder(origin).Encode(r)
}

func getConfigPath() (string, error) {
	conf := strings.Replace(INSTANCE_LISTING_FILE, "~", os.Getenv("HOME"), 1)
	confdir := path.Dir(conf)
	if _, err := os.Stat(confdir); err != nil {
		if os.IsNotExist(err) {
			return conf, os.MkdirAll(confdir, 0700)
		} else {
			return "", err
		}
	} else {
		return conf, nil
	}
}
