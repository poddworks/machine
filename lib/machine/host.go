package machine

type IpAddr struct {
	Pub  string
	Priv string
}

type Hosts struct {
	IpAddrs []IpAddr
}
