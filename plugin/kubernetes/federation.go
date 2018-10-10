package kubernetes

import (
	"errors"
	"fmt"

	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
)

// The federation node.Labels keys used.
const (
	// TODO: Do not hardcode these labels. Pull them out of the API instead.
	//
	// We can get them via ....
	//   import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//     metav1.LabelZoneFailureDomain
	//     metav1.LabelZoneRegion
	//
	// But importing above breaks coredns with flag collision of 'log_dir'

	LabelZone   = "failure-domain.beta.kubernetes.io/zone"
	LabelRegion = "failure-domain.beta.kubernetes.io/region"
)

// Federations is used from the federations plugin to return the service that should be
// returned as a CNAME for federation(s) to work.
func (k *Kubernetes) Federations(state request.Request, fname, fzone string) (msg.Service, error) {
	nodeName := k.localNodeName()
	fmt.Printf("Node Name: %v\n", nodeName)
	node, err := k.APIConn.GetNodeByName(nodeName)
	if err != nil {
		fmt.Printf("Could not find node with name: %v, %v\n", nodeName, err)
		return msg.Service{}, err
	}
	r, err := parseRequest(state)
	if err != nil {
		fmt.Printf("Parse Error: %v\n", err)
		return msg.Service{}, err
	}

	lz := node.Labels[LabelZone]
	lr := node.Labels[LabelRegion]

	if lz == "" || lr == "" {
		fmt.Printf("Labels Missing: Zone=%v, Region=%v\n", lz, lr)
		return msg.Service{}, errors.New("local node missing zone/region labels")
	}

	if r.endpoint == "" {
		fmt.Printf("CNAME target = %v\n", dnsutil.Join(r.service, r.namespace, fname, r.podOrSvc, lz, lr, fzone))
		return msg.Service{Host: dnsutil.Join(r.service, r.namespace, fname, r.podOrSvc, lz, lr, fzone)}, nil
	}

	return msg.Service{Host: dnsutil.Join(r.endpoint, r.service, r.namespace, fname, r.podOrSvc, lz, lr, fzone)}, nil
}
