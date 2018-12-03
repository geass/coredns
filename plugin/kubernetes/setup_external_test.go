package kubernetes

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestKubernetesParseExternal(t *testing.T) {
	tests := []struct {
		input            string   // Corefile data as string
		expectedExternal []string // expected count of defined zones.
		shouldErr        bool
	}{
		{`kubernetes example.com cluster.local {
			external example.com
		}`, []string{"example.com."}, false},
		{`kubernetes example.com another.domain.com cluster.local {
			external example.com another.domain.com
		}`, []string{"example.com.", "another.domain.com."}, false},
		{`kubernetes example.com cluster.local {
			external
		}`, []string{}, true},
	}

	for i, tc := range tests {
		c := caddy.NewTestController("dns", tc.input)
		k, err := kubernetesParse(c)
		if err != nil && !tc.shouldErr {
			t.Fatalf("Test %d: Expected no error, got %q", i, err)
		}
		if err == nil && tc.shouldErr {
			t.Fatalf("Test %d: Expected error, got none", i)
		}
		if err != nil && tc.shouldErr {
			// input should error
			continue
		}
		if len(k.externalZones) != len(tc.expectedExternal) {
			t.Errorf("Test %d: Expected external zones to be %v, got %v", i, tc.expectedExternal, k.externalZones)
		}
		for i := range k.externalZones {
			if k.externalZones[i] != tc.expectedExternal[i] {
				t.Errorf("Test %d: Expected external zones to be %v, got %v", i, tc.expectedExternal, k.externalZones)
			}
		}
	}
}
