package checks

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

// ProxyEnvKeys are the proxy-related env vars we read (upper and lower case).
var ProxyEnvKeys = []string{
	"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "NO_PROXY",
	"https_proxy", "http_proxy", "all_proxy", "no_proxy",
}

// ReadProxyEnvRaw returns the present proxy env vars with their raw values.
func ReadProxyEnvRaw() map[string]string {
	m := map[string]string{}
	for _, k := range ProxyEnvKeys {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			m[k] = v
		}
	}
	return m
}

// RedactProxyURL replaces any password in a proxy URL with "***", keeping the
// rest intact. It reconstructs the string manually so the mask is not
// percent-encoded by url.String().
func RedactProxyURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, hasPw := u.User.Password(); !hasPw {
		return raw
	}
	scheme := ""
	if u.Scheme != "" {
		scheme = u.Scheme + "://"
	}
	return fmt.Sprintf("%s%s:***@%s%s", scheme, u.User.Username(), u.Host, u.Path)
}

// RedactProxyEnv returns a display-safe copy: proxy URLs have credentials
// stripped; NO_PROXY (a host list, not a URL) is passed through unchanged.
func RedactProxyEnv(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if strings.Contains(strings.ToLower(k), "no_proxy") {
			out[k] = v
			continue
		}
		out[k] = RedactProxyURL(v)
	}
	return out
}

func getEnvAny(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// SelectEnvProxyForHost decides which proxy (if any) a client would use for
// host, honoring NO_PROXY. It returns the proxy URL, whether NO_PROXY excluded
// the host, and a parse error if the proxy URL is malformed.
func SelectEnvProxyForHost(env map[string]string, host string) (*url.URL, bool, error) {
	if noProxy := getEnvAny(env, "NO_PROXY", "no_proxy"); noProxy != "" && MatchNoProxy(noProxy, host) {
		return nil, true, nil
	}
	raw := getEnvAny(env, "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "HTTP_PROXY", "http_proxy")
	if raw == "" {
		return nil, false, nil
	}
	u, err := parseProxyURL(raw)
	return u, false, err
}

func parseProxyURL(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return url.Parse(raw)
}

// ProxyConnectResult is the outcome of an HTTP CONNECT tunnel attempt. On
// success Conn is the tunneled connection ready for a TLS handshake.
type ProxyConnectResult struct {
	Conn    net.Conn
	Status  int
	Elapsed time.Duration
	Err     error
}

// HTTPConnect dials an HTTP(S) proxy and issues CONNECT targetHostPort,
// returning the tunneled connection. This tests far more than "is the port
// listening": a listening-but-broken proxy (dead airport rule, expired node)
// fails here, which is exactly the case reachability pings miss.
//
// SOCKS proxies are not supported in v0.1 (see README); an explicit error is
// returned so the reason surfaces clearly.
func HTTPConnect(ctx context.Context, proxyURL *url.URL, targetHostPort string, timeout time.Duration) ProxyConnectResult {
	start := time.Now()
	if proxyURL == nil {
		return ProxyConnectResult{Err: fmt.Errorf("no proxy configured")}
	}
	if strings.HasPrefix(strings.ToLower(proxyURL.Scheme), "socks") {
		return ProxyConnectResult{
			Elapsed: time.Since(start),
			Err:     fmt.Errorf("socks proxy not supported in v0.1; set an HTTP(S)_PROXY instead"),
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	proxyHostPort := proxyURL.Host
	if proxyURL.Port() == "" {
		proxyHostPort = net.JoinHostPort(proxyURL.Hostname(), "8080")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyHostPort)
	if err != nil {
		return ProxyConnectResult{Elapsed: time.Since(start), Err: err}
	}

	deadline := time.Now().Add(timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	var req strings.Builder
	fmt.Fprintf(&req, "CONNECT %s HTTP/1.1\r\n", targetHostPort)
	fmt.Fprintf(&req, "Host: %s\r\n", targetHostPort)
	if proxyURL.User != nil {
		user := proxyURL.User.Username()
		pw, _ := proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pw))
		fmt.Fprintf(&req, "Proxy-Authorization: Basic %s\r\n", auth)
	}
	req.WriteString("\r\n")

	if _, err := io.WriteString(conn, req.String()); err != nil {
		conn.Close()
		return ProxyConnectResult{Elapsed: time.Since(start), Err: err}
	}

	line, err := readLine(conn)
	if err != nil {
		conn.Close()
		return ProxyConnectResult{Elapsed: time.Since(start), Err: err}
	}
	status := parseStatus(line)
	if status != 200 {
		conn.Close()
		return ProxyConnectResult{Status: status, Elapsed: time.Since(start), Err: fmt.Errorf("proxy returned %q", strings.TrimSpace(line))}
	}
	// Consume the remaining CONNECT response headers so the tunnel is clean.
	drainHeaders(conn)
	// Clear the deadline; the TLS handshake sets its own.
	_ = conn.SetDeadline(time.Time{})
	return ProxyConnectResult{Conn: conn, Status: status, Elapsed: time.Since(start)}
}
