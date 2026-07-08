package quota

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC) }

// writeCodexSession drops a session rollout file carrying a rate_limits event.
func writeCodexSession(t *testing.T, home string, primaryReset, secondaryReset int64, ts string, reached interface{}) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "archived_sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	reachedJSON := "null"
	if s, ok := reached.(string); ok {
		reachedJSON = fmt.Sprintf("%q", s)
	}
	line := fmt.Sprintf(`{"timestamp":%q,"event_msg":{"type":"token_count","rate_limits":{`+
		`"limit_id":"codex","plan_type":"plus","rate_limit_reached_type":%s,`+
		`"primary":{"used_percent":31.0,"window_minutes":300,"resets_at":%d},`+
		`"secondary":{"used_percent":47.0,"window_minutes":10080,"resets_at":%d}}}}`,
		ts, reachedJSON, primaryReset, secondaryReset)
	// A benign earlier line without rate_limits, to prove we scan for the right one.
	body := `{"timestamp":"2026-07-08T08:00:00Z","event_msg":{"type":"message"}}` + "\n" + line + "\n"
	f := filepath.Join(dir, "rollout-2026-07-08T08-30-00-abc.jsonl")
	if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadCodex_Windows(t *testing.T) {
	home := t.TempDir()
	now := fixedNow()
	writeCodexSession(t, home,
		now.Add(2*time.Hour).Unix(),  // primary still valid
		now.Add(-1*time.Hour).Unix(), // secondary already reset
		now.Add(-30*time.Minute).Format(time.RFC3339),
		nil)

	q := Reader{Home: home, Now: fixedNow}.Read("codex")
	if q == nil || !q.Available {
		t.Fatalf("expected available codex quota, got %+v", q)
	}
	if q.Plan != "plus" {
		t.Errorf("plan = %q, want plus", q.Plan)
	}
	if len(q.Windows) != 2 {
		t.Fatalf("windows = %d, want 2", len(q.Windows))
	}
	p := q.Windows[0]
	if p.Label != "5h" || p.UsedPercent != 31 || p.Expired {
		t.Errorf("primary = %+v, want 5h 31%% not-expired", p)
	}
	if p.ResetsInSec <= 0 || p.ResetsInSec > 2*3600 {
		t.Errorf("primary ResetsInSec = %d, want ~7200", p.ResetsInSec)
	}
	s := q.Windows[1]
	if s.Label != "weekly" || !s.Expired {
		t.Errorf("secondary = %+v, want weekly expired", s)
	}
	if q.SnapshotAge < 1700 || q.SnapshotAge > 1900 {
		t.Errorf("SnapshotAge = %d, want ~1800", q.SnapshotAge)
	}
	if q.SuggestsBlock() {
		t.Error("SuggestsBlock should be false (no full window, not reached)")
	}
}

func TestSuggestsBlock_ReachedFresh(t *testing.T) {
	home := t.TempDir()
	now := fixedNow()
	writeCodexSession(t, home,
		now.Add(2*time.Hour).Unix(),
		now.Add(48*time.Hour).Unix(),
		now.Add(-10*time.Minute).Format(time.RFC3339), // fresh snapshot
		"primary")
	q := Reader{Home: home, Now: fixedNow}.Read("codex")
	if !q.LimitReached {
		t.Fatal("LimitReached should be true")
	}
	if !q.SuggestsBlock() {
		t.Error("fresh limit-reached snapshot should SuggestsBlock")
	}
}

func TestSuggestsBlock_ReachedStale(t *testing.T) {
	home := t.TempDir()
	now := fixedNow()
	writeCodexSession(t, home,
		now.Add(2*time.Hour).Unix(),
		now.Add(48*time.Hour).Unix(),
		now.Add(-5*time.Hour).Format(time.RFC3339), // stale: >1h old
		"primary")
	q := Reader{Home: home, Now: fixedNow}.Read("codex")
	if q.SuggestsBlock() {
		t.Error("stale limit-reached snapshot must NOT SuggestsBlock")
	}
}

func TestReadClaude_Unavailable(t *testing.T) {
	q := Reader{Home: t.TempDir(), Now: fixedNow}.Read("claude")
	if q == nil || q.Available {
		t.Fatalf("claude quota should be unavailable, got %+v", q)
	}
	if q.Note == "" {
		t.Error("claude unavailable quota should carry an explanatory note")
	}
}

func TestReadCodex_NoData(t *testing.T) {
	q := Reader{Home: t.TempDir(), Now: fixedNow}.Read("codex")
	if q == nil || q.Available {
		t.Fatalf("empty home should yield unavailable codex quota, got %+v", q)
	}
}

func TestRead_UnknownTarget(t *testing.T) {
	if q := (Reader{Home: t.TempDir()}).Read("gemini"); q != nil {
		t.Errorf("unknown target should be nil, got %+v", q)
	}
}
