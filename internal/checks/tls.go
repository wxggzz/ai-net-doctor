package checks

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// TLSHandshake performs a TLS handshake over an already-open raw connection
// (direct TCP or an established proxy tunnel). It advertises only http/1.1 via
// ALPN so the subsequent HTTP probe can speak plain HTTP/1.1.
//
// It returns the wrapped connection as net.Conn plus a human-readable TLS
// version string, so callers need not import crypto/tls.
func TLSHandshake(ctx context.Context, raw net.Conn, serverName string, timeout time.Duration) (net.Conn, string, time.Duration, error) {
	start := time.Now()
	cfg := &tls.Config{
		ServerName: serverName,
		NextProtos: []string{"http/1.1"},
		MinVersion: tls.VersionTLS12,
	}
	tconn := tls.Client(raw, cfg)

	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := tconn.HandshakeContext(hctx); err != nil {
		return nil, "", time.Since(start), err
	}
	return tconn, tlsVersionName(tconn.ConnectionState().Version), time.Since(start), nil
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return "unknown"
	}
}
