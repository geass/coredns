package kubernetes

import (
	"github.com/miekg/dns"
)

type recordRequest struct {
	// The named port from the kubernetes DNS spec, this is the service part (think _https) from a well formed SRV record.
	port string
	// The protocol is usually _udp or _tcp (if set), and comes from the protocol part of a well formed SRV record.
	protocol string
	// The service name used in Kubernetes.
	service string
	// The namespace used in Kubernetes.
	namespace string
}

// parseRequest parses the qname to find all the elements we need for querying k8s. Anything
// that is not parsed will have the wildcard "*" value.
// Potential underscores are stripped from _port and _protocol.
func parseRequest(base string) (r recordRequest, err error) {
	// 2 Possible cases:
	//   * _port._protocol.service.namespace.zone
	//   * (service): service.namespace.zone
	//

	// return NODATA for apex queries
	if base == "" {
		return r, nil
	}
	segs := dns.SplitDomainName(base)

	// port and protocol default to wildcard behavior
	r.port = "*"
	r.protocol = "*"

	// start at the right and fill out recordRequest with the bits we find, so we look for
	// namespace.service and then _protocol._port

	last := len(segs) - 1
	if last < 0 {
		return r, nil
	}

	r.namespace = segs[last]
	last--
	if last < 0 {
		return r, nil
	}

	r.service = segs[last]
	last--
	if last < 0 {
		return r, nil
	}

	if last != 1 {
		return r, errInvalidRequest
	}

	r.protocol = stripUnderscore(segs[last])
	r.port = stripUnderscore(segs[last-1])

	return r, nil
}

// stripUnderscore removes a prefixed underscore from s.
func stripUnderscore(s string) string {
	if s[0] != '_' {
		return s
	}
	return s[1:]
}

// String return a string representation of r, it just returns all fields concatenated with dots.
// This is mostly used in tests.
func (r recordRequest) String() string {
	s := r.port
	s += "." + r.protocol
	s += "." + r.service
	s += "." + r.namespace
	return s
}
