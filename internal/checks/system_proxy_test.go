package checks

import "testing"

// Sample output of `scutil --proxy` with an HTTP(S) proxy enabled.
const sampleScutil = `<dictionary> {
  ExceptionsList : <array> {
    0 : *.local
    1 : 169.254/16
  }
  FTPPassive : 1
  HTTPEnable : 1
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  ProxyAutoConfigEnable : 0
}`

const sampleScutilDisabled = `<dictionary> {
  HTTPEnable : 0
  HTTPSEnable : 0
  ProxyAutoConfigEnable : 0
}`

func TestParseScutilProxy(t *testing.T) {
	sp := parseScutilProxy(sampleScutil)
	if !sp.HTTPEnabled || sp.HTTPProxy != "127.0.0.1" || sp.HTTPPort != "7890" {
		t.Errorf("HTTP fields wrong: %+v", sp)
	}
	if !sp.HTTPSEnabled || sp.HTTPSProxy != "127.0.0.1" || sp.HTTPSPort != "7890" {
		t.Errorf("HTTPS fields wrong: %+v", sp)
	}
	if !sp.Enabled() {
		t.Errorf("Enabled() should be true")
	}
	if got := sp.AsMap()["HTTPSProxy"]; got != "127.0.0.1:7890" {
		t.Errorf("AsMap HTTPSProxy = %q", got)
	}
	if u := sp.HTTPSURL(); u == nil || u.Scheme != "http" || u.Host != "127.0.0.1:7890" {
		t.Errorf("HTTPSURL = %+v", u)
	}
}

func TestParseScutilProxyDisabled(t *testing.T) {
	sp := parseScutilProxy(sampleScutilDisabled)
	if sp.Enabled() {
		t.Errorf("Enabled() should be false: %+v", sp)
	}
	if len(sp.AsMap()) != 0 {
		t.Errorf("AsMap should be empty: %v", sp.AsMap())
	}
	if sp.HTTPSURL() != nil {
		t.Errorf("HTTPSURL should be nil when disabled")
	}
}
