package kubernetes

import (
	"context"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	k8spkg "github.com/coredns/coredns/plugin/pkg/kubernetes"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
)

const transferLength = 2000

// Serial implements the Transferer interface.
func (k *Kubernetes) Serial(state request.Request) uint32 { return uint32(k.APIConn.Modified()) }

// MinTTL implements the Transferer interface.
func (k *Kubernetes) MinTTL(state request.Request) uint32 { return 30 }

// Transfer implements the Transferer interface.
func (k *Kubernetes) Transfer(ctx context.Context, state request.Request) (int, error) {

	if !k8spkg.TransferAllowed(state.IP(), k.TransferTo) {
		return dns.RcodeRefused, nil
	}

	// Get all services.
	rrs := make(chan dns.RR)
	go k.transfer(rrs, state.Zone)

	records := []dns.RR{}
	for r := range rrs {
		records = append(records, r)
	}

	if len(records) == 0 {
		return dns.RcodeServerFailure, nil
	}

	ch := make(chan *dns.Envelope)
	tr := new(dns.Transfer)

	soa, err := plugin.SOA(k, state.Zone, state, plugin.Options{})
	if err != nil {
		return dns.RcodeServerFailure, nil
	}

	records = append(soa, records...)
	records = append(records, soa...)
	go func(ch chan *dns.Envelope) {
		j, l := 0, 0
		log.Infof("Outgoing transfer of %d records of zone %s to %s started", len(records), state.Zone, state.IP())
		for i, r := range records {
			l += dns.Len(r)
			if l > transferLength {
				ch <- &dns.Envelope{RR: records[j:i]}
				l = 0
				j = i
			}
		}
		if j < len(records) {
			ch <- &dns.Envelope{RR: records[j:]}
		}
		close(ch)
	}(ch)

	tr.Out(state.W, state.Req, ch)
	// Defer closing to the client
	state.W.Hijack()
	return dns.RcodeSuccess, nil
}

func (k *Kubernetes) transfer(c chan dns.RR, zone string) {

	defer close(c)

	zonePath := msg.Path(zone, "coredns")
	serviceList := k.APIConn.ServiceList()
	for _, svc := range serviceList {
		if !k.namespaceExposed(svc.Namespace) {
			continue
		}
		svcBase := []string{zonePath, svc.Namespace, svc.Name}
		switch svc.Type {
		case api.ServiceTypeClusterIP, api.ServiceTypeNodePort, api.ServiceTypeLoadBalancer:
			clusterIP := net.ParseIP(svc.ClusterIP)
			if clusterIP != nil {
				for _, p := range svc.Ports {
					for _, ip := range svc.ExternalIPs {
						s := msg.Service{Host: ip, Port: int(p.Port), TTL: k.ttl}
						s.Key = strings.Join(svcBase, "/")

						// Change host from IP to Name for SRV records
						host := k8spkg.EmitAddressRecord(c, s)
						s.Host = host

						// Need to generate this to handle use cases for peer-finder
						// ref: https://github.com/coredns/coredns/pull/823
						c <- s.NewSRV(msg.Domain(s.Key), 100)

						// As per spec unnamed ports do not have a srv record
						// https://github.com/kubernetes/dns/blob/master/docs/specification.md#232---srv-records
						if p.Name == "" {
							continue
						}

						s.Key = strings.Join(append(svcBase, strings.ToLower("_"+string(p.Protocol)), strings.ToLower("_"+string(p.Name))), "/")

						c <- s.NewSRV(msg.Domain(s.Key), 100)
					}
				}
			}
		}
	}
	return
}
