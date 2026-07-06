package targets

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/wxggzz/ai-net-doctor/internal/checks"
	"github.com/wxggzz/ai-net-doctor/internal/model"
	"github.com/wxggzz/ai-net-doctor/internal/verdict"
)

// layerRank orders checks in the waterfall by their place in the stack. tcp and
// proxy share a rank (they are mutually exclusive at the same position).
var layerRank = map[model.Layer]int{
	model.LayerDNS:   0,
	model.LayerTCP:   1,
	model.LayerProxy: 1,
	model.LayerTLS:   2,
	model.LayerHTTP:  3,
	model.LayerAuth:  4,
}

// Per-layer timeouts. All are additionally bounded by the ctx budget deadline.
const (
	dnsTimeout  = 5 * time.Second
	tcpTimeout  = 6 * time.Second
	tlsTimeout  = 6 * time.Second
	httpTimeout = 8 * time.Second
)

// Probe runs the layered diagnostic for one target along the resolved path and
// returns an independent verdict. It short-circuits at the first broken layer
// and marks the remaining layers as skipped.
func Probe(ctx context.Context, spec Spec, plan PathPlan) model.TargetResult {
	start := time.Now()
	host := spec.APIHost
	port := spec.APIPort
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(host, port)

	var list []model.Check
	add := func(c model.Check) { list = append(list, c) }

	finalize := func(v model.Verdict, layer model.Layer, reason model.ReasonCode) model.TargetResult {
		present := map[model.Layer]bool{}
		for _, c := range list {
			present[c.Layer] = true
		}
		for _, l := range expectedLayers(plan.Mode) {
			if !present[l] {
				add(model.Check{Name: string(l), Layer: l, Skipped: true, PathMode: plan.Mode, Endpoint: addr})
			}
		}
		// Keep the waterfall in stack order regardless of the order checks were
		// appended (setup-error paths add their breakpoint check first).
		sort.SliceStable(list, func(i, j int) bool {
			return layerRank[list[i].Layer] < layerRank[list[j].Layer]
		})
		var fl *model.Layer
		if layer != "" {
			l := layer
			fl = &l
		}
		return model.TargetResult{
			Verdict:         v,
			FailedLayer:     fl,
			ReasonCode:      reason,
			LatencyMs:       time.Since(start).Milliseconds(),
			Host:            host,
			PathMode:        plan.Mode,
			NoProxyExcluded: plan.NoProxyExcluded,
			Checks:          list,
		}
	}

	// ---- setup errors: fail fast, perform no network I/O ----
	if spec.SetupError != "" {
		add(model.Check{
			Name: "config:" + host, Layer: model.LayerHTTP, PathMode: plan.Mode, Endpoint: addr,
			Detail: "仅支持 https base URL", Error: "unsupported base URL scheme (only https)",
		})
		return finalize(model.VerdictFail, model.LayerHTTP, spec.SetupError)
	}
	if plan.SetupError != "" {
		detail := "代理未配置"
		switch plan.SetupError {
		case model.ReasonEnvProxyNotConfigured:
			detail = "未检测到 HTTPS_PROXY / HTTP_PROXY / ALL_PROXY"
		case model.ReasonSystemProxyNotConfigured:
			detail = "系统 HTTPS 代理未开启"
		}
		add(model.Check{
			Name: "proxy-setup", Layer: model.LayerProxy, PathMode: plan.Mode, Endpoint: addr,
			Detail: detail, Error: detail,
		})
		return finalize(model.VerdictFail, model.LayerProxy, plan.SetupError)
	}

	budgetHit := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	// ---- DNS ----
	dns := checks.ResolveDNS(ctx, host, dnsTimeout)
	dnsCheck := model.Check{Name: "dns:" + host, Layer: model.LayerDNS, PathMode: plan.Mode, Endpoint: host, ElapsedMs: dns.Elapsed.Milliseconds()}
	if dns.Err != nil {
		dnsCheck.Error = errText(dns.Err)
		if plan.Mode == model.PathDirect {
			dnsCheck.Detail = "DNS resolve failed"
			add(dnsCheck)
			return finalize(model.VerdictFail, model.LayerDNS, model.ReasonDNSResolveFailed)
		}
		// Via a proxy the resolution happens remotely, so local DNS failure is
		// informational, not fatal.
		dnsCheck.Detail = "local DNS failed (non-fatal via proxy)"
		add(dnsCheck)
	} else {
		dnsCheck.OK = true
		dnsCheck.Detail = ipsText(dns.IPs)
		add(dnsCheck)
	}

	if budgetHit() {
		return finalize(model.VerdictFail, model.LayerTLS, model.ReasonBudgetExceeded)
	}

	// ---- TCP (direct) or PROXY CONNECT (proxy) ----
	var conn net.Conn
	if plan.Mode == model.PathDirect {
		c, elapsed, err := checks.DialTCP(ctx, addr, tcpTimeout)
		tcpCheck := model.Check{Name: "tcp:" + addr, Layer: model.LayerTCP, PathMode: plan.Mode, Endpoint: addr, ElapsedMs: elapsed.Milliseconds()}
		if err != nil {
			tcpCheck.Error = errText(err)
			tcpCheck.Detail = "TCP connect failed"
			add(tcpCheck)
			reason := model.ReasonTCPConnectFailed
			if isTimeout(err) {
				reason = model.ReasonTCPConnectTimeout
			}
			return finalize(model.VerdictFail, model.LayerTCP, reason)
		}
		tcpCheck.OK = true
		tcpCheck.Detail = "connected"
		add(tcpCheck)
		conn = c
	} else {
		res := checks.HTTPConnect(ctx, plan.ProxyURL, addr, tcpTimeout)
		pxCheck := model.Check{Name: "proxy-connect:" + plan.ProxyLabel, Layer: model.LayerProxy, PathMode: plan.Mode, Endpoint: addr, ElapsedMs: res.Elapsed.Milliseconds()}
		if res.Err != nil || res.Status != 200 {
			pxCheck.Error = errText(res.Err)
			var reason model.ReasonCode
			switch {
			case res.Status == 407:
				reason, pxCheck.Detail = model.ReasonProxyAuthRequired, "proxy requires authentication (407)"
			case isTimeout(res.Err):
				reason, pxCheck.Detail = model.ReasonProxyConnectTimeout, "proxy CONNECT timed out"
			default:
				reason, pxCheck.Detail = model.ReasonProxyConnectFailed, "proxy CONNECT failed"
			}
			add(pxCheck)
			return finalize(model.VerdictFail, model.LayerProxy, reason)
		}
		pxCheck.OK = true
		pxCheck.Detail = "CONNECT tunnel established"
		add(pxCheck)
		conn = res.Conn
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	if budgetHit() {
		return finalize(model.VerdictFail, model.LayerTLS, model.ReasonBudgetExceeded)
	}

	// ---- TLS ----
	tconn, tlsVer, tlsElapsed, err := checks.TLSHandshake(ctx, conn, host, tlsTimeout)
	tlsCheck := model.Check{Name: "tls:" + host, Layer: model.LayerTLS, PathMode: plan.Mode, Endpoint: addr, ElapsedMs: tlsElapsed.Milliseconds()}
	if err != nil {
		tlsCheck.Error = errText(err)
		tlsCheck.Detail = "TLS handshake failed"
		add(tlsCheck)
		return finalize(model.VerdictFail, model.LayerTLS, model.ReasonTLSHandshakeFailed)
	}
	tlsCheck.OK = true
	tlsCheck.Detail = "TLS " + tlsVer + " ok"
	add(tlsCheck)
	conn = tconn

	if budgetHit() {
		return finalize(model.VerdictFail, model.LayerHTTP, model.ReasonBudgetExceeded)
	}

	// ---- HTTP + auth classification ----
	endpoint := "https://" + host + spec.AuthPath
	hres := checks.HTTPOverConn(ctx, tconn, checks.HTTPRequest{
		Method:  spec.AuthMethod,
		Host:    host,
		Path:    spec.AuthPath,
		Headers: spec.AuthHeaders,
		Body:    spec.AuthBody,
	}, httpTimeout)
	httpCheck := model.Check{Name: "http:" + host + spec.AuthPath, Layer: model.LayerHTTP, PathMode: plan.Mode, Endpoint: endpoint, ElapsedMs: hres.Elapsed.Milliseconds()}
	if hres.Status == 0 {
		httpCheck.Error = errText(hres.Err)
		httpCheck.Detail = "no HTTP response"
		add(httpCheck)
		return finalize(model.VerdictFail, model.LayerHTTP, model.ReasonHTTPUnreachable)
	}
	httpCheck.OK = true
	httpCheck.Detail = fmt.Sprintf("HTTP %d", hres.Status)
	add(httpCheck)

	v, reason, layer := verdict.ClassifyHTTP(hres.Status)
	authCheck := model.Check{Name: "auth:" + host, Layer: model.LayerAuth, PathMode: plan.Mode, Endpoint: endpoint, OK: v != model.VerdictFail}
	switch reason {
	case model.ReasonAuthRequiredReachable:
		authCheck.Detail = fmt.Sprintf("reachable; HTTP %d (auth not verified — no credentials sent)", hres.Status)
	case model.ReasonOK:
		authCheck.Detail = fmt.Sprintf("reachable; HTTP %d", hres.Status)
	default:
		authCheck.Detail = fmt.Sprintf("HTTP %d", hres.Status)
	}
	add(authCheck)
	return finalize(v, layer, reason)
}

func expectedLayers(mode model.PathMode) []model.Layer {
	if mode == model.PathDirect {
		return []model.Layer{model.LayerDNS, model.LayerTCP, model.LayerTLS, model.LayerHTTP, model.LayerAuth}
	}
	return []model.Layer{model.LayerDNS, model.LayerProxy, model.LayerTLS, model.LayerHTTP, model.LayerAuth}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func ipsText(ips []net.IPAddr) string {
	var s []string
	for i, ip := range ips {
		if i >= 4 {
			break
		}
		s = append(s, ip.IP.String())
	}
	return strings.Join(s, ", ")
}
