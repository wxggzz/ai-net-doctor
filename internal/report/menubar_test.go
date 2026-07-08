package report

import (
	"strings"
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

func TestMenuBar(t *testing.T) {
	authLayer := model.LayerAuth
	tcpLayer := model.LayerTCP
	r := model.Report{
		NetworkPath: model.NetworkPath{Mode: model.PathDirect},
		Targets: map[string]model.TargetResult{
			"codex":  {Verdict: model.VerdictCheck, FailedLayer: &authLayer, ReasonCode: model.ReasonAuthRequiredReachable},
			"claude": {Verdict: model.VerdictFail, FailedLayer: &tcpLayer, ReasonCode: model.ReasonTCPConnectTimeout},
		},
		Warnings: []string{},
	}
	out := MenuBar(r, []string{"codex", "claude"}, "")
	lines := strings.Split(out, "\n")

	// Menu-bar title (first line) is the worst-verdict dot: FAIL -> 🔴.
	if lines[0] != "🔴" {
		t.Errorf("menu bar dot = %q, want 🔴", lines[0])
	}
	// A CHECK (auth reachable) is not a failure — it must NOT show "broke at".
	if strings.Contains(out, "broke at: auth") {
		t.Errorf("CHECK should not show a breakpoint:\n%s", out)
	}
	// A real FAIL must show its breakpoint layer.
	if !strings.Contains(out, "broke at: tcp") {
		t.Errorf("FAIL should show 'broke at: tcp':\n%s", out)
	}
	// SwiftBar structure: a separator and a refresh action.
	if !strings.Contains(out, "\n---\n") || !strings.Contains(out, "refresh=true") {
		t.Errorf("missing separator or refresh action:\n%s", out)
	}
}

// A reachable target with a quota reading appends a one-cell fuel gauge to the
// dot; a spent window shows ⛔ instead.
func TestMenuBarTitle_QuotaGauge(t *testing.T) {
	base := func(q *model.Quota) model.Report {
		return model.Report{
			NetworkPath: model.NetworkPath{Mode: model.PathDirect},
			Targets: map[string]model.TargetResult{
				"codex": {Verdict: model.VerdictCheck, ReasonCode: model.ReasonAuthRequiredReachable, Quota: q},
			},
			Warnings: []string{},
		}
	}

	// 47% used -> 53% remaining -> a mid-level block, prefixed by the 🟡 dot.
	half := base(&model.Quota{Available: true, Windows: []model.QuotaWindow{
		{Label: "weekly", WindowMinutes: 10080, UsedPercent: 47, ResetsInSec: 1000},
	}})
	title := strings.Split(MenuBar(half, []string{"codex"}, ""), "\n")[0]
	if !strings.HasPrefix(title, "🟡") || title == "🟡" {
		t.Errorf("expected dot + gauge, got %q", title)
	}
	if strings.ContainsAny(title, "⛔") {
		t.Errorf("healthy quota should not show ⛔, got %q", title)
	}

	// A spent (100%, still-valid) window -> ⛔ marker.
	spent := base(&model.Quota{Available: true, Windows: []model.QuotaWindow{
		{Label: "5h", WindowMinutes: 300, UsedPercent: 100, ResetsInSec: 600},
	}})
	if title := strings.Split(MenuBar(spent, []string{"codex"}, ""), "\n")[0]; !strings.Contains(title, "⛔") {
		t.Errorf("spent quota should show ⛔ in title, got %q", title)
	}

	// No quota data -> title is just the dot (unchanged behavior).
	none := base(nil)
	if title := strings.Split(MenuBar(none, []string{"codex"}, ""), "\n")[0]; title != "🟡" {
		t.Errorf("no quota should leave title as bare dot, got %q", title)
	}
}
