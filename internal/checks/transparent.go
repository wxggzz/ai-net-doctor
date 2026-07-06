package checks

import (
	"context"
	"net"
	"os/exec"
	"strings"
	"time"
)

// fakeIPNet is the RFC 2544 benchmarking range (198.18.0.0/15) that transparent
// proxies (Clash / sing-box / Surge "fake-ip" mode, TUN) hand back to clients
// instead of a real address. A public API host resolving into this range is a
// near-certain sign that traffic is being transparently tunneled, even though no
// env / system proxy is configured — the case where this tool would otherwise
// honestly-but-misleadingly report "direct".
var fakeIPNet = &net.IPNet{
	IP:   net.IPv4(198, 18, 0, 0),
	Mask: net.CIDRMask(15, 32),
}

// IsFakeIP reports whether ip falls in the fake-ip benchmarking range.
func IsFakeIP(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	return fakeIPNet.Contains(v4)
}

// FirstFakeIP returns the first fake-ip address in ips, if any.
func FirstFakeIP(ips []net.IPAddr) (net.IP, bool) {
	for _, ip := range ips {
		if IsFakeIP(ip.IP) {
			return ip.IP, true
		}
	}
	return nil, false
}

// tunnelIfacePrefixes are interface name prefixes that indicate a tunnel rather
// than a physical / direct link.
var tunnelIfacePrefixes = []string{"utun", "tun", "tap", "ipsec", "ppp", "wg"}

// LooksLikeTunnelIface reports whether an interface name looks like a tunnel.
func LooksLikeTunnelIface(name string) bool {
	name = strings.TrimSpace(name)
	for _, p := range tunnelIfacePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// DefaultRouteInterface returns the interface backing the default route (macOS:
// `route -n get default`). Best-effort: on other platforms or on error it
// returns "" and the error, and callers should treat an empty result as
// "unknown", never as failure.
func DefaultRouteInterface(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "route", "-n", "get", "default").Output()
	if err != nil {
		return "", err
	}
	return parseRouteInterface(string(out)), nil
}

// parseRouteInterface extracts the "interface: X" line from `route -n get
// default` output. Pure function for testability.
func parseRouteInterface(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
	}
	return ""
}

// TransparentProxyHint is the result of the transparent-proxy heuristic. It is
// advisory only: it never changes any target verdict, it only tells the user
// that a "direct" label may not reflect the real path.
type TransparentProxyHint struct {
	Suspected    bool   // high-confidence: a target resolved into the fake-ip range
	FakeIPHost   string // the host that resolved into fake-ip, if any
	FakeIPAddr   string // the fake-ip address observed
	DefaultIface string // default-route interface, if known (e.g. "utun4")
	TunnelIface  bool   // the default-route interface looks like a tunnel
}

// DetectTransparentProxy runs the heuristic for the given sample hosts. It is
// meant to be called only when the resolved path mode is "direct": its whole
// purpose is to catch the case where the client is silently tunneled despite no
// proxy being configured. It resolves each host (short, bounded) and inspects
// the default route interface. Fake-ip is the sole high-confidence trigger; the
// tunnel interface is corroborating context only, to avoid warning on every
// legitimate full-tunnel VPN user.
func DetectTransparentProxy(ctx context.Context, sampleHosts []string) TransparentProxyHint {
	var h TransparentProxyHint
	if iface, err := DefaultRouteInterface(ctx); err == nil {
		h.DefaultIface = iface
		h.TunnelIface = LooksLikeTunnelIface(iface)
	}
	for _, host := range sampleHosts {
		res := ResolveDNS(ctx, host, 3*time.Second)
		if res.Err != nil {
			continue
		}
		if ip, ok := FirstFakeIP(res.IPs); ok {
			h.Suspected = true
			h.FakeIPHost = host
			h.FakeIPAddr = ip.String()
			break
		}
	}
	return h
}
