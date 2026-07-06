package checks

import (
	"strings"
	"testing"
)

func TestRedactProxyURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"http://user:secretpass@127.0.0.1:7890", "http://user:***@127.0.0.1:7890"},
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890"},                 // no creds
		{"https://user@proxy.local:8080", "https://user@proxy.local:8080"}, // user only, no password
		{"socks5://user:pw@127.0.0.1:1080", "socks5://user:***@127.0.0.1:1080"},
	}
	for _, c := range cases {
		got := RedactProxyURL(c.raw)
		if got != c.want {
			t.Errorf("RedactProxyURL(%q) = %q, want %q", c.raw, got, c.want)
		}
		if strings.Contains(got, "secretpass") || strings.Contains(got, "pw@") {
			t.Errorf("RedactProxyURL(%q) leaked password: %q", c.raw, got)
		}
	}
}

func TestRedactProxyEnv(t *testing.T) {
	in := map[string]string{
		"HTTPS_PROXY": "http://user:secretpass@127.0.0.1:7890",
		"NO_PROXY":    "localhost,anthropic.com",
	}
	out := RedactProxyEnv(in)
	if strings.Contains(out["HTTPS_PROXY"], "secretpass") {
		t.Errorf("HTTPS_PROXY not redacted: %q", out["HTTPS_PROXY"])
	}
	if out["NO_PROXY"] != "localhost,anthropic.com" {
		t.Errorf("NO_PROXY should pass through unchanged, got %q", out["NO_PROXY"])
	}
}

func TestSelectEnvProxyForHost(t *testing.T) {
	t.Run("uses HTTPS_PROXY", func(t *testing.T) {
		env := map[string]string{"HTTPS_PROXY": "http://127.0.0.1:7890"}
		u, excluded, err := SelectEnvProxyForHost(env, "api.anthropic.com")
		if err != nil || excluded || u == nil {
			t.Fatalf("got u=%v excluded=%v err=%v", u, excluded, err)
		}
		if u.Host != "127.0.0.1:7890" {
			t.Fatalf("proxy host = %q", u.Host)
		}
	})

	t.Run("NO_PROXY excludes target", func(t *testing.T) {
		env := map[string]string{
			"HTTPS_PROXY": "http://127.0.0.1:7890",
			"NO_PROXY":    "anthropic.com",
		}
		u, excluded, err := SelectEnvProxyForHost(env, "api.anthropic.com")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !excluded {
			t.Fatalf("expected excluded=true")
		}
		if u != nil {
			t.Fatalf("expected nil proxy when excluded, got %v", u)
		}
	})

	t.Run("no proxy configured", func(t *testing.T) {
		u, excluded, err := SelectEnvProxyForHost(map[string]string{}, "api.anthropic.com")
		if err != nil || excluded || u != nil {
			t.Fatalf("got u=%v excluded=%v err=%v", u, excluded, err)
		}
	})

	t.Run("scheme-less proxy gets http://", func(t *testing.T) {
		env := map[string]string{"HTTP_PROXY": "127.0.0.1:8080"}
		u, _, err := SelectEnvProxyForHost(env, "example.com")
		if err != nil || u == nil {
			t.Fatalf("got u=%v err=%v", u, err)
		}
		if u.Scheme != "http" || u.Host != "127.0.0.1:8080" {
			t.Fatalf("parsed proxy = %+v", u)
		}
	})
}
