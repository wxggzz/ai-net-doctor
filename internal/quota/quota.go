// Package quota reads AI coding-tool rate-limit / usage windows from the local
// files those tools already write — no API calls, no credentials, no keychain.
//
// It answers a third diagnostic dimension beside network and auth: even when the
// path is fully reachable, a "stream disconnected" can be a spent quota rather
// than a network fault. Codex persists real rate-limit windows into its session
// logs; Claude Code does not persist any locally, so we say so honestly instead
// of guessing (reading Claude's quota would require account credentials or
// browser cookies, which this tool deliberately never touches).
package quota

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

// maxScan bounds how many recent session files we open looking for a snapshot.
const maxScan = 20

// Reader locates and parses local quota snapshots. The zero value works (uses
// the real home dir and time.Now); tests inject Home and Now.
type Reader struct {
	Home string           // defaults to the user's home dir
	Now  func() time.Time // defaults to time.Now
}

func (r Reader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r Reader) home() string {
	if r.Home != "" {
		return r.Home
	}
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// Read returns the quota snapshot for a target id ("codex" / "claude"), or nil
// if the target has no quota concept here.
func (r Reader) Read(target string) *model.Quota {
	switch target {
	case "codex":
		return r.readCodex()
	case "claude":
		return &model.Quota{
			Available: false,
			Note:      "Claude Code 不在本地持久化限流窗口；读取额度需账号凭据 / 浏览器 cookie，本工具不做。",
		}
	default:
		return nil
	}
}

func (r Reader) readCodex() *model.Quota {
	home := r.home()
	if home == "" {
		return &model.Quota{Available: false, Note: "无法定位用户主目录。"}
	}
	dirs := []string{
		filepath.Join(home, ".codex", "sessions"),
		filepath.Join(home, ".codex", "archived_sessions"),
	}
	rl, ts, src := newestRateLimits(dirs)
	if rl == nil {
		return &model.Quota{Available: false, Note: "未找到 Codex 本地额度记录（可能尚未使用过 Codex，或版本较旧）。"}
	}
	now := r.now()
	q := &model.Quota{
		Available:    true,
		Source:       relHome(src, home),
		Plan:         asString(rl["plan_type"]),
		LimitReached: rl["rate_limit_reached_type"] != nil,
	}
	if !ts.IsZero() {
		if age := now.Sub(ts); age > 0 {
			q.SnapshotAge = int64(age.Seconds())
		}
	}
	for _, key := range []string{"primary", "secondary"} {
		if w := window(rl[key], now); w != nil {
			q.Windows = append(q.Windows, *w)
		}
	}
	if len(q.Windows) == 0 {
		return &model.Quota{Available: false, Note: "Codex 额度记录存在但无可解析的窗口字段。"}
	}
	return q
}

// window turns one rate-limit sub-object into a QuotaWindow, or nil if it has no
// usable fields.
func window(v interface{}, now time.Time) *model.QuotaWindow {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	w := &model.QuotaWindow{}
	if f, ok := m["used_percent"].(float64); ok {
		w.UsedPercent = f
	}
	if f, ok := m["window_minutes"].(float64); ok {
		w.WindowMinutes = int(f)
	}
	if f, ok := m["resets_at"].(float64); ok {
		w.ResetsAt = int64(f)
		w.ResetsInSec = w.ResetsAt - now.Unix()
		w.Expired = w.ResetsInSec <= 0
	}
	if w.WindowMinutes == 0 && w.ResetsAt == 0 {
		return nil
	}
	w.Label = windowLabel(w.WindowMinutes)
	return w
}

func windowLabel(min int) string {
	switch min {
	case 300:
		return "5h"
	case 10080:
		return "weekly"
	}
	switch {
	case min <= 0:
		return "window"
	case min%1440 == 0:
		return strconv.Itoa(min/1440) + "d"
	case min%60 == 0:
		return strconv.Itoa(min/60) + "h"
	default:
		return strconv.Itoa(min) + "m"
	}
}

// newestRateLimits walks the session dirs newest-first and returns the last
// rate-limit object from the most recent file that carries one, with the
// snapshot timestamp and the source path.
func newestRateLimits(dirs []string) (map[string]interface{}, time.Time, string) {
	type fe struct {
		path string
		mod  time.Time
	}
	var files []fe
	for _, d := range dirs {
		_ = filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(p, ".jsonl") {
				return nil
			}
			if info, err := e.Info(); err == nil {
				files = append(files, fe{p, info.ModTime()})
			}
			return nil
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	for i, f := range files {
		if i >= maxScan {
			break
		}
		data, err := os.ReadFile(f.path)
		if err != nil || !bytes.Contains(data, []byte("rate_limits")) {
			continue
		}
		if rl, ts := lastRateLimits(data); rl != nil {
			return rl, ts, f.path
		}
	}
	return nil, time.Time{}, ""
}

// lastRateLimits scans a session file and returns the final rate-limit object
// (most recent turn) plus its timestamp.
func lastRateLimits(data []byte) (map[string]interface{}, time.Time) {
	var found map[string]interface{}
	var ts time.Time
	for _, line := range bytes.Split(data, []byte("\n")) {
		if !bytes.Contains(line, []byte("rate_limits")) {
			continue
		}
		var obj interface{}
		if json.Unmarshal(line, &obj) != nil {
			continue
		}
		if rl := findRateLimits(obj); rl != nil {
			found = rl
			ts = parseTimestamp(obj)
		}
	}
	return found, ts
}

// findRateLimits recursively locates a "rate_limits" object anywhere in the
// decoded JSON, since its nesting varies across Codex versions.
func findRateLimits(v interface{}) map[string]interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if rl, ok := t["rate_limits"].(map[string]interface{}); ok && isRateLimits(rl) {
			return rl
		}
		for _, val := range t {
			if r := findRateLimits(val); r != nil {
				return r
			}
		}
	case []interface{}:
		for _, val := range t {
			if r := findRateLimits(val); r != nil {
				return r
			}
		}
	}
	return nil
}

func isRateLimits(m map[string]interface{}) bool {
	_, p := m["primary"]
	_, s := m["secondary"]
	_, pt := m["plan_type"]
	return p || s || pt
}

func parseTimestamp(v interface{}) time.Time {
	m, ok := v.(map[string]interface{})
	if !ok {
		return time.Time{}
	}
	s, ok := m["timestamp"].(string)
	if !ok {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func relHome(p, home string) string {
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
