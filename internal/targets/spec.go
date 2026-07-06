// Package targets defines the per-target endpoint sets (Codex, Claude) and the
// layered probe that turns them into an independent TargetResult.
package targets

import (
	"net/url"
	"os"
	"strings"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

// Spec describes how to probe one target's API endpoint.
//
// NOTE: endpoints and env-var names below are the current best understanding.
// They are centralized here on purpose so they can be corrected in one place.
// See DESIGN.md §12 "待官方实现确认".
type Spec struct {
	Name        string // stable key: "codex" / "claude"
	Display     string // human label
	APIHost     string // host for the auth probe, e.g. api.anthropic.com
	APIPort     string // "443"
	AuthPath    string // origin-form path, e.g. /v1/messages
	AuthMethod  string // usually POST
	AuthBody    string // minimal body; content is irrelevant without credentials
	AuthHeaders map[string]string
	BaseURLEnv  string // env var that overrides the API base URL, if set
	// SetupError, when non-empty, means the spec is unusable as configured (e.g.
	// an http:// base URL override). The probe fails fast with this reason and
	// performs no network I/O.
	SetupError model.ReasonCode
}

// PathPlan is the resolved network path a probe should use for a target.
type PathPlan struct {
	RequestedMode   model.PathMode // what the user asked for / auto-selected
	Mode            model.PathMode // what the probe will actually do
	ProxyURL        *url.URL       // nil for direct
	ProxyLabel      string         // redacted host:port for display
	NoProxyExcluded bool           // requested proxy bypassed for this host by NO_PROXY
	// SetupError, when non-empty, means the requested path can't be set up (e.g.
	// forced --proxy env with no env proxy). The probe fails fast with it.
	SetupError model.ReasonCode
}

// ApplyOverride applies a *_BASE_URL env override to the spec's endpoint, if
// present. Only the endpoint is changed; the value is never emitted. A non-https
// scheme is rejected via SetupError rather than silently probed over TLS.
func (s Spec) ApplyOverride() Spec {
	if s.BaseURLEnv == "" {
		return s
	}
	raw := os.Getenv(s.BaseURLEnv)
	if raw == "" {
		return s
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return s
	}
	// v0.1 probes assume TLS. Refuse http:// (or anything else) rather than
	// mis-probe an http gateway over TLS and report a bogus TLS failure.
	if u.Scheme != "" && u.Scheme != "https" {
		s.SetupError = model.ReasonUnsupportedBaseURLScheme
		return s
	}
	s.APIHost = u.Hostname()
	if u.Port() != "" {
		s.APIPort = u.Port()
	}
	if p := strings.TrimSuffix(u.Path, "/"); p != "" {
		s.AuthPath = p + s.AuthPath
	}
	return s
}
