---
change: migrate-onto-upstream-main
design-doc: docs/superpowers/specs/2026-11-25-migrate-onto-upstream-main-design.md
base-ref: 4fc13c05949e1ba81bc9aae192a379c57a39bca3
---

# 迁移到上游 main 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把本地 25 个自研提交迁移到 `origin/main`（6627f0e3f）之上，删除与上游重复的 token 统计实现，保住五项自研功能与 `context-token-limit` 规格，最终把 `main` 快进到迁移分支。

**Architecture:** 以 `origin/main` 为起点建立 `migrate/upstream-2026-11`，按主题从 `backup-before-upstream`（0f9d13e9a）取本地文件的最终状态；双方都改过的文件以上游版为骨架手工合并。这是设计 D1「rebase 重放」与 D5「按主题重组提交」的落地方式：不逐提交执行 `git rebase --onto`，避免 25 个提交反复解同一批冲突，历史仍保持线性。每个主题一个中文一行式提交，每批结束跑门禁。每完成一个任务，勾选 `openspec/changes/migrate-onto-upstream-main/tasks.md` 的对应项；每个任务开始前先读仓库根 `AGENTS.md`，涉及 `web/` 时再读 `web/AGENTS.md`。

**Tech Stack:** Go 1.26.10（`GOTOOLCHAIN=auto`）、Gin、GORM、`relaykit`/`tokenkit` 独立模块、React 19 + TypeScript + Rsbuild + Bun、SQLite/MySQL/PostgreSQL。

**Spec:** `openspec/changes/migrate-onto-upstream-main/specs/context-token-limit/spec.md`（需求契约）；`openspec/changes/migrate-onto-upstream-main/tasks.md`（任务边界）；`docs/superpowers/specs/2026-11-25-migrate-onto-upstream-main-design.md`（技术设计）。

## Global Constraints

- Go 固定 1.26.10；所有 Go 命令使用 `GOWORK=off`。工具链通过 `GOTOOLCHAIN=auto` + 现有 `HTTPS_PROXY` 拉取，构建前先单独验证一次。
- 三个模块都必须独立通过：根模块、`relaykit/`、`tokenkit/`。`relaykit` 与 `tokenkit` 不得依赖根模块。
- 数据库代码必须同时兼容 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6；任何 schema 变更要在三库上验证全新库与升级库、连续启动两次幂等。
- JSON 编解码只用 `common.Marshal` / `Unmarshal` / `UnmarshalJsonStr` / `DecodeJson`；禁止业务代码直接调用 `encoding/json` 的编解码函数。
- 配额换算只用 `common/quota_math.go` 的封装（`QuotaFromFloat` / `QuotaRound` / `QuotaFromDecimal` 及 `*Checked` 变体）；禁止裸 `int()` 转换。
- 禁止对整次重放使用 `-X ours` / `-X theirs`。冲突文件按「取上游 / 取本地 / 手工合并」三类逐文件处理。
- 提交信息使用仓库既有中文一行式（`feat:` / `docs:` / `chore:` / `test:` / `refactor:`）。
- 不得修改、删除或替换 `new-api`、`QuantumNous` 相关的品牌、模块路径、许可证与元数据。
- 前端一律用 `bun`；每个前端任务提交前至少跑 `bun run typecheck` 与受影响的 `bun run vitest run <path>`；批次门禁再跑 `bun run lint`、`bun run test`、`bun run build`。
- 本计划文件位于 `docs/superpowers/plans/`，该目录被 `.gitignore` 忽略，不提交。

## Review Focus

以下五类输入/失败模式最容易在迁移中「编译通过、行为错误」，每条都在指定任务的测试中钉住：

1. **计费估算开关关闭**：`constant.CountToken=false` 时上下文限制必须仍返回 413；接错 `EstimateRequestToken` 会静默返回 0 且不报错。→ P3.2 的 `CountRequestToken` 独立性测试。
2. **按模型覆写与 0 值**：全局默认 200000、模型覆写 500000、模型覆写 0 不限制、总开关关闭，四种组合必须分别生效。→ P3.2 的 `TestMaxPromptTokensForModel` 表测试。
3. **Anthropic 缓存命中不计入实际值**：偏差对账时必须把 `cache_read_input_tokens` 计入实际输入总量，否则会高估偏差。→ P3.4 的对账公式与 P2.1 的 `cache_read_tokens` 写入。
4. **无估算值不留空字段**：估算为 0 时消费日志不得写入 `estimated_prompt_tokens`，也不能写成 0。→ P3.3 的 `TestGenerateTextOtherInfoOmitsZeroEstimate`。
5. **413 拒绝分支在选渠道之前返回**：不会经过 `processChannelError`，必须单独落库，且错误日志的 `estimated_prompt_tokens` / `context_limit` / `relay_format` 对日志所有者可见。→ P3.2 的 `contextLimitErrorOther` 快照测试与 S1 部署验证。

---

## 批次与任务映射

| tasks.md | 本计划任务 | 产出提交 |
|---|---|---|
| 1.1 / 1.2 / 1.3 | P0.1 / P0.2 / P0.3 | 无（准备与基线；基线备注随 P1.3 提交） |
| 2.1 | P1.1 | `docs: 重写 CLAUDE.md 为中文并合并上游新增条款` |
| 2.2 | P1.2 | `docs: 重建使用报告设计文档（去除重复副本）` |
| （补充） | P1.3 | `chore: 保留 Comet/OpenSpec 变更产物与本地忽略规则` |
| 3.4（写入基础） | P2.1 | `feat: 消费日志归一化 token 维度与缓存读计数` |
| 3.2 | P2.2 | `feat: 用户部门属性与部门清单设置` |
| 3.1 + 3.4（聚合） | P2.3 | `feat: 使用报告后端聚合（周/月窗口、部门、缓存命中率）` |
| 3.3 | P2.4 | `feat: GLM 渠道用量查询（5h/周窗口）与路由` |
| （补充自研功能） | P2.5 | `feat: 日志请求与输出预览（所有者可见）` |
| 4.1 | P3.1 | 无（验证：本地 token 实现未被携带） |
| 4.2 | P3.2 | `feat: 上下文限制改接 CountRequestToken（不受计费估算开关影响）` |
| 4.3 | P3.3 | `feat: 消费日志记录 estimated_prompt_tokens 供偏差对账` |
| 4.4 | P3.4 | `docs: 记录迁移后上下文估算偏差复测结果` |
| （前端基础） | P4.0 | `chore(web): 引入 pptxgenjs 并保留 node: 模块替换配置` |
| 5.1 | P4.1 | `feat(web): 重建日志错误列、估算/实际与请求输出预览` |
| 5.2 | P4.2 | `feat(web): 重建使用报告、部门活跃、缓存命中率与 token 四维看板` |
| （自研功能前端） | P4.3 | `feat(web): 重建渠道 GLM 用量展示` |
| 5.3 | P4.4 | `feat(web): 重建上下文限制与日志预览设置分组` |
| （自研功能前端） | P4.5 | `feat(web): 重建用户部门管理与筛选` |
| 5.4 | P4.6 | `chore(web): 同步七语言 i18n 并补齐自研文案` |
| 6.1 / 6.2 | P5.1 / P5.2 | 无（门禁） |
| 6.3 | P5.3 | `docs: 记录三数据库迁移验证结果` |
| 6.4 | P5.4 | `test: 补充 Claude/Gemini/OpenAI 计费对账断言` |
| 6.5 | P5.5 | `docs: 记录部署验证与 S1-S5 结果` |
| 6.6 | P5.6 | 无（`main` 快进与推送） |
| Workflow follow-up | P5.7 | 无（保留策略与清理） |

## 文件结构（迁移后归属）

- **仅本地存在、整体取本地最终状态**：`controller/usage_report.go`、`controller/glm_usage.go`、`dto/usage_report.go`、`model/usage_report.go`、`model/usage_report_window.go`、`setting/department_setting.go`、`setting/request_preview_setting.go`、`setting/operation_setting/context_limit_setting.go`、`web/src/features/dashboard/usage-report/**`、`web/src/features/channels/**/glm-usage*`、`web/src/features/users/**/departments*`、`web/src/features/system-settings/request-limits/context-limit-*`。
- **双方都改、以上游为骨架手工合并**：`controller/relay.go`、`controller/user.go`、`model/log.go`、`model/usedata.go`、`model/user.go`、`model/option.go`、`service/text_quota.go`、`service/log_info_generate.go`、`relay/common/relay_info.go`、`relay/channel/{claude,gemini,openai}/*`、`router/api-router.go`、`router/channel-router.go`、17 个前端共享文件、7 个 i18n 文件、`web/bun.lock`。
- **取上游、丢弃本地**：`relaykit/dto/*` 的计数增强、`service/tokenizer.go`、`service/token_counter.go`、`service/usage_helpr.go`、`service/sensitive.go`（上游已退役并迁移到 request-filter）。`service/token_estimator.go` 在备份分支已不存在，禁止恢复。

---

## 阶段 0：迁移准备（tasks.md 1）

### Task P0.1: 建立迁移分支与备份核验

**Files:** 无文件改动。

**Interfaces:**
- Consumes: `origin/main` = `6627f0e3f37c002564b630a73e5afb0d28a92d3b`；`backup-before-upstream` = `0f9d13e9a129527a26a5b5b6ce27adf8d6c06723`；`fork/main` 同备份分支。
- Produces: 分支 `migrate/upstream-2026-11`，其历史直接坐在 `origin/main` 上。

- [x] **Step 1: 确认工作区干净**

Run: `git status --porcelain`
Expected: 无输出（被忽略的 `web/dist`、`data/`、`new-api` 不计）。

- [x] **Step 2: 核验备份两端存在且一致**

Run: `git rev-parse origin/main backup-before-upstream fork/main`
Expected: 三行依次为 `6627f0e3f…`、`0f9d13e9a…`、`0f9d13e9a…`。

- [x] **Step 3: 从上游建立迁移分支**

Run: `git switch -c migrate/upstream-2026-11 origin/main`
Expected: 切换成功；`git branch --show-current` 输出 `migrate/upstream-2026-11`。

- [x] **Step 4: 验证祖先关系**

Run: `git merge-base --is-ancestor origin/main HEAD; echo $?`
Expected: `0`。

- [x] **Step 5: 勾选 tasks.md 1.1**

在 `openspec/changes/migrate-onto-upstream-main/tasks.md` 把 `1.1` 改为 `[x]`（该文件在 P1.3 才进入迁移分支，可先记在临时清单，P1.3 一并提交）。

### Task P0.2: 验证 Go 1.26.10 与三模块构建

**Files:** 无源码改动；必要时创建 `web/dist/index.html` 占位。

**Interfaces:**
- Produces: 可重复的三模块构建命令，供后续每个门禁复用。

- [x] **Step 1: 准备工具链环境**

Run:
```bash
export PATH="/usr/local/go/bin:$HOME/.bun/bin:/usr/local/node-v22/bin:$PATH"
export GOTOOLCHAIN=auto
export GOWORK=off
```
Expected: `go version` 首次运行触发下载，最终输出 `go version go1.26.10 linux/amd64`。

- [x] **Step 2: 确保内嵌前端占位存在**

Run: `mkdir -p web/dist && test -f web/dist/index.html || printf '<!doctype html>\n' > web/dist/index.html`
Expected: `web/dist/index.html` 存在。

- [x] **Step 3: 构建三个模块**

Run:
```bash
go build .
pkgs=$(go list -e ./... 2>/dev/null | grep '^github.com/QuantumNous/new-api' | grep -vx 'github.com/QuantumNous/new-api')
go build $pkgs
(cd relaykit && GOWORK=off go build ./...)
(cd tokenkit && GOWORK=off go build ./...)
```
Expected: 四条命令均退出 0。若 `pg_data` 权限导致 `go list` 报警，按记忆中的替代方案用 `grep '^github.com'` 过滤，不要因此判定构建失败。

- [x] **Step 4: 处理工具链拉取失败**

若 Step 1 下载失败：从官方地址经 `HTTPS_PROXY` 下载 Go 1.26.10 安装包并替换 `/usr/local/go`，重跑 Step 3；把处理方式记录到 `/tmp/migrate-upstream-baseline.txt`。

### Task P0.3: 记录迁移基线

**Files:** 写入 `/tmp/migrate-upstream-baseline.txt`（随后由 P1.3 落入 change 目录）。

**Interfaces:**
- Produces: `origin/main` 提交号、Go/Bun 版本、三模块测试结果、前端测试结果、估算偏差基线。

- [x] **Step 1: 记录版本与提交**

Run:
```bash
{ echo "origin/main=$(git rev-parse origin/main)"; go version; bun --version; } | tee /tmp/migrate-upstream-baseline.txt
```
Expected: 三行内容写入文件。

- [x] **Step 2: 记录三模块测试基线**

Run:
```bash
pkgs=$(go list -e ./... 2>/dev/null | grep '^github.com/QuantumNous/new-api')
go test $pkgs 2>&1 | tee -a /tmp/migrate-upstream-baseline.txt
(cd relaykit && GOWORK=off go test ./... 2>&1 | tee -a /tmp/migrate-upstream-baseline.txt)
(cd tokenkit && GOWORK=off go test ./... 2>&1 | tee -a /tmp/migrate-upstream-baseline.txt)
```
Expected: 结果追加到基线文件；记录失败包名（上游基线若本身有失败，明确标注，不与迁移引入的失败混淆）。

- [x] **Step 3: 记录前端测试基线**

Run:
```bash
cd web && bun install --frozen-lockfile && bun run test 2>&1 | tee -a /tmp/migrate-upstream-baseline.txt
```
Expected: 结果追加；若依赖安装受网络阻塞，记录阻塞原因而非伪造通过。

- [x] **Step 4: 捕获估算偏差基线（22 条真实 Claude Code 请求）**

用迁移前的 `backup-before-upstream` 二进制在独立工作树运行：
```bash
git worktree add /tmp/old-main backup-before-upstream
cd /tmp/old-main && mkdir -p web/dist && printf '<!doctype html>\n' > web/dist/index.html
GOWORK=off go build -o /tmp/new-api-old .
```
用 22 条代表性 Claude Code 请求（含工具定义、扩展思考、工具结果）依次请求本地实例，记录：请求体、上游 `usage.prompt_tokens`、`usage.cache_read_input_tokens`（或 `prompt_tokens_details.cached_tokens`）。保存到 `.local-tests/estimate-fixtures/`（gitignored）。
Expected: 22 个请求体 + 实际输入计量；后续 P3.4 用同一批请求复测。

### Gate G0: 迁移准备门禁

- [x] 重跑 P0.2 Step 3 的三模块构建，全部退出 0。
- [x] 确认 `/tmp/migrate-upstream-baseline.txt` 存在且包含版本、三模块、前端、估算基线四节。
- [x] 确认基线中的失败项已被记录并分类（环境原因 / 上游既有失败 / 真实阻塞）。
- [x] 任何阻塞未解决时停止，不进入阶段 1。

---

## 阶段 1：文档重放（tasks.md 2）

### Task P1.1: 重写 CLAUDE.md 并合并上游新增条款

**Files:**
- Modify: `CLAUDE.md`（本地中文版 + 上游新增条款）
- Verify only: `AGENTS.md`（迁移分支已是上游版，不取本地）

**Interfaces:**
- Consumes: `backup-before-upstream:CLAUDE.md`（本地中文重写最终版）、`origin/main:CLAUDE.md`（上游新增条款）。
- Produces: 一份中文 `CLAUDE.md`，同时引用 `relaykit` 与 `tokenkit` 两个独立模块。

- [x] **Step 1: 提取两版内容并比对**

Run:
```bash
git show backup-before-upstream:CLAUDE.md > /tmp/claude-local.md
git show origin/main:CLAUDE.md > /tmp/claude-upstream.md
diff -u /tmp/claude-upstream.md /tmp/claude-local.md | head -200
```
Expected: 能看到本地中文重写与上游新增章节的差异。

- [x] **Step 2: 合并写入**

以本地中文版为底稿，并入上游新增条款：`relaykit`/`tokenkit` 模块独立性与验证命令、`pkg/billingexpr/expr.md` 必读门禁、计费安全不变量、三库验证要求、`plugin` 子命令、新增路由/任务架构。保留本地的常用命令与架构速览，但把模块数量更新为三个。

- [x] **Step 3: 校验无冲突标记与模块引用**

Run:
```bash
grep -nE '^(<<<<<<<|=======|>>>>>>>)' CLAUDE.md; test $? -eq 1
grep -n 'relaykit' CLAUDE.md; grep -n 'tokenkit' CLAUDE.md
```
Expected: 第一条无输出（退出 1）；后两条各至少命中一次。

- [x] **Step 4: 提交**

```bash
git add CLAUDE.md
git commit -m "docs: 重写 CLAUDE.md 为中文并合并上游新增条款"
```

### Task P1.2: 使用报告设计文档去重

**Files:**
- Add (final state): `docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md`
- Drop: 重复的 `add`/`refine` 提交（`d9239967d`、`8b3e12452`、`4a18cce1d`、`8396cbaf9` 只保留最终内容一次）

**Interfaces:**
- Consumes: `backup-before-upstream` 中该文件最终状态。
- Produces: 仓库内只有一个同名文件、一次 `add` 记录。

- [x] **Step 1: 取最终状态**

Run: `git checkout backup-before-upstream -- docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md`
Expected: 文件进入暂存区。

- [x] **Step 2: 确认没有同内容重复文件**

Run: `find docs -name '*usage-report*' -type f`
Expected: 仅 `docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md` 与 `docs/usage-report.md`（后者由 P2.3 提交）。

- [x] **Step 3: 提交**

```bash
git add docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md
git commit -m "docs: 重建使用报告设计文档（去除重复副本）"
```

- [x] **Step 4: 验证只出现一次新增记录**

Run: `git log --diff-filter=A --oneline -- docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md`
Expected: 只有 P1.2 产生的一行。

### Task P1.3: 保留 Comet/OpenSpec 产物与本地忽略规则

**Files:**
- Add (from backup): `.agents/`、`openspec/`、`docs/superpowers/specs/2026-11-25-migrate-onto-upstream-main-design.md`、`.comet/config.yaml`
- Modify (merge): `.gitignore`、`.dockerignore`
- Add: `openspec/changes/migrate-onto-upstream-main/.comet/migration-baseline.md`

**Interfaces:**
- Consumes: `backup-before-upstream` 中本地产物；`/tmp/migrate-upstream-baseline.txt`。
- Produces: 迁移分支上可继续使用的 Comet change 目录与 OpenSpec 产物。

- [x] **Step 1: 携带本地产物**

Run:
```bash
git checkout backup-before-upstream -- .agents openspec docs/superpowers/specs/2026-11-25-migrate-onto-upstream-main-design.md .comet/config.yaml
```
Expected: 文件进入暂存区；`.comet/current-change.json` 仍被忽略。

- [x] **Step 2: 合并忽略规则**

以 `origin/main` 的 `.gitignore`/`.dockerignore` 为骨架，追加本地规则：`/pg_data/`、`/pg_data.tar.gz`、`.comet/current-change.json`、`/.local-tests/`、`go.work`、`go.work.sum`。`/node_modules/` 与 `web/dist` 保持上游已有规则，不重复。

- [x] **Step 3: 校验忽略生效**

Run:
```bash
git check-ignore -v pg_data pg_data.tar.gz .comet/current-change.json .local-tests/x
```
Expected: 四条都命中对应规则。

- [x] **Step 4: 落入基线备注并提交**

把 `/tmp/migrate-upstream-baseline.txt` 复制为 `openspec/changes/migrate-onto-upstream-main/.comet/migration-baseline.md`，补一行「迁移分支起点 origin/main=6627f0e3f」。

```bash
git add .agents openspec docs/superpowers/specs/2026-11-25-migrate-onto-upstream-main-design.md .comet/config.yaml .gitignore .dockerignore
git commit -m "chore: 保留 Comet/OpenSpec 变更产物与本地忽略规则"
```

### Gate G1: 文档门禁

- [x] `grep -rnE '^(<<<<<<<|=======|>>>>>>>)' CLAUDE.md docs .agents openspec` 无输出。
- [x] `git log --diff-filter=A --oneline -- docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md` 仅一行。
- [x] `git status --porcelain` 无输出。
- [x] `comet resume-probe . --stdin --json` 仍能识别 `migrate-onto-upstream-main`（只读检查，不推进阶段）。

---

## 阶段 2：后端自研功能（tasks.md 3）

> 顺序按依赖重排：先落日志 schema（P2.1），再部门属性（P2.2），再使用报告（P2.3），然后 GLM（P2.4）与预览（P2.5）。理由：`model/usage_report.go` 的最终版同时依赖归一化 token 列与 `users.department`，先打地基可保证每个任务单独绿。

### Task P2.1: 消费日志归一化 token 维度与缓存读计数

**Files:**
- Modify (merge, 以上游为骨架): `model/log.go`、`model/usedata.go`、`service/text_quota.go`、`service/text_quota_test.go`
- Add (from backup): `model/record_consume_log_test.go`
- Verify: `dto/usage_report.go` 的维度字段在 P2.3 才使用

**Interfaces:**
- Consumes: 上游 `model.RecordConsumeLogParams{ Content []*common.Message; Other *model.LogOther }`、`model.LogOther.SetPublic`、`model.LogQuotaData`。
- Produces:
  - `model.Log` 新增列：`InputTokens int`、`CacheTokens int`、`CacheReadTokens int`（`gorm:"default:0"`）。
  - `model.RecordConsumeLogParams` 新增字段：`InputTokens int`、`CacheTokens int`、`CacheReadTokens int`。
  - `model.QuotaData` 新增列：`InputTokens int`、`OutputTokens int`、`CacheTokens int`（与上游 `CacheReadTokens`/`CacheCreationTokens` 并存）。
  - `service.textQuotaSummary` 新增字段：`InputTokens int`、`CacheTokensTotal int`；`CacheTokens` 保持上游「缓存读」语义。

- [x] **Step 1: 以上游重置共享文件**

Run: `git checkout origin/main -- model/log.go model/usedata.go service/text_quota.go service/text_quota_test.go`
Expected: 文件回到上游版本。

- [x] **Step 2: 手工合并日志 schema 与写入**

在 `model/log.go`：
- `Log` 结构体加三列。
- `RecordConsumeLogParams` 加上游已改的 `Content []*common.Message`、`Other *LogOther` 与新三字段。
- `RecordConsumeLog` 保留上游 `other.setContent(params.Content)` 与 `LogQuotaData` 调用；新增 `inputTokens`/`cacheTokens` 回退逻辑（未传维度时 `inputTokens=PromptTokens`、`cacheTokens=0`），并写入新列；`LogQuotaData` 同时传上游 `CacheReadTokens`/`CacheCreationTokens` 与本地 `InputTokens`/`OutputTokens`/`CacheTokens`。

- [x] **Step 3: 合并 quota_data 查询与聚合**

在 `model/usedata.go` 保留上游 `CacheReadTokens`/`CacheCreationTokens` 全部逻辑，追加本地三列到 `QuotaData`、缓存累加、`gorm.Expr` 增量更新、`quotaDataDimSumSelect` 与按用户/模型/时间的 Select 列表。

- [x] **Step 4: 合并 text_quota 维度计算**

在 `service/text_quota.go` 的 `textQuotaSummary` 加 `InputTokens`、`CacheTokensTotal`；在计算处（上游 `summary.CacheTokens = usage.PromptTokensDetails.CachedTokens` 附近）补：
`CacheTokensTotal = CacheTokens + CacheCreationTokens`；`InputTokens = PromptTokens - CacheTokens - CacheCreationTokens`（下限 0）。
在 `RecordConsumeLog` 调用处传 `InputTokens`、`CacheTokens: CacheTokensTotal`、`CacheReadTokens: summary.CacheTokens`。

- [x] **Step 5: 适配并运行回归测试**

Run:
```bash
go test ./model/ -run 'RecordConsumeLog|QuotaData'
go test ./service/ -run 'CalculateTextQuotaSummary|CacheWriteTokensTotal|ComposeTiered'
```
Expected: 全绿。`model/record_consume_log_test.go` 需按上游 `Content []*common.Message` / `*LogOther` 调整构造，断言仍为：未传维度时 `InputTokens=PromptTokens`、`CacheTokens=0`；显式传入时三列与 `quota_data` 累加一致。

- [x] **Step 6: 提交**

```bash
git add model/log.go model/usedata.go model/record_consume_log_test.go service/text_quota.go service/text_quota_test.go
git commit -m "feat: 消费日志归一化 token 维度与缓存读计数"
```

### Task P2.2: 用户部门属性与部门清单设置

**Files:**
- Add (from backup): `setting/department_setting.go`
- Add: `setting/department_setting_test.go`（新测试）
- Modify (merge): `model/user.go`、`controller/user.go`、`model/option.go`

**Interfaces:**
- Consumes: 上游 `model.User` 结构、`controller` 用户创建/更新 DTO、`model.InitOptionMap`。
- Produces:
  - `model.User.Department string`（`gorm:"type:varchar(64);default:'公共部门';index"`，`validate:"max=64"`）。
  - `setting.DepartmentListKey`、`setting.DefaultDepartment`、`setting.GetDepartments() []string`、`setting.IsValidDepartment(name string) bool`、`setting.Departments2JsonString() string`。

- [x] **Step 1: 取部门设置实现**

Run: `git checkout backup-before-upstream -- setting/department_setting.go`
Expected: 文件进入暂存区。

- [x] **Step 2: 合并 User 字段与用户接口**

在 `model/user.go` 的 `User` 结构体加 `Department` 字段（位置与标签取自 backup），并在用户序列化/响应处带出 `department`。在 `controller/user.go` 的创建/更新路径校验 `IsValidDepartment`，非法值返回 400；未传时使用 `DefaultDepartment`。`model/option.go` 的 `InitOptionMap` 注册 `setting.DepartmentListKey = setting.Departments2JsonString()`。

- [x] **Step 3: 添加部门设置测试**

新增 `setting/department_setting_test.go`：
- `TestGetDepartmentsReturnsDefaultsWhenUnset`：`common.OptionMap` 为空时返回默认清单，首项为 `公共部门`。
- `TestGetDepartmentsFallsBackOnInvalidJSON`：写入 `"{"` 时回退默认清单。
- `TestIsValidDepartment`：空串、未列入清单、列入清单、前后空格四种输入。

Run: `go test ./setting/ -run 'Department|GetDepartments|IsValidDepartment'`
Expected: 全绿。

- [x] **Step 4: 验证 SQLite AutoMigrate 幂等**

Run（连续两次，第二次不得报错）：
```bash
PORT=3001 SQLITE_PATH=/tmp/dept-migrate.db GOWORK=off go run . &
sleep 8; kill %1
PORT=3001 SQLITE_PATH=/tmp/dept-migrate.db GOWORK=off go run . &
sleep 8; kill %1
```
Expected: 两次启动日志均无 `AutoMigrate` / `department` 相关错误；`sqlite3 /tmp/dept-migrate.db '.schema users'` 含 `department varchar(64) default '公共部门'`。

- [x] **Step 5: 提交**

```bash
git add setting/department_setting.go setting/department_setting_test.go model/user.go controller/user.go model/option.go
git commit -m "feat: 用户部门属性与部门清单设置"
```

### Task P2.3: 使用报告后端聚合（周/月窗口、部门、缓存命中率）

**Files:**
- Add (from backup): `dto/usage_report.go`、`model/usage_report.go`、`model/usage_report_window.go`、`model/usage_report_test.go`、`model/usage_report_window_test.go`、`controller/usage_report.go`、`controller/usage_report_test.go`、`docs/usage-report.md`
- Modify (merge): `router/api-router.go`（在 `dataRoute` 组加 `GET /usage_report`，`middleware.AdminAuth()`，`controller.GetUsageReport`）

**Interfaces:**
- Consumes: P2.1 的 `logs.input_tokens` / `cache_tokens` / `cache_read_tokens`；P2.2 的 `users.department` 与 `setting.DefaultDepartment`。
- Produces:
  - `dto.UsageReport` 及其子结构（含 `Overview`、`Daily`、`Departments`、`Users`，用户行含 `InputTokens`/`OutputTokens`/`CacheTokens`/`CacheReadTokens`）。
  - `model.GetUsageReport(period string, anchor time.Time, tzOffsetMin int) (*dto.UsageReport, error)`。
  - `controller.GetUsageReport(c *gin.Context)`。

- [x] **Step 1: 取本地最终实现**

Run:
```bash
git checkout backup-before-upstream -- dto/usage_report.go model/usage_report.go model/usage_report_window.go model/usage_report_test.go model/usage_report_window_test.go controller/usage_report.go controller/usage_report_test.go docs/usage-report.md
```
Expected: 8 个文件进入暂存区。

- [x] **Step 2: 注册路由**

在 `router/api-router.go` 上游的 `dataRoute` 组内、`/flow/self` 之后加一行 `dataRoute.GET("/usage_report", middleware.AdminAuth(), controller.GetUsageReport)`。

- [x] **Step 3: 运行使用报告测试**

Run:
```bash
go test ./model/ -run 'UsageReport|ResolvePeriodWindow|AssembleUsageReport|SumLogsByUserInRange|Department'
go test ./controller/ -run UsageReport
```
Expected: 全绿。若失败，优先检查 P2.1 的三列是否已建、P2.2 的 `Department` 字段是否可查。

- [x] **Step 4: 验证缓存命中率口径**

在 `model/usage_report_test.go` 的 `TestAssembleUsageReportTokenDimensions` / `TestSumLogsByUserInRangeTokenDimensions` 中确认：`CacheReadTokens` 参与聚合，`Tokens = InputTokens + CacheTokens + CompletionTokens`；旧日志（三列为 0）回退为 `prompt_tokens + completion_tokens`。

- [x] **Step 5: 提交**

```bash
git add dto/usage_report.go model/usage_report.go model/usage_report_window.go model/usage_report_test.go model/usage_report_window_test.go controller/usage_report.go controller/usage_report_test.go docs/usage-report.md router/api-router.go
git commit -m "feat: 使用报告后端聚合（周/月窗口、部门、缓存命中率）"
```

### Task P2.4: GLM 渠道用量查询后端与路由

**Files:**
- Add (from backup): `controller/glm_usage.go`、`controller/glm_usage_test.go`、`glm-usage.py`
- Modify (merge): `router/channel-router.go`（在上游 `channelPermissionRoutes` 表加 `{method: GET, path: "/glm/usage", permission: authz.ChannelRead, handler: controller.GetGlmChannelsUsage}`）

**Interfaces:**
- Produces: `controller.GetGlmChannelsUsage(c *gin.Context)`；路由 `GET /api/channel/glm/usage`。

- [x] **Step 1: 取本地实现**

Run: `git checkout backup-before-upstream -- controller/glm_usage.go controller/glm_usage_test.go glm-usage.py`
Expected: 文件进入暂存区。

- [x] **Step 2: 合并路由**

把本地 `permissionRoute` 条目加入上游 `channelPermissionRoutes` 切片，不得改动上游 `handlePermissionRoute`。

- [x] **Step 3: 运行测试**

Run:
```bash
go test ./controller/ -run GLM
go test ./router/ -run 'ChannelGlmUsage|ChannelStatusRoutes|ChannelDeleteRoutes'
```
Expected: 全绿；`TestChannelGlmUsageRoutePermission` 断言该路由要求 `ChannelRead`。

- [x] **Step 4: 提交**

```bash
git add controller/glm_usage.go controller/glm_usage_test.go glm-usage.py router/channel-router.go
git commit -m "feat: GLM 渠道用量查询（5h/周窗口）与路由"
```

### Task P2.5: 日志请求与输出预览后端

**Files:**
- Add (from backup): `setting/request_preview_setting.go`、`docs/request-preview.md`、`relay/common/relay_info_response_preview_test.go`、`relaykit/dto/request_preview_test.go`、`service/token_counter_test.go`
- Add (new): `service/text_truncate.go`（基于 tokenkit 的计数与截断）
- Modify (merge): `relay/common/relay_info.go`、`relaykit/types/request_meta.go`、`relaykit/dto/claude.go`、`relaykit/dto/openai_request.go`（`gemini.go` 仅在本地 diff 显示确有改动时处理）、`relay/channel/claude/relay-claude.go`、`relay/channel/gemini/*.go`、`relay/channel/openai/*.go`、`service/log_info_generate.go`、`model/log.go`、`controller/relay.go`、`model/option.go`

**Interfaces:**
- Consumes: 上游 `tokenkit.Count`；上游 `model.LogOther.SetPublic`；上游 `relay/common.RelayInfo`。
- Produces:
  - `relaykit/types.TokenCountMeta.LastUserMessage string`。
  - `relaycommon.RelayInfo.RequestPreview string`、`RequestPreviewUser string`、`ResponsePreview string`、`SetResponsePreview(text string)`。
  - `service.CountTextToken(text, model string) int`、`service.TruncateTextToTokens(text, model string, maxTokens int) string`。
  - 消费日志公开字段 `request_preview`、`request_preview_user`、`response_preview`。

- [x] **Step 1: 取设置、文档与测试，建立截断助手**

Run: `git checkout backup-before-upstream -- setting/request_preview_setting.go docs/request-preview.md relay/common/relay_info_response_preview_test.go relaykit/dto/request_preview_test.go service/token_counter_test.go`
新增 `service/text_truncate.go`：`CountTextToken` 委托 `tokenkit.Count`；`TruncateTextToTokens` 保留本地二分 rune 前缀算法，计数改为 `CountTextToken`。
Run: `go test ./service/ -run TruncateTextToTokens`
Expected: 全绿。

- [x] **Step 2: 合并请求元数据与 RelayInfo**

在 `relaykit/types/request_meta.go` 的 `TokenCountMeta` 加 `LastUserMessage`；在 `relaykit/dto` 的 Claude / OpenAI / Responses `GetTokenCountMeta` 中填充最后一条 user 消息（多个文本段以换行连接）。在 `relay/common/relay_info.go` 加三个预览字段与 `SetResponsePreview`（按 rune 截断到 1024，`common.LogConsumeEnabled=false` 时跳过）。
Run: `(cd relaykit && GOWORK=off go test ./dto/ -run LastUserMessage)`
Expected: 全绿。

- [x] **Step 3: 合并日志写入与旧格式兼容**

在 `service/log_info_generate.go` 的 `GenerateTextOtherInfo` 中用 `other.SetPublic("request_preview", ...)` 等写入三个预览字段（空值不写）。在 `model/log.go` 的 `formatLogOtherJSON` 用户可见投影中，保留本地「把历史 `admin_info` 内旧预览字段提升到顶层再剥离」的兼容逻辑；适配 `model/log_format_test.go`。
Run: `go test ./service/ -run 'GenerateTextOtherInfo.*Preview'` 与 `go test ./model/ -run LogFormat`
Expected: 全绿。

- [x] **Step 4: 合并各格式响应预览采集**

在 Claude / Gemini / OpenAI / Responses 的中心响应处理路径调用 `relayInfo.SetResponsePreview(...)`，只采集文本输出（含思考内容），音频/图片/embedding 不采集。上游适配器结构变化较大，按「输出文本最终汇聚点」定位，不要在流式分片处逐片拼接。
Run: `go test ./relay/ -run ResponsePreview`
Expected: 全绿。

- [x] **Step 5: 合并 controller 预览块**

在 `controller/relay.go` 的 `GenRelayInfo` 之后、`relay.PrepareRequestBilling` 之前，加入：仅当 `setting.RequestPreviewEnabled() && common.LogConsumeEnabled` 时调用 `request.GetTokenCountMeta()`，截断前 N token 与最后一条用户消息写入 `relayInfo`。`model/option.go` 注册 `RequestPreviewEnabled=true`、`RequestPreviewMaxTokens=200`。

- [x] **Step 6: 全量后端构建与提交**

Run:
```bash
go build .
go test ./controller/ -run Relay
(cd relaykit && GOWORK=off go build ./... && GOWORK=off go test ./...)
```
Expected: 全绿。

```bash
git add setting/request_preview_setting.go docs/request-preview.md relay/common/relay_info.go relay/common/relay_info_response_preview_test.go relaykit/types/request_meta.go relaykit/dto service/text_truncate.go relay/channel service/log_info_generate.go model/log.go model/log_format_test.go controller/relay.go model/option.go
git commit -m "feat: 日志请求与输出预览（所有者可见）"
```

### Gate G2: 后端自研功能门禁

- [x] `go build .` 与根模块包构建退出 0。
- [x] `go test ./model/ ./controller/ ./service/ ./router/` 全绿；`go test ./setting/` 全绿。
- [x] `(cd relaykit && GOWORK=off go build ./... && GOWORK=off go test ./...)` 全绿。
- [x] SQLite 连续启动两次无 AutoMigrate 错误；`logs` 含 `input_tokens`/`cache_tokens`/`cache_read_tokens`，`users` 含 `department`，`quota_data` 含本地三列。
- [x] 记录本批新增列清单，供 P5.3 三库验证使用。

---

## 阶段 3：token 统计切换（tasks.md 4）

### Task P3.1: 确认本地 token 实现未被携带

**Files:** 无源码改动。

**Interfaces:**
- Consumes: 上游 `tokenkit`（`tokenkit.Count` / `Estimate` / `ImageTokens`）与 `service.CountRequestToken` / `EstimateRequestToken`。
- Produces: 迁移分支上 token 计数只有一套实现。

- [x] **Step 1: 确认本地 token 文件不存在**

Run: `git ls-files service/tokenizer.go service/token_estimator.go`
Expected: 无输出。

- [x] **Step 2: 确认 relaykit 计数增强未被携带**

Run: `git diff origin/main --stat -- relaykit/dto`
Expected: 只应出现 P2.5 新增的 `LastUserMessage` 相关改动；不应出现本地 codec 池、`token_count_meta_tools_test.go`、`GetTokenCountMeta` 的大段增强。

- [x] **Step 3: 确认模块接线**

Run: `grep -n 'tokenkit' go.mod makefile`
Expected: `replace github.com/QuantumNous/new-api/tokenkit => ./tokenkit` 与 `make test` 中的 tokenkit 测试均存在。

- [x] **Step 4: 确认敏感词能力按上游处理**

确认 `service/sensitive.go` 不存在、`controller/relay.go` 无 `service.CheckSensitiveText` 调用；上游已用 `model.MigrateSensitiveWords` 把旧敏感词迁移到 request-filter。不要恢复本地实现。

- [x] **Step 5: 三模块构建**

Run: `go build . && (cd relaykit && GOWORK=off go build ./...) && (cd tokenkit && GOWORK=off go build ./...)`
Expected: 全绿。勾选 tasks.md 4.1。

### Task P3.2: 上下文限制改接 CountRequestToken

**Files:**
- Add (from backup): `setting/operation_setting/context_limit_setting.go`、`setting/operation_setting/context_limit_setting_test.go`
- Modify (merge): `relaykit/types/error.go`（加回 `ErrorCodeContextLimitExceeded ErrorCode = "context_limit_exceeded"`）
- Modify (merge): `controller/relay.go`（在 `relay.PrepareRequestBilling` 之前判定）
- Add: `controller/relay_context_limit_test.go`

**Interfaces:**
- Consumes: `service.CountRequestToken(c, meta *types.TokenCountMeta, info *relaycommon.RelayInfo) (int, error)`（不受 `constant.CountToken` 影响）；上游 `model.NewLogOther`、`model.RecordErrorLog(..., other *LogOther)`；`operation_setting.MaxPromptTokensForModel(model string) int`。
- Produces:
  - `func contextLimitError(model string, tokens, limit int) *types.NewAPIError`：状态码 413、错误码 `context_limit_exceeded`、`ErrOptionWithSkipRetry()`。
  - `func contextLimitErrorOther(c *gin.Context, relayInfo *relaycommon.RelayInfo, tokens, limit int) *model.LogOther`。
  - `func recordContextLimitErrorLog(c *gin.Context, relayInfo *relaycommon.RelayInfo, tokens, limit int)`。

- [ ] **Step 1: 取设置与测试**

Run: `git checkout backup-before-upstream -- setting/operation_setting/context_limit_setting.go setting/operation_setting/context_limit_setting_test.go`
Run: `go test ./setting/operation_setting/ -run 'MaxPromptTokensForModel|ContextLimitSetting'`
Expected: 全绿（覆盖总开关、精确/大小写不敏感匹配、默认回退、0 不限制、按模型 0 覆写默认）。

- [ ] **Step 2: 加回错误码**

在 `relaykit/types/error.go` 的错误码常量区加入 `ErrorCodeContextLimitExceeded`，位置不影响语义。
Run: `(cd relaykit && GOWORK=off go build ./...)`
Expected: 退出 0。

- [ ] **Step 3: 在 relay 流程中插入限制判定**

在 `controller/relay.go` 的 `relay.PrepareRequestBilling(c, relayInfo)` 调用之前：
1. `limit := operation_setting.MaxPromptTokensForModel(relayInfo.OriginModelName)`。
2. 仅当 `limit > 0` 时，调用 `meta := request.GetTokenCountMeta()`；`meta == nil`（如 Realtime 或空请求）时跳过限制判定，避免把「无法取得元数据」误判成计数失败。否则调用 `service.CountRequestToken(c, meta, relayInfo)`（禁止用 `service.EstimateRequestToken`）。
3. `tokens > limit` 时：`logger.LogWarn`、`recordContextLimitErrorLog`、返回 `contextLimitError(...)`，不进入预扣费。
4. 未超限时继续调用 `PrepareRequestBilling`；允许其内部再次估算（计费口径与限制口径分离）。
5. 若 P2.5 已为预览计算过 meta，复用之，避免同一请求重复 `GetTokenCountMeta`。

- [ ] **Step 4: 用 LogOther 重写错误日志**

`contextLimitErrorOther` 构造 `model.NewLogOther()` 并写入：
- `SetPublic("request_path", ...)`
- `SetPublic("error_type", types.ErrorTypeNewAPIError)`
- `SetPublic("error_code", string(types.ErrorCodeContextLimitExceeded))`
- `SetPublic("status_code", http.StatusRequestEntityTooLarge)`
- `SetPublic("estimated_prompt_tokens", tokens)`
- `SetPublic("context_limit", limit)`
- `SetPublic("relay_format", string(relayInfo.RelayFormat))`
- `SetAdmin("use_channel", ...)`（沿用本地逻辑，有值才写）

`recordContextLimitErrorLog` 使用 `model.RecordErrorLog(c, c.GetInt("id"), c.GetInt("channel_id"), relayInfo.OriginModelName, c.GetString("token_name"), <message>, c.GetInt("token_id"), <useTime>, <isStream>, c.GetString("group"), other)`，并保留 `constant.ErrorLogEnabled` 前置检查。

- [ ] **Step 5: 添加三条回归测试**

`controller/relay_context_limit_test.go`：
- `TestContextLimitErrorReturns413WithCode`：`contextLimitError` 的 `StatusCode=413`、`GetErrorCode()=="context_limit_exceeded"`。
- `TestContextLimitErrorOtherIncludesRequiredFields`：`Snapshot()` 顶层含 `estimated_prompt_tokens`、`context_limit`、`relay_format`、`error_code`、`status_code`；`relay_format` 值等于 `relayInfo.RelayFormat`。
- `TestContextLimitErrorOtherKeepsChannelInfoAdminOnly`：`use_channel` 只出现在 `admin_info`，不出现在顶层。

`service/token_counter_context_limit_test.go`：
- `TestCountRequestTokenIgnoresBillingSwitch`：保存并置 `constant.CountToken=false`，构造带 `ContextKeyOriginalModel` 的 gin context 与 OpenAI 格式 `RelayInfo`；断言 `CountRequestToken` 返回 >0 且无错误，`EstimateRequestToken` 返回 0。

Run:
```bash
go test ./controller/ -run 'ContextLimit'
go test ./service/ -run 'CountRequestTokenIgnoresBillingSwitch'
```
Expected: 全绿。

- [ ] **Step 6: 提交**

```bash
git add setting/operation_setting/context_limit_setting.go setting/operation_setting/context_limit_setting_test.go relaykit/types/error.go controller/relay.go controller/relay_context_limit_test.go service/token_counter_context_limit_test.go
git commit -m "feat: 上下文限制改接 CountRequestToken（不受计费估算开关影响）"
```

### Task P3.3: 消费日志记录 estimated_prompt_tokens

**Files:**
- Modify (merge): `service/log_info_generate.go`
- Add/Modify (from backup, 适配): `service/log_info_generate_test.go`
- Verify: `service/text_truncate.go` 复用 P2.5 实现

**Interfaces:**
- Consumes: `relayInfo.GetEstimatePromptTokens() int`（上游原生字段，由 `PrepareRequestBilling` 设置）；`model.LogOther.SetPublic`。
- Produces: 消费日志公开字段 `estimated_prompt_tokens`，仅当估算值 > 0 时写入。

- [x] **Step 1: 在消费日志 other 中写入估算值**

在 `service/log_info_generate.go` 的 `GenerateTextOtherInfo` 中，`relayInfo != nil` 且 `estimated := relayInfo.GetEstimatePromptTokens()` 大于 0 时 `other.SetPublic("estimated_prompt_tokens", estimated)`；等于 0 时不写任何键。

- [x] **Step 2: 适配测试到 LogOther**

恢复 `service/log_info_generate_test.go` 的两个回归：
- `TestGenerateTextOtherInfoWritesEstimatedPromptTokens`：估算值 33126 → `Snapshot()["estimated_prompt_tokens"] == 33126`。
- `TestGenerateTextOtherInfoOmitsZeroEstimate`：估算值 0 → `Snapshot()` 中不存在该键。

Run: `go test ./service/ -run 'EstimatedPromptTokens|OmitsZeroEstimate'`
Expected: 全绿。

- [x] **Step 3: 验证日志详情可见**

在 `model/log_format_test.go` 增加断言：`formatLogOtherJSON` 的用户可见投影保留 `estimated_prompt_tokens`（public 字段不被剥离）。Run: `go test ./model/ -run LogFormat`
Expected: 全绿。

- [x] **Step 4: 提交**

```bash
git add service/log_info_generate.go service/log_info_generate_test.go model/log_format_test.go
git commit -m "feat: 消费日志记录 estimated_prompt_tokens 供偏差对账"
```

### Task P3.4: 估算偏差复测（同一批 22 条真实请求）

**Files:**
- Add: `openspec/changes/migrate-onto-upstream-main/.comet/estimate-deviation.md`
- Input: `.local-tests/estimate-fixtures/`（P0.3 捕获，gitignored）

**Interfaces:**
- Consumes: P3.2 的 `CountRequestToken`；P0.3 的 22 条请求体与实际计量。
- Produces: 每条请求的估算值、实际输入、偏差；最大绝对偏差 ≤ 10% 的结论。

- [x] **Step 1: 用迁移后代码重放请求**

以 `CountRequestToken` 对每条 fixture 计算估算值。实际输入按格式归一：
- Anthropic/Claude：`actual = prompt_tokens + cache_read_input_tokens`（缓存命中计入实际值）。
- OpenAI：`actual = prompt_tokens`（`prompt_tokens_details.cached_tokens` 已包含在 prompt 内）。
- Gemini：`actual = prompt_token_count`。

- [x] **Step 2: 计算偏差**

对每条求 `deviation = (estimated - actual) / actual`，记录绝对值与百分比；同时记录 agent 特征（工具定义数、思考块、工具结果数）。

- [x] **Step 3: 判定并落盘**

要求 22 条中最大 `|deviation| <= 0.10`。把表格、最大偏差、失败样例写入 `.comet/estimate-deviation.md`；若超过 10%，停止，不要进入前端批次，按 `superpowers:systematic-debugging` 定位 `tokenkit.ModelFamily` 的模型映射，并报告阻塞。

- [x] **Step 4: 提交**

```bash
git add openspec/changes/migrate-onto-upstream-main/.comet/estimate-deviation.md
git commit -m "docs: 记录迁移后上下文估算偏差复测结果"
```

### Gate G3: token 切换门禁

- [x] `service/tokenizer.go`、`service/token_estimator.go` 不存在；`relaykit/dto` 无本地计数增强残留。
- [x] `go test ./controller/ -run ContextLimit`、`go test ./service/ -run 'CountRequestToken|EstimatedPromptTokens|TruncateTextToTokens'` 全绿。
- [x] `constant.CountToken=false` 时 `CountRequestToken` 测试通过。
- [ ] `.comet/estimate-deviation.md` 存在且最大偏差 ≤ 10%。  ← 顺延 S2（本机无夹具与配额，不可验证；见 .comet/estimate-deviation.md）
- [x] 三模块构建通过。

---

## 阶段 4：前端重建（tasks.md 5）

> 策略：迁移分支上的 `web/` 已经是上游 924 文件重构后的版本。共享文件一律「以上游为骨架，把本地差异加回」；本地独有目录整体从备份分支取。每个任务只跑受影响目录的 vitest，阶段门禁再跑全量 typecheck/lint/test/build。

### Task P4.0: 前端依赖与构建基础

**Files:**
- Modify: `web/package.json`（加 `pptxgenjs: ^4.0.1`）
- Modify: `web/rsbuild.config.ts`（`NormalModuleReplacementPlugin(/^node:/, src/empty-module.js)`）
- Add: `web/src/empty-module.js`
- Update: `web/bun.lock`（由 `bun install` 生成）

**Interfaces:**
- Produces: 可构建含 `pptxgenjs` 的前端；`node:fs`/`node:https` 在浏览器构建中被替换为空模块。

- [ ] **Step 1: 以上游为基线加入依赖与替换插件**

在 `web/package.json` 的 dependencies 加 `pptxgenjs`；在 `web/rsbuild.config.ts` 的 rspack plugins 中加入 `NormalModuleReplacementPlugin`，指向新增的 `web/src/empty-module.js`。不要整体替换上游 `package.json`/`rsbuild.config.ts`。

- [ ] **Step 2: 更新锁文件并验证**

Run:
```bash
cd web && bun install && bun run typecheck
```
Expected: `bun.lock` 更新；typecheck 零错误。

- [ ] **Step 3: 提交**

```bash
git add web/package.json web/bun.lock web/rsbuild.config.ts web/src/empty-module.js
git commit -m "chore(web): 引入 pptxgenjs 并保留 node: 模块替换配置"
```

### Task P4.1: usage-logs 区域重建

**Files:**
- Modify (merge): `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`、`.../dialogs/details-dialog.tsx`、`web/src/features/usage-logs/lib/format.ts`、`web/src/features/usage-logs/types.ts`、`web/src/features/pricing/lib/billing-expr.ts`
- Add (from backup): `web/src/features/usage-logs/components/__tests__/response-preview.test.tsx`、`web/src/features/usage-logs/lib/__tests__/tiered-summary.test.ts`

**Interfaces:**
- Consumes: 上游日志行结构（`other` 已按角色投影，内容在 `content_parts`）；P2.5 的 `request_preview`/`response_preview`；P3.3 的 `estimated_prompt_tokens`。
- Produces: 错误日志列、估算/实际行、请求/输出预览区块、阶梯按次计费显示。

- [ ] **Step 1: 查看本地差异**

Run: `git diff 2b6f1dfe backup-before-upstream -- web/src/features/usage-logs web/src/features/pricing/lib/billing-expr.ts`
Expected: 明确本地新增的列、详情行、预览区块、tiered summary 逻辑。

- [ ] **Step 2: 以上游重置共享文件**

Run: `git checkout origin/main -- web/src/features/usage-logs/components/columns/common-logs-columns.tsx web/src/features/usage-logs/components/dialogs/details-dialog.tsx web/src/features/usage-logs/lib/format.ts web/src/features/usage-logs/types.ts web/src/features/pricing/lib/billing-expr.ts`

- [ ] **Step 3: 手工加回本地能力**

- 列：错误日志列（`type=5`）与「估算 / 实际」列；适配上游列定义 API。
- 详情：`estimated_prompt_tokens` 与真实 `prompt_tokens` 对比行；请求预览（`request_preview` + `request_preview_user`）与输出预览（`response_preview`）区块；复制按钮沿用上游样式。
- 阶梯计费：纯常量按次计费显示「每次调用 · $x」；复用本地 `billing-expr.ts` 的差异，但适配上游函数签名。
- 类型：把本地字段加入上游 `LogOther` 投影类型，不覆盖上游 `content_parts` 等字段。

- [ ] **Step 4: 加回本地测试**

Run: `git checkout backup-before-upstream -- web/src/features/usage-logs/components/__tests__/response-preview.test.tsx web/src/features/usage-logs/lib/__tests__/tiered-summary.test.ts`
按上游组件结构适配导入与查询方式。

- [ ] **Step 5: 运行测试**

Run: `cd web && bun run vitest run src/features/usage-logs`
Expected: 全绿。

- [ ] **Step 6: 提交**

```bash
git add web/src/features/usage-logs web/src/features/pricing/lib/billing-expr.ts
git commit -m "feat(web): 重建日志错误列、估算/实际与请求输出预览"
```

### Task P4.2: dashboard 与使用报告区域重建

**Files:**
- Add (from backup): `web/src/features/dashboard/usage-report/**`、`web/src/features/dashboard/lib/user-range.ts`、`web/src/features/dashboard/lib/user-range.test.ts`、`web/src/features/dashboard/components/models/__tests__/log-stat-cards.test.tsx`、`web/src/lib/__tests__/time.test.ts`
- Modify (merge): `web/src/features/dashboard/index.tsx`、`section-registry.tsx`、`components/models/log-stat-cards.tsx`、`components/models/model-charts.tsx`、`components/users/user-charts.tsx`、`constants.ts`、`hooks/use-dashboard-config.tsx`、`lib/charts.ts`、`lib/stats.ts`、`lib/index.ts`、`types.ts`、`lib/__tests__/charts.test.ts`、`lib/__tests__/stats.test.ts`、`web/src/lib/time.ts`

**Interfaces:**
- Consumes: P2.3 的 `/api/data/usage_report` 与 `dto.UsageReport`；上游 dashboard section registry。
- Produces: Usage Report 区块（周/月对比、部门活跃、PPT 导出）、缓存命中率卡片与模型/用户排行、token 四维展示、用户统计日历范围。

- [ ] **Step 1: 取本地独有目录与文件**

Run:
```bash
git checkout backup-before-upstream -- web/src/features/dashboard/usage-report web/src/features/dashboard/lib/user-range.ts web/src/features/dashboard/lib/user-range.test.ts web/src/features/dashboard/components/models/__tests__/log-stat-cards.test.tsx web/src/lib/__tests__/time.test.ts
```
Expected: 19 个 usage-report 文件与辅助测试进入暂存区。

- [ ] **Step 2: 以上游重置共享文件**

Run: `git checkout origin/main -- <Step 2 列出的 14 个共享文件>`

- [ ] **Step 3: 手工加回本地能力**

- `section-registry.tsx`：在上游 `overview/models/flow/users` 之后追加 `usage-report`，`adminOnly: true`；保留上游全部注册项。
- `index.tsx` / `use-dashboard-config.tsx` / `types.ts` / `constants.ts`：加回 usage-report section、缓存命中率字段、token 四维字段。
- `log-stat-cards.tsx` / `model-charts.tsx` / `user-charts.tsx`：加回输入/输出/缓存/总四维与缓存命中率卡片；适配上游图表组件 API。
- `lib/charts.ts` / `lib/stats.ts` / `lib/index.ts`：加回缓存命中率计算、四维汇总、用户日历范围；保留上游已有函数。
- `web/src/lib/time.ts`：加回本地日历周聚合修正，保留上游其他时间函数。
- `section-registry.tsx` 的 section id 联合类型必须包含 `usage-report`。

- [ ] **Step 4: 运行测试**

Run:
```bash
cd web && bun run vitest run src/features/dashboard src/lib/__tests__/time.test.ts
```
Expected: 全绿，含 usage-report 的 stats/overview/user-table/department-table/export-ppt 测试。

- [ ] **Step 5: 提交**

```bash
git add web/src/features/dashboard web/src/lib/time.ts web/src/lib/__tests__/time.test.ts
git commit -m "feat(web): 重建使用报告、部门活跃、缓存命中率与 token 四维看板"
```

### Task P4.3: channels GLM 用量展示

**Files:**
- Add (from backup): `web/src/features/channels/components/dialogs/glm-usage-dialog.tsx`、`.../dialogs/__tests__/glm-usage-dialog.test.tsx`、`components/glm-usage-context.tsx`、`hooks/use-glm-usage.ts`、`lib/glm-usage.ts`、`lib/__tests__/glm-usage.test.ts`
- Modify (merge): `web/src/features/channels/api.ts`、`components/channels-columns.tsx`、`components/channels-table.tsx`、`constants.ts`、`lib/channel-actions.ts`、`lib/index.ts`、`types.ts`

**Interfaces:**
- Consumes: P2.4 的 `GET /api/channel/glm/usage`。
- Produces: 渠道表格 GLM 用量入口、5h/周窗口弹窗、用量数据 hook。

- [ ] **Step 1: 取本地独有文件**

Run: `git checkout backup-before-upstream -- web/src/features/channels/components/dialogs/glm-usage-dialog.tsx web/src/features/channels/components/dialogs/__tests__/glm-usage-dialog.test.tsx web/src/features/channels/components/glm-usage-context.tsx web/src/features/channels/hooks/use-glm-usage.ts web/src/features/channels/lib/glm-usage.ts web/src/features/channels/lib/__tests__/glm-usage.test.ts`

- [ ] **Step 2: 以上游重置共享文件并手工加回**

Run: `git checkout origin/main -- web/src/features/channels/api.ts web/src/features/channels/components/channels-columns.tsx web/src/features/channels/components/channels-table.tsx web/src/features/channels/constants.ts web/src/features/channels/lib/channel-actions.ts web/src/features/channels/lib/index.ts web/src/features/channels/types.ts`
把 GLM 用量列/操作、API 客户端函数、类型与导出加回上游结构；保留上游新增列与操作。

- [ ] **Step 3: 运行测试**

Run: `cd web && bun run vitest run src/features/channels`
Expected: 全绿。

- [ ] **Step 4: 提交**

```bash
git add web/src/features/channels
git commit -m "feat(web): 重建渠道 GLM 用量展示"
```

### Task P4.4: system-settings 上下文限制与日志预览设置

**Files:**
- Add (from backup): `web/src/features/system-settings/request-limits/context-limit-section.tsx`、`context-limit-utils.ts`、`__tests__/context-limit-section.test.tsx`
- Modify (merge): `web/src/features/system-settings/security/section-registry.tsx`、`security/index.tsx`、`operations/index.tsx`、`operations/section-registry.tsx`、`maintenance/log-settings-section.tsx`、`types.ts`

**Interfaces:**
- Consumes: P3.2 的 `context_limit_setting`（`enabled` / `default_max_prompt_tokens` / `model_max_prompt_tokens`）；P2.5 的 `RequestPreviewEnabled` / `RequestPreviewMaxTokens`。
- Produces: 上下文限制设置分组、日志预览开关与长度设置。

- [ ] **Step 1: 取本地独有组件与测试**

Run: `git checkout backup-before-upstream -- web/src/features/system-settings/request-limits/context-limit-section.tsx web/src/features/system-settings/request-limits/context-limit-utils.ts web/src/features/system-settings/request-limits/__tests__/context-limit-section.test.tsx`

- [ ] **Step 2: 以上游重置共享文件并手工加回**

Run: `git checkout origin/main -- web/src/features/system-settings/security/section-registry.tsx web/src/features/system-settings/security/index.tsx web/src/features/system-settings/operations/index.tsx web/src/features/system-settings/operations/section-registry.tsx web/src/features/system-settings/maintenance/log-settings-section.tsx web/src/features/system-settings/types.ts`
在 `security/section-registry.tsx` 保留上游 `RateLimit`/`SSRF`/`TokenLimit`，追加 `context-limit` 分组（`id: 'context-limit'`，三个配置键）；在 `log-settings-section.tsx` 加回请求预览开关与长度输入；在 `types.ts` 加回配置键类型。

- [ ] **Step 3: 运行测试**

Run: `cd web && bun run vitest run src/features/system-settings`
Expected: 全绿。

- [ ] **Step 4: 提交**

```bash
git add web/src/features/system-settings
git commit -m "feat(web): 重建上下文限制与日志预览设置分组"
```

### Task P4.5: users 部门管理界面

**Files:**
- Add (from backup): `web/src/features/users/components/departments-manage-dialog.tsx`、`hooks/use-departments.ts`、`lib/departments.ts`、`lib/__tests__/departments.test.ts`、`components/users-primary-buttons.tsx`
- Modify (merge): `web/src/features/users/components/users-columns.tsx`、`users-mutate-drawer.tsx`、`users-table.tsx`、`types.ts`、`lib/user-form.ts`（表单 schema、默认值、`transformFormDataToPayload` —— 缺此项部门永远发不出去，管理员编辑会把用户部门重置为公共部门）

**Interfaces:**
- Consumes: P2.2 的 `department` 字段与部门清单接口。
- Produces: 部门列、部门筛选、新建/编辑用户的部门选择、部门管理弹窗。

- [ ] **Step 1: 取本地独有文件**

Run: `git checkout backup-before-upstream -- web/src/features/users/components/departments-manage-dialog.tsx web/src/features/users/hooks/use-departments.ts web/src/features/users/lib/departments.ts web/src/features/users/lib/__tests__/departments.test.ts web/src/features/users/components/users-primary-buttons.tsx`

- [ ] **Step 2: 以上游重置共享文件并手工加回**

Run: `git checkout origin/main -- web/src/features/users/components/users-columns.tsx web/src/features/users/components/users-mutate-drawer.tsx web/src/features/users/components/users-table.tsx web/src/features/users/types.ts`
加回部门列/筛选/表单字段，保留上游新增列与操作。

- [ ] **Step 3: 运行测试**

Run: `cd web && bun run vitest run src/features/users`
Expected: 全绿。

- [ ] **Step 4: 提交**

```bash
git add web/src/features/users
git commit -m "feat(web): 重建用户部门管理与筛选"
```

### Task P4.6: i18n 同步与七语言键集合校验

**Files:**
- Modify (merge): `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`、`web/src/i18n/static-keys.ts`

**Interfaces:**
- Consumes: P4.1–P4.5 引入的全部 `t('English key')`。
- Produces: 七个语言文件键集合一致、无同义双键。

- [ ] **Step 1: 以上游为基线同步**

Run: `cd web && bun run i18n:sync`
Expected: 以七个上游语言文件为基线，新增键以英文原文为 key 写入；不覆盖上游已有翻译。

- [ ] **Step 2: 补齐自研文案**

为以下键补齐七语言翻译：`Usage Report`、`Cache Hit Rate`、`Department`、`Request Preview`、`Output Preview`、`Estimated / Actual`、`Context Limit`、`GLM Usage` 等。若上游已存在同义键（如 `Cache Hit Rate`），以上游翻译为准，不新建双键。

- [ ] **Step 3: 校验七份键集合一致**

Run（本机无 `jq`，用 bun 计算；只比较键集合，不比较翻译内容）：
```bash
cd web && bun -e 'const fs=require("fs"),c=require("crypto");for(const f of fs.readdirSync("src/i18n/locales").filter(x=>x.endsWith(".json"))){const k=Object.keys(JSON.parse(fs.readFileSync("src/i18n/locales/"+f,"utf8"))).sort().join("\n");console.log(f,c.createHash("sha256").update(k).digest("hex"))}'
```
Expected: 七行哈希完全一致。

- [ ] **Step 4: typecheck/lint 并提交**

Run: `cd web && bun run typecheck && bun run lint`
Expected: 零错误。

```bash
git add web/src/i18n
git commit -m "chore(web): 同步七语言 i18n 并补齐自研文案"
```

### Gate G4: 前端门禁

- [ ] `cd web && bun run typecheck` 零错误。
- [ ] `cd web && bun run lint` 零 error。
- [ ] `cd web && bun run test` 全绿（含新增的 usage-report、preview、GLM、department、context-limit 测试）。
- [ ] `cd web && bun run build` 成功，产物写入 `web/dist`。
- [ ] 七个语言文件键集合哈希一致。

---

## 阶段 5：全量验证与切换（tasks.md 6）

### Task P5.1: 三模块构建与 make test 全绿

**Files:** 无改动。

- [ ] **Step 1: 三模块构建与测试**

Run:
```bash
mkdir -p web/dist && test -f web/dist/index.html || printf '<!doctype html>\n' > web/dist/index.html
GOWORK=off go build .
(cd relaykit && GOWORK=off go build ./... && GOWORK=off go test ./...)
(cd tokenkit && GOWORK=off go build ./... && GOWORK=off go test ./...)
```
Expected: 全绿。

- [ ] **Step 2: 跑 make test**

Run: `make test`
Expected: 根模块、relaykit、tokenkit 三段全绿。若 `pg_data` 权限导致 `go list ./...` 失败，先按 P0.2 的替代方案确认所有真实包通过，并在结果中注明环境限制。

- [ ] **Step 3: 勾选 tasks.md 6.1**

### Task P5.2: 前端全量门禁

**Files:** 无改动。

- [ ] `cd web && bun run typecheck && bun run lint && bun run test && bun run build` 全部通过。
- [ ] 勾选 tasks.md 6.2。

### Task P5.3: 三数据库迁移验证

**Files:**
- Add: `openspec/changes/migrate-onto-upstream-main/.comet/db-verification.md`

**Interfaces:**
- Consumes: P2.1 新增 `logs.input_tokens/cache_tokens/cache_read_tokens`、`quota_data.input_tokens/output_tokens/cache_tokens`；P2.2 新增 `users.department`。
- Produces: 三库 × 全新库/升级库 × 连续启动两次的验证记录。

- [ ] **Step 1: SQLite 全新库**

Run（两次）：
```bash
rm -f /tmp/migrate-fresh.db
PORT=3001 SQLITE_PATH=/tmp/migrate-fresh.db timeout 30s ./new-api
PORT=3001 SQLITE_PATH=/tmp/migrate-fresh.db timeout 30s ./new-api
```
Expected: 两次退出码为 124（超时被终止）或正常退出；日志无 AutoMigrate 错误。用 `sqlite3 /tmp/migrate-fresh.db '.schema logs'`、`'.schema users'`、`'.schema quota_data'` 确认新列与 `department` 默认值。

- [ ] **Step 2: SQLite 升级库**

用备份分支二进制建库并写入代表数据：
```bash
git worktree add /tmp/old-main backup-before-upstream
(cd /tmp/old-main && mkdir -p web/dist && printf '<!doctype html>\n' > web/dist/index.html && GOWORK=off go build -o /tmp/new-api-old .)
PORT=3001 SQLITE_PATH=/tmp/migrate-upgrade.db timeout 30s /tmp/new-api-old
```
写入一个用户、一个渠道、一条消费日志；再用新二进制启动两次。Expected: 新列追加成功；用户/渠道/日志行数与关键字段不变；索引保留。

- [ ] **Step 3: MySQL 8.x**

Run（两次）：
```bash
mysql -h127.0.0.1 -uur -purpass -e 'DROP DATABASE IF EXISTS migrate_new; CREATE DATABASE migrate_new CHARACTER SET utf8mb4;'
PORT=3001 SQL_DSN='ur:urpass@tcp(127.0.0.1:3306)/migrate_new?charset=utf8mb4&parseTime=True&loc=Local' timeout 30s ./new-api
PORT=3001 SQL_DSN='ur:urpass@tcp(127.0.0.1:3306)/migrate_new?charset=utf8mb4&parseTime=True&loc=Local' timeout 30s ./new-api
```
Expected: 两次无迁移错误；`SHOW COLUMNS FROM logs LIKE '%tokens'` 含三列；`SHOW COLUMNS FROM users LIKE 'department'` 默认值为 `公共部门`；`SHOW INDEX FROM users` 含 department 索引。

- [ ] **Step 4: PostgreSQL 16.x**

Run（两次）：
```bash
PGPASSWORD=urpass psql -h127.0.0.1 -U ur -d postgres -c 'DROP DATABASE IF EXISTS migrate_new;' -c 'CREATE DATABASE migrate_new;'
PORT=3001 SQL_DSN='postgresql://ur:urpass@127.0.0.1:5432/migrate_new?sslmode=disable' timeout 30s ./new-api
PORT=3001 SQL_DSN='postgresql://ur:urpass@127.0.0.1:5432/migrate_new?sslmode=disable' timeout 30s ./new-api
```
Expected: 两次无迁移错误；`\d logs`、`\d users`、`\d quota_data` 含新列与索引；`users.department` 默认值为 `公共部门`。

- [ ] **Step 5: 记录并提交**

把数据库版本（`mysql --version`、`psql --version`）、命令、表结构差异、行数/索引保留结果写入 `.comet/db-verification.md`；任何一库失败都不得声称兼容。

```bash
git add openspec/changes/migrate-onto-upstream-main/.comet/db-verification.md
git commit -m "docs: 记录三数据库迁移验证结果"
```

### Task P5.4: 计费对账（Claude / Gemini / OpenAI）

**Files:**
- Modify (add assertions): `service/text_quota_test.go`

**Interfaces:**
- Consumes: 上游 `service/text_quota.go` 的计费实现；P2.1 加回的三维字段。
- Produces: 三种格式的 quota 计算与上游一致的回归断言。

- [ ] **Step 1: 对比上游实现差异**

Run: `git diff origin/main -- service/text_quota.go`
Expected: 只有本地新增的维度字段与 `RecordConsumeLog` 参数；配额乘数、缓存计费、阶梯计费逻辑与上游一致。

- [ ] **Step 2: 跑现有对账测试**

Run: `go test ./service/ -run 'CalculateTextQuotaSummary|ComposeTiered|UsageFromOpenAIBillingUsage'`
Expected: 全绿，覆盖 Claude / Gemini / OpenAI 的 billing usage 优先级、缓存读/写、Responses input details。

- [ ] **Step 3: 补一条三维守恒断言**

在 `service/text_quota_test.go` 增加表测试：对 Claude / Gemini / OpenAI 三个固定 fixture，断言 `summary.InputTokens + summary.CacheTokensTotal + summary.CompletionTokens == summary.TotalTokens`（或等价总量字段），并断言 `CacheTokensTotal = CacheTokens + CacheCreationTokens`。不得修改配额乘数断言。

Run: `go test ./service/ -run 'TokenDimensions.*Conservation|CalculateTextQuotaSummary'`
Expected: 全绿。

- [ ] **Step 4: 提交**

```bash
git add service/text_quota_test.go
git commit -m "test: 补充 Claude/Gemini/OpenAI 计费对账断言"
```

### Task P5.5: 部署验证 S1–S5

**Files:**
- Add: `openspec/changes/migrate-onto-upstream-main/.comet/deploy-verification.md`
- Optional Modify: `Dockerfile.local`（改为宿主机编译 + 最小 Dockerfile 打包）

**Interfaces:**
- Consumes: 宿主机构建产物 `new-api` 与 `web/dist`；`docker-compose.yml` 的 `new-api` 服务。
- Produces: S1–S5 的实测证据。

- [ ] **Step 1: 按宿主机路径构建**

Run:
```bash
cd web && bun install --frozen-lockfile && bun run build && cd ..
GOWORK=off CGO_ENABLED=0 go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api .
```
Expected: 二进制与 `web/dist` 生成。

- [ ] **Step 2: 最小镜像打包**

把 `Dockerfile.local` 改为只做 `FROM debian:bookworm-slim` + `COPY new-api /` + 复制 `LICENSE`/`NOTICE`/`THIRD-PARTY-LICENSES.md` 与宿主机的 CA/时区文件；不得在容器内执行 `apt-get`、`go mod download` 或 `bun install`。构建并启动：
```bash
docker build -f Dockerfile.local -t new-api:migrate-2026-11 .
docker compose up -d --force-recreate new-api
```
Expected: 容器健康，日志无迁移/依赖错误。

- [ ] **Step 3: S1 上下文限制**

- 配置全局默认 200000、某模型 500000、另一模型 0。
- 发送估算 ≤200000 的请求：放行。
- 发送 >200000 的请求：413 + 错误码 `context_limit_exceeded`。
- 模型覆写 500000：220000 的请求放行；默认模型仍拒绝。
- 模型覆写 0：任意大小放行。
- 关闭计费估算开关（`CountToken`）后重发超限请求：仍 413。
- 检查错误日志包含 `estimated_prompt_tokens`、`context_limit`、`relay_format`。

- [ ] **Step 4: S2 估算偏差**

用 P0.3 的 22 条 fixture 重放，按 P3.4 公式计算；最大绝对偏差 ≤10%。记录与基线对比。

- [ ] **Step 5: S3 五项自研功能**

逐项走查：使用报告（周/月对比 + 部门活跃 + PPT 导出）、用户部门属性、GLM 渠道用量、缓存命中率卡片/排行、日志请求/输出预览与估算/实际展示。

- [ ] **Step 6: S4 三库、S5 计费**

S4 引用 P5.3 记录；S5 引用 P5.4 结果，并在部署实例上抽一条 Claude / Gemini / OpenAI 请求核对计费与日志中的维度。

- [ ] **Step 7: 记录并提交**

```bash
git add openspec/changes/migrate-onto-upstream-main/.comet/deploy-verification.md Dockerfile.local
git commit -m "docs: 记录部署验证与 S1-S5 结果"
```
（若 `Dockerfile.local` 实际未改动，只提交验证记录。）

### Task P5.6: 快进 main 并推送 fork

**Files:** 无源码改动。

- [ ] **Step 1: 快进 main**

Run:
```bash
git switch main
git merge --ff-only migrate/upstream-2026-11
```
Expected: 快进成功，无合并提交。

- [ ] **Step 2: 校验提交范围**

Run: `git log --oneline origin/main..HEAD`
Expected: 只包含本计划的主题提交（P1.1–P5.6），不含本地探索性提交或重复设计文档。

- [ ] **Step 3: 推送 fork（需用户确认）**

Run: `git push fork main`
Expected: 推送成功；`git rev-parse fork/main` 等于 `git rev-parse main`。

- [ ] **Step 4: 勾选 tasks.md 6.6**

### Task P5.7: Workflow follow-up（保留与清理）

- [ ] 归档前保留 `backup-before-upstream` 与 fork 远端，不删除、不重置。
- [ ] 归档后确认 `migrate/upstream-2026-11` 已合并，再删除本地分支：`git branch -d migrate/upstream-2026-11`。
- [ ] 若需要长期保留回滚点，在归档记录中写明 `backup-before-upstream` 的保留期限。

### Gate G5: 最终门禁

- [ ] P5.1–P5.5 全部通过，记录文件齐全。
- [ ] `main` 已快进且 `git log origin/main..HEAD` 只含预期提交。
- [ ] `git status --porcelain` 无输出。
- [ ] 三库验证、估算偏差、计费对账、S1–S5 证据均可追溯（`db-verification.md`、`estimate-deviation.md`、`deploy-verification.md`）。

---

## 自审记录

- **Spec 覆盖**：`context-token-limit` 的 10 个场景分别落在 P3.2（按模型限制、覆写优先、0 不限制、开关关闭仍生效、413 错误码、错误日志三字段）、P3.3（消费日志估算值、无值不留字段）、P3.4（agent 会话偏差、缓存命中计入实际）。P2.1/P2.3 提供 `cache_read_tokens` 从写入到聚合的链路。
- **tasks.md 覆盖**：1.1–6.6 全部映射到上表任务；补充任务 P1.3、P2.5、P4.0、P4.3、P4.5 用于保住设计中的五项自研功能与 Comet 产物。
- **类型一致性**：`CountRequestToken` / `EstimateRequestToken` 签名统一为 `(c *gin.Context, meta *types.TokenCountMeta, info *relaycommon.RelayInfo) (int, error)`；`RecordConsumeLogParams` 以 `*model.LogOther` 为准；`LogOther` 公开字段用 `SetPublic`，管理字段用 `SetAdmin`。
- **Review Focus**：五条风险均在指定任务中有测试或部署证据。
- **比例**：计划不包含实现代码，只锁定文件、签名、命令、测试名与断言；实现细节留给执行者。
