# Brainstorm Summary

- Change: migrate-onto-upstream-main
- Date: 2026-11-25

## 确认的技术方案

- 分批重放：B0 准备 → B1 文档 → B2 后端自研 → B3 tokenkit 接管与上下文限制改接 → B4 前端 → B5 全量验证与切换。
- token 计数整体采用上游 `tokenkit`，删除本地 `tokenizer.go` codec 池、`token_estimator.go`、`relaykit/dto` 计数增强。
- 上下文限制改接 `service.CountRequestToken`（与上游 `EstimateRequestToken` 同签名，一行替换），使其不受 `constant.CountToken` 计费开关影响。
- 冲突文件分三类处置：取上游（被取代的实现）、取本地（上游不存在的自研功能）、手工合并（双方都改）。
- 前端采用逐文件三方合并（方案 A），`usage-report/` 整目录新增，`section-registry` 手工合并注册点，i18n 以上游为基线重跑后补自研键。
- Go 1.26.10 通过 `GOTOOLCHAIN=auto` 拉取；镜像沿用宿主机编译 + 最小 Dockerfile。
- 三库验证使用本地 SQLite 与 compose 临时 MySQL / PostgreSQL，各在全新库与升级库上启动两次。

## 关键取舍与风险

- 取舍：多花时间做逐文件三方合并，换取冲突面最小、改动意图完整；代价是 22 个前端文件需逐个过。
- 取舍：接受消费日志在计费估算开关关闭时不再带 `estimated_prompt_tokens`，而上下文限制仍然生效。
- 风险：上游 `relay/request_billing.go` 重构后，本地 `service/text_quota.go` 的计费路径可能出现编译通过但数值错误的语义冲突 → 用三格式 quota 对账兜底。
- 风险：rebase 中途损坏 → `backup-before-upstream` + fork 远端双备份。
- 风险：上游前端重构导致隐性不兼容 → 逐文件合并后由 S3 与前端测试兜底。

## 测试策略

单元（受影响包）→ 集成（三库迁移幂等）→ 端到端（镜像部署实测 S1/S2/S3）→ 对账（S4）。每批重放结束即跑该批门禁，不攒到最后。

## Spec Patch

`context-token-limit` 增加一条验收场景：计费估算开关关闭时，上下文限制仍然生效。
