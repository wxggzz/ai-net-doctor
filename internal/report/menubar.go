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

	// Menu-bar title: just the colored dot.
	fmt.Fprintf(&b, "%s\n", dot(overall))
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
