# 集成入口（skill）

CLI v0.1 已冻结为稳定内核。所有入口**只调用** `bin/ai-net-doctor`，读取它的
`--json` 输出并翻译成人话，**绝不复制或重算诊断逻辑**（verdict / failed_layer /
reason_code / remediation 一律以 CLI 为准）。

## 契约

- 数据源唯一：`./bin/ai-net-doctor --target all --json`
- 覆盖变量：`AI_NET_DOCTOR_BIN` 可指向别处安装的二进制
- 入口层不做判断，只做展示；退出码 `0` 全 OK / `2` 有 CHECK / `3` 有 FAIL

## Codex plugin

- 位置：`~/plugins/codex-network-doctor/`
  - `.codex-plugin/plugin.json` — manifest（v0.2.0，已改为由 Go CLI 驱动）
  - `skills/network-doctor/SKILL.md` — 指示 Codex 调 CLI 的 JSON 并翻译
  - `scripts/network_doctor.py` — **已收敛为薄转发 shim**（无诊断逻辑，转发到 Go
    CLI，映射 legacy `--timeout N`→`--budget N`）；旧原型备份为
    `scripts/network_doctor.legacy.py.bak`
- 用法：用户说“测一下网络 / VPN / Codex / Claude 连接”即触发。

## Claude Code skill

- 位置：`~/.claude/skills/network-doctor/SKILL.md`
- 用法：在 Claude Code 里输入 `/network-doctor`，或直接说要测网络/VPN/Codex/Claude。
  binary 缺失时 skill 会先 `go build` 再跑。

## 更新方式

改行为改 **CLI**（`.`），重新 `go build -o ./bin/ai-net-doctor
./cmd/ai-net-doctor` 即可，两个 skill 自动受益。改**展示**只动对应的 `SKILL.md`，
不要把判定逻辑写进 skill。

## 暂不做（按优先级排序，后续再议）

安装/调用体验打磨 → Raycast Script Command 完善 → 菜单栏 App → MCP / 流式断连探针 /
IPv6 黑洞 / captive portal。
