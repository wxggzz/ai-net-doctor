package targets

import (
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

// An http:// base URL must be refused (v0.1 probes assume TLS), not silently
// probed over TLS — which would report a bogus TLS failure.
func TestApplyOverride_RejectsHTTPScheme(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "http://127.0.0.1:8080")
	s := Codex().ApplyOverride()
	if s.SetupError != model.ReasonUnsupportedBaseURLScheme {
		t.Errorf("SetupError = %q, want UNSUPPORTED_BASE_URL_SCHEME", s.SetupError)
	}
	// Endpoint must be left untouched when the override is rejected.
	if s.APIHost != "api.openai.com" {
		t.Errorf("APIHost = %q, want api.openai.com (override rejected)", s.APIHost)
	}
}

func TestApplyOverride_AcceptsHTTPS(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://proxy.internal:8443/base")
	s := Claude().ApplyOverride()
	if s.SetupError != "" {
		t.Errorf("SetupError = %q, want empty", s.SetupError)
	}
	if s.APIHost != "proxy.internal" || s.APIPort != "8443" {
		t.Errorf("host/port = %s:%s, want proxy.internal:8443", s.APIHost, s.APIPort)
	}
	if s.AuthPath != "/base/v1/messages" {
		t.Errorf("AuthPath = %q, want /base/v1/messages", s.AuthPath)
	}
}

func TestApplyOverride_NoEnvIsNoop(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "")
	s := Codex().ApplyOverride()
	if s.SetupError != "" || s.APIHost != "api.openai.com" {
		t.Errorf("empty override should be a no-op, got %+v", s)
	}
}
