package report

import (
	"fmt"
	"strings"

	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

// MenuBar renders SwiftBar/xbar plugin output: a single colored dot for the menu
// bar, then a dropdown with each target's status and a few quick actions. self
// is the absolute path to this binary (used by the action items); pass "" to
// omit them.
//
// The CLI emits this text directly, so the plugin script needs no jq/python to
// parse JSON — it just runs `ai-net-doctor --menubar`.
func MenuBar(r model.Report, order []string, self string) string {
	var b strings.Builder

	var vs []model.Verdict
	for _, name := range order {
		if res, ok := r.Targets[name]; ok {
			vs = append(vs, res.Verdict)
		}
	}
	overall := verdict.WorstVerdict(vs...)

	// Menu-bar title: the colored health dot, plus a one-cell quota "fuel gauge"
	// when the path is reachable and we have a local quota reading — so a glance
	// shows both "can I connect" and "how much quota is left".
	fmt.Fprintf(&b, "%s\n", menuTitle(r, order, overall))
	b.WriteString("---\n")

	for _, name := range order {
		res, ok := r.Targets[name]
		if !ok {
			continue
		}
		ex := verdict.Explain(res.ReasonCode)
		fmt.Fprintf(&b, "%s  %s — %s | color=%s\n", dot(res.Verdict), displayName(name), res.Verdict, mbColor(res.Verdict))
		fmt.Fprintf(&b, "-- %s | color=#aaaaaa\n", mbClean(ex.Summary))
		if res.Verdict == model.VerdictFail && res.FailedLayer != nil {
			fmt.Fprintf(&b, "-- broke at: %s (%s)\n", *res.FailedLayer, res.ReasonCode)
		}
		if ex.Remediation != "" {
			fmt.Fprintf(&b, "-- → %s | color=#aaaaaa\n", mbClean(ex.Remediation))
		}
		if v, ok := buildQuotaView(res.Quota); ok {
			var parts []string
			for _, w := range v.Windows {
				seg := fmt.Sprintf("%s %d%%", w.Label, w.Percent)
				if w.Reset != "" {
					seg += " · " + w.Reset
				}
				parts = append(parts, seg)
			}
			color := "#8493ad"
			if v.Blocked {
				color = "#d1242f"
			}
			fmt.Fprintf(&b, "-- 额度: %s | color=%s\n", mbClean(strings.Join(parts, "  ·  ")), color)
		}
	}

	b.WriteString("---\n")
	fmt.Fprintf(&b, "Path: %s | color=#888888\n", mbClean(pathModeDesc(r.NetworkPath.Mode, r.NetworkPath)))
	for _, w := range r.Warnings {
		s := verdict.Explain(model.ReasonCode(w)).Summary
		if s == "" {
			s = w
		}
		fmt.Fprintf(&b, "⚠ %s | color=#c8860b\n", mbClean(s))
	}

	b.WriteString("---\n")
	b.WriteString("↻ Re-run now | refresh=true\n")
	if self != "" {
		fmt.Fprintf(&b, "Open detailed report | shell=%q param1=--verbose terminal=true\n", self)
	}
	b.WriteString("Repo & help | href=https://github.com/wxggzz/ai-net-doctor\n")
	return b.String()
}

// menuTitle builds the menu-bar title: the health dot, and — only when the path
// is reachable (not FAIL) and a local quota reading exists — a compact remaining
// indicator. It shows ⛔ when a window is actually spent, otherwise a single
// 8-level "fuel" block for the tightest (most-used, still-valid) window.
func menuTitle(r model.Report, order []string, overall model.Verdict) string {
	title := dot(overall)
	if overall == model.VerdictFail {
		return title
	}
	remaining := -1.0
	spent := false
	for _, name := range order {
		res, ok := r.Targets[name]
		if !ok || res.Quota == nil {
			continue
		}
		if res.Quota.SuggestsBlock() {
			spent = true
		}
		for _, w := range res.Quota.Windows {
			if w.Expired {
				continue
			}
			if rem := 100 - w.UsedPercent; remaining < 0 || rem < remaining {
				remaining = rem
			}
		}
	}
	switch {
	case spent:
		return title + "⛔"
	case remaining >= 0:
		return title + fuelGlyph(remaining)
	default:
		return title
	}
}

// fuelGlyph maps a remaining-quota percentage to one of 8 block heights
// (▁ nearly spent … █ full). Always visible for any remaining > 0.
func fuelGlyph(remainingPct float64) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	idx := int(remainingPct/100*7 + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx > 7 {
		idx = 7
	}
	return string(blocks[idx])
}

// dot is the colored status circle for the menu bar and per-target lines.
func dot(v model.Verdict) string {
	switch v {
	case model.VerdictOK:
		return "🟢"
	case model.VerdictCheck:
		return "🟡"
	case model.VerdictFail:
		return "🔴"
	default:
		return "⚪"
	}
}

func mbColor(v model.Verdict) string {
	switch v {
	case model.VerdictOK:
		return "#2ea043"
	case model.VerdictCheck:
		return "#c8860b"
	case model.VerdictFail:
		return "#d1242f"
	default:
		return "#888888"
	}
}

// mbClean strips characters that would break SwiftBar's line/param syntax.
func mbClean(s string) string {
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
