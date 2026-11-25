# 使用报告（周/月对比 + PPT 导出）设计文档

- 日期：2026-08-31
- 状态：待审阅
- 路径：管理后台数据看板新增「使用报告」分节

## 1. 背景与目标

管理员需要以周报/月报形式向上汇报或做运营复盘：全体用户的**使用频率**（请求次数）、**一周/一月内的活跃天数**、**费用**，以及与上一期的环比对比，并导出为**原生可编辑的 PPT**。

本设计将其做成 new-api 产品功能：后端新增只读统计聚合 API，前端在管理后台数据看板新增分节，浏览器端生成 .pptx。

## 2. 已确认的需求决策

| 决策点 | 结论 |
| --- | --- |
| 形态 | new-api 产品功能（非独立脚本、非一次性分析） |
| 视角 | 全体用户对比报告（总览 + 每用户明细），管理员使用 |
| 使用频率口径 | 消费日志（`logs.type=2`）条数为主，tokens 为辅助指标 |
| 费用口径 | `quota` 总和，展示时按 `common.QuotaPerUnit`（默认 500000 = $1）换算为美元 |
| 活跃天数 | 周期内"该用户存在至少一条消费日志"的自然天数 |
| PPT 可编辑性 | 原生可编辑（图表为 PPT chart 对象、文字为文本框），前端 `pptxgenjs` 生成 |
| 数据源 | 直查 `logs` 表（原始真相，活跃天数需按天精确统计） |

## 3. 非目标（明确不做）

- 定时自动生成 / 邮件推送报告
- 单用户视角报告页
- 导出 Excel / CSV / PDF
- 按分组（group）、渠道、模型的下钻维度（后续可扩展，本期不做）
- 服务端缓存与物化表（查询窗口小、走索引，先不做；若大站点实测慢再评估）
- 修改任何计费/配额写路径（本功能纯只读统计）

## 4. 总体架构与数据流

```
管理后台 /dashboard/usage-report（features/dashboard 新分节）
   │  GET /api/data/usage_report?period=week|month&anchor=YYYY-MM-DD&tz_offset=480
   ▼
controller.GetUsageReport（参数校验）
   ▼
model.GetUsageReport（走 LOG_DB，独立日志库时自动正确）
   │  今期/上期各拆为 N 个"天窗口"（UTC 秒级边界）
   │  每天一条聚合：COUNT/SUM ... WHERE type=2 AND created_at >= ? AND < ?
   │              GROUP BY user_id, username        （走 idx_created_at_type）
   ▼
内存组装：用户×天矩阵 → 活跃天数、今期/上期字段、逐天序列、总览
   ▼
前端渲染对比视图 ──「导出 PPT」──> pptxgenjs 生成 .pptx 下载
```

## 5. 后端设计

### 5.1 API

- 路由：`router/api-router.go` 的 `dataRoute` 组新增
  `GET /api/data/usage_report` → `middleware.AdminAuth()` + `controller.GetUsageReport`
- 查询参数：

| 参数 | 说明 | 校验 |
| --- | --- | --- |
| `period` | `week` 或 `month`，必填 | 非法值 → 400 |
| `anchor` | `YYYY-MM-DD`，可选，默认今天 | 按 `tz_offset` 时区解释；解析失败 → 400 |
| `tz_offset` | 浏览器时区偏移，分钟，可选默认 0 | 超出 [-840, 840] → 400 |

- 响应：沿用项目统一的 `{success, message, data}` 包装（参考 `controller/usedata.go`），错误走 `common.ApiError`。

### 5.2 时间窗规则

- 天边界由 `tz_offset` 定义（容器常为 UTC，用管理员浏览器时区切天），后端换算为 UTC 秒级边界后查询。
- 周：anchor 所在**自然周周一 00:00**（tz）为起点，长度 7 天；上期 = 起点 − 7 天。
- 月：anchor 所在**自然月 1 日 00:00**（tz）为起点，至下月 1 日（正确处理 12 月→次年 1 月）；上期 = 上一个自然月。
- 周期标签：周用 ISO 周号 `2026-W36`，月用 `2026-08`。

### 5.3 DTO（新增 `dto/usage_report.go`）

```go
type UsageReportPeriodInfo struct {
    Label     string // "2026-W36" / "2026-08"
    StartDate string // 用户可读，如 "2026-08-31"
    EndDate   string
}

type UsageReportOverview struct {
    Requests, PrevRequests             int64
    Quota, PrevQuota                   int64
    Tokens, PrevTokens                 int64
    ActiveUsers, PrevActiveUsers       int64
}

type UsageReportDaily struct {
    Date        string // "2026-08-25"
    Requests    int64
    Quota       int64
    ActiveUsers int64
}

type UsageReportUser struct {
    UserId                 int
    Username               string // 来自 logs 冗余字段，不 join users 表
    Requests, PrevRequests int64
    ActiveDays             int // 本期活跃天数
    PrevActiveDays         int
    Quota, PrevQuota       int64
    Tokens, PrevTokens     int64
}

type UsageReport struct {
    Period   string // "week" | "month"
    Current  UsageReportPeriodInfo
    Previous UsageReportPeriodInfo
    Overview UsageReportOverview
    Daily    []UsageReportDaily // 本期逐天（周=7 项，月=当月天数项）
    Users    []UsageReportUser  // 按本期 Requests 降序；仅上期活跃的用户以本期 0 保留（反映流失），同请求数时按 Username 升序
}
```

环比百分比不在后端计算（后端只给今期/上期原始值），由前端展示层计算并做除零保护。

### 5.4 model 层（新增 `model/usage_report.go`）

- `GetUsageReport(period string, anchorDate time.Time, tzOffsetMin int) (*dto.UsageReport, error)` 主入口。
- 纯函数 `usageWindows(period, anchor, tzOffset)` → 今期/上期的 `[]dayWindow{StartUTC, EndUTC int64, DateLabel string}`，便于单测。
- `sumLogsByUserInRange(start, end int64) ([]userDayAggRow, error)`，每天调用一次，写法沿用 `SumUsedQuota`（`model/log.go:634`）的 `LOG_DB.Table("logs").Select(...)` 模式：

```sql
SELECT user_id, username,
       COUNT(*) AS requests,
       COALESCE(SUM(quota), 0) AS quota,
       COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS tokens
FROM logs
WHERE type = 2 AND created_at >= ? AND created_at < ?
GROUP BY user_id, username
```

- 组装：`map[userId]*userAccum` 累计今期（矩阵 → ActiveDays = 有记录的天数）与上期字段；Users 按本期 Requests 降序；Overview/Daily 由同一批行汇总。
- 查询次数上限：今期 31 + 上期 31 = 62 条，串行执行。
- 跨方言保证：仅参数化 `>= ?` / `< ?` 与 `GROUP BY user_id, username`，`COALESCE` 三库通用；不使用任何日期方言函数。SQLite / MySQL >= 5.7.8 / PostgreSQL >= 9.6 / ClickHouse（若部署）均适用。

## 6. 前端设计（`web/src/features/dashboard/usage-report/`）

- 注册进 `features/dashboard/section-registry`，路由 `/dashboard/usage-report`，复用现有管理员守卫与看板布局。
- 组件（`components/`）：
  - `UsageReportSection`（入口）
  - `PeriodSwitcher`：`week | month` 切换 + 上一期/下一期；周期状态为组件内 state（不接入 URL search params，避免影响共享路由 `$section.tsx`）
  - `OverviewCards`：请求数 / 费用（$）/ tokens / 活跃用户，各带环比箭头（除零显示 "—"）
  - `DailyTrendChart`：@visactor/vchart 柱状（请求数）+ 折线（费用）
  - `UserTable`：TanStack Table，含活跃天数列，分页
  - `ExportButton`：触发导出
- `lib/`：
  - `report.ts`：React Query 数据获取，`queryKey: ['usage-report', period, anchor]`
  - `stats.ts`：环比百分比纯函数（含除零保护）
  - `export-ppt.ts`：pptxgenjs 组装与下载
- i18n：所有文案 `t('English key')`，补全 locale 文件后执行 `bun run i18n:sync`。
- 新依赖：`bun add pptxgenjs`；新文件补版权头（`bun run copyright`）。

## 7. PPT 结构（全部原生可编辑）

| 页 | 内容 |
| --- | --- |
| 1 封面 | 报告标题（使用周报/月报）+ 周期标签 + 生成时间 |
| 2 总览 | 今期 vs 上期四指标表（请求数/费用/tokens/活跃用户）+ 环比百分比 |
| 3 趋势 | 本期逐天请求数柱状图、费用折线图（pptxgenjs 原生 chart） |
| 4 用户对比 | Top 15 用户表（请求数环比、活跃天数、费用环比）+「其余 N 位用户」汇总行 |
| 5 费用分布 | 费用 Top 10 用户饼图 |

- 文件名：`usage-report-2026-W36.pptx` / `usage-report-2026-08.pptx`
- 文案语言跟随当前界面语言（`t()` 取值）。

## 8. 错误处理与边界

| 场景 | 行为 |
| --- | --- |
| `period` 非法 / `anchor` 解析失败 / `tz_offset` 超界 | 参数拒绝，沿用项目惯例：HTTP 200 + `{success:false, message}`（`common.ApiErrorMsg`） |
| 本期无任何消费数据 | 200 + 空数组；页面空状态；导出仍可用（全 0 +「本期无消费记录」标注） |
| 上期无数据（新站点） | 环比显示 "—"（除零保护） |
| 用户已删除 | 只用 `logs` 冗余的 `username`，不 join `users` 表（同时避开独立日志库跨库 join） |
| 超大用户量 | PPT 取 Top 15 + 汇总行；页面表格分页 |
| 独立日志库（LOG_SQL_DSN） | `LOG_DB` 已按配置指向日志库，无需额外处理 |
| 数据库查询失败 | `common.ApiError` 返回 500，页面错误态 |

## 9. 测试与验证计划

### 后端

- `model/usage_report_test.go`（testify `require`/`assert`），纯逻辑不依赖数据库：
  - 时间窗：周一起始、跨月/跨年（12 月→次年 1 月）、`tz_offset` 为正/负/0、anchor 默认值、上期推导
  - 组装：用户×天矩阵 → ActiveDays、今/上期字段合并、排序、Daily/Overview 汇总
- 聚合 SQL 属数据库行为变更，按 `AGENTS.md` 硬性要求执行**真机三库验证**：docker 分别启动 SQLite、MySQL（>= 5.7.8）、PostgreSQL（>= 9.6），完成迁移 + 造数 + `GetUsageReport` 结果三方比对；PR 中记录数据库版本、命令与结果。`make test` 全绿。

### 前端

- `__tests__/`：环比纯函数（含除零）、空数据渲染、周期切换交互、导出数据组装（mock pptxgenjs 边界）、总览卡与用户表的用户可见行为断言
- 提交前：`bun run typecheck`、`bun run lint`、`bun run test`、`bun run i18n:sync`、`bun run copyright`

## 10. 涉及文件清单

**新增**
- `dto/usage_report.go`
- `model/usage_report.go`、`model/usage_report_test.go`
- `controller/usage_report.go`
- `web/src/features/dashboard/usage-report/`（components/、lib/、`__tests__/`）
- locale 文件更新（各语言）

**修改**
- `router/api-router.go`（dataRoute 注册 1 行）
- `web/package.json`（新增 pptxgenjs）

**不触碰**：`relay/`、`relaykit/`、计费/配额写路径、现有看板接口。

## 11. 遵循的项目规则（AGENTS.md 映射）

- 数据库兼容三方言 + 真机验证 → §9
- JSON 序列化走 `common.Marshal` 等封装 → controller 实现
- 后端测试 testify 与测试质量要求 → §9
- 前端规范（i18n 英文键、typecheck/lint、测试目录 `__tests__/`）→ §6、§9
- 保护的品牌与署名信息不改动；PR 使用 `.agents/github/PR.md` 模板
