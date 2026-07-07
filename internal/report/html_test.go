package report

import (
	"strings"
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

func TestHTML(t *testing.T) {
	authLayer := model.LayerAuth
	r := model.Report{
		Host:        model.Host{ToolVersion: "0.1.0"},
		GeneratedAt: "2026-07-07T00:00:00+08:00",
		NetworkPath: model.NetworkPath{Mode: model.PathDirect},
		Targets: map[string]model.TargetResult{
			"codex": {
				Verdict: model.VerdictCheck, FailedLayer: &authLayer,
				ReasonCode: model.ReasonAuthRequiredReachable,
				Checks:     []model.Check{{Layer: model.LayerDNS, OK: true, Detail: "1.2.3.4"}},
			},
		},
		Warnings: []string{},
	}
	out := HTML(r, []string{"codex"})

	if !strings.HasPrefix(out, "<!doctype html>") {
		t.Error("missing doctype")
	}
	for _, want := range []string{"ai-net-doctor", "Codex / OpenAI", "CHECK", "1.2.3.4"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	// Self-contained: no external stylesheet / script / font / image loads.
	for _, bad := range []string{"<link", "<script", "src=\"http", "@import"} {
		if strings.Contains(out, bad) {
			t.Errorf("HTML not self-contained, found %q", bad)
		}
	}
}
