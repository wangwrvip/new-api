# Proposal

## Why

本地 fork 从分叉点 `2b6f1dfe`（2026-08-30）起积累了 25 个提交，上游同期前进了 460 个提交（1871 个文件、+31 万行）。其中上游 `bb28f2cca` 把 token 计数整体重写为独立的 `tokenkit` 模块，并用 54 个真实 Claude Code / Codex 会话窗口校准到 1% 偏差，能力覆盖且优于本地自行实现的统计增强。

结果是双向受损：本地拿不到上游三个月的新渠道、计费修复与前端重构；本地又在维护一套与上游重复、精度更低的 token 统计。现在需要一次性完成迁移，把维护面收敛为「上游实现 + 少量自研功能」。

## What Changes

- 把 25 个本地提交迁移到 `origin/main` 之上，解决 39 个直接冲突文件及前端语义冲突
- **BREAKING**（仅限内部实现）：删除本地 `service/tokenizer.go` 的 codec 连接池与 `relaykit/dto` 中的 `GetTokenCountMeta` 统计增强，token 计数统一委托上游 `tokenkit`
- 上下文限制（全局与按模型配置、413 拦截、`type=5` 错误日志）改接上游 `CountRequestToken` / `EstimateRequestToken`，不再依赖本地统计实现
- 保留并适配自研功能：使用报告（周/月对比 + PPT 导出）、用户部门属性、GLM 渠道用量查询、缓存命中率统计、日志请求/输出预览、日志「估算 / 实际」偏差展示
- 工具链升级：Go 1.25.1 → 1.26.10，`tiktoken-go` → v0.8.1，新增 `tokenkit` 独立模块
- 前端跟随上游对 `web/` 的 924 文件重构，重新挂载自研的三个 UI 区域
- 镜像构建流程改为「宿主机编译 + 最小 Dockerfile 打包」，避开容器内无外网的限制

## Capabilities

### New Capabilities

- `context-token-limit`: 按模型限制单次请求的输入上下文 token 数；超出上限时在调用上游之前拒绝请求并留下可审计的错误日志；本地估算值与上游返回的真实 `usage` 之间的偏差必须维持在可接受范围内，避免限制阈值失效

### Modified Capabilities

（无。仓库此前没有 OpenSpec 能力规格，本次是该能力的首次建档。）

## Impact

- **代码**：`relaykit/`、`relay/`、`service/`、`controller/`、`model/`、`web/`、`go.mod`、`go.sum`、`Dockerfile`、`.github/workflows/`
- **依赖**：Go 1.26.10；`tiktoken-go` v0.8.1；新增 `tokenkit` 模块（与 `relaykit` 同级的独立 go module）
- **数据**：`logs` 表新增字段（`cache_read_tokens` 等）沿用 GORM AutoMigrate 追加，SQLite / MySQL / PostgreSQL 三库向后兼容，无破坏性变更
- **部署**：Docker 镜像重建路径变化；`pg_data` 不进入构建上下文
- **风险**：上游对 `relay/` 与 `relaykit/` 的重构（329 个文件）可能与本地改动产生编译器检测不到的语义冲突，尤其是计费链路
