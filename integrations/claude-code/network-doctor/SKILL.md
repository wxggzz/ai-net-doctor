---
name: network-doctor
description: Diagnose whether the current network can reach Codex/OpenAI and Claude/Anthropic, and localize the first broken layer (DNS/TCP/TLS/HTTP/auth/proxy). Use when the user asks to test their network / VPN / proxy / airport / company network, or when Codex/Claude report stream disconnected, "error sending request", or connection timeouts.
---

# Network Doctor

Check whether the current network can actually use **Codex/OpenAI** and
**Claude/Anthropic**, and if not, point to the first broken layer.

## Single source of truth

All diagnosis is done by the `ai-net-doctor` CLI. **Do NOT re-derive any
conclusion yourself.** Run the CLI, read its JSON, and translate it into plain
language. The CLI already computed `verdict`, `failed_layer`, `reason_code`, and
`remediation`; your job is presentation, not judgment.

Binary: `ai-net-doctor` on `PATH` (override with `$AI_NET_DOCTOR_BIN`). If it's
missing, install it, then re-run:

```bash
go install github.com/wxggzz/ai-net-doctor/cmd/ai-net-doctor@latest
# or: brew install wxggzz/tap/ai-net-doctor
# or: curl -fsSL https://raw.githubusercontent.com/wxggzz/ai-net-doctor/main/scripts/install.sh | sh
```

## Workflow

1. Run (tests both targets, machine-readable):

   ```bash
   ai-net-doctor --target all --json
   ```

   - Only one target asked about → `--target codex` or `--target claude`.
   - Proxy suspected → add `--verbose`, or force `--proxy env` / `--proxy system`
     / `--direct`.

2. Parse the JSON (schema_version "1"). Key fields:
   - `network_path.mode` / `forced` — requested/selected path (direct/env/system).
   - `network_path.diverged`, `transparent_proxy_suspected`,
     `transparent_proxy_hint` — path-level advisories.
   - `targets.<name>.verdict` (`OK`/`CHECK`/`FAIL`), `failed_layer`,
     `reason_code`, `latency_ms`, `path_mode`, `no_proxy_excluded`, `checks[]`.
   - `warnings[]` and `remediation[]` — stable codes + ready-made advice.
   - `credentials_present` — booleans only.

3. Present, honoring the CLI verbatim:
   - One line per target: verdict + human summary. On `FAIL`, name the
     `failed_layer` and the failing check's detail; on `CHECK`
     (`AUTH_REQUIRED_REACHABLE`), explain the network is fine and the issue is
     auth/quota — the probe sends no credentials, so a 401 is expected, not a bug.
   - Surface `warnings` and `remediation` as-is; don't invent new advice.
   - State the path mode; if a target's `no_proxy_excluded` is true, say the
     proxy was bypassed for that host and it actually went direct.

## Rules

- Do NOT re-interpret HTTP status codes into your own verdict — trust
  `reason_code`. A 401/403/404/405 does not mean the network is broken.
- Do NOT print secrets. Credentials are reported present true/false only; proxy
  credentials are redacted. Never echo env values.
- Read-only: do NOT change network settings or start/stop VPN clients.
- Exit codes: `0` all OK · `2` at least one CHECK (no FAIL) · `3` any FAIL.
