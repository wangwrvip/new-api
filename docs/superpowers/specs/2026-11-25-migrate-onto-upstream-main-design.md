---
comet_change: migrate-onto-upstream-main
role: technical-design
canonical_spec: openspec
---

# 迁移到上游 main 的技术设计

需求与验收标准见 `openspec/changes/migrate-onto-upstream-main/specs/context-token-limit/spec.md`，动机与范围见同目录 `proposal.md`、`design.md`。本文只记录实现层面的技术决策、风险与验证方式。

## 1. 分批重放

### 决策

把 25 个本地提交按主题分成五批重放到 `origin/main` 之上，每批结束立刻跑该批门禁，不允许「先全部 rebase 完再统一测试」。

| 批次 | 内容 | 门禁 |
|---|---|---|
| B0 | 从 `origin/main` 建 `migrate/upstream-2026-11`；确认 Go 1.26.10 可拉取；记录基线 | 三模块 `go build ./...` |
| B1 | `CLAUDE.md` 中文重写；usage report 设计文档（去重） | 无冲突标记；文档引用与实际模块数一致 |
| B2 | 使用报告 → 部门属性 → GLM 用量 → 缓存命中率 | `go test ./model/ ./controller/` 对应用例 |
| B3 | tokenkit 接管；上下文限制改接 `CountRequestToken`；日志估算字段适配 | 三模块构建；413 场景本地验证 |
| B4 | usage-logs → dashboard → system-settings → i18n | `bun run typecheck`；相关组件测试 |
| B5 | 三库迁移、计费对账、部署实测、切换 `main` | S1–S5 全通过 |

### 理由

后端必须先于前端：前端的 `types.ts` 直接映射后端 DTO，先改前端会导致类型对不上。B3 放在 B2 之后，是因为 `estimated_prompt_tokens` 的写入点（`service/log_info_generate.go`）本身在 B2 里被改动。

替代方案是一次性 `rebase --onto` 后统一处理冲突。被否决：39 个冲突文件跨 5 个模块，一次性处理时无法判断某个冲突该保留哪一侧——门禁提供了判断依据。

## 2. tokenkit 接线

### 决策

- `controller/relay.go` 中的 `service.EstimateRequestToken(c, meta, relayInfo)` 改为 `service.CountRequestToken(c, meta, relayInfo)`。两者签名相同，无需适配层。
- 删除本地 `service/tokenizer.go` 的 codec 连接池、`service/token_estimator.go`、`relaykit/dto` 中为计数而加的增强，全部取上游版本。
- 消费日志继续读 `relayInfo.GetEstimatePromptTokens()`——上游 `relay/common/relay_info.go` 原生提供该字段。

### 边界条件

- 上游只在 `relay/request_billing.go` 且 `constant.CountToken` 为真时设置估算值。因此计费估算开关关闭时，消费日志不写 `estimated_prompt_tokens`（契约中的「无估算值不留空字段」），但上下文限制必须仍然生效——这正是接 `CountRequestToken` 的原因。
- 上下文限制需要在开关关闭时也能拿到 `meta`：`info.Request.GetTokenCountMeta()` 必须独立调用，不能复用 `PrepareRequestBilling` 里受开关保护的 `meta`。
- 上游 `EstimateRequestToken` 内部对 `CountToken` 关闭会直接返回 0，若误接该入口，限制会静默失效且不报错——这是本设计最需要防的回归。

### 替代方案

- 继续用本地估算器：将被删除，且精度低于上游。
- 新建独立计数入口：与上游重复，违背「只留一套实现」的目标。

## 3. 冲突处置

### 决策

39 个直接冲突文件按三类处置：

| 类别 | 判据 | 文件 |
|---|---|---|
| 取上游 | 本地实现被上游完整取代 | `relaykit/dto/*`、`service/tokenizer.go`、`service/token_counter.go`、`service/usage_helpr.go` |
| 取本地 | 上游完全没有对应功能 | `setting/operation_setting/context_limit_setting.go`、`controller/usage_report.go`、`controller/glm_usage.go`、`setting/department_setting.go`、`model/usage_report*`、`web/.../usage-report/` |
| 手工合并 | 双方都改过同一区域 | `controller/relay.go`、`model/log.go`、`model/usedata.go`、`service/log_info_generate.go`、`service/text_quota.go`、17 个前端文件、7 个 i18n 文件、`bun.lock` |

### 禁止事项

不允许对整次 rebase 使用 `-X ours` / `-X theirs`。`relay/` 与 `relaykit/` 被上游改造了 329 个文件，其中的计费与格式转换语义冲突会编译通过、运行时才暴露。

## 4. 前端重挂

### 决策

逐文件三方合并（方案 A）。理由：核对过自研改动过的 6 个前端文件，5 个上游仍然存在同名文件，上游是新增 `audit/` 与大量测试，不是搬走原文件，因此三方合并能覆盖大部分场景。

具体规则：

- `usage-report/` 整目录新增，不与上游冲突。
- `web/src/features/dashboard/section-registry.tsx`、`system-settings/security/section-registry.tsx` 手工合并：保留上游的注册项，追加自研项。
- `common-logs-columns.tsx`、`details-dialog.tsx` 手工合并：以上游结构为骨架，把「错误日志列」「估算/实际」行加回。
- i18n：以上游七个语言文件为基线，重跑 `bun run i18n:sync` 后补齐自研键，最后校验七份键集合一致。

### 边界条件

自研键里有 `Cache Hit Rate` 这类既可能被上游新增、也可能我们自己命名的键，合并时以上游已有翻译为准，避免同义双键。

## 5. Go 1.26 与构建

### 决策

- `GOTOOLCHAIN=auto`，通过 `HTTPS_PROXY` 拉取 go1.26.10 工具链；构建前先单独验证一次拉取成功。
- `tiktoken-go` 升到上游版本 v0.8.1。
- 镜像构建沿用已验证路径：宿主机 `bun run build` + `go build`，再用最小 Dockerfile `FROM <base> + COPY` 打包，避免容器内无外网。

### 替代方案

在 Dockerfile 内升级 Go 镜像并执行 `go mod download`。被否决：本机容器无外网，之前已因此失败一次。

## 6. 三数据库验证

### 决策

- SQLite：本地默认路径，全新库与升级库各启动两次。
- MySQL / PostgreSQL：用 compose 起临时实例，验证 `AutoMigrate` 建表、二次启动幂等、既有数据与索引保留。
- 重点验证新增列 `logs.cache_read_tokens` 与部门相关列在三库上的默认值与类型一致性。

### 理由

上游同期改过迁移逻辑（`fix(model): migrate legacy token key constraints` 等），本地新增列必须证明与上游迁移共存无冲突。仅单元测试或只测一个方言不足以证明。

## 风险

| 风险 | 影响 | 缓解 |
|---|---|---|
| 计费语义冲突 | 数值错误但编译通过 | B3 后做 Claude / Gemini / OpenAI 三格式 quota 对账 |
| 计费开关关闭导致限制失效 | 阈值完全不起作用 | 接 `CountRequestToken`；补 Spec 场景；S1 覆盖开关关闭状态 |
| rebase 中途损坏 | 无法继续 | `backup-before-upstream` 分支 + fork 远端 |
| 前端隐性不兼容 | 自研字段不显示 | 逐文件合并 + S3 + 组件测试 |
| Go 工具链拉取失败 | 构建阻塞 | B0 先验证，失败则用宿主机安装包 |
