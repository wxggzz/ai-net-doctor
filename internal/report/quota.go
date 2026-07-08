package report

import (
	"fmt"
	"strings"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

// quotaView is the render-ready form of a quota snapshot, shared by the text,
// menu-bar and HTML renderers so they present quota identically.
type quotaView struct {
	Plan    string
	Age     string // "快照 2h 前" or ""
	Blocked bool
	Windows []quotaWindowView
}

type quotaWindowView struct {
	Label   string // "5h" / "weekly"
	Percent int    // rounded used_percent
	Reset   string // "重置 1h43m" / "已重置" / ""
	Expired bool
	Full    bool // >=100 and still valid
}

// buildQuotaView returns the render model and whether there is anything to show.
func buildQuotaView(q *model.Quota) (quotaView, bool) {
	if q == nil || !q.Available || len(q.Windows) == 0 {
		return quotaView{}, false
	}
	v := quotaView{Plan: q.Plan, Blocked: q.SuggestsBlock()}
	if q.SnapshotAge > 0 {
		v.Age = "快照 " + humanDur(q.SnapshotAge) + "前"
	}
	for _, w := range q.Windows {
		wv := quotaWindowView{
			Label:   w.Label,
			Percent: int(w.UsedPercent + 0.5),
			Expired: w.Expired,
			Full:    !w.Expired && w.UsedPercent >= 100,
		}
		switch {
		case w.Expired:
			wv.Reset = "已重置"
		case w.ResetsInSec > 0:
			wv.Reset = "重置 " + humanDur(w.ResetsInSec)
		}
		v.Windows = append(v.Windows, wv)
	}
	return v, true
}

// quotaTextLines renders the compact per-target quota line for text output. In
// non-verbose mode an unavailable snapshot is silent; verbose surfaces its note.
func quotaTextLines(q *model.Quota, verbose bool) []string {
	v, ok := buildQuotaView(q)
	if !ok {
		if verbose && q != nil && !q.Available && q.Note != "" {
			return []string{"        额度: " + q.Note}
		}
		return nil
	}
	var parts []string
	for _, w := range v.Windows {
		seg := fmt.Sprintf("%s %d%%", w.Label, w.Percent)
		if w.Reset != "" {
			seg += " · " + w.Reset
		}
		parts = append(parts, seg)
	}
	var meta []string
	if v.Plan != "" {
		meta = append(meta, v.Plan)
	}
	if v.Age != "" {
		meta = append(meta, v.Age)
	}
	tail := ""
	if len(meta) > 0 {
		tail = "  (" + strings.Join(meta, " · ") + ")"
	}
	icon := "📊"
	if v.Blocked {
		icon = "⛔"
	}
	return []string{"        " + icon + " 额度: " + strings.Join(parts, "  |  ") + tail}
}

// humanDur formats a duration in seconds as a compact "1d2h" / "3h40m" / "12m".
func humanDur(sec int64) string {
	if sec <= 0 {
		return "0m"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	switch {
	case h >= 24:
		return fmt.Sprintf("%dd%dh", h/24, h%24)
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}
