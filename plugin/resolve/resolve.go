package resolve

import (
	"context"
	"reflect"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// CNAMEResolve performs CNAME target resolution on all CNAMEs in the response
type Resolve struct {
	Next  plugin.Handler
	Zones []string
}

// Name implements the Handler interface.
func (c Resolve) Name() string { return name() }
func name() string                  { return "resolve" }

// ServeDNS implements the plugin.Handle interface.
func (c Resolve) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r, Context: ctx}

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
		if a.Header().Rrtype == dns.TypeCNAME {
			// Lookup CNAME targets by querying against the plugin chain, using another non-writer
			target := nonwriter.New(nw)
			r2 := r.Copy()
			r2.SetQuestion(a.(*dns.CNAME).Target, state.QType())
			rcode2, err := plugin.NextOrFailure(c.Name(), c.Next, ctx, target, r2)
			if err != nil || target.Msg == nil || !plugin.ClientWrite(rcode2) {
				continue
			}
			// Add answer to the answer section
			nw.Msg.Answer = addTarget(nw.Msg.Answer, target.Msg.Answer)
		}
		if a.Header().Rrtype == dns.TypeSRV {
			// Lookup SRV targets by querying against the plugin chain, using another non-writer
			target := nonwriter.New(nw)
			r2 := r.Copy()
			r2.SetQuestion(a.(*dns.SRV).Target, state.QType())
			rcode2, err := plugin.NextOrFailure(c.Name(), c.Next, ctx, target, r2)
			if err != nil || target.Msg == nil || !plugin.ClientWrite(rcode2) {
				continue
			}
			// Add answer to the extra/additional section
			nw.Msg.Extra = addTarget(nw.Msg.Extra, target.Msg.Answer)
		}

	}

	// Write the response to the client
	w.WriteMsg(nw.Msg)
	return rcode, err
}

// addTarget adds the answers from 'target' to the answers in 'clientResponse' ensuring that targets are not already in the
// client response (not creating duplicates)
func addTarget(clientRR, targetRR []dns.RR) []dns.RR {
	for _, t := range targetRR {
		unique := true
		for _, b := range clientRR {
			if rrDiff(t, b) {
				continue
			}
			unique = false
			break
		}
		if unique {
			clientRR = append(clientRR, t)
		}
	}
	return clientRR
}


// rrDiff returns true if the two dns.RR are different in name, type, or target
func rrDiff(a, b dns.RR) bool {
	if a.Header().Name != b.Header().Name {
		return true
	}
	if a.Header().Rrtype != b.Header().Rrtype {
		return true
	}
	if a.Header().Rrtype == dns.TypeA && !a.(*dns.A).A.Equal(b.(*dns.A).A) {
		return true
	}
	if a.Header().Rrtype == dns.TypeAAAA && !a.(*dns.AAAA).AAAA.Equal(b.(*dns.AAAA).AAAA) {
		return true
	}
	if a.Header().Rrtype == dns.TypeCNAME && a.(*dns.CNAME).Target != b.(*dns.CNAME).Target {
		return true
	}
	if a.Header().Rrtype == dns.TypeSRV && a.(*dns.SRV).Target != b.(*dns.SRV).Target {
		return true
	}
	if a.Header().Rrtype == dns.TypeMX && a.(*dns.MX).Mx != b.(*dns.MX).Mx {
		return true
	}
	if a.Header().Rrtype == dns.TypeTXT && !reflect.DeepEqual(a.(*dns.TXT).Txt, b.(*dns.TXT).Txt) {
		return true
	}
	// ... there's gotta be a better way to do this...
	return false
}
