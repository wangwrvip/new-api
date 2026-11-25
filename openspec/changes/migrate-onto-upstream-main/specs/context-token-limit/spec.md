# Spec Delta

## Purpose

限定单次请求可携带的输入上下文规模，在转发上游之前拒绝超出模型上限的请求，并保证本地估算与上游真实计量的偏差足够小，使阈值真正生效。

## ADDED Requirements

### Requirement: 按模型限制输入上下文

系统 SHALL 在每次 **HTTP 中继请求**转发前计算输入侧 token 总数并与该模型的上限比较。全局默认上限与按模型覆写同时存在时，按模型配置优先；上限为 0 表示不限制。WebSocket 中继（`/v1/responses`）与异步任务端点（`controller.RelayTask` 路径）不在本次范围内；经 `controller.Relay` 进入的 task-plugin 协议 HTTP 端点（如 `openai_responses.create`、`openai_image.generate`/`edit`）仍受本限制约束。

#### Scenario: 未超限放行

- **WHEN** 请求的估算输入 token 数小于等于该模型上限
- **THEN** 请求正常转发至上游，且不因限制产生拒绝

#### Scenario: 超限拒绝

- **WHEN** 请求的估算输入 token 数大于该模型上限
- **THEN** 系统在调用上游之前拒绝请求，返回 HTTP 413 与错误码 `context_limit_exceeded`

#### Scenario: 按模型覆写优先

- **WHEN** 全局默认上限为 200000，某模型单独配置为 500000
- **THEN** 该模型接受估算值不超过 500000 的请求，其余模型仍按 200000 判定

#### Scenario: 上限为 0 表示不限制

- **WHEN** 某模型的上限配置为 0
- **THEN** 该模型不做上下文限制，任意大小的请求均放行

#### Scenario: 计费估算开关关闭时限制仍生效

- **WHEN** 运维关闭了请求 token 估算的计费开关
- **THEN** 上下文限制仍按模型上限判定，超出上限的请求依旧返回 HTTP 413

#### Scenario: WebSocket 与异步任务端点不在本次范围

- **WHEN** 请求通过 WebSocket 中继（`/v1/responses`）或异步任务端点（`controller.RelayTask` 路径）提交
- **THEN** 本 change 不对其输入上下文做上限判定，该覆盖由后续独立 change 跟进
- **AND** 经 `controller.Relay` 进入的 task-plugin 协议 HTTP 端点不适用该例外，仍按本需求判定

### Requirement: 估算偏差可控

系统 SHALL 估算请求的输入 token 数，使其与上游返回的真实输入计量偏差不超过 10%，且在 agent 风格的多轮工具调用会话中同样成立。

#### Scenario: agent 会话估算

- **WHEN** 请求包含大量工具定义、扩展思考块与工具结果
- **THEN** 估算值与上游真实计量之间的偏差在 ±10% 以内

#### Scenario: 缓存命中计入实际值

- **WHEN** 上游返回的输入计量不含缓存命中 token（Anthropic 语义）
- **THEN** 计算偏差时把缓存读 token 计入实际输入总量

### Requirement: 拒绝可审计

系统 SHALL 为每次因上下文限制被拒绝的请求写入错误日志，记录估算 token 数、生效上限与请求格式。

#### Scenario: 错误日志含关键字段

- **WHEN** 请求因超出上限被拒绝
- **THEN** 系统写入一条错误日志，且其附加信息包含 `estimated_prompt_tokens`、`context_limit` 与 `relay_format`

### Requirement: 估算值可对账

系统 SHALL 在每次成功请求的消费日志中记录本地估算的输入 token 数，供与上游真实计量逐条对比。

#### Scenario: 消费日志记录估算值

- **WHEN** 请求成功完成并写入消费日志
- **THEN** 该日志的附加信息包含 `estimated_prompt_tokens`

#### Scenario: 无估算值时不留空字段

- **WHEN** 本次请求没有可用的本地估算值
- **THEN** 消费日志不写入 `estimated_prompt_tokens` 字段
