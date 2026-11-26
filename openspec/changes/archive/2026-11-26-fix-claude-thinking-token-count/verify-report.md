# 验证报告：fix-claude-thinking-token-count

日期：2026-11-26
分支：`migrate/upstream-2026-11`
模式：light（hotfix）
提交：`7fa6898f2`

## Summary

| 检查项 | 结果 | 证据 |
| --- | --- | --- |
| tasks.md 全部完成 | PASS | 8/8 复选框为 `[x]` |
| 改动文件与任务一致 | PASS | 提交区间 `12890cbe9...HEAD` 仅 2 个文件：`relaykit/dto/claude.go`、`relaykit/dto/token_count_meta_test.go` |
| 编译通过 | PASS | `comet check run ... build`：exit=0（relaykit + service/relay/controller） |
| 相关测试通过 | PASS | `comet check run ... verify`：exit=0；relaykit 全量、service token/context-limit、controller context-limit 全绿 |
| 无明显安全问题 | PASS | diff 仅新增文本计数分支与测试常量，无密钥、无 unsafe、无外部输入执行 |
| 代码审查 | SKIPPED | `review_mode: off`（hotfix 默认）；本次为单点行为修复，已有 RED/GREEN 回归测试覆盖 |

## RED / GREEN 证据

未修复代码上运行新增测试：

```text
"user\nsummarize the plan\nassistant\nvisible answer" does not contain
"step one: inspect the request\nstep two: trace the estimator"
"user\nsummarize the plan\nassistant\nvisible answer" does not contain
"ENCRYPTED-REDACTED-THINKING-PAYLOAD"
--- FAIL: TestClaudeTokenCountMetaCountsThinkingBlocks
```

修复后同一测试：

```text
--- PASS: TestClaudeTokenCountMetaCountsThinkingBlocks
```

失败原因正是 thinking / redacted_thinking 文本未进入 `CombineText`，与 proposal 根因一致。

## 规模评估说明

`comet state scale` 因 tasks.md 复选框数（8）建议 full；实际改动为 2 个实现文件 + 1 个 delta spec、无跨模块协调。按 hotfix 规则保持 light，理由：三个任务分组的复选框被逐条计数，实际修复面是单文件行为分支加一个测试。

归档入口发现 OpenSpec 要求 `specs` artifact 完整，因此补充 delta spec `specs/context-token-limit/spec.md`，在既有“估算偏差可控”需求下新增 thinking / redacted_thinking 两个验收场景；实现与测试对应场景。

## 覆盖范围

- `thinking` 块文本计入 `CombineText`。
- `redacted_thinking.data` 计入 `CombineText`。
- `LastUserMessage` 仍只来自 user 文本，thinking 不污染请求日志预览。
- 既有 text / image / tool_use / tool_result 行为未见回归。

## 已知限制

- 未重新部署并实测真实 glm-5.3-flash / deepseek 会话偏差；本次验证停留在单元与模块级。部署后若仍有明显偏差，需按 proposal 的已知限制另立 change 校准 `FamilyUnknown` 模型 tokenizer。
- 未改动前端、计费公式、数据库或公开 API。
