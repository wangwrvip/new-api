# 验证报告：migrate-onto-upstream-main

日期：2026-11-26
分支：`migrate/upstream-2026-11` @ `a404554ae`（`main` 同步）
验证模式：full（23 tasks / 1 capability / 1985 changed files）

## Summary

| 维度 | 状态 |
| --- | --- |
| Completeness | 23/23 tasks 完成；4/4 requirements 有实现 |
| Correctness | 4/4 requirements、11/11 scenarios 有测试或部署证据 |
| Coherence | 基本遵循设计决策；D1/D8 有已记录的裁决偏差 |

## 构建与测试证据

- 根模块 `go build .`：PASS。
- 根模块真实包测试（12 packages，`pg_data` 权限替代路径）：PASS。
- `relaykit`、`tokenkit` 各自 build + test：PASS。
- 前端 typecheck：1,543 files，0 error。
- 前端全量测试：256 files / 3041 tests，PASS。
- 前端 build：PASS（`web/dist/index.html`）。
- 前端 lint：147 errors vs `origin/main` 基线 148，无新增；改动文件定向 lint 0 error。

## Requirement / Scenario 覆盖

| Requirement | 实现 | 证据 |
| --- | --- | --- |
| 按模型限制输入上下文 | `controller/relay.go` 的 `enforceContextLimit`，在预扣费与 keep-alive 前判定 | `controller/relay_context_limit_test.go`：413/code、错误日志字段、覆写、0、开关关闭、fail-open；部署 S1 |
| 估算偏差可控 | `tokenkit` `FamilyGLM` + cl100k；`service/token_counter.go` 固定开销 | P5.5b 复查；S2 100K/300K/500K（OpenAI/Anthropic）偏差 ≤0.01% |
| 拒绝可审计 | 413 错误日志含 `estimated_prompt_tokens`、`context_limit`、`relay_format` | `TestContextLimitErrorOtherIncludesRequiredFields`；部署 S1 |
| 估算值可对账 | `service/log_info_generate.go` 写 `estimated_prompt_tokens`；无值不写 | `TestGenerateTextOtherInfo*`；部署日志抽样 |

Scenarios：11/11 有对应测试或部署证据；`WebSocket/异步任务端点不在范围` 已在 spec 明确并记录为后续 change。

## 设计决策核对

- D2 token 计数采用上游 tokenkit：是；GLM 走 cl100k + 固定开销（P5.5b）。
- D3 接 `CountRequestToken`：是；开关关闭仍生效（测试）。
- D4 自研功能只适配不重写：是（五项功能均有测试/静态证据）。
- D7 前端以上游为准重建：是（P4.0–P4.6）。
- D8 数据库只追加：是；**MySQL 未验证**，owner 裁决当前 PG 部署、后续不用 MySQL，不声称三库兼容（WARNING）。
- D1 rebase：实际为基于 `origin/main` 重建分支并保留旧历史于 `backup-before-upstream`；已在账本 Ruling 记录（WARNING）。

## 已知限制（非阻塞）

1. MySQL 未验证（owner 裁决）；不得声称三库兼容。
2. S3 管理接口 `/api/data/usage_report`、`/api/channel/glm/usage` 无 dashboard 凭据，仅静态/接口 401 证据，未做浏览器人工验收。
3. S2 未实测 `FamilyClaude`/`FamilyClaude47`；GLM 中文小样本约 +12% 为已知边缘。
4. ClickHouse 日志库未实跑；每次启动 ALTER 的权限要求需在部署文档说明。
5. 遗留 deferred minor 见账本与 `final-review.md` §3/§5。

## 最终整分支审查

- 无 Critical。
- I-1 隐私、I-3 `.gitignore`、M-1/M-2/M-3/M-6/M-7/M-12 已在 `7b0600943` 修复，scoped re-review PASS。
- I-2 MySQL 由 owner 裁决接受。

## 结论

无 CRITICAL。存在已记录的 WARNING/限制（MySQL、S3 浏览器验收、Claude 家族 S2、ClickHouse 实跑），均已由 owner 裁决或明确记录为后续跟进。**可进入 archive（带已知限制）**。
