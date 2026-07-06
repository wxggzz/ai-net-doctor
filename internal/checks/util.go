// Package checks holds the low-level network primitives. Each primitive is
// deliberately small and side-effect free so it can be composed by targets.Probe
// and tested in isolation.
package checks

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// readLine reads a single CRLF-terminated line directly from conn, one byte at
// a time. Reading unbuffered matters after an HTTP CONNECT: we must not consume
// bytes belonging to the subsequent TLS stream into a bufio buffer.
func readLine(conn net.Conn) (string, error) {
	var buf []byte
	one := make([]byte, 1)
	for {
		if _, err := conn.Read(one); err != nil {
			return string(buf), err
		}
		if one[0] == '\n' {
			return strings.TrimRight(string(buf), "\r"), nil
		}
		buf = append(buf, one[0])
		if len(buf) > 8192 {
			return string(buf), fmt.Errorf("response line too long")
		}
	}
}

// drainHeaders consumes header lines up to and including the blank separator.
func drainHeaders(conn net.Conn) {
	for {
		line, err := readLine(conn)
		if err != nil || line == "" {
			return
		}
	}
}

// parseStatus extracts the numeric status code from an HTTP status line such as
// "HTTP/1.1 401 Unauthorized". Returns 0 if it cannot be parsed.
func parseStatus(line string) int {
	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 2 {
		return 0
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return code
}
