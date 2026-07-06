package verdict

import (
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

func TestClassifyHTTP(t *testing.T) {
	cases := []struct {
		status int
		verd   model.Verdict
		reason model.ReasonCode
		layer  model.Layer
	}{
		{200, model.VerdictOK, model.ReasonOK, ""},
		{301, model.VerdictOK, model.ReasonOK, ""},
		{401, model.VerdictCheck, model.ReasonAuthRequiredReachable, model.LayerAuth},
		{403, model.VerdictCheck, model.ReasonAuthRequiredReachable, model.LayerAuth},
		{429, model.VerdictCheck, model.ReasonAuthRequiredReachable, model.LayerAuth},
		{407, model.VerdictFail, model.ReasonProxyAuthRequired, model.LayerProxy},
		{404, model.VerdictOK, model.ReasonOK, ""}, // reachable
		{500, model.VerdictOK, model.ReasonOK, ""}, // reachable
	}
	for _, c := range cases {
		v, r, l := ClassifyHTTP(c.status)
		if v != c.verd || r != c.reason || l != c.layer {
			t.Errorf("ClassifyHTTP(%d) = (%s,%s,%s), want (%s,%s,%s)",
				c.status, v, r, l, c.verd, c.reason, c.layer)
		}
	}
}

// A 401/403 must be classified as auth-reachable (network fine), never as a
// network failure. This is the tool's core "network vs auth" distinction.
func TestAuthNotNetworkFailure(t *testing.T) {
	for _, status := range []int{401, 403} {
		v, r, _ := ClassifyHTTP(status)
		if v == model.VerdictFail {
			t.Errorf("status %d wrongly classified as FAIL", status)
		}
		if r != model.ReasonAuthRequiredReachable {
			t.Errorf("status %d reason = %s, want AUTH_REQUIRED_REACHABLE", status, r)
		}
	}
}

func TestWorstVerdict(t *testing.T) {
	cases := []struct {
		in   []model.Verdict
		want model.Verdict
	}{
		{[]model.Verdict{model.VerdictOK, model.VerdictOK}, model.VerdictOK},
		{[]model.Verdict{model.VerdictOK, model.VerdictCheck}, model.VerdictCheck},
		{[]model.Verdict{model.VerdictCheck, model.VerdictFail}, model.VerdictFail},
		{[]model.Verdict{}, model.VerdictOK},
	}
	for _, c := range cases {
		if got := WorstVerdict(c.in...); got != c.want {
			t.Errorf("WorstVerdict(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestExitCode(t *testing.T) {
	if ExitCode(model.VerdictOK) != 0 {
		t.Error("OK should be 0")
	}
	if ExitCode(model.VerdictCheck) != 2 {
		t.Error("CHECK should be 2")
	}
	if ExitCode(model.VerdictFail) != 3 {
		t.Error("FAIL should be 3")
	}
}

func TestExplainHasRemediationForFailures(t *testing.T) {
	for _, r := range []model.ReasonCode{
		model.ReasonDNSResolveFailed,
		model.ReasonTCPConnectTimeout,
		model.ReasonProxyConnectFailed,
		model.ReasonProxyAuthRequired,
	} {
		ex := Explain(r)
		if ex.Summary == "" || ex.Remediation == "" {
			t.Errorf("Explain(%s) missing text: %+v", r, ex)
		}
	}
}
