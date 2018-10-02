package kubernetes

import (
	"sync"

	api "k8s.io/api/core/v1"
)

type endpointips struct {
	store map[string]endpoints
	mutex sync.Mutex
}

type endpoints map[string]map[string]bool

func NewEndpointIPs() *endpointips {
	epips := new(endpointips)
	epips.store = make(map[string]endpoints)
	return epips
}

func (dns *dnsControl) AddEndpoints(obj interface{}) {
	if dns.revIdxAllEndpoints {
		return
	}

	ep, ok := obj.(*api.Endpoints)
	if !ok {
		svc, isSvc := obj.(*api.Service)
		if !isSvc {
			return
		}
		if svc.Spec.ClusterIP != api.ClusterIPNone {
			o, exists, err := dns.epLister.GetByKey(metaNamespaceKey(svc.GetNamespace(), svc.GetName()))
			if err != nil {
				return
			}
			if !exists {
				return
			}
			ep, ok := o.(*api.Endpoints)
			if !ok {
				return
			}
			dns.DeleteEndpoints(ep)
		}
		return
	}
	o, exists, err := dns.svcLister.GetByKey(metaNamespaceKey(ep.GetNamespace(), ep.GetName()))

	if err != nil {
		return
	}
	if exists {
		svc, ok := o.(*api.Service)
		if !ok {
			return
		}
		if svc.Spec.ClusterIP != api.ClusterIPNone {
			return
		}
	}
	for _, s := range ep.Subsets {
		for _, a := range s.Addresses {
			dns.addEpToMap(a.IP, ep)
		}
	}
}

func (dns *dnsControl) DeleteEndpoints(obj interface{}) {
	if dns.revIdxAllEndpoints {
		return
	}
	ep, ok := obj.(*api.Endpoints)
	if !ok {
		return
	}
	o, exists, err := dns.svcLister.GetByKey(metaNamespaceKey(ep.GetNamespace(), ep.GetName()))
	if err != nil {
		return
	}
	if !exists {
		return
	}
	svc, ok := o.(*api.Service)
	if !ok {
		return
	}
	if svc.Spec.ClusterIP != api.ClusterIPNone {
		return
	}
	for ip := range dns.headlessEndpoints.store {
		dns.deleteIpFromMap(ip)
	}
}

func (dns *dnsControl) UpdateEndpoints(oldObj, newObj interface{}) {
	if dns.revIdxAllEndpoints {
		return
	}
	oldEp, ok := oldObj.(*api.Endpoints)
	newEp, fine := newObj.(*api.Endpoints)
	if !(ok && fine) {
		return
	}
	o, exists, err := dns.svcLister.GetByKey(metaNamespaceKey(oldEp.GetNamespace(), oldEp.GetName()))
	if err != nil {
		return
	}
	if !exists {
		return
	}
	svc, ok := o.(*api.Service)
	if !ok {
		return
	}
	if svc.Spec.ClusterIP != api.ClusterIPNone {
		return
	}
	for _, os := range oldEp.Subsets {
		for _, oa := range os.Addresses {
			found := false
		FindDelete:
			for _, ns := range newEp.Subsets {
				for _, na := range ns.Addresses {
					if oa.IP == na.IP {
						found = true
						break FindDelete
					}
				}
			}
			if !found {
				// remove item from map
				dns.deleteIpFromMap(oa.IP)
			}
		}
	}
	// Find new IPs (IPs in newEp, but not in oldEp)
	for _, ns := range newEp.Subsets {
		for _, na := range ns.Addresses {
			found := false
		FindAdd:
			for _, os := range oldEp.Subsets {
				for _, oa := range os.Addresses {
					if oa.IP == na.IP {
						found = true
						break FindAdd
					}
				}
			}
			if !found {
				// add item to map
				dns.addEpToMap(na.IP, newEp)
			}
		}
	}
}

func (dns *dnsControl) addEpToMap(ip string, ep *api.Endpoints) {
	namespace := ep.GetNamespace()
	name := ep.GetName()
	dns.headlessEndpoints.mutex.Lock()
	if dns.headlessEndpoints.store[ip] == nil {
		dns.headlessEndpoints.store[ip] = make(endpoints)
	}
	if dns.headlessEndpoints.store[ip][namespace] == nil {
		dns.headlessEndpoints.store[ip][namespace] = make(map[string]bool)
	}
	dns.headlessEndpoints.store[ip][namespace][name] = true
	dns.headlessEndpoints.mutex.Unlock()
}

func (dns *dnsControl) deleteIpFromMap(ip string) {
	dns.headlessEndpoints.mutex.Lock()
	delete(dns.headlessEndpoints.store, ip)
	dns.headlessEndpoints.mutex.Unlock()
}
