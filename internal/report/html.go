package report

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

// HTML renders a self-contained, dependency-free report page (inline CSS, no
// external fonts/scripts/images) suitable for opening in a browser. It is
// display-only: every conclusion still comes from the CLI.
func HTML(r model.Report, order []string) string {
	v := buildHTMLView(r, order)
	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, v); err != nil {
		// Templates are static; an error here is a programming bug. Fall back to
		// a minimal page rather than crashing a double-click entry point.
		return "<!doctype html><meta charset=utf-8><pre>" + template.HTMLEscapeString(err.Error()) + "</pre>"
	}
	return buf.String()
}

type htmlView struct {
	Version     string
	GeneratedAt string
	Overall     string
	OverallDot  string
	PathLabel   string
	PathDesc    string
	Warnings    []string
	Targets     []htmlTarget
}

type htmlTarget struct {
	Name        string
	Verdict     string
	Class       string
	Dot         string
	Summary     string
	Remediation string
	NoProxyNote string
	Checks      []htmlCheck
}

type htmlCheck struct {
	Layer      string
	Mark       string
	MarkClass  string
	Timing     string
	Detail     string
	Breakpoint bool
}

func buildHTMLView(r model.Report, order []string) htmlView {
	var verdicts []model.Verdict
	for _, name := range order {
		if res, ok := r.Targets[name]; ok {
			verdicts = append(verdicts, res.Verdict)
		}
	}
	overall := verdict.WorstVerdict(verdicts...)

	pathLabel := "自动选择路径"
	if r.NetworkPath.Forced {
		pathLabel = "请求路径"
	}

	var warnings []string
	for _, w := range r.Warnings {
		text := verdict.Explain(model.ReasonCode(w)).Summary
		if text == "" {
			text = w
		}
		if w == string(model.ReasonTransparentProxySuspected) && r.NetworkPath.TransparentProxyHint != "" {
			text += "（依据：" + r.NetworkPath.TransparentProxyHint + "）"
		}
		warnings = append(warnings, text)
	}

	var targets []htmlTarget
	for _, name := range order {
		res, ok := r.Targets[name]
		if !ok {
			continue
		}
		ex := verdict.Explain(res.ReasonCode)
		t := htmlTarget{
			Name:        displayName(name),
			Verdict:     string(res.Verdict),
			Class:       verdictClass(res.Verdict),
			Dot:         dot(res.Verdict),
			Summary:     ex.Summary,
			Remediation: ex.Remediation,
		}
		if res.NoProxyExcluded {
			t.NoProxyNote = "NO_PROXY 排除了 " + res.Host + "，该 target 实际未走代理，而是 direct。"
		}
		var failed model.Layer
		if res.FailedLayer != nil {
			failed = *res.FailedLayer
		}
		for _, c := range res.Checks {
			hc := htmlCheck{Layer: string(c.Layer)}
			switch {
			case c.Skipped:
				hc.Mark, hc.MarkClass = "—", "skip"
			case c.OK:
				hc.Mark, hc.MarkClass = "✓", "ok"
			default:
				hc.Mark, hc.MarkClass = "✗", "fail"
			}
			if !c.Skipped {
				hc.Timing = fmt.Sprintf("%dms", c.ElapsedMs)
			}
			hc.Detail = c.Detail
			if c.Error != "" {
				hc.Detail = c.Error
			}
			hc.Breakpoint = !c.Skipped && !c.OK && res.Verdict == model.VerdictFail && c.Layer == failed
			t.Checks = append(t.Checks, hc)
		}
		targets = append(targets, t)
	}

	return htmlView{
		Version:     r.Host.ToolVersion,
		GeneratedAt: r.GeneratedAt,
		Overall:     string(overall),
		OverallDot:  dot(overall),
		PathLabel:   pathLabel,
		PathDesc:    pathModeDesc(r.NetworkPath.Mode, r.NetworkPath),
		Warnings:    warnings,
		Targets:     targets,
	}
}

func verdictClass(v model.Verdict) string {
	switch v {
	case model.VerdictOK:
		return "ok"
	case model.VerdictCheck:
		return "check"
	case model.VerdictFail:
		return "fail"
	default:
		return "skip"
	}
}

var htmlTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ai-net-doctor 报告</title>
<style>
  :root {
    --bg: #f6f8fa; --card: #ffffff; --fg: #1f2328; --muted: #6e7781;
    --border: #d0d7de; --ok: #2ea043; --check: #bf8700; --fail: #cf222e; --skip: #8c959f;
    --shadow: 0 1px 3px rgba(0,0,0,.08);
  }
  @media (prefers-color-scheme: dark) {
    :root { --bg:#0d1117; --card:#161b22; --fg:#e6edf3; --muted:#8b949e;
      --border:#30363d; --ok:#3fb950; --check:#d29922; --fail:#f85149; --skip:#6e7681;
      --shadow: 0 1px 3px rgba(0,0,0,.4); }
  }
  * { box-sizing: border-box; }
  body { margin:0; background:var(--bg); color:var(--fg);
    font-family: ui-sans-serif,-apple-system,"Segoe UI",Roboto,"PingFang SC","Microsoft YaHei",sans-serif;
    line-height:1.55; padding:24px; }
  .wrap { max-width:760px; margin:0 auto; }
  header { display:flex; align-items:baseline; justify-content:space-between; flex-wrap:wrap; gap:8px; margin-bottom:4px; }
  h1 { font-size:20px; margin:0; font-weight:650; }
  .sub { color:var(--muted); font-size:13px; }
  .overall { font-size:15px; font-weight:600; margin:12px 0 20px; }
  .banner { background:var(--card); border:1px solid var(--border); border-left:4px solid var(--check);
    border-radius:8px; padding:10px 14px; margin:8px 0; font-size:14px; box-shadow:var(--shadow); }
  .card { background:var(--card); border:1px solid var(--border); border-radius:10px;
    padding:16px 18px; margin:14px 0; box-shadow:var(--shadow); border-left:5px solid var(--skip); }
  .card.ok { border-left-color:var(--ok); }
  .card.check { border-left-color:var(--check); }
  .card.fail { border-left-color:var(--fail); }
  .card h2 { font-size:16px; margin:0 0 6px; display:flex; align-items:center; gap:8px; }
  .badge { font-size:12px; font-weight:700; padding:2px 8px; border-radius:999px; color:#fff; }
  .badge.ok { background:var(--ok); } .badge.check { background:var(--check); } .badge.fail { background:var(--fail); }
  .summary { font-size:14px; margin:6px 0; }
  .rem { font-size:13px; color:var(--muted); margin:6px 0; }
  .noproxy { font-size:13px; color:var(--fail); margin:6px 0; }
  .fall { font-family: ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; font-size:12.5px;
    margin-top:10px; border-top:1px solid var(--border); padding-top:10px; overflow-x:auto; }
  .row { display:flex; gap:10px; white-space:nowrap; padding:1px 0; }
  .mk { width:1.2em; text-align:center; }
  .mk.ok { color:var(--ok); } .mk.fail { color:var(--fail); } .mk.skip { color:var(--skip); }
  .lyr { width:3.5em; color:var(--muted); }
  .tm { width:5em; color:var(--skip); text-align:right; }
  .dt { color:var(--fg); }
  .brk { color:var(--fail); font-weight:700; margin-left:6px; }
  footer { color:var(--muted); font-size:12px; margin-top:22px; text-align:center; }
  footer a { color:var(--muted); }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>ai-net-doctor <span class="sub">v{{.Version}}</span></h1>
    <span class="sub">{{.GeneratedAt}}</span>
  </header>
  <div class="sub">{{.PathLabel}}：{{.PathDesc}}</div>
  <div class="overall">总体：{{.OverallDot}} {{.Overall}}</div>

  {{range .Warnings}}<div class="banner">⚠️ {{.}}</div>{{end}}

  {{range .Targets}}
  <div class="card {{.Class}}">
    <h2>{{.Dot}} {{.Name}} <span class="badge {{.Class}}">{{.Verdict}}</span></h2>
    <div class="summary">{{.Summary}}</div>
    {{if .NoProxyNote}}<div class="noproxy">⚠️ {{.NoProxyNote}}</div>{{end}}
    {{if .Remediation}}<div class="rem">→ {{.Remediation}}</div>{{end}}
    <div class="fall">
      {{range .Checks}}<div class="row"><span class="mk {{.MarkClass}}">{{.Mark}}</span><span class="lyr">{{.Layer}}</span><span class="tm">{{.Timing}}</span><span class="dt">{{.Detail}}{{if .Breakpoint}}<span class="brk">← 断点</span>{{end}}</span></div>
      {{end}}
    </div>
  </div>
  {{end}}

  <footer>
    诊断不发送你的密钥；结论由 ai-net-doctor CLI 计算，本页仅展示。<br>
    <a href="https://github.com/wxggzz/ai-net-doctor">github.com/wxggzz/ai-net-doctor</a>
  </footer>
</div>
</body>
</html>
`))
