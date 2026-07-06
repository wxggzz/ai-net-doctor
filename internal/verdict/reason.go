package verdict

import "github.com/wxggzz/ai-net-doctor/internal/model"

// Explanation is human-facing text for a reason code. Summary states what was
// observed; Remediation is the concrete next step (may be empty).
type Explanation struct {
	Summary     string
	Remediation string
}

// Explain returns localized (zh) text for a reason code. This is the single
// place the reason_code -> user text mapping lives, shared by text output and,
// later, by skills/MCP.
func Explain(reason model.ReasonCode) Explanation {
	switch reason {
	case model.ReasonOK:
		return Explanation{"网络可达，服务正常响应。", ""}
	case model.ReasonAuthRequiredReachable:
		return Explanation{
			"网络完全可达（DNS/TCP/TLS/HTTP 全通）；返回 401/403 属正常——诊断不会使用你的密钥。",
			"若 Codex/Claude 仍报错，问题在认证或额度、不在网络：检查登录状态、API key、订阅额度。",
		}
	case model.ReasonDNSResolveFailed:
		return Explanation{
			"DNS 解析失败——域名无法解析为 IP。",
			"检查 DNS / VPN 的 DNS 设置，或改用 1.1.1.1 / 8.8.8.8 后重试。",
		}
	case model.ReasonTCPConnectTimeout:
		return Explanation{
			"TCP 连接超时——能解析域名但连不上目标端口。",
			"多半被墙 / 路由异常 / VPN 未生效：开启或切换 VPN 节点后重试。",
		}
	case model.ReasonTCPConnectFailed:
		return Explanation{
			"TCP 连接失败——端口拒绝或不可达。",
			"检查本地网络 / 防火墙，或切换 VPN 节点。",
		}
	case model.ReasonTLSHandshakeFailed:
		return Explanation{
			"TLS 握手失败——可能被中间人拦截或证书被改写。",
			"检查是否有企业代理 / 抓包工具在改写 HTTPS；换网络或关闭 HTTPS 拦截后重试。",
		}
	case model.ReasonHTTPUnreachable:
		return Explanation{
			"连接已建立但拿不到 HTTP 响应——请求中途被重置。",
			"多半被 QoS/DPI 干扰或代理不稳：切换节点 / 代理后重试。",
		}
	case model.ReasonProxyConnectTimeout:
		return Explanation{
			"代理 CONNECT 超时——代理端口在监听，但隧道建不起来。",
			"代理规则可能失效 / 节点已挂：重启代理客户端或切换节点后重试。",
		}
	case model.ReasonProxyConnectFailed:
		return Explanation{
			"代理 CONNECT 失败——无法通过本地代理建立隧道。",
			"确认代理客户端在运行、端口正确；重启代理后重试。",
		}
	case model.ReasonProxyAuthRequired:
		return Explanation{
			"代理要求认证（407）。",
			"在代理 URL 中补上正确的 user:pass，或修正代理认证配置。",
		}
	case model.ReasonNoProxyExcludesTarget:
		return Explanation{
			"NO_PROXY 把该目标排除了，客户端会直连该地址。",
			"若希望 Codex/Claude 走代理，请从 NO_PROXY 中移除对应域名。",
		}
	case model.ReasonEnvSystemProxyDiverged:
		return Explanation{
			"环境变量代理与系统代理不一致。",
			"命令行工具吃环境变量代理、浏览器吃系统代理；确认 CLI 用的是你以为的代理。",
		}
	case model.ReasonBudgetExceeded:
		return Explanation{
			"诊断超时——在预算时间内未完成。",
			"网络非常慢或卡死：增大 --budget 或切换更快的网络后重试。",
		}
	case model.ReasonEnvProxyNotConfigured:
		return Explanation{
			"你指定了 --proxy env，但没有检测到 HTTPS_PROXY / HTTP_PROXY / ALL_PROXY，因此无法测试 env proxy。",
			"设置 HTTPS_PROXY=http://host:port 后重试，或去掉 --proxy env 走自动路径。",
		}
	case model.ReasonSystemProxyNotConfigured:
		return Explanation{
			"你指定了 --proxy system，但系统 HTTPS 代理未开启，因此无法测试系统代理。",
			"在“系统设置 → 网络 → 代理”里开启 HTTPS 代理后重试，或去掉 --proxy system。",
		}
	case model.ReasonUnsupportedBaseURLScheme:
		return Explanation{
			"*_BASE_URL 使用了非 https 协议；v0.1 仅支持 https base URL，已拒绝以避免误判。",
			"把 OPENAI_BASE_URL / ANTHROPIC_BASE_URL 改成 https:// 开头，或取消该变量后重试。",
		}
	case model.ReasonSystemProxySetButEnvEmpty:
		return Explanation{
			"检测到系统代理已开启，但环境变量代理为空；浏览器可能能上，但 Codex/Claude CLI 可能不走代理。",
			"如需让 CLI 走同一代理，请设置 HTTPS_PROXY=http://host:port，或用 --proxy system 进行测试。",
		}
	case model.ReasonTransparentProxySuspected:
		return Explanation{
			"疑似透明代理 / TUN（fake-ip）：未配置任何代理，但流量很可能被隧道接管。",
			"下方各 target 显示的“direct”结果实际可能走了隧道，不代表真实直连路径；" +
				"如需按真实路径诊断，请在代理客户端里切到系统代理/环境变量代理模式，或临时关闭 TUN 后重跑。",
		}
	default:
		return Explanation{string(reason), ""}
	}
}
