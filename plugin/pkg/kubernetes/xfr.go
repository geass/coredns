package kubernetes

import (
	"net"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/miekg/dns"
)

// EmitAddressRecord generates a new A or AAAA record based on the msg.Service and writes it to
// a channel.
// emitAddressRecord returns the host name from the generated record.
func EmitAddressRecord(c chan dns.RR, message msg.Service) string {
	ip := net.ParseIP(message.Host)
	var host string
	dnsType, _ := message.HostType()
	switch dnsType {
	case dns.TypeA:
		arec := message.NewA(msg.Domain(message.Key), ip)
		host = arec.Hdr.Name
		c <- arec
	case dns.TypeAAAA:
		arec := message.NewAAAA(msg.Domain(message.Key), ip)
		host = arec.Hdr.Name
		c <- arec
	}
	return host
}

// TransferAllowed checks if incoming request for transferring the zone is allowed according to the ACLs.
// Note: This is copied from zone.transferAllowed, but should eventually be factored into a common transfer pkg.
func TransferAllowed(ip string, transferTo []string) bool {
	for _, t := range transferTo {
		if t == "*" {
			return true
		}
		// If remote IP matches we accept.
		remote := ip
		to, _, err := net.SplitHostPort(t)
		if err != nil {
			continue
		}
		if to == remote {
			return true
		}
	}
	return false
}