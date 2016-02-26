package docker

import (
	"bytes"
	"encoding/json"
)

type DaemonConfig struct {
	ApiCorsHeaders       []string          `json:"api-cors-headers,omitempty"`
	AuthorizationPlugins []string          `json:"authorization-plugins,omitempty"`
	Bip                  string            `json:"bip,omitempty"`
	Bridge               string            `json:"bridge,omitempty"`
	CgroupParent         string            `json:"cgroup-parent,omitempty"`
	ClusterAdvertise     string            `json:"cluster-advertise,omitempty"`
	ClusterStore         string            `json:"cluster-store,omitempty"`
	CLusterStoreOpts     []string          `json:"cluster-store-opts,omitempty"`
	Debug                bool              `json:"debug,omitempty"`
	DefaultGateway       string            `json:"default-gateway,omitempty"`
	DefaultGatewayV6     string            `json:"default-gateway-v6,omitempty"`
	DefaultUlimits       map[string]string `json:"default-ulimits,omitempty"`
	Dns                  []string          `json:"dns,omitempty"`
	DnsOpts              []string          `json:"dns-opts,omitempty"`
	DnsSearch            []string          `json:"dns-search,omitempty"`
	ExecOpts             []string          `json:"exec-opts,omitempty"`
	ExecRoot             string            `json:"exec-root,omitempty"`
	FixedCidr            string            `json:"fixed-cidr,omitempty"`
	FixedCidrV6          string            `json:"fixed-cidr-v6,omitempty"`
	Graph                string            `json:"graph,omitempty"`
	Group                string            `json:"group,omitempty"`
	Hosts                []string          `json:"hosts,omitempty"`
	Icc                  bool              `json:"icc,omitempty"`
	Ip                   string            `json:"ip,omitempty"`
	IpForward            bool              `json:"ip-forward,omitempty"`
	IpMask               bool              `json:"ip-mask,omitempty"`
	Iptables             bool              `json:"iptables,omitempty"`
	IpV6                 bool              `json:"ipv6,omitempty"`
	Labels               []string          `json:"labels,omitempty"`
	LogDriver            string            `json:"log-driver,omitempty"`
	LogLevel             string            `json:"log-level,omitempty"`
	LogOpts              []string          `json:"log-opts,omitempty"`
	Mtu                  int               `json:"mtu,omitempty"`
	Pidfile              string            `json:"pidfile,omitempty"`
	SelinuxEnabled       bool              `json:"selinux-enabled,omitempty"`
	StorageDriver        string            `json:"storage-driver,omitempty"`
	StorageOpts          []string          `json:"storage-opts,omitempty"`
	Tls                  bool              `json:"tls,omitempty"`
	TlsCACert            string            `json:"tlscacert,omitempty"`
	TlsCert              string            `json:"tlscert,omitempty"`
	TlsKey               string            `json:"tlskey,omitempty"`
	TlsVerify            bool              `json:"tlsverify,omitempty"`
	UserlandProxy        bool              `json:"userland-proxy,omitempty"`
	UsernsRemap          string            `json:"userns-remap,omitempty"`
}

func (d *DaemonConfig) Reader() (*bytes.Buffer, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(data), nil
}

func (d *DaemonConfig) AddHost(host ...string) {
	for _, h := range host {
		var exist = false
		for _, oh := range d.Hosts {
			exist = exist || (oh == h)
		}
		if !exist {
			d.Hosts = append(d.Hosts, h)
		}
	}
}

func LoadDaemonConfig(data []byte) (*DaemonConfig, error) {
	cfg := new(DaemonConfig)
	err := json.Unmarshal(data, cfg)
	return cfg, err
}
