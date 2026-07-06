package checks

import "testing"

func TestMatchNoProxy(t *testing.T) {
	cases := []struct {
		name    string
		noProxy string
		host    string
		want    bool
	}{
		{"exact", "api.anthropic.com", "api.anthropic.com", true},
		{"subdomain via bare domain", "anthropic.com", "api.anthropic.com", true},
		{"subdomain via leading dot", ".anthropic.com", "api.anthropic.com", true},
		{"no match", "openai.com", "api.anthropic.com", false},
		{"wildcard", "*", "api.anthropic.com", true},
		{"list with match", "localhost,127.0.0.1,anthropic.com", "api.anthropic.com", true},
		{"list no match", "localhost,example.com", "api.anthropic.com", false},
		{"entry with port", "anthropic.com:443", "api.anthropic.com", true},
		{"case insensitive", "ANTHROPIC.COM", "API.Anthropic.Com", true},
		{"not a false suffix", "example.com", "notexample.com", false},
		{"empty noproxy", "", "api.anthropic.com", false},
		{"empty host", "anthropic.com", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchNoProxy(c.noProxy, c.host); got != c.want {
				t.Fatalf("MatchNoProxy(%q, %q) = %v, want %v", c.noProxy, c.host, got, c.want)
			}
		})
	}
}
