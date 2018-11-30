package kubernetes

import (
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
)

// Reverse implements the ServiceBackend interface.
func (k *Kubernetes) Reverse(state request.Request, exact bool, opt plugin.Options) ([]msg.Service, error) {

	ip := dnsutil.ExtractAddressFromReverse(state.Name())
	if ip == "" {
		_, e := k.Records(state, exact)
		return nil, e
	}

	records := k.serviceRecordForIP(ip, state.Name())
	if len(records) == 0 {
		return records, errNoItems
	}
	return records, nil
}

// serviceRecordForIP gets a service record with a cluster ip matching the ip argument
// If a service cluster ip does not match, it checks all endpoints
func (k *Kubernetes) serviceRecordForIP(ip, name string) []msg.Service {
	// First check services with service ips (cluster or external ips)
	for _, service := range k.APIConn.SvcIndexReverse(ip) {
		if len(k.Namespaces) > 0 && !k.namespaceExposed(service.Namespace) {
			continue
		}

		if service.ClusterIP == ip {
			if k.opts.expose == exposeExternal {
				continue
			}
			domain := strings.Join([]string{service.Name, service.Namespace, Svc, k.primaryZone()}, ".")
			return []msg.Service{{Host: domain, TTL: k.ttl}}
		}

		if k.opts.expose == exposeCluster {
			continue
		}
		domain := strings.Join([]string{service.Name, service.Namespace, k.externalZones[0]}, ".")
		return []msg.Service{{Host: domain, TTL: k.ttl}}
	}
	// no service ips were found, and we are only exposing external records, so exit
	if k.opts.expose == exposeExternal {
		return nil
	}
	// No service ips match, and this is an internal query, search endpoints
	for _, ep := range k.APIConn.EpIndexReverse(ip) {
		if len(k.Namespaces) > 0 && !k.namespaceExposed(ep.Namespace) {
			continue
		}
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if addr.IP == ip {
					domain := strings.Join([]string{endpointHostname(addr, k.endpointNameMode), ep.Name, ep.Namespace, Svc, k.primaryZone()}, ".")
					return []msg.Service{{Host: domain, TTL: k.ttl}}
				}
			}
		}
	}
	return nil
}
