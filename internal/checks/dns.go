package checks

import (
	"context"
	"net"
	"time"
)

// DNSResult is the outcome of a DNS resolution.
type DNSResult struct {
	IPs     []net.IPAddr
	Elapsed time.Duration
	Err     error
}

// ResolveDNS resolves host to IP addresses, bounded by timeout (and by any
// deadline already on ctx).
func ResolveDNS(ctx context.Context, host string, timeout time.Duration) DNSResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var r net.Resolver
	ips, err := r.LookupIPAddr(ctx, host)
	return DNSResult{IPs: ips, Elapsed: time.Since(start), Err: err}
}
