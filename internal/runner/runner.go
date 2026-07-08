// Package runner resolves the network path, runs each target's probe
// concurrently under a shared time budget, and assembles the Report.
package runner

import (
	"context"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wxggzz/ai-net-doctor/internal/checks"
	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/quota"
	"github.com/wxggzz/ai-net-doctor/internal/targets"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

// Options controls a diagnostic run.
type Options struct {
	Targets   []string      // ordered: subset of {"codex","claude"}
	Budget    time.Duration // total wall-clock budget
	ForceMode string        // "", "direct", "env", "system"
}

var specFor = map[string]func() targets.Spec{
	"codex":  targets.Codex,
	"claude": targets.Claude,
}

// Run executes the diagnostic and returns a fully-populated Report.
func Run(ctx context.Context, opts Options) model.Report {
	ctx, cancel := context.WithTimeout(ctx, opts.Budget)
	defer cancel()

	envRaw := checks.ReadProxyEnvRaw()
	sysProxy, _ := checks.ReadSystemProxy(ctx)
	forced := opts.ForceMode != ""
	mode := determineMode(opts.ForceMode, envRaw)

	report := model.Report{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		Host:          model.Host{OS: runtime.GOOS, Arch: runtime.GOARCH, ToolVersion: model.Version},
		NetworkPath: model.NetworkPath{
			Mode:        mode,
			Forced:      forced,
			ProxyEnv:    checks.RedactProxyEnv(envRaw),
			SystemProxy: sysProxy.AsMap(),
			Diverged:    proxyDiverged(envRaw, sysProxy),
		},
		Targets:            map[string]model.TargetResult{},
		CredentialsPresent: checks.CredentialPresence(),
		Warnings:           []string{},
		Remediation:        []string{},
	}

	warnSet := map[string]bool{}
	if report.NetworkPath.Diverged {
		warnSet[string(model.ReasonEnvSystemProxyDiverged)] = true
	}

	// Resolve each target's spec and path plan up front, so we know which targets
	// will actually touch the network. A setup error (an http base URL, or a
	// forced-but-missing proxy) fails fast with no network I/O.
	type probeUnit struct {
		name  string
		spec  targets.Spec
		plan  targets.PathPlan
		warns []string
	}
	var units []probeUnit
	anyNetworkProbe := false
	for _, name := range opts.Targets {
		makeSpec, ok := specFor[name]
		if !ok {
			continue
		}
		spec := makeSpec().ApplyOverride()
		plan, warns := buildPlan(mode, forced, envRaw, sysProxy, spec.APIHost)
		if spec.SetupError == "" && plan.SetupError == "" {
			anyNetworkProbe = true
		}
		units = append(units, probeUnit{name: name, spec: spec, plan: plan, warns: warns})
	}

	// Path-level advisories only make sense if at least one target actually walks
	// the network path. When every target fails at setup, they would be noise.
	if anyNetworkProbe {
		// System proxy on, but no env proxy: the browser may work while the CLI
		// (which honors env proxies) goes direct. Only meaningful when direct.
		if systemProxySetButEnvEmpty(mode, envRaw, sysProxy) {
			warnSet[string(model.ReasonSystemProxySetButEnvEmpty)] = true
		}
		// Transparent-proxy heuristic: catch a silently-tunneled client that no
		// env/system proxy reveals. Only probe hosts that will actually be tested.
		if mode == model.PathDirect {
			var hosts []string
			seen := map[string]bool{}
			for _, u := range units {
				if u.spec.SetupError != "" || u.plan.SetupError != "" {
					continue
				}
				if !seen[u.spec.APIHost] {
					seen[u.spec.APIHost] = true
					hosts = append(hosts, u.spec.APIHost)
				}
			}
			if hint := checks.DetectTransparentProxy(ctx, hosts); hint.Suspected {
				report.NetworkPath.TransparentProxySuspected = true
				report.NetworkPath.TransparentProxyHint = transparentHintText(hint)
				warnSet[string(model.ReasonTransparentProxySuspected)] = true
			}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, u := range units {
		wg.Add(1)
		go func(name string, spec targets.Spec, plan targets.PathPlan, warns []string) {
			defer wg.Done()
			res := targets.Probe(ctx, spec, plan)
			mu.Lock()
			report.Targets[name] = res
			for _, w := range warns {
				warnSet[w] = true
			}
			mu.Unlock()
		}(u.name, u.spec, u.plan, u.warns)
	}
	wg.Wait()

	// Third dimension: attach each target's locally-read quota snapshot (no
	// network, no credentials). It never changes a verdict; it only lets us add
	// "the path is fine but the quota is spent" when a reachable target is
	// rate-limited — the case where "stream disconnected" is not the network.
	qr := quota.Reader{}
	for _, name := range opts.Targets {
		res, ok := report.Targets[name]
		if !ok {
			continue
		}
		q := qr.Read(name)
		if q == nil {
			continue
		}
		res.Quota = q
		report.Targets[name] = res
		if q.SuggestsBlock() && res.Verdict != model.VerdictFail {
			warnSet[string(model.ReasonQuotaLimitReached)] = true
		}
	}

	for w := range warnSet {
		report.Warnings = append(report.Warnings, w)
	}
	sort.Strings(report.Warnings)

	// Deduplicated remediation. Path-level advisories (proxy setup / NO_PROXY /
	// divergence) go first so they aren't buried under per-target advice.
	remSet := map[string]bool{}
	addRem := func(reason model.ReasonCode) {
		r := verdict.Explain(reason).Remediation
		if r != "" && !remSet[r] {
			remSet[r] = true
			report.Remediation = append(report.Remediation, r)
		}
	}
	for _, reason := range []model.ReasonCode{
		model.ReasonQuotaLimitReached,
		model.ReasonTransparentProxySuspected,
		model.ReasonSystemProxySetButEnvEmpty,
		model.ReasonNoProxyExcludesTarget,
		model.ReasonEnvSystemProxyDiverged,
	} {
		if warnSet[string(reason)] {
			addRem(reason)
		}
	}
	for _, name := range opts.Targets {
		if res, ok := report.Targets[name]; ok {
			addRem(res.ReasonCode)
		}
	}
	return report
}

func determineMode(force string, env map[string]string) model.PathMode {
	switch force {
	case "direct":
		return model.PathDirect
	case "env":
		return model.PathEnv
	case "system":
		return model.PathSystem
	}
	if hasEnvProxy(env) {
		return model.PathEnv
	}
	return model.PathDirect
}

func hasEnvProxy(env map[string]string) bool {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if v, ok := env[k]; ok && v != "" {
			return true
		}
	}
	return false
}

// buildPlan resolves the concrete path a probe should take for host.
//
// Key rule: if the user FORCED a proxy path but it isn't available, we do NOT
// silently fall back to direct — we return a SetupError so the probe fails with
// a clear reason. Only in auto mode (forced == false) does an unavailable proxy
// legitimately mean "go direct". NO_PROXY exclusion always downgrades to direct
// (that is what the real client does) but is surfaced as a warning + per-target
// flag rather than hidden.
func buildPlan(mode model.PathMode, forced bool, env map[string]string, sys checks.SystemProxy, host string) (targets.PathPlan, []string) {
	var warns []string
	switch mode {
	case model.PathEnv:
		pu, excluded, err := checks.SelectEnvProxyForHost(env, host)
		if excluded {
			warns = append(warns, string(model.ReasonNoProxyExcludesTarget))
			return targets.PathPlan{RequestedMode: model.PathEnv, Mode: model.PathDirect, NoProxyExcluded: true}, warns
		}
		if err != nil || pu == nil {
			if forced {
				// Explicitly asked to test env proxy, but there is none. Fail loudly.
				return targets.PathPlan{RequestedMode: model.PathEnv, Mode: model.PathEnv, SetupError: model.ReasonEnvProxyNotConfigured}, warns
			}
			return targets.PathPlan{RequestedMode: model.PathDirect, Mode: model.PathDirect}, warns
		}
		return targets.PathPlan{RequestedMode: model.PathEnv, Mode: model.PathEnv, ProxyURL: pu, ProxyLabel: proxyLabel(pu)}, warns
	case model.PathSystem:
		pu := sys.HTTPSURL()
		if pu == nil {
			if forced {
				return targets.PathPlan{RequestedMode: model.PathSystem, Mode: model.PathSystem, SetupError: model.ReasonSystemProxyNotConfigured}, warns
			}
			return targets.PathPlan{RequestedMode: model.PathDirect, Mode: model.PathDirect}, warns
		}
		return targets.PathPlan{RequestedMode: model.PathSystem, Mode: model.PathSystem, ProxyURL: pu, ProxyLabel: proxyLabel(pu)}, warns
	default:
		return targets.PathPlan{RequestedMode: model.PathDirect, Mode: model.PathDirect}, warns
	}
}

// systemProxySetButEnvEmpty reports the common "browser works, CLI doesn't"
// setup: a system HTTPS proxy is configured but no env proxy is, and we're going
// direct (auto or forced). It never changes a verdict — advisory only.
func systemProxySetButEnvEmpty(mode model.PathMode, env map[string]string, sys checks.SystemProxy) bool {
	return mode == model.PathDirect && sys.HTTPSURL() != nil && !hasEnvProxy(env)
}

// proxyLabel returns a display-safe host:port (userinfo is never part of URL.Host).
func proxyLabel(pu *url.URL) string {
	if pu == nil {
		return ""
	}
	return pu.Host
}

// transparentHintText renders the concrete evidence behind a transparent-proxy
// suspicion into one redaction-safe line (no secrets, only host + fake IP +
// interface name).
func transparentHintText(h checks.TransparentProxyHint) string {
	parts := []string{}
	if h.FakeIPHost != "" && h.FakeIPAddr != "" {
		parts = append(parts, h.FakeIPHost+" 解析到 fake-ip "+h.FakeIPAddr)
	}
	if h.TunnelIface && h.DefaultIface != "" {
		parts = append(parts, "默认路由走隧道接口 "+h.DefaultIface)
	}
	if len(parts) == 0 {
		return "疑似透明代理 / TUN"
	}
	return strings.Join(parts, "；")
}

// proxyDiverged reports whether the env proxy and the system HTTPS proxy point
// to different endpoints (both must be present to diverge).
func proxyDiverged(env map[string]string, sys checks.SystemProxy) bool {
	var e string
	if pu, _, err := checks.SelectEnvProxyForHost(env, "example.com"); err == nil && pu != nil {
		e = pu.Host
	}
	var s string
	if u := sys.HTTPSURL(); u != nil {
		s = u.Host
	}
	if e == "" || s == "" {
		return false
	}
	return e != s
}
