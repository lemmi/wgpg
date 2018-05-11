package wgpg

import (
	"bytes"
	"testing"
)

var (
	validaddresses = [...][5]string{
		{
			"0.0.0.0/0",
			"0.0.0.0/0",
			"0.0.0.0/0",
			"255.255.255.255/0",
			"0.0.0.0/32",
		}, {
			"1.2.3.4/8",
			"1.0.0.0/8",
			"1.0.0.0/8",
			"1.255.255.255/8",
			"1.2.3.4/32",
		}, {
			"1.2.3.4/16",
			"1.2.0.0/16",
			"1.2.0.0/16",
			"1.2.255.255/16",
			"1.2.3.4/32",
		}, {
			"1.2.3.4/24",
			"1.2.3.0/24",
			"1.2.3.0/24",
			"1.2.3.255/24",
			"1.2.3.4/32",
		}, {
			"1.2.3.4/32",
			"1.2.3.4/32",
			"1.2.3.4/32",
			"1.2.3.4/32",
			"1.2.3.4/32",
		}, {
			"255.255.255.255/32",
			"255.255.255.255/32",
			"255.255.255.255/32",
			"255.255.255.255/32",
			"255.255.255.255/32",
		}, {
			"::1/128",
			"::1/128",
			"::1/128",
			"::1/128",
			"::1/128",
		},
	}
	invalidaddresses = [...]string{
		"1.2.3.4/33",
		"0.0.0.256/0",
		"1::0::1/128",
		"::1/129",
		"10.10.10.10.10",
		"0.0.0.0",
		"::",
	}
)

func TestRoundtrip(t *testing.T) {
	for _, s := range validaddresses {
		want := s[0]
		ip, err := ParseIP(want)
		if err != nil {
			t.Errorf("Failed to parse address %q: %s", want, err.Error())
			continue
		}
		have := ip.String()
		t.Logf("Testing roundtrip: %q -> %q", want, have)
		if have != want {
			t.Errorf("Roundtrip failed. want: %q, have: %q", want, have)
			continue
		}
	}
}

func TestNetwork(t *testing.T) {
	for _, s := range validaddresses {
		in := s[0]
		want := s[1]
		ip, err := ParseIP(in)
		if err != nil {
			t.Errorf("Failed to parse address %q: %s", in, err.Error())
			continue
		}
		network := ip.Network()
		have := network.String()
		t.Logf("Testing network: %q -> %q", in, have)
		if have != want {
			t.Errorf("Network failed. want: %q, have: %q [%x]", want, have, ip.Addr)
			continue
		}
	}
}

func TestV4Prefix(t *testing.T) {
	want := ip4in6prefix
	for _, s := range validaddresses {
		in := s[0]
		ip, err := ParseIP(in)
		if err != nil {
			t.Errorf("Failed to parse address %q: %s", in, err.Error())
			continue
		}
		t.Logf("Testing v4 Prefix: %q -> [%x]", in, ip.Addr)

		hasprefix := bytes.HasPrefix(ip.Addr[:], want[:])
		isv4 := ip.AddrLen == 32
		if isv4 != hasprefix {
			t.Errorf("IPv4 needs 4in6 prefix. ip: %q, bytes: [%x]", in, ip.Addr)
			continue
		}
	}
}

func TestRange(t *testing.T) {
	for _, s := range validaddresses {
		in, wantstart, wantend := s[0], s[2], s[3]
		ip, err := ParseIP(in)
		if err != nil {
			t.Errorf("Failed to parse address %q: %s", in, err.Error())
			continue
		}
		havestart, haveend := ip.Range()

		t.Logf("Testing range %q: [%q %q]", in, havestart, haveend)
		if wantstart != havestart.String() {
			t.Errorf("Invalid range for %q. want start: %q, have start %q", in, wantstart, havestart)
			continue
		}
		if wantend != haveend.String() {
			t.Errorf("Invalid range for %q. want end: %q, have end %q", in, wantstart, haveend)
			continue
		}
	}
}
func TestHost(t *testing.T) {
	for _, s := range validaddresses {
		in, want := s[0], s[4]
		ip, err := ParseIP(in)
		if err != nil {
			t.Errorf("Failed to parse address %q: %s", in, err.Error())
			continue
		}
		have := ip.Host().String()
		if have != want {
			t.Errorf("Expected host %q for %q, have %q", want, in, have)
			continue
		}
	}
}

func iterHosts(t *testing.T, network IP) {
	ones, bits := network.NetMask().Size()
	if bits == ones {
		t.Skip("range has no addressess")
	}

	n := 1 << uint(bits-ones)

	if n > 1<<22 {
		t.Skipf("n too large: %d", n)
	}
	if testing.Short() && n > 1<<16 {
		t.Skipf("n too large for short test: %d", n)
	}

	t.Parallel()

	s, e := network.Range()
	for i := 0; i < n-1; i++ {
		ip, err := GetIP(s, e, i)
		if err != nil {
			t.Fatalf("Unexpected error: %q", err.Error())
		}
		if !network.Contains(ip) {
			t.Fatalf("No error, but ip out of range: %q not in %q", ip, network)
		}
	}
	ip, err := GetIP(s, e, n-1)
	if err == nil {
		t.Fatalf("Expected out of addresses error for %q in %q", ip, network)
	}
}
func TestGetIP(t *testing.T) {
	for _, s := range validaddresses {
		in := s[0]
		ip, err := ParseIP(in)
		if err != nil {
			t.Errorf("Failed to parse address %q: %s", in, err.Error())
			continue
		}
		t.Run(in, func(t *testing.T) {
			iterHosts(t, ip)
		})
	}
}
func TestInvalid(t *testing.T) {
	for _, in := range invalidaddresses {
		ip, err := ParseIP(in)
		if err == nil {
			t.Errorf("Expected error for invalid address %q, got %q", in, ip)
		}
	}
}

func TestIPSetParse(t *testing.T) {
	var buf bytes.Buffer
	sep := ""
	for _, s := range validaddresses {
		buf.WriteString(sep)
		buf.WriteString(s[0])
		sep = ", "
	}
	in := buf.String()

	var is IPSet
	err := is.UnmarshalText([]byte(in))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	inter := is.String()
	t.Log(inter)

	var is2 IPSet
	err = is2.UnmarshalText([]byte(inter))
	if err != nil {
		t.Fatal("Unexpected error", err)
	}
	have := is2.String()
	t.Log(have)
	if inter != have {
		t.Fatalf("IPSet doesn't roundtrip:\n%q != %q", inter, have)
	}
}

func TestIPSetParseInvalid(t *testing.T) {
	var buf bytes.Buffer
	sep := ""
	for _, s := range invalidaddresses {
		buf.WriteString(sep)
		buf.WriteString(s)
		sep = ", "
	}
	in := buf.String()

	var is IPSet
	err := is.UnmarshalText([]byte(in))
	if err == nil {
		t.Fatal("Expected error")
	}
}
