# 修复 Claude thinking 块未计入 token 估算

## 问题描述

Claude Messages 请求中的 `thinking` 与 `redacted_thinking` 内容块没有被计入上下文 token 估算。真实 Claude Code 会话经 `/v1/messages` 转发时，本地估算持续低于上游实际 usage：

| 模型 | 实测上游 tokens | 本地估算 | 偏差 |
| --- | --- | --- | --- |
| glm-5.3-flash（日志 25753/25754/25755） | 413,099 | 371,084 | -10.2% |
| glm-5.3（日志 25735） | 401,896 | 365,634 | -9.0% |
| deepseek-v4.1-flash（日志 25746） | 440,307 | 354,701 | -19.4% |

受控实验进一步确认：仅包含 768,000 字符 `thinking` 的请求，上游 `input_tokens=132,025`，本地估算仅 `128`（日志 25758）。不含 thinking 的合成 GLM 大上下文测试偏差为 0.000%，说明偏差由 thinking 块缺失导致，而非 tokenizer 本身。

## 根因分析

`relaykit/dto/claude.go` 的 `ClaudeRequest.GetTokenCountMeta()` 在遍历消息内容块时只处理 `text`、`image`、`tool_use`、`tool_result`。`ClaudeMediaMessage` 已经定义了 `Thinking *string` 与 `Data string` 字段，但 switch 没有对应分支，导致：

1. `thinking` 块的 `thinking` 文本完全不进入 `TokenCountMeta.CombineText`；
2. `redacted_thinking` 块的加密 `data` 也不计入。

`CombineText` 是 `service.CountRequestToken` 的唯一文本输入，因此这两类块对估算完全不可见。上下文限额判断（`controller/relay.go` 的 `enforceContextLimit`）与预扣费都基于该估算，低估会让超限请求漏过检查、预扣费偏低。

## 修复目标

- `thinking` 块的非空 `thinking` 文本计入 `CombineText`。
- `redacted_thinking` 块的非空 `data` 计入 `CombineText`。
- 不改变 `LastUserMessage` 语义：thinking 属于 assistant 历史，不作为最后用户消息预览。
- 不改变 system、普通 text、image、tool_use、tool_result 的既有计数行为。
- 新增回归测试，删除修复后测试必须失败。

## 非目标

- 不改 tokenizer 权重、固定开销公式或前端展示。
- 不处理 Gemini / OpenAI Responses 的同类推理块（如有需要另立 change）。
- 不改数据库 schema、公开 API 或计费规则。

## 影响范围

- `relaykit/dto/claude.go`：`GetTokenCountMeta` 的消息 content switch。
- `relaykit/dto/token_count_meta_test.go`：新增 Claude thinking 回归测试。
- 验证：`relaykit` 独立 build/test，根模块 `service`/`relay` build 与相关 token 测试。

## 已知限制

本次为 hotfix 级修复。若修复后真实会话偏差仍显著，剩余误差可能来自 `deepseek` 等 `FamilyUnknown` 模型的 tokenizer 近似，需要另立 change 校准。
