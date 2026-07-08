package report

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

// HTML renders a self-contained, dependency-free report page (inline CSS, no
// external fonts/scripts/images) themed as a connectivity "vitals" panel: each
// target's layered probe is drawn as a signal path that flows green to the
// first break. It is display-only; every conclusion comes from the CLI.
func HTML(r model.Report, order []string) string {
	v := buildHTMLView(r, order)
	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, v); err != nil {
		return "<!doctype html><meta charset=utf-8><pre>" + template.HTMLEscapeString(err.Error()) + "</pre>"
	}
	return buf.String()
}

type htmlView struct {
	Version         string
	GeneratedAt     string
	Overall         string // verdict word (OK/CHECK/FAIL)
	OverallState    string // ok/check/fail
	OverallTitle    string // plain-language headline
	OverallSubtitle string
	PathDesc        string
	Advisories      []string
	Targets         []htmlTarget
}

type htmlTarget struct {
	Name         string
	Verdict      string
	State        string
	Latency      string
	Summary      string
	Remediation  string
	NoProxyNote  string
	ErrorCallout string
	Nodes        []htmlNode
	Timeline     []htmlSeg // per-phase latency segments (where the time went)
	TotalMs      int64
	Quota        *htmlQuota // locally-read rate-limit windows (nil = none to show)
	Raw          []htmlCheck
}

// htmlQuota is the render model for a target's quota panel.
type htmlQuota struct {
	Blocked bool
	Meta    string // "plus · 快照 2h前"
	Windows []htmlQuotaWin
}

type htmlQuotaWin struct {
	Label   string
	Percent int
	Reset   string
	Class   string       // fill color class: q-normal / q-high / q-full / q-expired
	Style   template.CSS // pre-computed "width:NN%" (trusted CSS)
}

// htmlSeg is one phase in a target's latency waterfall.
type htmlSeg struct {
	Class string
	Label string
	Ms    int64
	Style template.CSS // pre-computed "width:NN%" (trusted CSS)
}

type phaseMs struct {
	layer model.Layer
	ms    int64
}

type htmlNode struct {
	Layer      string // uppercased acronym
	State      string // ok/fail/skip
	Mark       string // ✓ / ✕ / –
	Ring       bool   // reached but not verified (auth on CHECK)
	Timing     string
	LinkActive bool // the connector leading into this node is "live" (green)
}

type htmlCheck struct {
	Layer     string
	Mark      string
	MarkClass string
	Timing    string
	Detail    string
}

func buildHTMLView(r model.Report, order []string) htmlView {
	var verdicts []model.Verdict
	for _, name := range order {
		if res, ok := r.Targets[name]; ok {
			verdicts = append(verdicts, res.Verdict)
		}
	}
	overall := verdict.WorstVerdict(verdicts...)

	title, sub := overallCopy(overall)

	var advisories []string
	for _, w := range r.Warnings {
		text := verdict.Explain(model.ReasonCode(w)).Summary
		if text == "" {
			text = w
		}
		if w == string(model.ReasonTransparentProxySuspected) && r.NetworkPath.TransparentProxyHint != "" {
			text += "（依据：" + r.NetworkPath.TransparentProxyHint + "）"
		}
		advisories = append(advisories, text)
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
			State:       verdictClass(res.Verdict),
			Latency:     fmtLatency(res.LatencyMs),
			Summary:     ex.Summary,
			Remediation: ex.Remediation,
		}
		if res.NoProxyExcluded {
			t.NoProxyNote = "NO_PROXY 排除了 " + res.Host + "，该 target 实际未走代理，而是直连。"
		}

		var failed model.Layer
		if res.FailedLayer != nil {
			failed = *res.FailedLayer
		}
		prevOK := false
		var phases []phaseMs
		for _, c := range res.Checks {
			// Signal-path node.
			n := htmlNode{Layer: upperLayer(c.Layer), LinkActive: prevOK}
			switch {
			case c.Skipped:
				n.State, n.Mark = "skip", "–"
			case c.OK:
				n.State, n.Mark = "ok", "✓"
			default:
				n.State, n.Mark = "fail", "✕"
			}
			if !c.Skipped {
				n.Timing = fmt.Sprintf("%dms", c.ElapsedMs)
			}
			// Auth reached but not verified (CHECK): green node with an amber ring.
			if res.Verdict == model.VerdictCheck && c.Layer == model.LayerAuth && c.OK {
				n.Ring = true
			}
			t.Nodes = append(t.Nodes, n)
			prevOK = n.State == "ok"

			// Raw waterfall row.
			rc := htmlCheck{Layer: string(c.Layer), Timing: n.Timing}
			switch {
			case c.Skipped:
				rc.Mark, rc.MarkClass = "—", "skip"
			case c.OK:
				rc.Mark, rc.MarkClass = "✓", "ok"
			default:
				rc.Mark, rc.MarkClass = "✗", "fail"
			}
			rc.Detail = c.Detail
			if c.Error != "" {
				rc.Detail = c.Error
			}
			t.Raw = append(t.Raw, rc)

			if !c.Skipped && c.ElapsedMs > 0 {
				phases = append(phases, phaseMs{c.Layer, c.ElapsedMs})
			}

			// Error callout = the breakpoint's detail (FAIL only).
			if res.Verdict == model.VerdictFail && !c.Skipped && !c.OK && c.Layer == failed {
				if c.Error != "" {
					t.ErrorCallout = c.Error
				} else {
					t.ErrorCallout = c.Detail
				}
			}
		}
		var total int64
		for _, p := range phases {
			total += p.ms
		}
		if total > 0 {
			t.TotalMs = total
			for _, p := range phases {
				t.Timeline = append(t.Timeline, htmlSeg{
					Class: segClass(p.layer),
					Label: upperLayer(p.layer),
					Ms:    p.ms,
					Style: template.CSS(fmt.Sprintf("width:%.3f%%", float64(p.ms)/float64(total)*100)),
				})
			}
		}
		if qv, ok := buildQuotaView(res.Quota); ok {
			hq := &htmlQuota{Blocked: qv.Blocked}
			var meta []string
			if qv.Plan != "" {
				meta = append(meta, qv.Plan)
			}
			if qv.Age != "" {
				meta = append(meta, qv.Age)
			}
			hq.Meta = strings.Join(meta, " · ")
			for _, w := range qv.Windows {
				cls := "q-normal"
				switch {
				case w.Expired:
					cls = "q-expired"
				case w.Full:
					cls = "q-full"
				case w.Percent >= 80:
					cls = "q-high"
				}
				width := w.Percent
				if width > 100 {
					width = 100
				}
				hq.Windows = append(hq.Windows, htmlQuotaWin{
					Label:   w.Label,
					Percent: w.Percent,
					Reset:   w.Reset,
					Class:   cls,
					Style:   template.CSS(fmt.Sprintf("width:%d%%", width)),
				})
			}
			t.Quota = hq
		}
		targets = append(targets, t)
	}

	return htmlView{
		Version:         r.Host.ToolVersion,
		GeneratedAt:     r.GeneratedAt,
		Overall:         string(overall),
		OverallState:    verdictClass(overall),
		OverallTitle:    title,
		OverallSubtitle: sub,
		PathDesc:        pathModeDesc(r.NetworkPath.Mode, r.NetworkPath),
		Advisories:      advisories,
		Targets:         targets,
	}
}

func overallCopy(v model.Verdict) (title, sub string) {
	switch v {
	case model.VerdictOK:
		return "全部可用", "Codex 与 Claude 均可正常访问。"
	case model.VerdictFail:
		return "连接受阻", "有目标不可达——见下方链路断点。"
	default:
		return "网络可达", "链路全通；返回 401 属正常——诊断不会使用你的密钥，问题若有在认证或额度。"
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

func upperLayer(l model.Layer) string {
	switch l {
	case model.LayerDNS:
		return "DNS"
	case model.LayerTCP:
		return "TCP"
	case model.LayerTLS:
		return "TLS"
	case model.LayerHTTP:
		return "HTTP"
	case model.LayerAuth:
		return "AUTH"
	case model.LayerProxy:
		return "PROXY"
	default:
		return string(l)
	}
}

// segClass maps a layer to its latency-waterfall color (a cool teal→violet ramp,
// deliberately distinct from the green/amber/red status language).
func segClass(l model.Layer) string {
	switch l {
	case model.LayerDNS:
		return "seg-dns"
	case model.LayerTCP:
		return "seg-tcp"
	case model.LayerProxy:
		return "seg-proxy"
	case model.LayerTLS:
		return "seg-tls"
	case model.LayerHTTP:
		return "seg-http"
	default:
		return "seg-tcp"
	}
}

func fmtLatency(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	return fmt.Sprintf("%.1f s", float64(ms)/1000)
}

var htmlTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ai-net-doctor · 连通性报告</title>
<style>
  :root{
    --bg:#0b0f18; --grid:#111726; --panel:#131b2b; --inset:#0e1523; --border:#243149;
    --text:#e7edf7; --muted:#8493ad; --faint:#4a5878; --track:#2b3854;
    --ok:#34d399; --warn:#f4c150; --fail:#fb6f6c;
    --mono:ui-monospace,SFMono-Regular,"SF Mono",Menlo,Consolas,monospace;
    --sans:ui-sans-serif,-apple-system,"Segoe UI",Roboto,"PingFang SC","Microsoft YaHei",sans-serif;
  }
  @media (prefers-color-scheme: light){
    :root{ --bg:#eef1f7; --grid:#e5e9f2; --panel:#ffffff; --inset:#f3f6fb; --border:#d9e0ee;
      --text:#16202f; --muted:#586178; --faint:#95a0b6; --track:#d3dbea;
      --ok:#0f9d63; --warn:#b07d13; --fail:#d94643; }
  }
  *{box-sizing:border-box}
  html{-webkit-text-size-adjust:100%}
  body{margin:0;background:
      radial-gradient(1100px 500px at 50% -10%, var(--grid), transparent 70%), var(--bg);
    color:var(--text);font-family:var(--sans);line-height:1.55;
    padding:28px 20px 40px;-webkit-font-smoothing:antialiased}
  .wrap{max-width:720px;margin:0 auto}
  a{color:inherit}

  header{display:flex;align-items:center;justify-content:space-between;gap:12px;
    padding-bottom:14px;border-bottom:1px solid var(--border);margin-bottom:22px}
  .brand{display:flex;align-items:center;gap:9px;font-family:var(--mono);font-size:14px;letter-spacing:.02em}
  .brand .ver{color:var(--faint);font-size:12px}
  .dotpulse{width:9px;height:9px;border-radius:50%;background:var(--muted);position:relative}
  .dotpulse.ok{background:var(--ok)} .dotpulse.check{background:var(--warn)} .dotpulse.fail{background:var(--fail)}
  .meta{font-family:var(--mono);font-size:11.5px;color:var(--faint)}

  .hero{margin:4px 0 24px}
  .hero .pill{vertical-align:middle}
  .hero h1{display:inline-block;margin:0 0 0 10px;font-size:26px;font-weight:680;letter-spacing:-.01em;vertical-align:middle}
  .hero-sub{margin:12px 0 14px;color:var(--muted);font-size:14.5px;max-width:56ch}
  .chip{display:inline-block;font-family:var(--mono);font-size:11px;letter-spacing:.06em;
    color:var(--muted);background:var(--inset);border:1px solid var(--border);
    border-radius:999px;padding:4px 11px}

  .pill{font-family:var(--mono);font-size:11px;font-weight:700;letter-spacing:.12em;
    padding:4px 9px;border-radius:6px;color:#08111f}
  .pill.ok{background:var(--ok)} .pill.check{background:var(--warn)} .pill.fail{background:var(--fail);color:#fff}

  .eyebrow{font-family:var(--mono);font-size:10.5px;letter-spacing:.22em;text-transform:uppercase;
    color:var(--faint);margin:0 0 10px}

  .advisories{margin:0 0 22px}
  .adv{display:flex;gap:10px;font-size:13.5px;color:var(--text);padding:9px 13px;margin:7px 0;
    background:var(--inset);border:1px solid var(--border);border-left:3px solid var(--warn);border-radius:8px}
  .adv::before{content:"▲";color:var(--warn);font-size:11px;line-height:1.5}

  .panel{background:var(--panel);border:1px solid var(--border);border-radius:14px;
    padding:18px 20px;margin:16px 0;position:relative;overflow:hidden}
  .panel::before{content:"";position:absolute;inset:0 auto 0 0;width:3px;background:var(--faint)}
  .panel.ok::before{background:var(--ok)} .panel.check::before{background:var(--warn)} .panel.fail::before{background:var(--fail)}
  .p-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:16px}
  .p-head h2{margin:0;font-size:16.5px;font-weight:640}
  .p-meta{display:flex;align-items:center;gap:10px}
  .lat{font-family:var(--mono);font-size:12px;color:var(--muted)}

  /* signature: the signal path */
  .path{display:flex;align-items:flex-start;gap:0;overflow-x:auto;padding:4px 2px 2px;scrollbar-width:none}
  .path::-webkit-scrollbar{display:none}
  .node{flex:1 0 auto;min-width:66px;display:flex;flex-direction:column;align-items:center;gap:7px;position:relative}
  .node:not(:first-child)::before{content:"";position:absolute;left:-50%;top:12px;width:100%;height:2px;
    background:var(--track);z-index:0}
  .node.link:not(:first-child)::before{background:var(--ok)}
  .node .mk{position:relative;z-index:1;width:26px;height:26px;border-radius:50%;display:grid;place-items:center;
    border:2px solid var(--faint);background:var(--panel);font-size:13px;font-weight:700}
  .node.ok .mk{border-color:var(--ok);color:var(--ok)}
  .node.fail .mk{border-color:var(--fail);background:var(--fail);color:#fff}
  .node.skip .mk{border-style:dashed;color:var(--faint)}
  .node.ring .mk{box-shadow:0 0 0 3px color-mix(in srgb,var(--warn) 34%,transparent)}
  .node .lyr{font-family:var(--mono);font-size:10.5px;font-weight:600;letter-spacing:.14em;color:var(--muted)}
  .node .tm{font-family:var(--mono);font-size:10.5px;color:var(--faint)}

  /* latency waterfall: where the time went (cool palette, distinct from status) */
  .timeline{margin-top:15px}
  .tbar{display:flex;height:9px;border-radius:6px;overflow:hidden;background:var(--inset);border:1px solid var(--border)}
  .seg{min-width:2px;height:100%}
  .sw{width:8px;height:8px;border-radius:2px;display:inline-block;flex:none}
  .seg-dns{background:#4cc9c0} .seg-tcp{background:#4aa3f0} .seg-proxy{background:#4aa3f0}
  .seg-tls{background:#7b8cf4} .seg-http{background:#b07cf0}
  .tlegend{display:flex;flex-wrap:wrap;gap:6px 14px;margin-top:9px;
    font-family:var(--mono);font-size:11px;color:var(--muted)}
  .tlegend .lg{display:inline-flex;align-items:center;gap:5px}
  .tlegend .tot{color:var(--faint)}

  /* quota: local rate-limit windows (a third dimension beside network/auth) */
  .quota{margin-top:15px;padding-top:13px;border-top:1px dashed var(--border)}
  .q-head{display:flex;align-items:baseline;justify-content:space-between;gap:10px;margin-bottom:9px}
  .q-eyebrow{font-family:var(--mono);font-size:10.5px;letter-spacing:.2em;text-transform:uppercase;color:var(--faint)}
  .quota.blocked .q-eyebrow{color:var(--fail)}
  .q-meta{font-family:var(--mono);font-size:11px;color:var(--faint)}
  .q-row{display:flex;align-items:center;gap:10px;margin:6px 0}
  .q-lbl{font-family:var(--mono);font-size:11px;color:var(--muted);width:3.6em;flex:none}
  .q-track{flex:1;height:7px;border-radius:5px;background:var(--inset);border:1px solid var(--border);overflow:hidden}
  .q-fill{display:block;height:100%;border-radius:5px}
  .q-fill.q-normal{background:#4cc9c0} .q-fill.q-high{background:var(--warn)}
  .q-fill.q-full{background:var(--fail)} .q-fill.q-expired{background:var(--track)}
  .q-pct{font-family:var(--mono);font-size:11.5px;color:var(--text);width:3em;text-align:right;flex:none}
  .q-reset{font-family:var(--mono);font-size:11px;color:var(--faint);width:8.5em;text-align:right;flex:none}
  @media (max-width:480px){ .q-reset{display:none} }

  .summary{font-size:14px;margin:16px 0 0}
  .callout{font-family:var(--mono);font-size:12.5px;color:var(--fail);background:color-mix(in srgb,var(--fail) 10%,transparent);
    border:1px solid color-mix(in srgb,var(--fail) 35%,transparent);border-radius:8px;padding:9px 12px;margin-top:10px;
    overflow-x:auto}
  .note{font-size:13px;color:var(--fail);margin:10px 0 0}
  .rem{font-size:13px;color:var(--muted);margin:10px 0 0}

  details.raw{margin-top:14px;border-top:1px solid var(--border);padding-top:10px}
  details.raw summary{cursor:pointer;font-family:var(--mono);font-size:11px;letter-spacing:.14em;
    text-transform:uppercase;color:var(--faint);list-style:none}
  details.raw summary::-webkit-details-marker{display:none}
  details.raw summary::before{content:"▸ ";color:var(--faint)}
  details.raw[open] summary::before{content:"▾ "}
  .rawlist{font-family:var(--mono);font-size:12px;margin-top:10px}
  .rrow{display:flex;gap:10px;white-space:nowrap;padding:1.5px 0}
  .rmk{width:1.2em;text-align:center}
  .rmk.ok{color:var(--ok)} .rmk.fail{color:var(--fail)} .rmk.skip{color:var(--faint)}
  .rlyr{width:3.6em;color:var(--muted)}
  .rtm{width:5em;text-align:right;color:var(--faint)}
  .rdt{color:var(--text)}

  footer{margin-top:26px;padding-top:14px;border-top:1px solid var(--border);
    color:var(--faint);font-size:12px;text-align:center}
  footer a{color:var(--muted);text-decoration:none;border-bottom:1px solid var(--border)}

  @media (prefers-reduced-motion: no-preference){
    .panel{animation:rise .5s cubic-bezier(.2,.7,.2,1) both}
    .panel:nth-child(2){animation-delay:.05s}
    @keyframes rise{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
    .dotpulse.ok::after,.dotpulse.check::after,.dotpulse.fail::after{content:"";position:absolute;inset:0;
      border-radius:50%;background:inherit;animation:pulse 2.4s ease-out infinite}
    @keyframes pulse{from{opacity:.55;transform:scale(1)}to{opacity:0;transform:scale(2.6)}}
  }
  @media (max-width:480px){ .hero h1{font-size:22px} body{padding:20px 14px 32px} }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <div class="brand"><span class="dotpulse {{.OverallState}}"></span>ai-net-doctor <span class="ver">v{{.Version}}</span></div>
    <div class="meta">{{.GeneratedAt}}</div>
  </header>

  <section class="hero">
    <span class="pill {{.OverallState}}">{{.Overall}}</span><h1>{{.OverallTitle}}</h1>
    <p class="hero-sub">{{.OverallSubtitle}}</p>
    <span class="chip">路径 · {{.PathDesc}}</span>
  </section>

  {{if .Advisories}}
  <section class="advisories">
    <p class="eyebrow">提示 · advisories</p>
    {{range .Advisories}}<div class="adv">{{.}}</div>{{end}}
  </section>
  {{end}}

  <section>
    {{range .Targets}}
    <article class="panel {{.State}}">
      <div class="p-head">
        <h2>{{.Name}}</h2>
        <div class="p-meta"><span class="lat">{{.Latency}}</span><span class="pill {{.State}}">{{.Verdict}}</span></div>
      </div>
      <div class="path">
        {{range .Nodes}}<div class="node {{.State}}{{if .Ring}} ring{{end}}{{if .LinkActive}} link{{end}}"><span class="mk">{{.Mark}}</span><span class="lyr">{{.Layer}}</span><span class="tm">{{.Timing}}</span></div>{{end}}
      </div>
      {{if .Timeline}}
      <div class="timeline">
        <div class="tbar">{{range .Timeline}}<span class="seg {{.Class}}" style="{{.Style}}"></span>{{end}}</div>
        <div class="tlegend">{{range .Timeline}}<span class="lg"><i class="sw {{.Class}}"></i>{{.Label}} {{.Ms}}ms</span>{{end}}<span class="lg tot">总计 {{.TotalMs}}ms</span></div>
      </div>
      {{end}}
      {{with .Quota}}
      <div class="quota{{if .Blocked}} blocked{{end}}">
        <div class="q-head"><span class="q-eyebrow">额度 · quota</span>{{if .Meta}}<span class="q-meta">{{.Meta}}</span>{{end}}</div>
        {{range .Windows}}
        <div class="q-row">
          <span class="q-lbl">{{.Label}}</span>
          <span class="q-track"><span class="q-fill {{.Class}}" style="{{.Style}}"></span></span>
          <span class="q-pct">{{.Percent}}%</span>
          <span class="q-reset">{{.Reset}}</span>
        </div>
        {{end}}
      </div>
      {{end}}
      <p class="summary">{{.Summary}}</p>
      {{if .ErrorCallout}}<div class="callout">{{.ErrorCallout}}</div>{{end}}
      {{if .NoProxyNote}}<p class="note">⚠ {{.NoProxyNote}}</p>{{end}}
      {{if .Remediation}}<p class="rem">→ {{.Remediation}}</p>{{end}}
      <details class="raw"><summary>原始分层数据</summary>
        <div class="rawlist">{{range .Raw}}<div class="rrow"><span class="rmk {{.MarkClass}}">{{.Mark}}</span><span class="rlyr">{{.Layer}}</span><span class="rtm">{{.Timing}}</span><span class="rdt">{{.Detail}}</span></div>{{end}}</div>
      </details>
    </article>
    {{end}}
  </section>

  <footer>
    诊断不发送你的密钥；结论由 ai-net-doctor CLI 计算，本页仅展示。<br>
    <a href="https://github.com/wxggzz/ai-net-doctor">github.com/wxggzz/ai-net-doctor</a>
  </footer>
</div>
</body>
</html>
`))
