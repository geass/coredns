// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	k8spkg "github.com/coredns/coredns/plugin/pkg/kubernetes"
	"github.com/coredns/coredns/plugin/pkg/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Kubernetes implements a plugin that connects to a Kubernetes cluster.
type Kubernetes struct {
	Next          plugin.Handler
	Zones         []string
	Upstream      upstream.Upstream
	APIServerList []string
	APIProxy      *apiProxy
	APICertAuth   string
	APIClientCert string
	APIClientKey  string
	ClientConfig  clientcmd.ClientConfig
	APIConn       k8spkg.DNSController
	Namespaces    map[string]bool
	Fall          fall.F
	ttl           uint32
	opts          k8spkg.DNSControlOpts

	primaryZoneIndex int
	TransferTo       []string
}

// New returns a initialized Kubernetes. It default interfaceAddrFunc to return 127.0.0.1. All other
// values default to their zero value, primaryZoneIndex will thus point to the first zone.
func New(zones []string) *Kubernetes {
	k := new(Kubernetes)
	k.Zones = zones
	k.Namespaces = make(map[string]bool)
	k.ttl = defaultTTL

	return k
}

const (
	// Svc is the DNS schema for kubernetes services
	Svc = "svc"
	// defaultTTL to apply to all answers.
	defaultTTL = 5
)

var (
	errNoItems        = errors.New("no items found")
	errNsNotExposed   = errors.New("namespace is not exposed")
	errInvalidRequest = errors.New("invalid query name")
)

// Services implements the ServiceBackend interface.
func (k *Kubernetes) Services(state request.Request, exact bool, opt plugin.Options) (svcs []msg.Service, err error) {
	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {

	case dns.TypeTXT:
		return []msg.Service{}, nil

	case dns.TypeNS:
		// We can only get here if the qname equals the zone, see ServeDNS in handler.go.
		svc := msg.Service{Host: state.LocalIP(), Key: msg.Path(state.QName(), "coredns")}
		return []msg.Service{svc}, nil
	}

	s, e := k.Records(state, false)

	return s, e
}

// primaryZone will return the first non-reverse zone being handled by this plugin
func (k *Kubernetes) primaryZone() string { return k.Zones[k.primaryZoneIndex] }

// Lookup implements the ServiceBackend interface.
func (k *Kubernetes) Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Upstream.Lookup(state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (k *Kubernetes) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest
}

func (k *Kubernetes) getClientConfig() (*rest.Config, error) {
	if k.ClientConfig != nil {
		return k.ClientConfig.ClientConfig()
	}
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}

	// Connect to API from in cluster
	if len(k.APIServerList) == 0 {
		cc, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		cc.ContentType = "application/vnd.kubernetes.protobuf"
		return cc, err
	}

	// Connect to API from out of cluster
	endpoint := k.APIServerList[0]
	if len(k.APIServerList) > 1 {
		// Use a random port for api proxy, will get the value later through listener.Addr()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes api proxy: %v", err)
		}
		k.APIProxy = &apiProxy{
			listener: listener,
			handler: proxyHandler{
				HealthCheck: healthcheck.HealthCheck{
					FailTimeout: 3 * time.Second,
					MaxFails:    1,
					Path:        "/",
					Interval:    5 * time.Second,
				},
			},
		}
		k.APIProxy.handler.Hosts = make([]*healthcheck.UpstreamHost, len(k.APIServerList))
		for i, entry := range k.APIServerList {

			uh := &healthcheck.UpstreamHost{
				Name: strings.TrimPrefix(entry, "http://"),

				CheckDown: func(upstream *proxyHandler) healthcheck.UpstreamHostDownFunc {
					return func(uh *healthcheck.UpstreamHost) bool {

						fails := atomic.LoadInt32(&uh.Fails)
						if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
							return true
						}
						return false
					}
				}(&k.APIProxy.handler),
			}

			k.APIProxy.handler.Hosts[i] = uh
		}
		k.APIProxy.Handler = &k.APIProxy.handler

		// Find the random port used for api proxy
		endpoint = fmt.Sprintf("http://%s", listener.Addr())
	}
	clusterinfo.Server = endpoint

	if len(k.APICertAuth) > 0 {
		clusterinfo.CertificateAuthority = k.APICertAuth
	}
	if len(k.APIClientCert) > 0 {
		authinfo.ClientCertificate = k.APIClientCert
	}
	if len(k.APIClientKey) > 0 {
		authinfo.ClientKey = k.APIClientKey
	}

	overrides.ClusterInfo = clusterinfo
	overrides.AuthInfo = authinfo
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	cc, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cc.ContentType = "application/vnd.kubernetes.protobuf"
	return cc, err

}

// InitKubeCache initializes a new Kubernetes cache.
func (k *Kubernetes) InitKubeCache() (err error) {
	config, err := k.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes notification controller: %q", err)
	}

	if k.opts.LabelSelector != nil {
		var selector labels.Selector
		selector, err = meta.LabelSelectorAsSelector(k.opts.LabelSelector)
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", k.opts.LabelSelector, err)
		}
		k.opts.Selector = selector
	}

	k.opts.InitPodCache = false
	k.opts.InitEndpointsCache = false
	k.opts.Zones = k.Zones
	k.opts.EndpointNameMode = false
	k.opts.ExposeExternalIPs = true
	k.APIConn = k8spkg.NewDNSController(kubeClient, k.opts)

	return err
}

// Records looks up services in kubernetes.
func (k *Kubernetes) Records(state request.Request, exact bool) ([]msg.Service, error) {
	base, _ := dnsutil.TrimZone(state.Name(), state.Zone)
	if base == ""{
		return nil, nil
	}

	r, e := parseRequest(base)
	if e != nil {
		return nil, e
	}

	if dnsutil.IsReverse(state.Name()) > 0 {
		return nil, errNoItems
	}

	if !wildcard(r.namespace) && !k.namespaceExposed(r.namespace) {
		return nil, errNsNotExposed
	}

	services, err := k.findServices(r, state.Zone)
	return services, err
}

// serviceFQDN returns the k8s cluster dns spec service FQDN for the service (or endpoint) object.
func serviceFQDN(obj meta.Object, zone string) string {
	return dnsutil.Join(obj.GetName(), obj.GetNamespace(), Svc, zone)
}

// findServices returns the services matching r from the cache.
func (k *Kubernetes) findServices(r recordRequest, zone string) (services []msg.Service, err error) {
	zonePath := msg.Path(zone, "coredns")

	err = errNoItems
	if wildcard(r.service) && !wildcard(r.namespace) {
		// If namespace exist, err should be nil, so that we return nodata instead of NXDOMAIN
		if k.namespace(r.namespace) {
			err = nil
		}
	}

	var serviceList []*object.Service

	// handle empty service name
	if r.service == "" {
		if k.namespace(r.namespace) || wildcard(r.namespace) {
			// NODATA
			return nil, nil
		}
		// NXDOMAIN
		return nil, errNoItems
	}

	if wildcard(r.service) || wildcard(r.namespace) {
		serviceList = k.APIConn.ServiceList()
	} else {
		idx := object.ServiceKey(r.service, r.namespace)
		serviceList = k.APIConn.SvcIndex(idx)
	}

	for _, svc := range serviceList {
		if !(match(r.namespace, svc.Namespace) && match(r.service, svc.Name)) {
			continue
		}

		// If namespace has a wildcard, filter results against Corefile namespace list.
		// (Namespaces without a wildcard were filtered before the call to this function.)
		if wildcard(r.namespace) && !k.namespaceExposed(svc.Namespace) {
			continue
		}

		// ClusterIP service
		for _, p := range svc.Ports {
			if !(match(r.port, p.Name) && match(r.protocol, string(p.Protocol))) {
				continue
			}

			err = nil

			for _, ip := range svc.ExternalIPs {
				s := msg.Service{Host: ip, Port: int(p.Port), TTL: k.ttl}
				s.Key = strings.Join([]string{zonePath, svc.Namespace, svc.Name}, "/")

				services = append(services, s)
			}
		}
	}
	return services, err
}

// match checks if a and b are equal taking wildcards into account.
func match(a, b string) bool {
	if wildcard(a) {
		return true
	}
	if wildcard(b) {
		return true
	}
	return strings.EqualFold(a, b)
}

// wildcard checks whether s contains a wildcard value defined as "*" or "any".
func wildcard(s string) bool {
	return s == "*" || s == "any"
}
