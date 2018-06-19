package cnameresolver

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// CNAMEResolve performs CNAME target resolution on all CNAMEs in the response
type CNAMEResolve struct {
	Next  plugin.Handler
	Zones []string
}

// ServeDNS implements the plugin.Handle interface.
func (c CNAMEResolve) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r, Context: ctx}

	if state.QType() != dns.TypeA && state.QType() != dns.TypeAAAA {
		return plugin.NextOrFailure(c.Name(), c.Next, ctx, w, r)
	}

	zone := plugin.Zones(c.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(c.Name(), c.Next, ctx, w, r)
	}

	// Run the query through the rest of the plugin chain using a non-writer
	nw := nonwriter.New(w)
	rcode, err := plugin.NextOrFailure(c.Name(), c.Next, ctx, nw, r)
	if err != nil || !plugin.ClientWrite(rcode) {
		return rcode, err
	}

	// Look at each answer and do lookups for any CNAME answers
	for i := 0; i < len(nw.Msg.Answer); i++ {
		a := nw.Msg.Answer[i]
		if a.Header().Rrtype != dns.TypeCNAME {
			continue
		}

		// Lookup CNAME targets by querying against the plugin chain, using another non-writer
		lookup := nonwriter.New(nw)
		r2 := r.Copy()
		r2.SetQuestion(a.(*dns.CNAME).Target, state.QType())
		rcode2, err := plugin.NextOrFailure(c.Name(), c.Next, ctx, lookup, r2)
		if err != nil || lookup.Msg == nil || !plugin.ClientWrite(rcode2) {
			continue
		}

		// Make sure targets are not already in the client response (dont create duplicates)
		unique := true
		for _, t := range lookup.Msg.Answer {
			for _, b := range nw.Msg.Answer {
				if t.Header().Name != b.Header().Name {
					continue
				}
				if t.Header().Rrtype != b.Header().Rrtype {
					continue
				}
				if t.Header().Rrtype == dns.TypeCNAME && t.(*dns.CNAME).Target != b.(*dns.CNAME).Target {
					continue
				}
				if t.Header().Rrtype == dns.TypeA && !t.(*dns.A).A.Equal(b.(*dns.A).A) {
					continue
				}
				if t.Header().Rrtype == dns.TypeAAAA && !t.(*dns.AAAA).AAAA.Equal(b.(*dns.AAAA).AAAA) {
					continue
				}
				unique = false
				break
			}
			if unique {
				nw.Msg.Answer = append(nw.Msg.Answer, t)
			}
		}
	}

	// Write the response to the client
	w.WriteMsg(nw.Msg)
	return rcode, err
}

// Name implements the Handler interface.
func (c CNAMEResolve) Name() string { return "cnameresolver" }
