---
name: network-doctor
description: Diagnose whether the network can reach Codex/OpenAI and Claude/Anthropic, and localize the first broken layer (DNS/TCP/TLS/HTTP/auth/proxy). Use when Codex reports stream disconnected, "error sending request", connection timeouts, or the user asks to test their network / VPN / proxy / airport / company network for Codex or Claude connectivity.
---

# Network Doctor

Diagnose whether the current network can actually use **Codex/OpenAI** and
**Claude/Anthropic**, and if not, point to the first broken layer.

## Single source of truth

All diagnosis is done by the `ai-net-doctor` CLI. **You (the model) must NOT
re-derive any conclusion.** Run the CLI, read its JSON, and translate it into
plain language. Never decide "reachable / broken" yourself from an HTTP status —
the CLI already computed `verdict`, `failed_layer`, `reason_code`, and
`remediation`.

Binary: `ai-net-doctor` on `PATH` (override with `$AI_NET_DOCTOR_BIN`). Install:

```bash
go install github.com/wxggzz/ai-net-doctor/cmd/ai-net-doctor@latest
# or: brew install wxggzz/tap/ai-net-doctor
# or: curl -fsSL https://raw.githubusercontent.com/wxggzz/ai-net-doctor/main/scripts/install.sh | sh
```

## Workflow

1. Run (machine-readable, both targets):

   ```bash
   ai-net-doctor --target all --json
   ```

   - Only Codex asked about → `--target codex`; only Claude → `--target claude`.
   - Suspected proxy issue → add `--verbose`, or force `--proxy env` /
     `--proxy system` / `--direct`.

2. Parse the JSON (schema_version "1"):
   - `network_path.mode` / `forced`, `diverged`, `transparent_proxy_suspected`.
   - `targets.<name>.verdict` (`OK`/`CHECK`/`FAIL`), `failed_layer`,
     `reason_code`, `latency_ms`, `path_mode`, `no_proxy_excluded`, `checks[]`.
   - `warnings[]`, `remediation[]`, `credentials_present` (booleans only).

3. Present, honoring the CLI verbatim:
   - One line per target: verdict + human summary. On `FAIL`, name the
     `failed_layer`. On `CHECK` (`AUTH_REQUIRED_REACHABLE`), explain the network
     is fine and the issue is auth/quota — the probe sends no credentials, so a
     401 is expected, not a bug.
   - Surface `warnings` and `remediation` as-is.

## Rules

- Do NOT re-interpret HTTP status codes into your own verdict — trust
  `reason_code`.
- Do NOT print secrets. Read-only: never change network settings or VPN clients.
- Exit codes: `0` all OK · `2` at least one CHECK (no FAIL) · `3` any FAIL.
