# 日志请求预览与输出预览

「请求预览」在日志详情中展示请求正文内容（首段文本），便于用户复查自己的请求、管理员判断请求是否被滥用（如恶意灌入无关内容）。「输出预览」则记录模型实际输出的文本开头，便于核对响应内容与排查异常输出。

## 权限与入口

- **日志所有者与管理员可见**：三个预览字段（`request_preview` / `request_preview_user` / `response_preview`）写在 `logs.other` 顶层，用户在「我的日志」详情中可查看自己请求的输入与输出内容；管理员在全部日志详情中可查看任意用户的预览。
- 历史兼容：旧版本将这三个字段写在 `admin_info` 下（仅管理员可见）。用户查询时 `formatUserLogs`（`model/log.go`）会调用 `formatLogOtherJSON`，后者在剥离 `admin_info` 前把其中的旧预览字段提升到顶层；提升逻辑实际在 `model/log_other.go` 的 `hoistLegacyPreviews`。管理员侧前端对旧日志回退读取 `admin_info`。
- **隐私提示**：开启预览后用户能看到自己发给模型的正文与模型输出（内容本就来自用户自己的请求）。若部署方不希望向用户展示，关闭预览开关即可。
- 入口：日志 → 消费日志 → 点击某条日志展开详情，在「请求预览 / 最后一条用户消息 / 输出预览」区块查看。

## 设置项（管理后台 → 设置 → 系统设置）

| 设置项 | 键 | 默认 | 说明 |
| --- | --- | --- | --- |
| 请求预览开关 | `RequestPreviewEnabled` | `true` | 关闭后不再采集请求预览；已写入的历史日志不受影响 |
| 请求预览 Token 上限 | `RequestPreviewMaxTokens` | `200` | 截断阈值，有效范围 1–2000，越界自动收敛 |

## 采集逻辑

- 仅在请求包含文本（chat / claude / gemini / 图片与音频的文本部分等所有文本类格式）时采集。预览文本由 controller 在预扣费之前**另外调用一次** `GetTokenCountMeta()` 重建（`TokenCountMeta.CombineText`），并不复用计费阶段已算好的结果，因此同一请求会解析两次；不额外解析上游载荷。
- 预览包含**两块**：
  - **请求内容**（`request_preview`）：请求文本的前 N 个 token，含系统提示词等结构内容，可用于排查提示词注入。
  - **最后一条用户消息**（`request_preview_user`）：用户在输入框实际输入的内容（`TokenCountMeta.LastUserMessage`，从结构化消息中取最后一条 role=user 的文本）。带巨型系统提示词的请求（如 Claude Code CLI）前 N 个 token 全被系统提示词占用，用户问题必须从这一块看。只有 OpenAI chat/completions、Claude Messages、OpenAI Responses 三种请求格式会填充该字段；Gemini 格式（`GeminiChatRequest.GetTokenCountMeta`）不设置 `LastUserMessage`，Gemini 日志的该字段恒为空。
- 截断以 token 为单位而非字符：`service.TruncateTextToTokens` 用二分法找到**达到**上限的最小 rune 前缀，估算粒度为 rune 时可能略超上限（不是「不超过上限的最大前缀」）；CJK 与英文按各自估算器（tiktoken / rune 加权估算）处理。
- 截断有两道硬上限，与估算结果无关：搜索窗口最多 `8 × 上限` rune，返回文本最多 8192 rune。即使某个模型族把空白估算为 0 token（tokenkit 的 Gemini 权重，空格计 0），空白密集文本也不会整段返回；`maxTokens` 超过 8192 时按 8192 处理，非正值返回空。
- 关闭开关或未开启消费日志（`LogConsumeEnabled`）时均不采集。
- 采集在预扣费之前完成，不影响计费与响应路径。

## 存储与读取

- 存储：`other.request_preview` 与 `other.request_preview_user`（顶层），随日志库（`logs` 表）持久化；配置独立日志库（LOG_DSN）时自动写入日志库。
- 读取：管理端 `GetAllLogs` 原样返回，`FormatAdminLogs` 仅剥离 `root_info`，故两个预览字段对管理员（含普通管理员）可见。

## 输出预览

- **采集内容**：模型输出文本的**前 1024 个字符**（按字符而非字节截断，CJK 不会被切断），包含正文与思考（reasoning/thinking）内容，与计费使用的文本累积口径一致。工具调用的覆盖范围随流式/非流式而异：
  - **流式**：预览取流式计费的累积文本，会包含工具调用名与参数分片。OpenAI 累加器写入工具名与参数，Claude 流式累积写入 tool_use 名称与 `partial_json`，Gemini 流式累积写入 function call 名与参数，Responses 累积器写入 `function_call_arguments.delta`。
  - **非流式**：OpenAI 与 Claude 只含正文与思考，不含工具调用；Gemini 非流式复用计费用的 `geminiResponseUsageText`，同样包含 function call 名与参数；Responses 非流式取 `ExtractOutputTextFromResponses`，只含 `output_text`。
- **无独立设置项**：消费日志开启（`LogConsumeEnabled`）即采集，截断上限固定 1K；关闭消费日志则完全不采集。
- **采集位置**：各文本格式族的中心响应处理器，文本取自已有的全文累积器（usage 估算用），不额外解析：
  - OpenAI 兼容（覆盖绝大多数渠道）：`OaiStreamHandler` / `OpenaiHandler`
  - Claude：`ClaudeStreamHandler` / `ClaudeHandler`
  - Gemini：`geminiStreamHandler`（Chat / 原生 / Responses 流式共用）+ 三个非流式 handler
  - Responses 家族：`OaiResponsesStreamHandler` / `OaiResponsesHandler` / `chat_via_responses` / `responses_via_chat`
- **覆盖范围**：自带响应处理器的少数小渠道（dify、cohere、tencent、cloudflare、palm、xai 等）暂不采集输出预览；音频、图片、embedding 等非文本格式不采集。
- 存储：`other.response_preview`（顶层，日志所有者与管理员可见）。
