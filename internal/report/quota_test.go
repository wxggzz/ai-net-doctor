package report

import (
	"strings"
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

func reportWithQuota(q *model.Quota) model.Report {
	return model.Report{
		Host:        model.Host{ToolVersion: "0.1.0"},
		GeneratedAt: "2026-07-08T09:00:00+08:00",
		NetworkPath: model.NetworkPath{Mode: model.PathDirect},
		Targets: map[string]model.TargetResult{
			"codex": {
				Verdict:    model.VerdictCheck,
				ReasonCode: model.ReasonAuthRequiredReachable,
				Checks:     []model.Check{{Layer: model.LayerDNS, OK: true, Detail: "1.2.3.4"}},
				Quota:      q,
			},
		},
		Warnings: []string{},
	}
}

func sampleQuota() *model.Quota {
	return &model.Quota{
		Available:   true,
		Plan:        "plus",
		SnapshotAge: 1800,
		Windows: []model.QuotaWindow{
			{Label: "5h", WindowMinutes: 300, UsedPercent: 4, ResetsInSec: 17832},
			{Label: "weekly", WindowMinutes: 10080, UsedPercent: 47, ResetsInSec: 281068},
		},
	}
}

func TestQuota_TextAndMenuBarAndHTML(t *testing.T) {
	r := reportWithQuota(sampleQuota())
	order := []string{"codex"}

	txt := Text(r, order, false)
	for _, want := range []string{"额度:", "5h 4%", "weekly 47%", "重置", "plus"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text output missing %q\n%s", want, txt)
		}
	}

	mb := MenuBar(r, order, "")
	if !strings.Contains(mb, "-- 额度:") || !strings.Contains(mb, "5h 4%") {
		t.Errorf("menubar output missing quota line:\n%s", mb)
	}

	html := HTML(r, order)
	for _, want := range []string{`class="quota"`, "额度 · quota", "q-fill", ">5h<", ">4%<"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

// A spent, still-valid window flips the fill to the "full" (danger) class and
// marks the panel blocked.
func TestQuota_BlockedRendersFull(t *testing.T) {
	q := &model.Quota{
		Available: true,
		Windows:   []model.QuotaWindow{{Label: "5h", WindowMinutes: 300, UsedPercent: 100, ResetsInSec: 600}},
	}
	if !q.SuggestsBlock() {
		t.Fatal("100% non-expired window should SuggestsBlock")
	}
	html := HTML(reportWithQuota(q), []string{"codex"})
	if !strings.Contains(html, "q-fill q-full") {
		t.Errorf("expected q-full fill for a spent window:\n%s", html)
	}
	if !strings.Contains(html, `class="quota blocked"`) {
		t.Error("expected blocked quota panel")
	}
}

// An expired window keeps its last-known percent but is drawn greyed and labeled
// as reset, and must NOT trigger a block.
func TestQuota_ExpiredIsGreyNotBlocking(t *testing.T) {
	q := &model.Quota{
		Available: true,
		Windows:   []model.QuotaWindow{{Label: "5h", WindowMinutes: 300, UsedPercent: 100, Expired: true, ResetsInSec: -60}},
	}
	if q.SuggestsBlock() {
		t.Error("expired window must not SuggestsBlock")
	}
	html := HTML(reportWithQuota(q), []string{"codex"})
	if !strings.Contains(html, "q-fill q-expired") {
		t.Errorf("expected q-expired fill:\n%s", html)
	}
	if !strings.Contains(html, "已重置") {
		t.Error("expired window should render 已重置")
	}
}

// Unavailable quota (Claude) is silent in the default view, surfaced in verbose.
func TestQuota_UnavailableVerboseOnly(t *testing.T) {
	q := &model.Quota{Available: false, Note: "本地不可得"}
	r := reportWithQuota(q)
	if strings.Contains(Text(r, []string{"codex"}, false), "本地不可得") {
		t.Error("unavailable quota note should be hidden in non-verbose text")
	}
	if !strings.Contains(Text(r, []string{"codex"}, true), "本地不可得") {
		t.Error("unavailable quota note should appear in verbose text")
	}
}

func TestHumanDur(t *testing.T) {
	cases := map[int64]string{
		0:      "0m",
		45:     "45s",
		600:    "10m",
		3660:   "1h1m",
		281068: "3d6h",
	}
	for in, want := range cases {
		if got := humanDur(in); got != want {
			t.Errorf("humanDur(%d) = %q, want %q", in, got, want)
		}
	}
}
