// Package upstream abstracts a upstream lookups so that plugins
// can handle them in an unified way.
package upstream

import (
	"errors"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/request"
)

// Upstream is used to resolve CNAME targets
type Upstream struct {
	self    bool
	Forward *proxy.Proxy
}

// NewUpstream creates a new Upstream for given destination(s)
func NewUpstream(dests []string) (Upstream, error) {
	u := Upstream{}
	if len(dests) == 0 {
		return u, errors.New("no upstreams")
	}
	if dests[0] != "@self" {
		u.self = false
		ups, err := dnsutil.ParseHostPortOrFile(dests...)
		if err != nil {
			return u, err
		}
		p := proxy.NewLookup(ups)
		u.Forward = &p
		return u, nil
	}
	if len(dests) > 1 {
		return u, errors.New("upstreams found after @self")
	}
	u.self = true
	return u, nil
}

// Lookup routes lookups to Self or Forward
func (u Upstream) Lookup(state request.Request, name string, typ uint16, opt plugin.Options) (*dns.Msg, error) {
	if u.self {
		// lookup via self
		req := new(dns.Msg)
		req.SetQuestion(name, typ)
		state.SizeAndDo(req)
		nw := nonwriter.New(state.W)
		state2 := request.Request{W: nw, Req: req}
		server := opt.Context.Value(dnsserver.ServerKey).(*dnsserver.Server)
		server.ServeDNS(opt.Context, state2.W, req)
		return nw.Msg, nil
	}
	if u.Forward != nil {
		return u.Forward.Lookup(state, name, typ)
	}
	return &dns.Msg{}, nil
}
