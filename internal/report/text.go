package report

import (
	"fmt"
	"strings"

	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

// Text renders a human-facing report. order fixes the target display order;
// verbose adds a per-layer waterfall with the first breakpoint marked.
func Text(r model.Report, order []string, verbose bool) string {
	var b strings.Builder

	pathLabel := "自动选择路径"
	if r.NetworkPath.Forced {
		pathLabel = "请求路径"
	}
	fmt.Fprintf(&b, "ai-net-doctor %s  —  %s: %s\n", r.Host.ToolVersion, pathLabel, pathDesc(r.NetworkPath))
	if r.NetworkPath.Diverged {
		b.WriteString("  ⚠️  环境变量代理与系统代理不一致（CLI 吃 env、浏览器吃系统代理）\n")
	}
	if hasWarning(r, model.ReasonSystemProxySetButEnvEmpty) {
		b.WriteString("  ⚠️  检测到系统代理已开启，但环境变量代理为空；浏览器可能能上，但 Codex/Claude CLI 可能不走代理\n")
	}
	if r.NetworkPath.TransparentProxySuspected {
		b.WriteString("  ⚠️  疑似透明代理 / TUN（fake-ip）——下方“direct”结果可能实际走了隧道\n")
		if r.NetworkPath.TransparentProxyHint != "" {
			fmt.Fprintf(&b, "        依据: %s\n", r.NetworkPath.TransparentProxyHint)
		}
	}
	b.WriteString("\n")

	var verdicts []model.Verdict
	for _, name := range order {
		res, ok := r.Targets[name]
		if !ok {
			continue
		}
		verdicts = append(verdicts, res.Verdict)
		ex := verdict.Explain(res.ReasonCode)
		fmt.Fprintf(&b, "%s  %-5s  %s\n", symbol(res.Verdict), res.Verdict, displayName(name))
		fmt.Fprintf(&b, "        %s\n", ex.Summary)
		// The requested path was bypassed for this target: say so explicitly, or
		// the user is misled into thinking the proxy was tested.
		if res.NoProxyExcluded {
			fmt.Fprintf(&b, "        ⚠️  NO_PROXY 排除了 %s，该 target 实际未走代理，而是 direct\n", res.Host)
		}
		if ex.Remediation != "" {
			fmt.Fprintf(&b, "        → %s\n", ex.Remediation)
		}
		if verbose {
			fmt.Fprintf(&b, "        实际路径: %s\n", pathModeDesc(res.PathMode, r.NetworkPath))
			b.WriteString(waterfall(res))
		}
		b.WriteString("\n")
	}

	overall := verdict.WorstVerdict(verdicts...)
	fmt.Fprintf(&b, "总体: %s %s\n", symbol(overall), overall)
	return b.String()
}

func waterfall(res model.TargetResult) string {
	var b strings.Builder
	var failed model.Layer
	if res.FailedLayer != nil {
		failed = *res.FailedLayer
	}
	for _, c := range res.Checks {
		var mark, note string
		switch {
		case c.Skipped:
			mark = "—"
			note = "(skipped)"
		case c.OK:
			mark = "✓"
		default:
			mark = "✗"
		}
		timing := ""
		if !c.Skipped {
			timing = fmt.Sprintf("%dms", c.ElapsedMs)
		}
		detail := c.Detail
		if c.Error != "" {
			detail = c.Error
		}
		line := fmt.Sprintf("          %s %-6s %-7s %s", mark, c.Layer, timing, detail)
		if note != "" {
			line += " " + note
		}
		if !c.Skipped && !c.OK && c.Layer == failed {
			line += "   ← 断点"
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	return b.String()
}

func symbol(v model.Verdict) string {
	switch v {
	case model.VerdictOK:
		return "✅"
	case model.VerdictCheck:
		return "⚠️ "
	case model.VerdictFail:
		return "❌"
	default:
		return "•"
	}
}

func displayName(name string) string {
	switch name {
	case "codex":
		return "Codex / OpenAI"
	case "claude":
		return "Claude Code / Anthropic"
	default:
		return name
	}
}

func pathDesc(np model.NetworkPath) string {
	return pathModeDesc(np.Mode, np)
}

// pathModeDesc renders any path mode (not just np.Mode) using the proxy details
// carried in np, so a target's actual path can be shown even when it differs
// from the requested one.
func pathModeDesc(mode model.PathMode, np model.NetworkPath) string {
	switch mode {
	case model.PathEnv:
		if p, ok := firstOf(np.ProxyEnv, "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "HTTP_PROXY", "http_proxy"); ok {
			return "env proxy (" + p + ")"
		}
		return "env proxy"
	case model.PathSystem:
		if p, ok := np.SystemProxy["HTTPSProxy"]; ok {
			return "system proxy (" + p + ")"
		}
		return "system proxy"
	default:
		return "direct"
	}
}

// hasWarning reports whether reason is present in the report's warnings list.
func hasWarning(r model.Report, reason model.ReasonCode) bool {
	for _, w := range r.Warnings {
		if w == string(reason) {
			return true
		}
	}
	return false
}

func firstOf(m map[string]string, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v, true
		}
	}
	return "", false
}
