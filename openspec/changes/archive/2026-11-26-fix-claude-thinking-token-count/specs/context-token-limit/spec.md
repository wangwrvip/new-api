# Spec Delta

## MODIFIED Requirements

### Requirement: 估算偏差可控

系统 SHALL 估算请求的输入 token 数，使其与上游返回的真实输入计量偏差不超过 10%，且在 agent 风格的多轮工具调用会话中同样成立。Claude Messages 历史中的 `thinking` 与 `redacted_thinking` 内容块属于模型读取的上下文，SHALL 计入估算文本；它们不得进入最后用户消息预览。

#### Scenario: agent 会话估算

- **WHEN** 请求包含大量工具定义、扩展思考块与工具结果
- **THEN** 估算值与上游真实计量之间的偏差在 ±10% 以内

#### Scenario: Claude 扩展思考块计入估算

- **WHEN** Claude Messages 请求的 assistant 历史包含非空 `thinking` 块
- **THEN** 该块文本计入请求输入 token 估算
- **AND** 该文本不出现在最后用户消息预览中

#### Scenario: Claude 加密思考块计入估算

- **WHEN** Claude Messages 请求包含非空 `redacted_thinking.data`
- **THEN** 该加密数据计入请求输入 token 估算

#### Scenario: 缓存命中计入实际值

- **WHEN** 上游返回的输入计量不含缓存命中 token（Anthropic 语义）
- **THEN** 计算偏差时把缓存读 token 计入实际输入总量
