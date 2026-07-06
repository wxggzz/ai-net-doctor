package checks

import "os"

// credentialKeys are the auth-related env vars whose presence (never value) we
// report. Values are never read into output.
var credentialKeys = []string{
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
}

// CredentialPresence reports which known credential env vars are set, as
// booleans only. It never exposes any value.
func CredentialPresence() map[string]bool {
	m := map[string]bool{}
	for _, k := range credentialKeys {
		v, ok := os.LookupEnv(k)
		m[k] = ok && v != ""
	}
	return m
}
