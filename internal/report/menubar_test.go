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
