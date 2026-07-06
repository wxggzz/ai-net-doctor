# ai-net-doctor

一键判断当前网络能否稳定使用 **Codex** 与 **Claude Code**，并在失败时定位到具体层级
（DNS / TCP / TLS / HTTP / 认证 / 代理）。面向经常使用 VPN、代理、机场、公司网络的用户。

> 完整设计与取舍见 [`DESIGN.md`](./DESIGN.md)。本文件是 v0.1 的使用说明。

## 为什么

Codex / Claude Code 报 `stream disconnected`、`error sending request`、连接超时时，
用户很难判断是 DNS、代理、路由还是认证的问题。本工具**按客户端真实走的路径**逐层探测，
给出每个 target 独立的 `OK / CHECK / FAIL` 结论和第一个断点，并区分「网络不通」和
「认证/额度问题」。

诊断结论（`verdict` / `failed_layer` / `reason_code`）全部由 CLI 算死；任何上层
（点击入口、后续的 skill / MCP）只做展示，不做判断。

## 构建

需要 Go 1.22+（零第三方依赖，仅用标准库）。

```bash
go build -o ./bin/ai-net-doctor ./cmd/ai-net-doctor
```

## 使用

```bash
ai-net-doctor                    # 默认 --target all，人话报告
ai-net-doctor --target codex     # 只测 Codex / OpenAI
ai-net-doctor --target claude    # 只测 Claude Code / Anthropic
ai-net-doctor --verbose          # 分层瀑布，标出第一个断点
ai-net-doctor --json             # 机器可读，schema 版本化
ai-net-doctor --budget 20        # 总时间预算（秒），默认 15
ai-net-doctor --direct           # 强制直连（忽略代理）
ai-net-doctor --proxy env        # 强制走环境变量代理
ai-net-doctor --proxy system     # 强制走 macOS 系统代理 (scutil --proxy)
```

未指定路径时（自动模式），若设置了 `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` 则默认走
env 代理，否则直连；报告顶部会写明实际使用的 path mode。

**强制模式不会静默降级**：如果你显式指定 `--proxy env` 但没有任何 env 代理变量，或指定
`--proxy system` 但系统 HTTPS 代理未开启，工具会判为 **FAIL**（`ENV_PROXY_NOT_CONFIGURED` /
`SYSTEM_PROXY_NOT_CONFIGURED`），而不是偷偷改成直连——因为那样会让你误以为代理路径被测过了。
报告顶部区分「请求路径」（你要求/自动选择的）与每个 target verbose 里的「实际路径」；
若 `NO_PROXY` 把某个 target 排除、导致它实际走直连，会在该 target 下明确提示。

### 退出码

| 码 | 含义 |
|----|------|
| 0  | 全部 OK |
| 2  | 至少一个 CHECK，无 FAIL |
| 3  | 至少一个 FAIL |

> 说明：诊断**不发送你的密钥**，所以 API 端点会返回 401，这被判为 `CHECK`
> (`AUTH_REQUIRED_REACHABLE`)——它恰恰证明 DNS/TCP/TLS/HTTP 全通、网络没问题。

### 点击入口（不打开终端也能用）

- macOS：双击 `scripts/ai-net-doctor.command`，会开一个 Terminal 窗口跑诊断。
- Raycast：把 `scripts/raycast-ai-net-doctor.sh` 加为 Script Command。

两个入口都只调用已构建的 `./bin/ai-net-doctor`，不含任何诊断逻辑。

## 输出字段（`--json`，schema v1）

顶层：`schema_version` / `generated_at` / `host` / `network_path` /
`targets` / `credentials_present` / `warnings` / `remediation`。

每个 target：`verdict`（OK/CHECK/FAIL）、`failed_layer`
（dns/tcp/tls/http/auth/proxy/route/ipv6 或 null）、`reason_code`（稳定枚举）、
`latency_ms`、`checks[]`。每个 check：`name` / `layer` / `ok` / `skipped` /
`elapsed_ms` / `detail` / `error` / `endpoint` / `path_mode`。

### reason_code 枚举（v0.1）

导致 target 判定 **FAIL** 的：
`DNS_RESOLVE_FAILED` · `TCP_CONNECT_TIMEOUT` · `TCP_CONNECT_FAILED` ·
`TLS_HANDSHAKE_FAILED` · `HTTP_UNREACHABLE` · `PROXY_CONNECT_TIMEOUT` ·
`PROXY_CONNECT_FAILED` · `PROXY_AUTH_REQUIRED` · `ENV_PROXY_NOT_CONFIGURED` ·
`SYSTEM_PROXY_NOT_CONFIGURED` · `UNSUPPORTED_BASE_URL_SCHEME`

判定为 **OK / CHECK** 的：`OK` · `AUTH_REQUIRED_REACHABLE`

仅出现在 `warnings`、**不改变任何 target 判定**的路径级提示：
`NO_PROXY_EXCLUDES_TARGET` · `ENV_SYSTEM_PROXY_DIVERGED` · `BUDGET_EXCEEDED` ·
`SYSTEM_PROXY_SET_BUT_ENV_EMPTY` · `TRANSPARENT_PROXY_SUSPECTED`

### 透明代理 / TUN 提示

当结果显示 `direct`、但探测到目标域名解析到 fake-ip 段（`198.18.0.0/15`，
Clash / sing-box / Surge 等 TUN 模式的典型特征）时，报告会给出
`TRANSPARENT_PROXY_SUSPECTED` 警告：说明「你以为在直连，实际流量很可能被隧道接管」。
这是**只提示、不改判定**的启发式（`network_path.transparent_proxy_suspected`），
仅在 direct 模式下运行，避免对正常代理用户误报。默认路由接口若是 `utunN` 会作为佐证一并列出。

## 安全 / 隐私

- 默认只做本地诊断，无遥测、无上传。
- 只读取白名单 env（代理相关 + `*_BASE_URL`）；不 dump 全量 env。
- 密钥/令牌（`OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN`）
  只报 `present: true/false`，绝不回显。
- 代理 URL 中的 `user:pass` 会脱敏为 `***`。
- 认证探测**不发送**任何密钥。

## 端点 / 变量（待官方确认）

集中在 `internal/targets/codex.go` 与 `claude.go`：

- Codex/OpenAI：`api.openai.com` + `/v1/responses`，base URL 覆盖 `OPENAI_BASE_URL`
- Claude/Anthropic：`api.anthropic.com` + `/v1/messages`，base URL 覆盖 `ANTHROPIC_BASE_URL`

这些是当前最佳理解，需以两者官方实现为准（见 `DESIGN.md` §12）。

> **base URL 仅支持 https（v0.1）**：探测逻辑假定 TLS。如果 `*_BASE_URL` 设成 `http://…`，
> 工具会判为 `UNSUPPORTED_BASE_URL_SCHEME`（FAIL）并拒绝探测，而不是拿 TLS 去打 HTTP 网关、
> 报出假的 TLS 失败。需要 http 网关支持时再议。

## 已知限制（v0.1）

- 代理仅支持 HTTP(S) CONNECT；**SOCKS 代理暂不支持**（会明确报错提示改用 HTTP 代理）。
- 未做流式断连探针（`stream disconnected` 复现）、IPv6 黑洞检测、captive portal —— 见路线图 P1。
- 系统代理解析仅 macOS（`scutil --proxy`）；其他平台该项为空。

## 测试

```bash
go test ./...
```

## 目录结构

```
cmd/ai-net-doctor      CLI 入口（参数解析 + 渲染）
internal/model         JSON schema / 枚举
internal/checks        网络原语：dns/tcp/tls/http/proxy/system_proxy/no_proxy
internal/targets       Codex/Claude 端点集 + 分层探测引擎 (probe.go)
internal/verdict       分层判定 + reason_code → 文案
internal/runner        路径解析 + 并发编排 + 总预算
internal/report        text / json 渲染
scripts                点击入口（.command / Raycast）
```
