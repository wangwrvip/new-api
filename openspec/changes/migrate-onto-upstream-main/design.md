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

1. 建立 `migrate/upstream-2026-11` 分支，从 `origin/main` 出发。
2. 分四批重放本地提交：文档 → model/controller 后端 → web → 大特性（上下文限制、缓存命中率）。
3. 每批结束时跑该批的最小验证（构建 + 受影响包测试），不攒到最后。
4. 全量验证 S1–S5 后，把 `main` 快进到迁移分支。
5. 回滚：`git reset --hard backup-before-upstream`，或从 fork 重新拉取。

## Open Questions

- 上游 `web/` 重构后是否已有与「请求预览」等价的能力，若有则自研预览可简化为配置开关（不影响本次范围与任务拆分，迁移时按实际情况决定）。
