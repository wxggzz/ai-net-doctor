package checks

import (
	"context"
	"net"
	"time"
)

// DialTCP opens a TCP connection to addr (host:port), bounded by timeout and by
// any deadline already on ctx.
func DialTCP(ctx context.Context, addr string, timeout time.Duration) (net.Conn, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	return conn, time.Since(start), err
}
