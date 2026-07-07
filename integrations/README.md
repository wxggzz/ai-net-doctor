# Integrations

Thin **skills** that expose `ai-net-doctor` inside AI coding agents. They only
call the CLI and translate its JSON — they never re-derive the verdict, so the
diagnosis stays in one place (the CLI).

First install the CLI (any one):

```bash
go install github.com/wxggzz/ai-net-doctor/cmd/ai-net-doctor@latest
# or: brew install wxggzz/tap/ai-net-doctor
# or: curl -fsSL https://raw.githubusercontent.com/wxggzz/ai-net-doctor/main/scripts/install.sh | sh
```

All skills locate the binary as `ai-net-doctor` on `PATH` (override with
`AI_NET_DOCTOR_BIN`).

## Claude Code — `/network-doctor`

Copy the skill into your Claude Code skills directory:

```bash
mkdir -p ~/.claude/skills
cp -r integrations/claude-code/network-doctor ~/.claude/skills/
```

Then in Claude Code, run `/network-doctor` or just ask to test your network /
VPN / Codex / Claude connectivity.

## Codex — personal plugin

Copy the plugin into your Codex plugins directory:

```bash
cp -r integrations/codex/codex-network-doctor ~/plugins/
```

Then ask Codex to "run a network diagnostic for Codex and Claude".

## The contract

- **Single source of truth**: `ai-net-doctor --target all --json`.
- Skills present `verdict` / `failed_layer` / `reason_code` / `remediation`
  verbatim; they must not re-interpret HTTP status codes into their own verdict.
- Read-only: never change network settings or touch VPN clients.
- Never print secrets (the CLI already reports credentials as present-only and
  redacts proxy credentials).
