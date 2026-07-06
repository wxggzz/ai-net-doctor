// Package verdict turns raw probe outcomes into stable verdict / reason codes.
// This is where "is my network able to use Codex / Claude" is decided — never
// in the UI or a skill.
package verdict

import "github.com/wxggzz/ai-net-doctor/internal/model"

// ClassifyHTTP maps an HTTP status received from a target's API endpoint into a
// verdict. The key insight: receiving ANY well-formed HTTP status proves the
// full network path (DNS+TCP+TLS+HTTP, and proxy if used) works. A 401/403 then
// means the problem — if any — is auth/quota, not the network.
func ClassifyHTTP(status int) (model.Verdict, model.ReasonCode, model.Layer) {
	switch {
	case status >= 200 && status < 400:
		// 2xx success, or 3xx redirect — reachable either way.
		return model.VerdictOK, model.ReasonOK, ""
	case status == 401 || status == 403 || status == 429:
		// Reached the auth layer. We intentionally sent no credentials, so a
		// 401/403 is expected and proves the network is fine.
		return model.VerdictCheck, model.ReasonAuthRequiredReachable, model.LayerAuth
	case status == 407:
		return model.VerdictFail, model.ReasonProxyAuthRequired, model.LayerProxy
	default:
		// Any other received status (400/404/405/5xx) still means the HTTP
		// layer answered, i.e. the network path is up.
		return model.VerdictOK, model.ReasonOK, ""
	}
}

var rank = map[model.Verdict]int{
	model.VerdictOK:    0,
	model.VerdictCheck: 1,
	model.VerdictFail:  2,
}

// WorstVerdict returns the most severe verdict among the inputs.
func WorstVerdict(vs ...model.Verdict) model.Verdict {
	worst := model.VerdictOK
	for _, v := range vs {
		if rank[v] > rank[worst] {
			worst = v
		}
	}
	return worst
}

// ExitCode maps an overall verdict to a process exit code.
//
//	0 = all OK, 2 = at least one CHECK (no FAIL), 3 = at least one FAIL.
func ExitCode(v model.Verdict) int {
	switch v {
	case model.VerdictFail:
		return 3
	case model.VerdictCheck:
		return 2
	default:
		return 0
	}
}
