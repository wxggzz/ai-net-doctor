package checks

import "strings"

// MatchNoProxy reports whether host is excluded from proxying by a NO_PROXY
// spec. It follows the common convention: comma-separated entries, "*" matches
// everything, a leading dot or bare domain matches the domain and its
// subdomains. Ports in entries are ignored.
func MatchNoProxy(noProxy, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, raw := range strings.Split(noProxy, ",") {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "" {
			continue
		}
		if entry == "*" {
			return true
		}
		// Drop a trailing :port if present.
		if i := strings.LastIndex(entry, ":"); i > 0 && !strings.Contains(entry[i+1:], ".") {
			entry = entry[:i]
		}
		entry = strings.TrimPrefix(entry, ".")
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}
