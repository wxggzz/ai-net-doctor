// Package model defines the stable data model and JSON schema (version 1)
// emitted by ai-net-doctor. Everything that upper layers (skills, MCP, click
// entry points) consume is defined here. The CLI computes verdict / failed
// layer / reason code; upper layers must only display these, never re-derive.
package model

// Version is the tool version reported in output.
const Version = "0.1.0"

// SchemaVersion is the JSON schema version. Bump on breaking field changes.
const SchemaVersion = "1"

// Verdict is a per-target conclusion computed entirely by the CLI.
type Verdict string

const (
	VerdictOK    Verdict = "OK"    // real path fully reachable
	VerdictCheck Verdict = "CHECK" // reachable but with a caveat (e.g. auth not verified)
	VerdictFail  Verdict = "FAIL"  // real path broke at some layer
)

// Layer identifies where in the stack a check sits / where the first break is.
type Layer string

const (
	LayerDNS   Layer = "dns"
	LayerTCP   Layer = "tcp"
	LayerTLS   Layer = "tls"
	LayerHTTP  Layer = "http"
	LayerAuth  Layer = "auth"
	LayerProxy Layer = "proxy"
	LayerRoute Layer = "route"
	LayerIPv6  Layer = "ipv6"
)

// PathMode is the network path a check actually used.
type PathMode string

const (
	PathDirect PathMode = "direct"
	PathEnv    PathMode = "env"
	PathSystem PathMode = "system"
)

// ReasonCode is a stable enum. Upper layers map it to localized text; do not
// rename existing values.
type ReasonCode string

const (
	ReasonOK                     ReasonCode = "OK"
	ReasonDNSResolveFailed       ReasonCode = "DNS_RESOLVE_FAILED"
	ReasonTCPConnectTimeout      ReasonCode = "TCP_CONNECT_TIMEOUT"
	ReasonTCPConnectFailed       ReasonCode = "TCP_CONNECT_FAILED"
	ReasonTLSHandshakeFailed     ReasonCode = "TLS_HANDSHAKE_FAILED"
	ReasonHTTPUnreachable        ReasonCode = "HTTP_UNREACHABLE"
	ReasonAuthRequiredReachable  ReasonCode = "AUTH_REQUIRED_REACHABLE"
	ReasonProxyConnectTimeout    ReasonCode = "PROXY_CONNECT_TIMEOUT"
	ReasonProxyConnectFailed     ReasonCode = "PROXY_CONNECT_FAILED"
	ReasonProxyAuthRequired      ReasonCode = "PROXY_AUTH_REQUIRED"
	ReasonNoProxyExcludesTarget  ReasonCode = "NO_PROXY_EXCLUDES_TARGET"
	ReasonEnvSystemProxyDiverged ReasonCode = "ENV_SYSTEM_PROXY_DIVERGED"
	ReasonBudgetExceeded         ReasonCode = "BUDGET_EXCEEDED"
	// ReasonEnvProxyNotConfigured / ReasonSystemProxyNotConfigured: the user
	// explicitly forced a proxy path (--proxy env / --proxy system) but no such
	// proxy is available. We must NOT silently fall back to direct — that would
	// mislead the user into thinking the proxy path was tested.
	ReasonEnvProxyNotConfigured    ReasonCode = "ENV_PROXY_NOT_CONFIGURED"
	ReasonSystemProxyNotConfigured ReasonCode = "SYSTEM_PROXY_NOT_CONFIGURED"
	// ReasonUnsupportedBaseURLScheme: a *_BASE_URL override used a non-https
	// scheme; v0.1 probes assume TLS, so we refuse rather than mis-probe.
	ReasonUnsupportedBaseURLScheme ReasonCode = "UNSUPPORTED_BASE_URL_SCHEME"
	// ReasonTransparentProxySuspected is a path-level advisory (not a target
	// verdict): traffic looks transparently tunneled despite no proxy being
	// configured, so a "direct" result may not reflect the real path.
	ReasonTransparentProxySuspected ReasonCode = "TRANSPARENT_PROXY_SUSPECTED"
	// ReasonSystemProxySetButEnvEmpty is a path-level advisory: a system proxy is
	// on but no env proxy is set, so the browser may work while the CLI (which
	// reads env proxies) goes direct.
	ReasonSystemProxySetButEnvEmpty ReasonCode = "SYSTEM_PROXY_SET_BUT_ENV_EMPTY"
)

// Check is a single layer probe result.
type Check struct {
	Name      string   `json:"name"`
	Layer     Layer    `json:"layer"`
	OK        bool     `json:"ok"`
	Skipped   bool     `json:"skipped"`
	ElapsedMs int64    `json:"elapsed_ms"`
	Detail    string   `json:"detail"`
	Error     string   `json:"error"`
	Endpoint  string   `json:"endpoint"`
	PathMode  PathMode `json:"path_mode"`
}

// TargetResult is the independent conclusion for one target (codex / claude).
type TargetResult struct {
	Verdict     Verdict    `json:"verdict"`
	FailedLayer *Layer     `json:"failed_layer"` // null when nothing failed
	ReasonCode  ReasonCode `json:"reason_code"`
	LatencyMs   int64      `json:"latency_ms"`
	Host        string     `json:"host"`      // the API host actually probed
	PathMode    PathMode   `json:"path_mode"` // the path this target actually took
	// NoProxyExcluded is set when the requested proxy path was bypassed for this
	// target because NO_PROXY matched its host (so PathMode is direct).
	NoProxyExcluded bool    `json:"no_proxy_excluded"`
	Checks          []Check `json:"checks"`
}

// NetworkPath describes the path selection context (redacted).
type NetworkPath struct {
	Mode        PathMode          `json:"mode"`         // requested/selected path mode
	Forced      bool              `json:"forced"`       // true if the user forced the mode via --direct/--proxy
	ProxyEnv    map[string]string `json:"proxy_env"`    // redacted
	SystemProxy map[string]string `json:"system_proxy"` // host:port only, no secrets
	Diverged    bool              `json:"diverged"`
	// TransparentProxySuspected is set when the path is reported as "direct" but
	// heuristics (fake-ip DNS, tunnel default route) suggest traffic is actually
	// being tunneled. Advisory only — target verdicts are unaffected.
	TransparentProxySuspected bool   `json:"transparent_proxy_suspected"`
	TransparentProxyHint      string `json:"transparent_proxy_hint,omitempty"`
}

// Host is basic host info.
type Host struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	ToolVersion string `json:"tool_version"`
}

// Report is the top-level document emitted with --json.
type Report struct {
	SchemaVersion      string                  `json:"schema_version"`
	GeneratedAt        string                  `json:"generated_at"`
	Host               Host                    `json:"host"`
	NetworkPath        NetworkPath             `json:"network_path"`
	Targets            map[string]TargetResult `json:"targets"`
	CredentialsPresent map[string]bool         `json:"credentials_present"` // presence only, never values
	Warnings           []string                `json:"warnings"`
	Remediation        []string                `json:"remediation"`
}
