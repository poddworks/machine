package machine

import (
	"net"
)

func LookupSRV(service, proto, zone string) (targets []*net.SRV, err error) {
	_, targets, err = net.LookupSRV(service, proto, zone)
	return
}
