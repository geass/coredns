package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/watch"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var dnsTestCases = []test.Case{
	// A Service
	{
		Qname: "svc1.testns.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.testns.example.com.	5	IN	A	1.2.3.4"),
		},
	},
	// A Service (wildcard)
	{
		Qname: "svc1.*.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.*.example.com.  5       IN      A       1.2.3.4"),
		},
	},
	{
		Qname: "svc1.testns.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("svc1.testns.example.com.	5	IN	SRV	0 100 80 svc1.testns.example.com.")},
		Extra: []dns.RR{test.A("svc1.testns.example.com.  5       IN      A       1.2.3.4")},
	},
	// SRV Service (wildcard)
	{
		Qname: "svc1.*.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("svc1.*.example.com.	5	IN	SRV	0 100 80 svc1.testns.example.com.")},
		Extra: []dns.RR{test.A("svc1.testns.example.com.  5       IN      A       1.2.3.4")},
	},
	// SRV Service (wildcards)
	{
		Qname: "*.any.svc1.*.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{test.SRV("*.any.svc1.*.example.com.	5	IN	SRV	0 100 80 svc1.testns.example.com.")},
		Extra: []dns.RR{test.A("svc1.testns.example.com.  5       IN      A       1.2.3.4")},
	},
	// A Service (wildcards)
	{
		Qname: "*.any.svc1.*.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("*.any.svc1.*.example.com.  5       IN      A       1.2.3.4"),
		},
	},
	// SRV Service Not udp/tcp
	{
		Qname: "*._not-udp-or-tcp.svc1.testns.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("example.com.	30	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
	// SRV Service
	{
		Qname: "_http._tcp.svc1.testns.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc1.testns.example.com.	5	IN	SRV	0 100 80 svc1.testns.example.com."),
		},
		Extra: []dns.RR{
			test.A("svc1.testns.example.com.	5	IN	A	1.2.3.4"),
		},
	},
	// AAAA Service (with an existing A record, but no AAAA record)
	{
		Qname: "svc1.testns.example.com.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	30	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
	// AAAA Service (non-existing service)
	{
		Qname: "svc0.testns.example.com.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("example.com.	30	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
	// A Service (non-existing service)
	{
		Qname: "svc0.testns.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("example.com.	30	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
	// A Service (non-existing namespace)
	{
		Qname: "svc0.svc-nons.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			test.SOA("example.com.	30	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
	// AAAA Service
	{
		Qname: "svc6.testns.example.com.", Qtype: dns.TypeAAAA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.AAAA("svc6.testns.example.com.	5	IN	AAAA	1:2::5"),
		},
	},
	// SRV
	{
		Qname: "_http._tcp.svc6.testns.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("_http._tcp.svc6.testns.example.com.	5	IN	SRV	0 100 80 svc6.testns.example.com."),
		},
		Extra: []dns.RR{
			test.AAAA("svc6.testns.example.com.	5	IN	AAAA	1:2::5"),
		},
	},
	// SRV
	{
		Qname: "svc6.testns.example.com.", Qtype: dns.TypeSRV,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.SRV("svc6.testns.example.com.	5	IN	SRV	0 100 80 svc6.testns.example.com."),
		},
		Extra: []dns.RR{
			test.AAAA("svc6.testns.example.com.	5	IN	AAAA	1:2::5"),
		},
	},
	{
		Qname: "example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	303	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
	{
		Qname: "testns.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Ns: []dns.RR{
			test.SOA("example.com.	303	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
}

func TestServeDNS(t *testing.T) {
	k := New([]string{"example.com."})
	k.APIConn = &APIConnServeTest{}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	k.Namespaces = map[string]bool{"testns": true}
	ctx := context.TODO()

	for i, tc := range dnsTestCases {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d expected no error, got %v", i, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d, got nil message and no error for %q", i, r.Question[0].Name)
		}

		// Before sorting, make sure that CNAMES do not appear after their target records
		test.CNAMEOrder(t, resp)

		test.SortAndCheck(t, resp, tc)
	}
}

var notSyncedTestCases = []test.Case{
	{
		// We should get ServerFailure instead of NameError for missing records when we kubernetes hasn't synced
		Qname: "svc0.testns.example.com.", Qtype: dns.TypeA,
		Rcode: dns.RcodeServerFailure,
		Ns: []dns.RR{
			test.SOA("example.com.	303	IN	SOA	ns.dns.example.com. hostmaster.example.com. 1499347823 7200 1800 86400 60"),
		},
	},
}

func TestNotSyncedServeDNS(t *testing.T) {

	k := New([]string{"example.com."})
	k.APIConn = &APIConnServeTest{
		notSynced: true,
	}
	k.Next = test.NextHandler(dns.RcodeSuccess, nil)
	k.Namespaces = map[string]bool{"testns": true}
	ctx := context.TODO()

	for i, tc := range notSyncedTestCases {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d expected no error, got %v", i, err)
			return
		}
		if tc.Error != nil {
			continue
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d, got nil message and no error for %q", i, r.Question[0].Name)
		}

		// Before sorting, make sure that CNAMES do not appear after their target records
		test.CNAMEOrder(t, resp)

		test.SortAndCheck(t, resp, tc)
	}
}

type APIConnServeTest struct {
	notSynced bool
}

func (a APIConnServeTest) HasSynced() bool                            { return !a.notSynced }
func (APIConnServeTest) Run()                                         { return }
func (APIConnServeTest) Stop() error                                  { return nil }
func (APIConnServeTest) EpIndexReverse(string) []*object.Endpoints    { return nil }
func (APIConnServeTest) SvcIndexReverse(string) []*object.Service     { return nil }
func (APIConnServeTest) Modified() int64                              { return time.Now().Unix() }
func (APIConnServeTest) SetWatchChan(watch.Chan)                      {}
func (APIConnServeTest) Watch(string) error                           { return nil }
func (APIConnServeTest) StopWatching(string)                          {}
func (APIConnServeTest) EpIndex(s string) []*object.Endpoints         { return nil }
func (APIConnServeTest) EndpointsList() []*object.Endpoints           { return nil }
func (APIConnServeTest) GetNodeByName(name string) (*api.Node, error) { return nil, nil }
func (APIConnServeTest) SvcIndex(s string) []*object.Service          { return svcIndex[s] }

func (APIConnServeTest) GetNamespaceByName(name string) (*api.Namespace, error) {
	if name == "pod-nons" { // handler_pod_verified_test.go uses this for non-existent namespace.
		return &api.Namespace{}, nil
	}
	return &api.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
	}, nil
}

func (APIConnServeTest) PodIndex(string) []*object.Pod {
	a := []*object.Pod{
		{Namespace: "podns", PodIP: "10.240.0.1"}, // Remote IP set in test.ResponseWriter
	}
	return a
}

var svcIndex = map[string][]*object.Service{
	"svc1.testns": {
		{
			Name:        "svc1",
			Namespace:   "testns",
			Type:        api.ServiceTypeClusterIP,
			ClusterIP:   "10.0.0.1",
			ExternalIPs: []string{"1.2.3.4"},
			Ports: []api.ServicePort{
				{Name: "http", Protocol: "tcp", Port: 80},
			},
		},
	},
	"svc6.testns": {
		{
			Name:        "svc6",
			Namespace:   "testns",
			Type:        api.ServiceTypeClusterIP,
			ClusterIP:   "10.0.0.3",
			ExternalIPs: []string{"1:2::5"},
			Ports: []api.ServicePort{
				{Name: "http", Protocol: "tcp", Port: 80},
			},
		},
	},
}

func (APIConnServeTest) ServiceList() []*object.Service {
	var svcs []*object.Service
	for _, svc := range svcIndex {
		svcs = append(svcs, svc...)
	}
	return svcs
}
