# Comet Design Handoff

- Change: migrate-onto-upstream-main
- Phase: design
- Mode: compact
- Context hash: e180bcd68a04251becaa5318c8ec65996f88324f8d430a5231660164171e5866

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/migrate-onto-upstream-main/proposal.md

- Source: openspec/changes/migrate-onto-upstream-main/proposal.md
- Lines: 1-35
- SHA256: 6ae3306c7ca3951bdc7f006d69f6053610823d81c8952f8333b77dcdfbc3c355

```md
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

```

## openspec/changes/migrate-onto-upstream-main/design.md

- Source: openspec/changes/migrate-onto-upstream-main/design.md
- Lines: 1-89
- SHA256: 310494b094e79f51607f6cca26671e7b7cc499a06c6a1cd73af9cc065884b858

[TRUNCATED]

```md
# Design

## Context

动机见 `proposal.md`。这里只记录影响实现方式的现状与约束：

- 分叉点 `2b6f1dfe`（2026-08-30），本地 25 个提交，上游 460 个提交，直接冲突 39 个文件。
- 上游 `bb28f2cca` 已把 token 计数抽成独立 `tokenkit` 模块，并保留 `service.EstimateRequestToken` / `CountRequestToken` 两个入口；`CountRequestToken` 不受 `constant.CountToken` 计费开关影响。
- 上游 `relay/`（193 文件）与 `relaykit/`（136 文件）有大范围改造，`web/` 有 924 文件重构。
- 本地自研功能全部位于上游不存在的文件或文件内的独立片段中，没有被上游同名实现覆盖。
- 仓库此前没有 OpenSpec 规格，`context-token-limit` 是首个能力建档。

## Goals / Non-Goals

**Goals:**

- 用一次可回滚的迁移，让 `main` 与上游同步，同时保住五项自研功能。
- 迁移后 token 计数只有一套实现（上游 tokenkit），不留双份逻辑。
- 迁移过程每一步都可构建、可测试，不产生「编译通过但行为错误」的中间态。

**Non-Goals:**

- 不为自研功能补写规格（除 `context-token-limit`）。
- 不向上游提交 PR，不改变上游分支。
- 不追求 25 个提交逐条语义等价。

## Decisions

### D1：用 rebase 重放本地提交，而不是 merge

merge 会在 39 个冲突文件上产生一次巨型合并提交，且后续每次同步上游都要再解一遍同样的冲突。rebase 让本地提交直接坐在 `origin/main` 之上，冲突只解一次，历史保持线性。

代价是每次冲突解决都要重新生成提交，且中途状态不可推送。用 `backup-before-upstream` 分支与 fork 远端双保险。

### D2：token 计数整体采用上游 tokenkit，删除本地实现

本地 `service/tokenizer.go` 的 codec 池、`relaykit/dto` 中为计数而加的 `GetTokenCountMeta` 增强，全部丢弃。上游已覆盖同样的缺陷清单，且按模型家族校准、按供应商规则计算图片 token、修复了 BPE 二次复杂度。

保留本地唯一不可替代的部分：**计数结果的消费方式**（上下文限制、日志对账），这些接在上游出口上。

### D3：上下文限制接 `CountRequestToken`

上游有两个入口：`EstimateRequestToken` 受 `constant.CountToken` 开关控制，`CountRequestToken` 不受。上下文限制属于安全边界而不是计费估算，运维关闭计费估算时限制必须依然生效，因此接 `CountRequestToken`。

替代方案是继续用本地估算器，但它将被删除；另一个方案是新建独立计数入口，与上游重复。

### D4：自研功能只做适配，不重写

五项自研功能（使用报告、部门属性、GLM 用量、缓存命中率、请求/输出预览）在上游都不存在。迁移时以上游的周边接口为准做适配，内部实现保持原样，避免把迁移变成重构。

### D5：提交粒度按主题重组，允许丢弃被取代的提交

本地 25 个提交中有探索性中间态与重复提交（两组重复的设计文档）。迁移时按主题合并为可独立解释的提交；纯 token 统计增强的提交整体丢弃。提交信息仍使用仓库既有的中文一行式。

### D6：在独立分支作业，`main` 保持可用

迁移在 `migrate/upstream-2026-11` 上进行。只有 S1–S5 全部通过后才把 `main` 指过去。部署验证仍在 `main` 对应的镜像上进行，避免验证期间无法回退。

### D7：前端以上游为准重建，再逐区挂载

`web/` 的 924 文件重构无法逐行合并。策略是：冲突文件一律取上游版本，然后按区域（usage-logs / dashboard / system-settings）重新挂载自研字段与组件，i18n 以上游文件为基线重跑 `i18n:sync` 后补齐自研键。

### D8：数据库只做追加，不做改写

新增字段（`cache_read_tokens`、部门属性相关列）全部由 AutoMigrate 追加，三库（SQLite / MySQL / PostgreSQL）验证建表与二次启动幂等。不做数据回填，历史行的新字段为 0。

### D9：Go 1.26 与镜像构建走宿主机

`GOTOOLCHAIN=auto` 通过代理拉取 1.26.10 工具链；镜像构建沿用「宿主机构建前端与二进制 + 最小 Dockerfile 打包」，不再依赖容器内网络。

## Risks / Trade-offs

- **语义冲突**（上游改了 `relay/`、`relaykit/` 的接口，本地改动编译通过但行为错误）→ 迁移后对 Claude / Gemini / OpenAI 三条链路做计费对账，并保留具名回归用例；不整批使用 `-X ours/theirs`。
- **前端重挂遗漏**（自研字段在上游重构后的组件树上失效）→ 按区域逐项走查，S3 场景覆盖全部五项。
- **rebase 中途损坏** → `backup-before-upstream` + fork 远端双备份；任何阶段可整体回退。
- **估算精度回退**（本地 ±5% 换上游 1% bias，理论更好，但模型映射差异可能引入新偏差）→ 迁移后用同一批真实 Claude Code 请求复测，S2 作为门禁。
- **Go 1.26 工具链拉取失败** → 迁移前先验证 `GOTOOLCHAIN=auto` 能下载；失败则改用宿主机安装包。

## Migration Plan


```

Full source: openspec/changes/migrate-onto-upstream-main/design.md

## openspec/changes/migrate-onto-upstream-main/tasks.md

- Source: openspec/changes/migrate-onto-upstream-main/tasks.md
- Lines: 1-47
- SHA256: c5025240b3c818025cf76e51ffe90e8d6541ab4f25bb9d214dcc18db621951d3

```md
# Tasks

## 1. 迁移准备

- [ ] 1.1 从 `origin/main` 创建 `migrate/upstream-2026-11` 分支；验证 `git merge-base --is-ancestor origin/main HEAD` 返回 0
- [ ] 1.2 验证 Go 1.26.10 工具链可通过 `GOTOOLCHAIN=auto` 拉取；验证根模块、`relaykit/`、`tokenkit/` 三个模块 `go build ./...` 全部通过
- [ ] 1.3 记录迁移基线：`origin/main` 提交号、三模块测试基线结果、前端 `bun run test` 基线结果，写入 change 目录备注

## 2. 批次一：文档重放

- [ ] 2.1 重放 CLAUDE.md 中文重写提交；冲突处以中文版为准并并入上游新增条款（如计费规则必读门禁）；验证文件内无 `<<<<<<<` 标记且引用的模块数量与实际一致
- [ ] 2.2 重放 usage report 设计文档提交，丢弃内容重复的第二份；验证 `docs/` 下无同名重复文件、`git log --diff-filter=A` 只出现一次

## 3. 批次二：后端自研功能

- [ ] 3.1 重放使用报告相关提交（`model/usage_report*`、`controller/usage_report.go`、`dto/usage_report.go`）；验证 `go test ./model/ ./controller/ -run UsageReport` 通过
- [ ] 3.2 重放用户部门属性提交（`setting/department_setting.go`、`model/user.go`）；验证模型测试通过，且 SQLite 下 AutoMigrate 连续启动两次不报错
- [ ] 3.3 重放 GLM 渠道用量提交（`controller/glm_usage.go`、路由注册、渠道前端）；验证 `go test ./controller/ -run GLM` 通过且路由注册测试覆盖新增端点
- [ ] 3.4 重放缓存命中率统计提交（`cache_read_tokens` 从日志写入到使用报告聚合）；验证 `go test ./model/ -run RecordConsumeLog` 与使用报告聚合测试通过

## 4. 批次三：token 统计切换

- [ ] 4.1 丢弃本地的 token 统计增强提交（`service/tokenizer.go` codec 池、`relaykit/dto` 计数增强、`service/token_estimator.go`），采用上游 `tokenkit`；验证 `cd relaykit && GOWORK=off go build ./...` 与根模块构建通过
- [ ] 4.2 把上下文限制接到上游 `CountRequestToken`，保持 413 拦截与错误日志契约；验证超限请求返回 413 且错误码为 `context_limit_exceeded`
- [ ] 4.3 适配消费日志的 `estimated_prompt_tokens` 写入与前端「估算 / 实际」展示；验证新增字段在日志详情接口返回中可见
- [ ] 4.4 用同一批真实 Claude Code 请求复测估算偏差，与迁移前基线对比；验证偏差绝对值不超过 10%

## 5. 批次四：前端重建

- [ ] 5.1 以上游为基线重建 usage-logs 区域（错误日志列、估算/实际详情、请求与输出预览）；验证相关组件测试通过
- [ ] 5.2 以上游为基线重建 dashboard 区域（使用报告周/月对比、部门活跃、缓存命中率卡片与排行）；验证 `charts`、`stats` 单测与用户表测试通过
- [ ] 5.3 以上游为基线重建 system-settings 的上下文限制分组并完成路由与 section 注册；验证组件测试通过
- [ ] 5.4 重跑 `bun run i18n:sync` 并补齐自研键；验证七个语言文件的键集合一致，`bun run typecheck` 零错误

## 6. 全量验证与切换

- [ ] 6.1 三模块构建与 `make test` 全绿（根模块、`relaykit`、`tokenkit`）
- [ ] 6.2 前端 `bun run typecheck`、`bun run lint`、`bun run test` 全部通过
- [ ] 6.3 三数据库迁移验证：SQLite / MySQL / PostgreSQL 各在全新库与升级库上启动两次，确认建表幂等且既有数据与索引保留
- [ ] 6.4 计费对账：Claude / Gemini / OpenAI 三种格式的 quota 计算结果与上游实现一致
- [ ] 6.5 部署验证：构建镜像、`docker compose up -d --force-recreate new-api`，实测 S1 上下文限制、S2 估算偏差、S3 五项自研功能
- [ ] 6.6 把 `main` 快进到迁移分支并推送到 fork；验证 `git log origin/main..HEAD` 只包含预期提交

## Workflow follow-up

- 归档前确认 `backup-before-upstream` 分支与 fork 远端的保留策略
- 归档后清理 `migrate/upstream-2026-11` 分支（若已合并）

```

## openspec/changes/migrate-onto-upstream-main/specs/context-token-limit/spec.md

- Source: openspec/changes/migrate-onto-upstream-main/specs/context-token-limit/spec.md
- Lines: 1-73
- SHA256: 1def1d8b09986dab5e7c86b95583d67ce40466db414b7c460f22114709a035ca

```md
# Spec Delta

## Purpose

限定单次请求可携带的输入上下文规模，在转发上游之前拒绝超出模型上限的请求，并保证本地估算与上游真实计量的偏差足够小，使阈值真正生效。

## ADDED Requirements

### Requirement: 按模型限制输入上下文

系统 SHALL 在每次转发请求前计算输入侧 token 总数并与该模型的上限比较。全局默认上限与按模型覆写同时存在时，按模型配置优先；上限为 0 表示不限制。

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

```
