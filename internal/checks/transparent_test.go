package checks

import (
	"net"
	"testing"
)

func TestIsFakeIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"198.18.0.1", true}, // start of range
		{"198.18.255.255", true},
		{"198.19.0.0", true},     // /15 spans .18 and .19
		{"198.19.255.255", true}, // end of range
		{"198.17.255.255", false},
		{"198.20.0.0", false},
		{"1.1.1.1", false},
		{"104.18.32.47", false}, // typical CDN address
		{"::1", false},
	}
	for _, c := range cases {
		got := IsFakeIP(net.ParseIP(c.ip))
		if got != c.want {
			t.Errorf("IsFakeIP(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestFirstFakeIP(t *testing.T) {
	ips := []net.IPAddr{
		{IP: net.ParseIP("104.18.0.1")},
		{IP: net.ParseIP("198.18.1.2")},
		{IP: net.ParseIP("198.18.3.4")},
	}
	ip, ok := FirstFakeIP(ips)
	if !ok || ip.String() != "198.18.1.2" {
		t.Fatalf("FirstFakeIP = %v, %v; want 198.18.1.2, true", ip, ok)
	}

	clean := []net.IPAddr{{IP: net.ParseIP("1.1.1.1")}, {IP: net.ParseIP("8.8.8.8")}}
	if _, ok := FirstFakeIP(clean); ok {
		t.Errorf("FirstFakeIP on clean addrs returned ok=true")
	}
}

func TestLooksLikeTunnelIface(t *testing.T) {
	cases := map[string]bool{
		"utun4":     true,
		"utun0":     true,
		"tun0":      true,
		"tap0":      true,
		"ipsec0":    true,
		"ppp0":      true,
		"wg0":       true,
		"en0":       false,
		"lo0":       false,
		"bridge100": false,
		"":          false,
	}
	for name, want := range cases {
		if got := LooksLikeTunnelIface(name); got != want {
			t.Errorf("LooksLikeTunnelIface(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseRouteInterface(t *testing.T) {
	out := `   route to: default
destination: default
       mask: default
    gateway: 198.18.0.1
  interface: utun4
      flags: <UP,GATEWAY,DONE,STATIC>
`
	if got := parseRouteInterface(out); got != "utun4" {
		t.Errorf("parseRouteInterface = %q, want utun4", got)
	}
	if got := parseRouteInterface("no interface line here"); got != "" {
		t.Errorf("parseRouteInterface(none) = %q, want empty", got)
	}
}
