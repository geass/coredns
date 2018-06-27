package test

import (
	"testing"

	"github.com/miekg/dns"
)

func TestResolveA(t *testing.T) {
	corefile := `.:0 {
        resolve .

 		# CNAME
		template IN ANY cname.test. {
			match ".*"
			answer "cname.test. 60 IN CNAME target.test."
		}

		# Target
		template IN ANY target.test. {
			match ".*"
			answer "target.test 60 IN A 1.2.3.4"
		}
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	// Test that an A query returns a CNAME and an A record
	m := new(dns.Msg)
	m.SetQuestion("cname.test.", dns.TypeA)
	m.SetEdns0(4096, true) // need this?

	r, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if len(r.Answer) < 2 {
		t.Fatalf("Expected 2 answers, got %v", len(r.Answer))
	}
	if x := r.Answer[0].(*dns.CNAME).Target; x != "target.test." {
		t.Fatalf("Expected CNAME target to be target.test. got %s", x)
	}
	if x := r.Answer[1].(*dns.A).A.String(); x != "1.2.3.4" {
		t.Fatalf("Incorrect A record for CNAME, expected 1.2.3.4 got %s", x)
	}

	// Test that a CNAME query only returns a CNAME
	m = new(dns.Msg)
	m.SetQuestion("cname.test.", dns.TypeCNAME)
	m.SetEdns0(4096, true) // need this?

	r, err = dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if len(r.Answer) != 1 {
		t.Fatalf("Expected 1 answer, got %v", len(r.Answer))
	}
	if x := r.Answer[0].(*dns.CNAME).Target; x != "target.test." {
		t.Fatalf("Expected CNAME target to be target.test. got %s", x)
	}
}

func TestResolveCNAMELoop(t *testing.T) {
	corefile := `.:0 {
        resolve .

 		# CNAME
		template IN ANY cname.test. {
			match ".*"
			answer "cname.test. 60 IN CNAME target.test."
		}

		# Target
		template IN ANY target.test. {
			match ".*"
			answer "target.test. 60 IN CNAME cname.test."
		}
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	// Test that an A query returns a CNAME and an A record
	m := new(dns.Msg)
	m.SetQuestion("test.", dns.TypeA)
	m.SetEdns0(4096, true) // need this?

	r, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if len(r.Answer) != 2 {
		t.Fatalf("Expected 2 answers, got %v", len(r.Answer))
	}
	if x := r.Answer[0].(*dns.CNAME).Target; x != "target.test." {
		t.Fatalf("Expected first CNAME target to be target.test. got %s", x)
	}
	if x := r.Answer[1].(*dns.CNAME).Target; x != "cname.test." {
		t.Fatalf("Expected second CNAME target to be cname.test. got %s", x)
	}
}

func TestResolveSRV(t *testing.T) {
	corefile := `.:0 {
        resolve .

 		# CNAME
		template IN ANY cname.test. {
			match ".*"
			answer "cname.test. 60 IN CNAME srv.test."
		}

		# SRV
		template IN SRV srv.test. {
			match ".*"
			answer "_http._tcp.srv.test. 3600 IN SRV 0 0 80  a.test."
		}

		# A
		template IN A a.test. {
			match ".*"
			answer "a.test. 60 IN A 1.2.3.4"
		}

}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	// Test that an A query returns a CNAME and an A record
	m := new(dns.Msg)
	m.SetQuestion("cname.test.", dns.TypeA)
	m.SetEdns0(4096, true) // need this?

	r, err := dns.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send msg: %s", err)
	}
	if r.Rcode == dns.RcodeServerFailure {
		t.Fatalf("Rcode should not be dns.RcodeServerFailure")
	}
	if len(r.Answer) != 2 {
		t.Fatalf("Expected 2 answers, got %v", len(r.Answer))
	}
	if x := r.Answer[0].(*dns.CNAME).Target; x != "srv.test." {
		t.Fatalf("Expected CNAME target to be srv.test. got %s", x)
	}
	if x := r.Answer[1].(*dns.SRV).Target; x != "a.test." {
		t.Fatalf("Expected SRV target to be a.test. got %s", x)
	}
	if len(r.Extra) != 1 {
		t.Fatalf("Expected 1 extra recrod, got %v", len(r.Answer))
	}
	if x := r.Extra[0].(*dns.A).A.String(); x != "1.2.3.4" {
		t.Fatalf("Incorrect extra record for SRV, expected 1.2.3.4 got %s", x)
	}

}
