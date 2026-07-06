package targets

// Codex returns the probe spec for Codex / OpenAI.
//
// The auth probe hits the API endpoint with no credentials, expecting a 401 —
// which proves DNS+TCP+TLS+HTTP (and proxy, if used) all work. See DESIGN.md.
func Codex() Spec {
	return Spec{
		Name:       "codex",
		Display:    "Codex / OpenAI",
		APIHost:    "api.openai.com",
		APIPort:    "443",
		AuthPath:   "/v1/responses",
		AuthMethod: "POST",
		AuthBody:   `{}`,
		AuthHeaders: map[string]string{
			"Content-Type": "application/json",
		},
		BaseURLEnv: "OPENAI_BASE_URL",
	}
}
