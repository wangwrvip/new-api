# 设计：Claude thinking 块计入上下文估算

## 修复方案

在 `ClaudeRequest.GetTokenCountMeta()` 的消息内容 switch 中增加两个分支，复用已有字段，不扩展 DTO：

```go
case "thinking":
    // Extended-thinking blocks are part of the context the model reads.
    if media.Thinking != nil && *media.Thinking != "" {
        texts = append(texts, *media.Thinking)
    }
case "redacted_thinking":
    if media.Data != "" {
        texts = append(texts, media.Data)
    }
```

### 为什么放在消息 content 的 switch 内

`thinking` / `redacted_thinking` 按 Anthropic Messages 协议只出现在消息内容块中，不会出现在 `system`。放在消息循环内与其它内容块共用同一套拼接和 `CombineText` 生成路径，改动最小。

### 为什么只写 `texts`，不写 `userTexts`

`userTexts` 用于 `LastUserMessage`，即请求日志里的最后一条用户消息预览。thinking 属于 assistant 历史，写进去会污染预览。因此两个新分支只追加到 `texts`。

### 为什么 `redacted_thinking.data` 也计数

该字段保存上游加密后的推理内容，虽然不可读，但仍占用上游上下文。受控实验与真实会话都表明这部分会体现在上游 `input_tokens` 中。

## 兼容性

- 不改任何公开类型、方法签名或 JSON 字段。
- `relaykit` 仍为独立模块，不引入根模块依赖。
- 对不含 thinking 块的既有请求，输出与修复前完全一致。
- 上下文限额与预扣费会自动使用更完整的估算，无需改调用方。

## 测试策略

在现有 `relaykit/dto/token_count_meta_test.go` 中新增一个聚焦测试，覆盖：

1. assistant 消息含 `thinking` 块 → `CombineText` 包含其文本；
2. `redacted_thinking` 块 → `CombineText` 包含其 `data`；
3. `LastUserMessage` 仍只来自 user 文本，不包含 thinking；
4. 普通 text/tool 内容仍被计数。

先运行测试确认在未修复代码上失败（RED），再实施修复并确认转绿（GREEN）。
