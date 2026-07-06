package checks

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// HTTPRequest is a minimal request description for the raw HTTP/1.1 probe.
type HTTPRequest struct {
	Method  string
	Host    string // Host header (also the TLS SNI target)
	Path    string // origin-form request target, e.g. /v1/messages
	Headers map[string]string
	Body    string
}

// HTTPResult carries the status line result of the probe.
type HTTPResult struct {
	Status  int
	Line    string
	Elapsed time.Duration
	Err     error
}

// HTTPOverConn writes a minimal HTTP/1.1 request over an established
// connection (typically a *tls.Conn) and reads just the status line. We only
// need the status code to classify reachability vs auth, so the response body
// is never read. "Connection: close" is sent so the server tears down after.
//
// We intentionally send NO credentials: for API endpoints this yields a 401,
// which proves the whole network path works without touching the user's key.
func HTTPOverConn(ctx context.Context, conn net.Conn, req HTTPRequest, timeout time.Duration) HTTPResult {
	start := time.Now()

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	method := req.Method
	if method == "" {
		method = "GET"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s HTTP/1.1\r\n", method, req.Path)
	fmt.Fprintf(&b, "Host: %s\r\n", req.Host)
	fmt.Fprintf(&b, "User-Agent: ai-net-doctor/%s\r\n", "0.1")
	fmt.Fprintf(&b, "Accept: */*\r\n")
	for k, v := range req.Headers {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	if req.Body != "" {
		fmt.Fprintf(&b, "Content-Length: %d\r\n", len(req.Body))
	}
	b.WriteString("Connection: close\r\n\r\n")
	if req.Body != "" {
		b.WriteString(req.Body)
	}

	if _, err := io.WriteString(conn, b.String()); err != nil {
		return HTTPResult{Elapsed: time.Since(start), Err: err}
	}

	line, err := readLine(conn)
	if err != nil {
		return HTTPResult{Line: line, Elapsed: time.Since(start), Err: err}
	}
	return HTTPResult{Status: parseStatus(line), Line: strings.TrimSpace(line), Elapsed: time.Since(start)}
}
