package runner

import (
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/checks"
	"github.com/wxggzz/ai-net-doctor/internal/model"
)

func TestDetermineMode(t *testing.T) {
	cases := []struct {
		force string
		env   map[string]string
		want  model.PathMode
	}{
		{"direct", nil, model.PathDirect},
		{"env", nil, model.PathEnv},
		{"system", nil, model.PathSystem},
		{"", nil, model.PathDirect},
		{"", map[string]string{"HTTPS_PROXY": "http://127.0.0.1:7897"}, model.PathEnv},
	}
	for _, c := range cases {
		if got := determineMode(c.force, c.env); got != c.want {
			t.Errorf("determineMode(%q, %v) = %s, want %s", c.force, c.env, got, c.want)
		}
	}
}

// Forced --proxy env with no env proxy must NOT downgrade to direct; it must
// carry a setup error so the probe fails with a clear reason.
func TestBuildPlan_ForceEnvNoProxy(t *testing.T) {
	plan, _ := buildPlan(model.PathEnv, true, map[string]string{}, checks.SystemProxy{}, "api.openai.com")
	if plan.SetupError != model.ReasonEnvProxyNotConfigured {
		t.Errorf("SetupError = %q, want ENV_PROXY_NOT_CONFIGURED", plan.SetupError)
	}
	if plan.Mode == model.PathDirect {
		t.Errorf("forced env with no proxy must not silently become direct")
	}
}

// Forced --proxy system with no system proxy must fail loudly, not go direct.
func TestBuildPlan_ForceSystemNoProxy(t *testing.T) {
	plan, _ := buildPlan(model.PathSystem, true, map[string]string{}, checks.SystemProxy{}, "api.openai.com")
	if plan.SetupError != model.ReasonSystemProxyNotConfigured {
		t.Errorf("SetupError = %q, want SYSTEM_PROXY_NOT_CONFIGURED", plan.SetupError)
	}
	if plan.Mode == model.PathDirect {
		t.Errorf("forced system with no proxy must not silently become direct")
	}
}

// Auto mode (not forced) with no env proxy legitimately goes direct.
func TestBuildPlan_AutoNoProxyGoesDirect(t *testing.T) {
	// determineMode already resolves auto+no-proxy to direct.
	mode := determineMode("", map[string]string{})
	plan, _ := buildPlan(mode, false, map[string]string{}, checks.SystemProxy{}, "api.openai.com")
	if plan.Mode != model.PathDirect {
		t.Errorf("Mode = %s, want direct", plan.Mode)
	}
	if plan.SetupError != "" {
		t.Errorf("auto direct must not carry a setup error, got %q", plan.SetupError)
	}
}

// NO_PROXY exclusion downgrades to direct but is surfaced (warning + flag),
// never hidden.
func TestBuildPlan_NoProxyExcludes(t *testing.T) {
	env := map[string]string{
		"HTTPS_PROXY": "http://127.0.0.1:7897",
		"NO_PROXY":    "api.openai.com",
	}
	plan, warns := buildPlan(model.PathEnv, true, env, checks.SystemProxy{}, "api.openai.com")
	if !plan.NoProxyExcluded {
		t.Errorf("NoProxyExcluded = false, want true")
	}
	if plan.Mode != model.PathDirect {
		t.Errorf("Mode = %s, want direct (NO_PROXY bypasses the proxy)", plan.Mode)
	}
	if plan.SetupError != "" {
		t.Errorf("NO_PROXY exclusion is not a setup error, got %q", plan.SetupError)
	}
	if !containsStr(warns, string(model.ReasonNoProxyExcludesTarget)) {
		t.Errorf("warns = %v, want to contain NO_PROXY_EXCLUDES_TARGET", warns)
	}
}

func TestBuildPlan_EnvProxyHappy(t *testing.T) {
	env := map[string]string{"HTTPS_PROXY": "http://127.0.0.1:7897"}
	plan, _ := buildPlan(model.PathEnv, true, env, checks.SystemProxy{}, "api.openai.com")
	if plan.Mode != model.PathEnv || plan.ProxyURL == nil {
		t.Errorf("expected env-proxy plan with a proxy URL, got %+v", plan)
	}
	if plan.SetupError != "" {
		t.Errorf("happy path must have no setup error, got %q", plan.SetupError)
	}
}

func TestSystemProxySetButEnvEmpty(t *testing.T) {
	sysOn := checks.SystemProxy{HTTPSEnabled: true, HTTPSProxy: "127.0.0.1", HTTPSPort: "7897"}
	sysOff := checks.SystemProxy{}
	envSet := map[string]string{"HTTPS_PROXY": "http://127.0.0.1:7897"}

	if !systemProxySetButEnvEmpty(model.PathDirect, map[string]string{}, sysOn) {
		t.Error("direct + system proxy on + env empty should warn")
	}
	if systemProxySetButEnvEmpty(model.PathDirect, envSet, sysOn) {
		t.Error("env proxy set -> should not warn")
	}
	if systemProxySetButEnvEmpty(model.PathEnv, map[string]string{}, sysOn) {
		t.Error("non-direct mode -> should not warn")
	}
	if systemProxySetButEnvEmpty(model.PathDirect, map[string]string{}, sysOff) {
		t.Error("no system proxy -> should not warn")
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
