package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

func sampleReport() model.Report {
	authLayer := model.LayerAuth
	return model.Report{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   "2026-07-04T12:00:00+08:00",
		Host:          model.Host{OS: "darwin", Arch: "arm64", ToolVersion: model.Version},
		NetworkPath:   model.NetworkPath{Mode: model.PathDirect, ProxyEnv: map[string]string{}, SystemProxy: map[string]string{}},
		Targets: map[string]model.TargetResult{
			"codex": {Verdict: model.VerdictOK, FailedLayer: nil, ReasonCode: model.ReasonOK, LatencyMs: 120},
			"claude": {Verdict: model.VerdictCheck, FailedLayer: &authLayer, ReasonCode: model.ReasonAuthRequiredReachable, LatencyMs: 300,
				Checks: []model.Check{{Name: "dns", Layer: model.LayerDNS, OK: true}}},
		},
		CredentialsPresent: map[string]bool{"ANTHROPIC_API_KEY": false},
		Warnings:           []string{},
		Remediation:        []string{},
	}
}

func TestJSONSchemaFields(t *testing.T) {
	out, err := JSON(sampleReport())
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	for _, k := range []string{"schema_version", "generated_at", "host", "network_path", "targets", "credentials_present", "warnings", "remediation"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing top-level field %q", k)
		}
	}
	if m["schema_version"] != "1" {
		t.Errorf("schema_version = %v, want \"1\"", m["schema_version"])
	}
}

func TestJSONFailedLayerNullWhenNil(t *testing.T) {
	out, err := JSON(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	// codex has no failed layer -> must serialize as JSON null, not "".
	if !strings.Contains(out, `"failed_layer": null`) {
		t.Errorf("expected a null failed_layer in output:\n%s", out)
	}
}

func TestJSONNeverLeaksSecretValues(t *testing.T) {
	// credentials_present must be booleans only.
	out, _ := JSON(sampleReport())
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	creds, _ := m["credentials_present"].(map[string]any)
	for k, v := range creds {
		if _, ok := v.(bool); !ok {
			t.Errorf("credentials_present[%q] = %T, want bool", k, v)
		}
	}
}
