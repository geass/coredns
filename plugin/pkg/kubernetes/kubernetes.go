package kubernetes

import (
	"strings"

	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/kubernetes/object"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const(
	// Svc is the DNS schema for kubernetes services
	Svc = "svc"
	// Pod is the DNS schema for kubernetes pods
	Pod = "pod"
)

// ServiceFQDN returns the k8s cluster dns spec service FQDN for the service (or endpoint) object.
func ServiceFQDN(obj meta.Object, zone string) string {
	return dnsutil.Join(obj.GetName(), obj.GetNamespace(), Svc, zone)
}

// PodFQDN returns the k8s cluster dns spec FQDN for the pod.
func PodFQDN(p *object.Pod, zone string) string {
	if strings.Contains(p.PodIP, ".") {
		name := strings.Replace(p.PodIP, ".", "-", -1)
		return dnsutil.Join(name, p.GetNamespace(), Pod, zone)
	}

	name := strings.Replace(p.PodIP, ":", "-", -1)
	return dnsutil.Join(name, p.GetNamespace(), Pod, zone)
}

// EndpointFQDN returns a list of k8s cluster dns spec service FQDNs for each subset in the endpoint.
func EndpointFQDN(ep *object.Endpoints, zone string, endpointNameMode bool) []string {
	var names []string
	for _, ss := range ep.Subsets {
		for _, addr := range ss.Addresses {
			names = append(names, dnsutil.Join(EndpointHostname(addr, endpointNameMode), ServiceFQDN(ep, zone)))
		}
	}
	return names
}

// EndpointHostname constructs the hostname of an endpoint address
func EndpointHostname(addr object.EndpointAddress, endpointNameMode bool) string {
	if addr.Hostname != "" {
		return addr.Hostname
	}
	if endpointNameMode && addr.TargetRefName != "" {
		return addr.TargetRefName
	}
	if strings.Contains(addr.IP, ".") {
		return strings.Replace(addr.IP, ".", "-", -1)
	}
	if strings.Contains(addr.IP, ":") {
		return strings.Replace(addr.IP, ":", "-", -1)
	}
	return ""
}