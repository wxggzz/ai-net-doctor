# ai-net-doctor 设计方案

> 面向 VPN / 代理 / 机场 / 公司网络用户的「AI Coding 网络连接诊断工具」。
> 目标：一键判断当前网络能否稳定使用 **Codex** 与 **Claude Code**，并在失败时定位到具体层级。
>
> - 文档版本：v0.2（Codex 二次评审后，待实现）
> - 日期：2026-07-04
> - 状态：待 Claude Code 实现第一版
> - 说明：本文档保留「设计决策 + 理由」与「开放问题」，并补充 Codex 二次评审后的产品入口判断。

---

## 0. TL;DR（给评审者的三句话）

1. **产品核心是一个零依赖的纯 CLI，不是 skill。** 因为用户最需要它的时刻（网络挂掉）正是 Codex/Claude Code 模型调用也失败的时刻——依赖 AI 客户端来"总结诊断结果"的形态在那一刻不可用。
2. **诊断必须测客户端真实走的路径。** 裸 socket 直连和走代理是两条路；Codex(Rust)/Claude Code(Node) 主要吃环境变量代理。如果测的路径和客户端不一致，会给出误导性的绿灯。
3. **结论由 CLI 算死（verdict + failed_layer + reason_code），所有上层（skill/MCP/菜单栏）只做展示与翻译**，禁止让模型看原始数据自行下诊断（防幻觉、保证多端一致）。
4. **第一版不能只交付命令行。** CLI 是诊断内核，但 VPN 用户最想要的是"点一下知道能不能用"；MVP 应同时提供一个轻量点击入口（macOS `.command` 或 Raycast Script Command），先不做重 GUI。

---

## 1. 背景与目标

### 1.1 问题
经常使用 VPN / 代理 / 机场 / 公司网络的开发者，在使用 Codex 和 Claude Code 时频繁遇到：
- `stream disconnected before completion`
- `error sending request for url` / 请求失败
- Claude Code 无响应、API 连接失败
- VPN 失效、代理端口异常、节点挂掉

这些错误信息高度雷同，用户很难判断问题出在 **DNS / TCP / TLS / HTTP / 认证 / 代理 / 路由** 哪一层，只能反复瞎试（切节点、重启客户端、重开终端）。

### 1.2 目标
- 一条命令，几秒内给出 Codex / Claude Code 各自的红绿灯结论。
- 失败时定位到**第一个断裂的层级**，并给出可执行的下一步建议。
- 能区分「网络不通」与「认证 / 权限 / 额度问题」——这是消解大部分瞎猜的关键。
- 在网络异常、无法访问 pip / 外网的环境下依然能运行（零第三方依赖）。

### 1.3 非目标（Non-Goals）
- 不修改任何网络配置、不启停 VPN / 代理客户端。
- 不做流量代理、不做加速、不做节点测速排名。
- 不做自动重连 / 心跳续跑（那是独立工具的职责，本工具只做诊断）。
- 默认不上传任何数据、不做遥测。

---

## 2. 核心设计原则

| 原则 | 含义 | 为什么 |
|---|---|---|
| **CLI-first** | 纯 CLI 是产品本体，可独立安装运行 | 网络挂时 AI wrapper（skill）连同模型调用一起失效，此刻只有纯 CLI / 非 AI 入口可用 |
| **测真实路径** | 按每个 target 客户端**实际使用的路径**测（含代理），支持 `--direct` / `--via-proxy` 双跑对比 | 避免"裸 socket 直连成功但客户端走代理失败"的绿灯误报 |
| **分层定位** | 明确输出第一个断裂层级：DNS→TCP→TLS→HTTP→认证；旁路层：代理 / 路由 | 这是本工具唯一真正的价值主张，不能只输出一个总 ok |
| **CLI 出结论，上层只翻译** | verdict / failed_layer / reason_code 全在 CLI 里算死 | 防模型幻觉，保证 skill / MCP / 菜单栏多端结论一致 |
| **每 target 独立判定** | Codex 和 Claude 各自有独立 verdict，互不污染，也不被 public_ip 等 ancillary 检查拖下水 | 避免"ipify 被墙但 Anthropic 完全正常"被误报成红灯 |
| **本地优先 + 默认脱敏** | 默认只做本地诊断，无遥测无上传；敏感值一律脱敏或只报存在性 | 安全与信任是这类工具的底线 |
| **Go 单二进制** | 核心 CLI 使用 Go 实现，目标是零运行时依赖、单二进制分发 | 网络 / pip / Python 环境不可用时仍能跑，跨平台分发更简单 |

### 2.1 语言与入口决策

- 核心 CLI 使用 **Go**，不再以 Python 标准库作为正式实现目标。
- Go 更适合本工具的分发场景：单二进制、跨平台、网络库成熟、并发模型自然，且用户无需安装 Python 或 pip 依赖。
- 但产品验收不能停在"命令能跑"。CLI 是内核，用户入口需要更轻：
  - 第一版同时提供 macOS `.command` 双击入口或 Raycast Script Command。
  - 该入口只调用 `ai-net-doctor --target all --json` 或普通文本输出，不包含诊断逻辑。
  - 菜单栏 App 属于 P2，不进入第一版，避免 UI 把核心诊断拖复杂。

---

## 3. 架构总览

### 3.1 分层
```
┌─────────────────────────────────────────────────────────┐
│  展示层（薄适配，只翻译 CLI 的结论，不做诊断判断）           │
│  - Codex skill        （.codex-plugin + skill）           │
│  - Claude skill       （/network-doctor）                 │
│  - MCP server         （network_diagnose，可选，P1）       │
│  - .command / Raycast  （轻量非 AI 入口，P0.5）             │
│  - 菜单栏 App          （常驻可视化入口，P2）               │
│  - Hook               （默认不做，见 §6）                  │
├─────────────────────────────────────────────────────────┤
│  CLI（产品本体，零依赖，可独立运行）                        │
│  ai-net-doctor  →  人话报告 / JSON（schema 版本化）         │
├─────────────────────────────────────────────────────────┤
│  core（编排 + 分层判定 + 脱敏）                             │
│  runner（并行 + 总预算） / verdict（分层 + reason_code）    │
│  checks（dns/tcp/tls/http/proxy/streaming/captive/ipv6）  │
│  targets（codex / claude：端点集 + 判定逻辑）              │
└─────────────────────────────────────────────────────────┘
```

### 3.2 各 surface 的定位与优先级
| Surface | 定位 | 优先级 | 说明 |
|---|---|---|---|
| **CLI** | 产品本体 | **P0** | 一切上层的唯一数据来源 |
| **Codex skill** | 分发渠道 | P0 | 薄转述层，调 `--json` 后转人话 |
| **Claude skill** `/network-doctor` | 分发渠道 | P0 | 同上，Claude Code 侧主力形态 |
| **轻量非 AI 入口**（macOS `.command` / Raycast / shell alias） | 用户入口 / 兜底入口 | **P0.5** | 用户不一定愿意打开终端；网络挂时 AI 入口不可用 |
| **菜单栏 App** | 常驻可视化入口 | P2 | 体验最好，但不适合作为第一版起点 |
| **MCP server** | 结构化调用 | P1（可选） | 只有需要被其他 agent 程序化调用时才值得，功能与 skill 重叠 |
| **Hook** | 自动预检 | P2（默认关） | 风险高，详见 §6 |

> **反过度设计说明**：原始设想一次铺开 6 个 surface。本方案收敛为「Go CLI 内核 + 轻量点击入口」为第一版验收标准；Codex/Claude skill 是薄包装，MCP 可选，Hook 默认不做。

---

## 4. 诊断维度

### 4.1 基础网络
- 默认路由、DNS、IPv4/IPv6、网卡 / VPN interface。
- **IPv6 黑洞检测（VPN 头号杀手）**：`AAAA` 有记录但 v6 实际 TCP 连不通 → Happy Eyeballs 先试 v6 再超时回退，表现为"时快时慢 / 偶发卡死 / stream disconnected"。必须显式检测「AAAA 解析成功 + v6 连接失败」这个组合并告警。

### 4.2 VPN / 代理
- **env 代理 vs macOS 系统代理并排对比**：CLI（Codex/Claude Code）主要吃 `HTTP(S)_PROXY` / `ALL_PROXY` 环境变量；浏览器里的 claude.ai / chatgpt.com 吃 `scutil --proxy` 系统代理。两者可以不一致——"浏览器能上但 CLI 连不上"多半在此。要展示差异并指出。
- `HTTP_PROXY / HTTPS_PROXY / ALL_PROXY / NO_PROXY`（含小写变体）；`NO_PROXY` 是否把目标域排除了。
- 本地代理端口 `7890 / 7897 / 1080 / 8080`：不仅测"在监听"，还要做**真实 `CONNECT <target>:443` 隧道测试**。机场客户端"进程还在但规则挂了"就是端口在听、CONNECT 失败。
- 代理认证：`407 Proxy Authentication Required` 单独识别。

### 4.3 公网身份（默认关，隐私）
- public IP、ASN、国家地区、VPN 前后 IP 是否变化。
- **默认关闭或收敛到单一中立来源**（如 `https://cloudflare.com/cdn-cgi/trace` 返回 `ip=`），避免同时打多个第三方 IP 服务（都会把用户 IP 泄露给第三方）。ASN / geo 为 opt-in。

### 4.4 Codex / OpenAI
- 端点集：`chatgpt.com`、`chatgpt.com/backend-api/codex/responses`（ChatGPT 登录态）、`api.openai.com/v1/responses`（API key 态）、`auth.openai.com`（登录）。
- 逐层：DNS / TCP / TLS / HTTP 状态 / 延迟。
- ⚠️ 待核实：Codex CLI 当前实际请求的域名/路径需以官方最新实现为准（见 §12 开放问题）。

### 4.5 Claude Code / Anthropic
- 端点集：`api.anthropic.com/v1/messages`（主）、`claude.ai` / `console.anthropic.com`（登录态）、telemetry（如 `statsig.anthropic.com`）。
- `ANTHROPIC_BASE_URL` 覆盖；Bedrock / Vertex 分支：`CLAUDE_CODE_USE_BEDROCK` / `CLAUDE_CODE_USE_VERTEX` / `ANTHROPIC_BEDROCK_BASE_URL` / `ANTHROPIC_VERTEX_BASE_URL`。
- 认证变量（`ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN` 等）**只报存在与否，绝不打印值**。
- ⚠️ 待核实：以上变量名与端点需以官方最新实现为准（见 §12）。

### 4.6 流式稳定性探针（`--stream`，P1，核心场景）
一次性可达性探测**测不出** `stream disconnected`。需要一个探针：向流式端点建连并**持续读 N 秒 SSE / chunked**，检测中途断流 / 空闲超时 / 缓冲代理截断。这是本工具立命之本，必须专门覆盖。

### 4.7 Captive portal 检测（机场 / 咖啡厅 / 酒店刚需）
- 请求 `http://captive.apple.com`（期望 `Success`）或 `http://www.gstatic.com/generate_204`（期望 204）。
- 返回 200 带 HTML = 被劫持到登录页，判定为「需先完成网络登录」。

---

## 5. 分层判定模型（verdict）

### 5.1 层级顺序
```
DNS → TCP → TLS → HTTP → 认证
（旁路层：PROXY / ROUTE / IPV6，出现问题时优先归因到旁路层）
```
判定「第一个断裂的层级」并短路后续（后续标记为 skipped）。

### 5.2 verdict 取值
- `OK`：目标真实路径全通。
- `CHECK`：可达但有隐患（如认证缺失、IPv6 黑洞、captive portal、延迟异常）。
- `FAIL`：真实路径在某一层断裂。

### 5.3 「网络不通」vs「认证 / 权限问题」判别（关键能力）
用一个**带认证语义的探测请求**（真实方法 + 最小 / 空 body），按响应形态归类：
```
向 api.anthropic.com/v1/messages（或 OpenAI responses）发最小请求：
├─ 连接被拒 / 超时 / TLS 握手失败       → 网络层（DNS/TCP/TLS/代理/路由）
├─ 407 Proxy Auth Required             → 代理认证问题
├─ 拿到结构良好的 401/403 JSON 错误体    → 网络完全 OK，纯认证 / 额度问题 ✅
└─ 2xx / 正常                          → 全通
```
> 拿到 API 自身返回的、格式良好的 401 JSON，就**证明了 DNS+TCP+TLS+HTTP+代理全通**，问题 100% 落在认证 / 额度。
>
> 注意：不能用 `HEAD` 探测——很多端点只接受 `POST`，`HEAD` 拿到的 405 既证明不了真实调用路径，也提取不出这个认证信号。

---

## 6. 上层形态决策（skill / plugin / hook / MCP）

### 6.1 Codex 侧
- 沿用 `.codex-plugin + skill` 形态。**skill 只做两件事**：调 CLI `--json`；把 CLI 已算好的 `verdict` 转人话 + 给 remediation。
- **诊断结论由 CLI 决定，模型只负责翻译语气和给修复建议**，不得自行"看数据下判断"。

### 6.2 Claude Code 侧
| 形态 | 建议 | 理由 |
|---|---|---|
| **Slash skill** `/network-doctor` | ✅ 主力（P0） | 用户主动触发、零副作用，调 CLI + 转述 |
| **MCP server** `network_diagnose` | 🟡 可选（P1） | 只有需要被其他 agent 程序化调用时才值得，与 skill 重叠 |
| **Hook**（SessionStart / UserPromptSubmit） | 🔴 默认不做 | UserPromptSubmit 每次预检会给**每条 prompt** 加延迟 + 噪音；SessionStart 若 fail-closed 可能误伤正常启动。真要做只能：用户显式开启 + 仅在检测到明确报错时提示 + 绝不阻塞，且默认关 |
| **重 plugin** | ❌ 不做 | skill 已足够 |

结论：**Claude 侧 = `/network-doctor` skill（P0）+ 可选 MCP（P1），Hook 默认关。**

---

## 7. CLI 设计

```bash
ai-net-doctor                       # 默认 all，人话输出
ai-net-doctor --target codex|claude|all
ai-net-doctor --json                # 机器可读，schema 版本化
ai-net-doctor -v / --verbose        # 分层瀑布（高级用户）
ai-net-doctor --quick               # 跳过慢 / 可选项（public IP、streaming）
ai-net-doctor --stream              # 流式断连复现探针
ai-net-doctor --via-proxy | --direct  # 强制路径，用于对比
ai-net-doctor --budget 15           # 总时间预算（秒），不是 per-check 叠加
ai-net-doctor --explain             # 打印 reason_code → 修复建议
ai-net-doctor --no-public-ip        # 显式关闭公网 IP 探测（默认即关）
```

**退出码**：`0` 全 OK；`2` CHECK / 降级；`3` FAIL。供 Hook / 脚本判断。

**执行模型**：所有 check **并行执行 + 总时间预算**，避免 per-check timeout 串行叠加导致"一键要等一分钟"。

---

## 8. 目录结构（Go 项目）

```
ai-net-doctor/
├─ go.mod
├─ README.md
├─ DESIGN.md                    # 本文档
├─ PROMPT_FOR_CLAUDE_CODE.md     # 第一版实现任务说明
├─ cmd/
│  └─ ai-net-doctor/
│     └─ main.go                 # 参数解析 + 渲染入口
├─ internal/
│  ├─ model/
│  │  └─ model.go                # JSON schema / check / target result
│  ├─ runner/
│  │  └─ runner.go               # 并行编排 + 总预算
│  ├─ checks/
│  │  ├─ dns.go  tcp.go  tls.go  http.go
│  │  ├─ proxy.go                # env + 系统代理 + CONNECT 隧道测试
│  │  ├─ system_proxy_darwin.go  # scutil --proxy parser
│  │  ├─ no_proxy.go
│  │  ├─ streaming.go            # P1：SSE / chunked 持连探针
│  │  ├─ captive.go              # P1：captive portal
│  │  └─ ipv6.go                 # P1：AAAA 黑洞检测
│  ├─ targets/
│  │  ├─ codex.go                # 端点集 + 判定逻辑
│  │  └─ claude.go
│  ├─ verdict/
│  │  ├─ verdict.go              # 分层判定
│  │  └─ reason.go               # reason_code → remediation
│  └─ report/
│     ├─ text.go                 # 人话（普通 / -v 两档）
│     └─ json.go                 # schema v1
├─ scripts/
│  ├─ ai-net-doctor.command      # P0.5：macOS 双击入口，调用 Go CLI
│  └─ raycast-ai-net-doctor.sh   # P0.5：Raycast Script Command，可选
├─ adapters/
│  ├─ codex-plugin/             # .codex-plugin/plugin.json + skill（薄转述）
│  ├─ claude-skill/             # /network-doctor
│  └─ mcp/                      # 可选，P1
└─ internal/**/*_test.go         # verdict / proxy / no_proxy / schema / parser 回归
```

---

## 9. 输出格式

### 9.1 普通用户（默认）
三行以内 + 每 target 红绿灯 + 一句最可能原因 + 一条下一步：
```
Codex   ✅ OK      (通过代理 127.0.0.1:7890，延迟 220ms)
Claude  ❌ FAIL    在「代理 CONNECT」这一层断了
        → 代理端口在监听但隧道建不起来，多半是机场规则失效 / 节点挂了。
          建议：切换节点或重启代理客户端后重试。
```

### 9.2 高级用户（`-v`）
分层瀑布，显示第一个断点：
```
Claude Code
  DNS   api.anthropic.com ✅ 34ms  → 160.79.x.x
  TCP   :443              ✅ 45ms
  PROXY CONNECT via 7890  ❌ 5001ms timeout        ← 断点
  TLS   —  (skipped)
  HTTP  —  (skipped)
  路径: HTTPS_PROXY=http://127.0.0.1:7890
```

### 9.3 JSON（schema 版本化）
```jsonc
{
  "schema_version": "1",
  "generated_at": "2026-07-04T12:00:00+08:00",
  "host": { "os": "darwin 25.5.0", "tool_version": "0.2.0" },
  "network_path": {
    "proxy_env": { "HTTPS_PROXY": "http://127.0.0.1:7890" },  // 已脱敏
    "system_proxy": { "HTTPSProxy": "..." },
    "diverged": true   // env 与系统代理是否不一致
  },
  "targets": {
    "claude": {
      "verdict": "FAIL",                       // OK | CHECK | FAIL
      "failed_layer": "proxy",                 // dns|tcp|tls|http|auth|proxy|route|ipv6|null
      "reason_code": "PROXY_CONNECT_TIMEOUT",  // 稳定枚举，供上层映射文案
      "latency_ms": 5001,
      "checks": [ { "layer": "dns", "ok": true, "elapsed_ms": 34 } ]
    },
    "codex": { }
  },
  "warnings": ["ipv6_aaaa_present_but_unreachable"],
  "remediation": ["switch_proxy_node"]         // reason_code → 建议 的稳定映射
}
```
**关键：`verdict` / `failed_layer` / `reason_code` 由 CLI 算死，skill / MCP / 菜单栏只做展示，保证多端结论一致。**

---

## 10. 安全 / 隐私

- **必须脱敏 / 只报存在性（绝不回显值）**：
  - `ANTHROPIC_API_KEY`、`ANTHROPIC_AUTH_TOKEN`、`OPENAI_API_KEY`，以及泛匹配 `*_KEY / *_TOKEN / *_SECRET`。
  - 代理 URL 中的 `user:pass` 凭据。
  - 认证变量一律报 `present: true/false`，永不回显。
- **不做全量 env dump**：只读取白名单内的代理 / base_url 相关变量。
- **public IP**：默认关闭或收敛到单一中立来源；任何"分享报告"必须默认打码 + 发送前预览。
- **默认只做本地诊断**：无遥测、无自动上传、无日志外发。
- 若未来做 `--share`：必须 显式触发 + 强制脱敏 + 本地生成后让用户先看再发。

---

## 11. 功能优先级与路线图

### 11.1 优先级
**P0（否则工具不可信）**
- 按 target 真实路径测（`--direct` / `--via-proxy` 双跑），消除"socket 直连 vs 走代理"路径分歧。
- 失败分层 + 第一断点定位（DNS/TCP/TLS/HTTP/认证/代理/路由）。
- 每 target 独立 verdict，不被 ancillary 检查污染。
- 认证 vs 网络 判别（401-JSON 探测法）。
- 并行执行 + 总时间预算。
- 脱敏加固 + JSON `schema_version`。
- 无硬编码路径，CLI 可独立安装运行。

**P1（强烈建议）**
- 流式探针 `--stream`（复现 `stream disconnected`，核心场景）。
- IPv6 黑洞检测、Captive portal 检测。
- 代理端口真实 CONNECT 隧道测试。
- 非 AI 入口（菜单栏 / Raycast / shell 别名）。
- `--explain`：`reason_code → 修复建议` 稳定映射库。

**P2（可选增强）**
- MCP server（结构化工具）。
- 历史 / 趋势（多次采样看抖动、丢包、延迟稳定性）。
- ASN / 地理（opt-in，默认关）。
- Windows / Linux 对等支持、`--share`（脱敏 + 预览）、self-update。

### 11.2 路线图
1. **v0.1（Go CLI MVP + 轻量入口）**：初始化 Go 项目；实现 target 独立 verdict、真实路径、认证判别、并行预算、脱敏、JSON schema；提供 macOS `.command` 或 Raycast Script Command。
2. **v0.2（VPN 场景补齐）**：`--stream`、IPv6 黑洞、captive portal、代理 CONNECT 隧道、`--explain` 建议库。
3. **v0.3（AI 工具适配）**：Codex skill、Claude Code `/network-doctor` skill，均为 CLI 的薄转述层。
4. **v0.4（生态）**：MCP、`--share`（脱敏预览）、菜单栏 App 可行性验证。
5. **v0.5+（增强）**：历史趋势 / 抖动、ASN/geo（opt-in）、Windows / Linux 对等支持。

---

## 12. 留给评审者（Codex）的开放问题

1. **端点 / 变量准确性**：§4.4 / §4.5 中 Codex 与 Claude Code 的真实请求域名、路径、环境变量名，需以两者当前官方实现为准。请指出是否有过时或缺失（例如 Codex 是否还有其他必经域名、Claude 的 telemetry / 更新检查域名是否需纳入）。
2. **认证探测的合规性**：用最小 `POST` 触发 401 来判别"网络 vs 认证"，是否会触发风控 / 速率限制？是否需要更保守的方式（例如仅 OPTIONS，或读取错误体但限频）？
3. **流式探针实现**：以标准库实现 SSE / chunked 持连读 N 秒是否稳妥？如何区分"服务端正常结束"与"中途异常断流"？N 取多少合理（默认 10s？）？
4. **总时间预算 vs 并行**：并行大量 check 是否会引入误差（如同时握手影响延迟测量）？预算耗尽时的降级策略如何设计更合理？
5. **零依赖 vs 能力**：坚持纯标准库是否会在 CONNECT 隧道测试、IPv6 探测、TLS 细节上受限？哪些点值得破例引入依赖？
6. **语言选型**：第一版已决定使用 Go 单二进制。后续仅需评估 Go 是否足以覆盖 TLS / 代理 / 跨平台细节，暂不切 Rust，除非 Go 实现遇到明确瓶颈。
7. **verdict / reason_code 枚举**：§5、§9 的 `failed_layer` 与 `reason_code` 取值集合是否完备？有无遗漏的常见失败态（如 MTU、DNS 污染、证书被中间人替换）？

---

## 附录 A：现有雏形（codex-network-doctor）已知问题（新项目须避免重蹈）

> 位置：`~/plugins/codex-network-doctor/scripts/network_doctor.py`

1. **路径分歧（最严重）**：`check_tls` / `check_dns` 用裸 socket 直连绕过代理，`check_http` 用 `urllib` 自动走代理——两条路径不一致，导致走代理场景下绿灯误报。
2. **无分层判定**：只把 check 平铺成 list，`report["ok"]` 是所有 check 的与，没有"第一个断点在哪"。
3. **单一 ok 污染**：第三方 `public_ip`（ipify）失败会拖垮整体判定。
4. **串行 + per-check 6s**：最坏几十秒，违背"一键"体验。
5. **HEAD 探测丢信号**：非 2xx 一律当 reachable，取不出"网络 vs 认证"的关键信号。
6. **硬编码绝对路径**：`SKILL.md` 内写死 `~/plugins/...`，分发即失效。
7. **JSON 无 schema 版本**：下游一改就崩。
8. **第三方 IP 泄露**：同时打 ipify + ifconfig.me，两个第三方都拿到用户 IP。

以上问题在本方案 §2 / §5 / §7 / §10 已逐条对应修正。
