package checks

import (
	"context"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// SystemProxy is the macOS system HTTP(S) proxy configuration, as read from
// `scutil --proxy`. CLI tools (Codex/Claude Code) usually ignore this and honor
// env-var proxies instead, so comparing the two surfaces "browser works but CLI
// doesn't" cases.
type SystemProxy struct {
	HTTPEnabled  bool
	HTTPProxy    string
	HTTPPort     string
	HTTPSEnabled bool
	HTTPSProxy   string
	HTTPSPort    string
}

// Enabled reports whether any system proxy is on.
func (s SystemProxy) Enabled() bool { return s.HTTPEnabled || s.HTTPSEnabled }

// AsMap returns a JSON-friendly, secret-free view (host:port only).
func (s SystemProxy) AsMap() map[string]string {
	m := map[string]string{}
	if s.HTTPSEnabled && s.HTTPSProxy != "" {
		m["HTTPSProxy"] = net.JoinHostPort(s.HTTPSProxy, orDefault(s.HTTPSPort, "443"))
	}
	if s.HTTPEnabled && s.HTTPProxy != "" {
		m["HTTPProxy"] = net.JoinHostPort(s.HTTPProxy, orDefault(s.HTTPPort, "80"))
	}
	return m
}

// HTTPSURL returns the system HTTPS proxy as an http:// CONNECT proxy URL, or
// nil if not enabled.
func (s SystemProxy) HTTPSURL() *url.URL {
	if !s.HTTPSEnabled || s.HTTPSProxy == "" {
		return nil
	}
	return &url.URL{Scheme: "http", Host: net.JoinHostPort(s.HTTPSProxy, orDefault(s.HTTPSPort, "443"))}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ReadSystemProxy runs `scutil --proxy` and parses it. On non-macOS hosts (or
// if scutil is unavailable) it returns an empty config with the error.
func ReadSystemProxy(ctx context.Context) (SystemProxy, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "scutil", "--proxy").Output()
	if err != nil {
		return SystemProxy{}, err
	}
	return parseScutilProxy(string(out)), nil
}

// parseScutilProxy parses `scutil --proxy` output. Pure function for testability.
//
// Example line format:
//
//	<dictionary> {
//	  HTTPEnable : 1
//	  HTTPSProxy : 127.0.0.1
//	}
func parseScutilProxy(out string) SystemProxy {
	var sp SystemProxy
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "HTTPEnable":
			sp.HTTPEnabled = val == "1"
		case "HTTPProxy":
			sp.HTTPProxy = val
		case "HTTPPort":
			sp.HTTPPort = val
		case "HTTPSEnable":
			sp.HTTPSEnabled = val == "1"
		case "HTTPSProxy":
			sp.HTTPSProxy = val
		case "HTTPSPort":
			sp.HTTPSPort = val
		}
	}
	return sp
}
