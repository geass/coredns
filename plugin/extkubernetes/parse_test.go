package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestParseRequest(t *testing.T) {
	tests := []struct {
		query    string
		expected string // output from r.String()
	}{
		// valid SRV request
		{"_http._tcp.webs.mynamespace.inter.webs.tests.", "http.tcp.webs.mynamespace"},
		// wildcard acceptance
		{"*.any.inter.webs.tests.", "*.*.*.any"},
		// bare zone
		{"inter.webs.tests.", "..."},
	}
	for i, tc := range tests {
		m := new(dns.Msg)
		m.SetQuestion(tc.query, dns.TypeA)
		state := request.Request{Zone: zone, Req: m}
		base, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		r, e := parseRequest(base)
		if e != nil {
			t.Errorf("Test %d, expected no error, got '%v'.", i, e)
		}
		rs := r.String()
		if rs != tc.expected {
			t.Errorf("Test %d, expected (stringyfied) recordRequest: %s, got %s", i, tc.expected, rs)
		}
	}
}

func TestParseInvalidRequest(t *testing.T) {
	invalid := []string{
		"too.long.for.what.I.am.trying.to.pod.inter.webs.tests.", // Too long.
	}

	for i, query := range invalid {
		m := new(dns.Msg)
		m.SetQuestion(query, dns.TypeA)
		state := request.Request{Zone: zone, Req: m}
		base, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		if _, e := parseRequest(base); e == nil {
			t.Errorf("Test %d: expected error from %s, got none", i, query)
		}
	}
}

const zone = "inter.webs.tests."
