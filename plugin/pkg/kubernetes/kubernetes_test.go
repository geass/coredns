package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/kubernetes/object"
)

func TestServiceFQDN(t *testing.T) {
	fqdn := ServiceFQDN(
		&object.Service{
			Name:      "svc1",
			Namespace: "testns",
		}, "cluster.local")

	expected := "svc1.testns.svc.cluster.local."
	if fqdn != expected {
		t.Errorf("Expected '%v', got '%v'.", expected, fqdn)
	}
}

func TestPodFQDN(t *testing.T) {
	fqdn := PodFQDN(
		&object.Pod{
			Name:      "pod1",
			Namespace: "testns",
			PodIP:     "10.10.0.10",
		}, "cluster.local")

	expected := "10-10-0-10.testns.pod.cluster.local."
	if fqdn != expected {
		t.Errorf("Expected '%v', got '%v'.", expected, fqdn)
	}
	fqdn = PodFQDN(
		&object.Pod{
			Name:      "pod1",
			Namespace: "testns",
			PodIP:     "aaaa:bbbb:cccc::zzzz",
		}, "cluster.local")

	expected = "aaaa-bbbb-cccc--zzzz.testns.pod.cluster.local."
	if fqdn != expected {
		t.Errorf("Expected '%v', got '%v'.", expected, fqdn)
	}
}

func TestEndpointFQDN(t *testing.T) {
	fqdns := EndpointFQDN(
		&object.Endpoints{
			Subsets: []object.EndpointSubset{
				{
					Addresses: []object.EndpointAddress{
						{
							IP:       "172.0.0.1",
							Hostname: "ep1a",
						},
						{
							IP: "172.0.0.2",
						},
					},
				},
			},
			Name:      "svc1",
			Namespace: "testns",
		}, "cluster.local", false)

	expected := []string{
		"ep1a.svc1.testns.svc.cluster.local.",
		"172-0-0-2.svc1.testns.svc.cluster.local.",
	}

	for i := range fqdns {
		if fqdns[i] != expected[i] {
			t.Errorf("Expected '%v', got '%v'.", expected[i], fqdns[i])
		}
	}
}

func TestEndpointHostname(t *testing.T) {
	var tests = []struct {
		ip               string
		hostname         string
		expected         string
		podName          string
		endpointNameMode bool
	}{
		{"10.11.12.13", "", "10-11-12-13", "", false},
		{"10.11.12.13", "epname", "epname", "", false},
		{"10.11.12.13", "", "10-11-12-13", "hello-abcde", false},
		{"10.11.12.13", "epname", "epname", "hello-abcde", false},
		{"10.11.12.13", "epname", "epname", "hello-abcde", true},
		{"10.11.12.13", "", "hello-abcde", "hello-abcde", true},
	}
	for _, test := range tests {
		result := EndpointHostname(object.EndpointAddress{IP: test.ip, Hostname: test.hostname, TargetRefName: test.podName}, test.endpointNameMode)
		if result != test.expected {
			t.Errorf("Expected endpoint name for (ip:%v hostname:%v) to be '%v', but got '%v'", test.ip, test.hostname, test.expected, result)
		}
	}
}