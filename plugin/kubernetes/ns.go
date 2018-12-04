package kubernetes

import (
	"net"
	"strings"

	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
)

func isDefaultNS(name, zone string) bool {
	return strings.Index(name, defaultNSName) == 0 && strings.Index(name, zone) == len(defaultNSName)
}

func (k *Kubernetes) nsAddr(external bool) []*dns.A {
	var (
		svcName      string
		svcNamespace string
	)

	rr := new(dns.A)
	localIP := k.interfaceAddrsFunc()
	rr.A = localIP

FindEndpoint:
	for _, ep := range k.APIConn.EpIndexReverse(localIP.String()) {
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if localIP.Equal(net.ParseIP(addr.IP)) {
					svcNamespace = ep.Namespace
					svcName = ep.Name
					break FindEndpoint
				}
			}
		}
	}

	if len(svcName) == 0 {
		rr.Hdr.Name = defaultNSName
		rr.A = localIP
		return []*dns.A{rr}
	}

	if !external {
		for _, svc := range k.APIConn.SvcIndex(object.ServiceKey(svcName, svcNamespace)) {
			if svc.ClusterIP == api.ClusterIPNone {
				// this should never happen because coredns should always have a static cluster ip
				rr.A = localIP
			} else {
				rr.A = net.ParseIP(svc.ClusterIP)
			}
			break
		}
		rr.Hdr.Name = strings.Join([]string{svcName, svcNamespace, "svc."}, ".")

		return []*dns.A{rr}
	}

	var nsARecs []*dns.A
	name := strings.Join([]string{svcName, "." , svcNamespace , "."}, "")

	for _, svc := range k.APIConn.SvcIndex(object.ServiceKey(svcName, svcNamespace)) {
		for _, ip := range svc.ExternalIPs {
			rr := new(dns.A)
			rr.A = net.ParseIP(ip)
			rr.Hdr.Name = name
			nsARecs = append(nsARecs, rr)
		}
		break
	}

	if len(nsARecs) == 0 {
		rr := new(dns.A)
		rr.A = nil
		rr.Hdr.Name = name
		return []*dns.A{rr}
	}
	return nsARecs
}

const defaultNSName = "ns.dns."