<!-- 按 .agents/github/PR.md 模板填写；向上游提 PR 前需先按 .agents/github/ISSUE.md 创建并链接 issue -->

## Agent

- Tool: Claude Code (CLI)
- Tool version: 6.3.0 (superpowers 6.3.0)
- Model (full id): glm-5.3
- Host: CLI
- Date (UTC): 2026-08-31

## Links

- Closes # （尚无 issue；按模板要求，正式提交前需先用 `.agents/github/ISSUE.md` 创建功能 issue 并链接）
- Related: 设计文档 `docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md`（随本 PR 一并入库）

## User request

> "如果我要从后台数据库采集用户统计数据，统计本周和上周的使用频率，一周使用多少天，费用对比，统计本月和上月的使用频率，费用对比，输出ppt，要如何做"

（后续澄清确认：做成 new-api 产品功能；全体用户对比视角；请求次数为主要频率口径；PPT 需原生可编辑。）

## Out of scope — refuse

- Matched: no
- If yes: 不适用

## Kind

- [x] New feature

## Issue facts

- Actual behavior: not applicable（新功能而非缺陷）
- Impact: 管理员无法在后台直接生成周/月使用对比报告并导出 PPT
- Frequency: not applicable
- Evidence: not applicable
- Applicable types: frontend（管理后台新分节）；后端新增只读统计 API，不涉及 relay / billing 写路径

## Change

新增管理员「使用报告」看板分节与配套聚合 API：

- `GET /api/data/usage_report?period=week|month&anchor=YYYY-MM-DD&tz_offset=<min>`（AdminAuth）。`model.GetUsageReport` 将周期拆为逐天时间窗（报告时区自然日，换算为 UTC 秒边界），每天一条参数化聚合查询（`type=2 AND created_at >= ? AND < ?` GROUP BY user_id），内存组装总览/逐天/用户三级数据，含"活跃天数"（有消费记录的自然天数）与上期对比；仅上期活跃的用户保留并以本期 0 呈现（流失可见）。
- 前端 `web/src/features/dashboard/usage-report/`：周期切换（周/月 + 上/下一期）、总览卡（请求数/费用/tokens/活跃用户 + 环比）、逐天趋势图（vchart 柱+线）、用户明细表、导出按钮（pptxgenjs 浏览器端生成 5 页原生可编辑 PPT：封面/总览/趋势/Top15 用户表+其余汇总/费用 Top10 饼图）。
- 分节注册进 `section-registry`（admin-only），i18n 键同步 7 种语言，zh 已翻译。

为什么可行/安全：聚合只用参数化 `>= ?`/`< ?`、`GROUP BY user_id, username` 与 `COALESCE`，无任何日期方言函数，三库行为一致；查询走 `idx_created_at_type` 索引，单次报告最多 62 条轻量查询；纯只读，不触碰计费/配额写路径。

## Research

### Duplicate / prior art

- Search queries (issues, PRs): "usage report", "PPT export", "report export", "周报", "export dashboard"
- What already existed and why this is not a duplicate: 现有 `/api/data/*`（GetAllQuotaDates 等）基于 `quota_data` 周期聚合表服务数据看板，无周/月对比、无活跃天数语义、无任何 PPT/报表导出能力；`usage-logs` 仅明细分页。未发现重复实现。

### Docs and code

- https://docs.newapi.ai/ ：数据看板文档仅描述既有 quota_data 看板，无用户周/月报告导出功能。
- https://deepwiki.com/QuantumNous/new-api ：结构确认 logs/quota_data 双数据源；活跃天数需逐天精确统计，只能来自 logs。
- README / repo docs：无报表导出相关内容。
- Code paths and what they imply for this change: `model/log.go` 的 `SumUsedQuota` 展示了 LOG_DB 聚合惯例；`model/usedata.go` 的 quota_data 语义不满足活跃天数精度 → 选择直查 logs；`router/api-router.go` dataRoute 组为管理端数据 API 的既有挂载点。

### Alternatives considered

- Option A: 复用 quota_data 聚合表（开发量最小，但语义耦合、活跃天数靠时间片反推不精确）
- Option B: 前端拉明细日志自行聚合（数据量大时不可行）
- Why this approach: 直查 logs 逐天窗口（方案一）：语义精确、跨方言零风险、窗口小成本可控。

## Files

| Path | Why |
| --- | --- |
| `dto/usage_report.go` | 报告响应结构 |
| `model/usage_report_window.go` (+test) | 周/月窗口与上期推导纯函数（周一起始、tz 偏移、跨月跨年） |
| `model/usage_report.go` (+test) | 逐天聚合查询与报告组装 |
| `controller/usage_report.go` (+test) | 参数校验与响应 |
| `router/api-router.go` | dataRoute 注册一行 |
| `web/src/features/dashboard/usage-report/**` | 分节全部组件/hook/lib/测试 |
| `web/src/features/dashboard/index.tsx`、`section-registry.tsx` | 分节注册与 admin 可见性（改为集合驱动） |
| `web/src/i18n/locales/*` | 新增 20 键 ×7 语言（zh 已译） |
| `web/package.json` | 新增 pptxgenjs |

## Behavior

- Before: 管理后台无使用报告；无法导出周/月对比 PPT。
- After: 管理员在 数据看板 → 使用报告 可查看本周/月 vs 上期（请求数、活跃天数、费用、tokens、活跃用户）并一键导出原生可编辑 PPT；支持周/月切换与任意历史周期回看；非管理员不可见。
- Explicit non-goals / leftover work: 定时生成/邮件推送、单用户报告、Excel/PDF 导出、按 group/model 下钻、服务端缓存；zh-TW/fr/ru/ja/vi 暂以英文回退。

## Verification

- Commands and results:
  - `make test`（根模块 + relaykit）：45 packages ok，0 FAIL
  - `GOWORK=off go test ./model/ -run 'TestResolvePeriodWindow|TestAssembleUsageReport'`：11 PASS
  - `GOWORK=off go test ./controller/ -run TestGetUsageReport`：5 子用例 PASS
  - 前端 `bun run typecheck` PASS；`bun run vitest run` 63 文件 / 423 测试全过（含新增 4 文件 17 测试）；oxlint 对本分支涉及文件 0 error
- Manual steps and observed result: 启动真实服务（SQLite，GIN_MODE=release）：完成 setup → sqlite3 造 5 条跨周消费日志 → `curl /api/data/usage_report`（Bearer access_token）核对 overview/daily/users 全部数字与手算一致（含仅上期用户排序列、月报 31 天、`period=day`/`anchor=bad` 均按预期拒绝；未认证 401）。
- UI: screenshot or recording (or why none): CLI 环境无法截图；建议合并前 `make dev-api && make dev-web` 人工过一遍页面与 PPT 打开效果。
- Tests added or updated, or why none: 新增后端 16 个单测 + 前端 17 个单测（时间窗边界、组装不变量、参数拒绝、环比除零、Top15 截断、空状态渲染）。
- Databases / providers / platforms exercised: 真机验证矩阵——SQLite（glebarez/sqlite v1.9.0）/ MySQL 8.0.46（apt，ur_verify 库）/ PostgreSQL 16.15（apt，ur_verify 库）各跑同一集成断言全 PASS；SQL 未用版本特定特性，未另测 5.7/9.6 最低版本。
- Not verified: 浏览器内 PPT 实际下载后用 Office/WPS 打开的视觉效果（slide 内容有单测，pptx API 调用经 typecheck 编译校验）；ClickHouse 方言未实测（部署少见，SQL 为标准语法）。

## Risks

- Failure modes: 超大 logs 表时 62 条聚合查询变慢（有索引 + 时间窗，风险低；后续可加缓存）；tz 极端值已限幅 ±840。
- Billing / quota / auth impact: 纯只读统计，无计费/配额写路径影响；端点 AdminAuth 保护，非管理员不可见、不可调。
- Follow-ups: 其余语言翻译补全；PPT 版式精修；如需下钻（group/model/channel）可在同 API 上扩展参数。

## Scope check

- Single focused change: yes（一个只读统计 API + 对应前端分节）
- Secrets included: no
- Out of scope (Coding Plan / reverse-engineered channel / third-party wrapper / Codex): no

---

**AI 声明**：当前 git 用户（Admin）非本仓库历史核心开发者。本 PR 的代码与文档由 AI（Claude Code / glm-5.3）生成并在 AI 辅助下完成验证。
