package targets

// Claude returns the probe spec for Claude Code / Anthropic.
//
// The auth probe hits /v1/messages with no x-api-key, expecting a 401. Getting
// a well-formed 401 back proves the network path is fully up. See DESIGN.md.
func Claude() Spec {
	return Spec{
		Name:       "claude",
		Display:    "Claude Code / Anthropic",
		APIHost:    "api.anthropic.com",
		APIPort:    "443",
		AuthPath:   "/v1/messages",
		AuthMethod: "POST",
		AuthBody:   `{}`,
		AuthHeaders: map[string]string{
			"Content-Type":      "application/json",
			"anthropic-version": "2023-06-01",
		},
		BaseURLEnv: "ANTHROPIC_BASE_URL",
	}
}
