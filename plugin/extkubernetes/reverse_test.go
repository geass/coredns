package kubernetes

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/watch"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type APIConnReverseTest struct{}

func (APIConnReverseTest) HasSynced() bool                    { return true }
func (APIConnReverseTest) Run()                               { return }
func (APIConnReverseTest) Stop() error                        { return nil }
func (APIConnReverseTest) PodIndex(string) []*object.Pod      { return nil }
func (APIConnReverseTest) EpIndex(string) []*object.Endpoints { return nil }
func (APIConnReverseTest) EndpointsList() []*object.Endpoints { return nil }
func (APIConnReverseTest) EpIndexReverse(ip string) []*object.Endpoints {return nil}
func (APIConnReverseTest) ServiceList() []*object.Service     { return nil }
func (APIConnReverseTest) Modified() int64                    { return 0 }
func (APIConnReverseTest) SetWatchChan(watch.Chan)            {}
func (APIConnReverseTest) Watch(string) error                 { return nil }
func (APIConnReverseTest) StopWatching(string)                {}

func (APIConnReverseTest) SvcIndex(svc string) []*object.Service {
	if svc != "svc1.testns" {
		return nil
	}
	svcs := []*object.Service{
		{
			Name:      "svc1",
			Namespace: "testns",
			ClusterIP: "192.168.1.100",
			ExternalIPs: []string{"1.2.3.4"},
			Ports:     []api.ServicePort{{Name: "http", Protocol: "tcp", Port: 80}},
		},
	}
	return svcs

}

func (a APIConnReverseTest) SvcIndexReverse(ip string) []*object.Service {
	if ip == "1.2.3.4" {
		return a.SvcIndex("svc1.testns")
	}
	return nil
}

func (APIConnReverseTest) GetNodeByName(name string) (*api.Node, error) {
	return &api.Node{
		ObjectMeta: meta.ObjectMeta{
			Name: "test.node.foo.bar",
		},
	}, nil
}

func (APIConnReverseTest) GetNamespaceByName(name string) (*api.Namespace, error) {
	return &api.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
	}, nil
}

func TestReverse(t *testing.T) {

	k := New([]string{"example.com.", "2.1.in-addr.arpa."})
	k.APIConn = &APIConnReverseTest{}

	tests := []test.Case{
		{
			Qname: "4.3.2.1.in-addr.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Answer: []dns.RR{
				test.PTR("4.3.2.1.in-addr.arpa.     5     IN      PTR       svc1.testns.example.com."),
			},
		},
		{
			Qname: "5.3.2.1.in-addr.arpa.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeNameError,
			Ns: []dns.RR{
				test.SOA("2.1.in-addr.arpa.	300	IN	SOA	ns.dns.2.1.in-addr.arpa. hostmaster.2.1.in-addr.arpa. 1502782828 7200 1800 86400 60"),
			},
		},
		{
			Qname: "example.org.example.com.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeNameError,
			Ns: []dns.RR{
				test.SOA("example.com.       300     IN      SOA     ns.dns.example.com. hostmaster.example.com. 1502989566 7200 1800 86400 60"),
			},
		},
		{
			Qname: "svc1.testns.example.com.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeSuccess,
			Ns: []dns.RR{
				test.SOA("example.com.       300     IN      SOA     ns.dns.example.com. hostmaster.example.com. 1502989566 7200 1800 86400 60"),
			},
		},
		{
			Qname: "svc1.testns.2.1.in-addr.arpa.", Qtype: dns.TypeA,
			Rcode: dns.RcodeNameError,
			Ns: []dns.RR{
				test.SOA("2.1.in-addr.arpa.       300     IN      SOA     ns.dns.2.1.in-addr.arpa. hostmaster.2.1.in-addr.arpa. 1502989566 7200 1800 86400 60"),
			},
		},
		{
			Qname: "100.0.0.10.example.com.", Qtype: dns.TypePTR,
			Rcode: dns.RcodeNameError,
			Ns: []dns.RR{
				test.SOA("example.com.       300     IN      SOA     ns.dns.example.com. hostmaster.example.com. 1502989566 7200 1800 86400 60"),
			},
		},
	}

	ctx := context.TODO()
	for i, tc := range tests {
		r := tc.Msg()

		w := dnstest.NewRecorder(&test.ResponseWriter{})

		_, err := k.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d: expected no error, got %v", i, err)
			return
		}

		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d: got nil message and no error for: %s %d", i, r.Question[0].Name, r.Question[0].Qtype)
		}
		test.SortAndCheck(t, resp, tc)
	}
}
