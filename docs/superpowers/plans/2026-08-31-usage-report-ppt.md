# 使用报告（周/月对比 + PPT 导出）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理后台新增「使用报告」看板分节：后端只读聚合 API 统计全体用户周/月请求次数、活跃天数、费用（含上期对比），前端展示并导出原生可编辑 PPT。

**Architecture:** 后端 `GET /api/data/usage_report`（AdminAuth）逐天时间窗聚合 `logs` 表（type=2），纯参数化 SQL 保证 SQLite/MySQL/PostgreSQL 三方言一致；前端 `features/dashboard/usage-report/` 新分节，React Query 取数，pptxgenjs 在浏览器生成 .pptx。

**Tech Stack:** Go 1.22+ / Gin / GORM v2 / testify；React 19 / TypeScript / TanStack Query+Table / @visactor/vchart / i18next / pptxgenjs / Vitest+RTL / Bun

**Spec:** `docs/superpowers/specs/2026-08-31-usage-report-ppt-design.md`（本计划依据该 spec 制定，执行前先读）

## Global Constraints

- 数据库兼容：只用参数化 `>= ?`/`< ?`、`GROUP BY user_id, username`、`COALESCE`；**禁止**任何日期方言函数（`date()`/`FROM_UNIXTIME`/`to_timestamp`）。涉及数据库行为的改动必须在真实 SQLite、MySQL(>=5.7.8)、PostgreSQL(>=9.6) 上验证（Task 4），PR 中记录版本/命令/结果
- 后端 JSON：业务代码 marshal/unmarshal 必须走 `common.Marshal`/`common.Unmarshal`（controller 响应用框架 `c.JSON`/`common.ApiSuccess` 为既有惯例）
- 后端测试：testify `require`（setup/致命）+ `assert`（非致命值比较）；不写无不变量价值的测试
- API 错误响应沿用项目惯例：HTTP 200 + `{success:false, message}`（`common.ApiErrorMsg`），**不是** HTTP 400（spec §8 的"400"指业务拒绝，以此为准）
- 费用换算：`quota / common.QuotaPerUnit`（500000 = $1）仅发生在前端展示层（`formatQuota`/`quotaUnitsToDollars`），后端与 DTO 只传原始 quota
- `tz_offset` 取值范围 [-840, 840] 分钟；周期边界用 `time.FixedZone`（无 DST）
- 前端：Bun；所有文案 `t('English key')`（英文原文为键）；组件 props 不解构；测试放模块 `__tests__/`；新文件补版权头（`bun run copyright`）
- 前端周期选择用组件内 state（不用 URL search params——避免改动共享路由 `$section.tsx` 的行为，spec §6 的 validateSearch 以此细化）
- 不触碰：`relay/`、`relaykit/`、计费/配额写路径、现有看板接口
- 品牌与署名信息受保护（AGENTS.md Project Governance），不得修改
- 每个任务以 conventional commit 收尾；PR body 用 `.agents/github/PR.md` 模板，且当前 git 用户非历史核心开发者时须声明 AI 生成/协助

---

### Task 1: 时间窗纯函数（周/月窗口 + 上期推导）

**Files:**
- Create: `model/usage_report_window.go`
- Test: `model/usage_report_window_test.go`

**Interfaces:**
- Produces: `resolvePeriodWindow(period string, anchor time.Time, tzOffsetMin int, previous bool) (periodWindow, error)`；`periodWindow{Label, StartDate, EndDate string; Days []dayWindow}`；`dayWindow{StartUTC, EndUTC int64; Date string}`。Task 2/4 依赖这些签名。

- [ ] **Step 1: 写失败测试**

`model/usage_report_window_test.go`：

```go
package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustUTC(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}

func TestResolvePeriodWindowWeek(t *testing.T) {
	// 2026-09-02 是周三，本周周一应为 2026-08-31
	w, err := resolvePeriodWindow("week", mustUTC(t, "2026-09-02T10:30:00Z"), 0, false)
	require.NoError(t, err)
	assert.Equal(t, "2026-W36", w.Label)
	assert.Equal(t, "2026-08-31", w.StartDate)
	assert.Equal(t, "2026-09-06", w.EndDate)
	require.Len(t, w.Days, 7)
	assert.Equal(t, "2026-08-31", w.Days[0].Date)
	assert.Equal(t, "2026-09-06", w.Days[6].Date)
	// UTC 边界：周一 00:00 UTC 起 24h
	assert.Equal(t, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC).Unix(), w.Days[0].StartUTC)
	assert.Equal(t, w.Days[0].StartUTC+86400, w.Days[0].EndUTC)
}

func TestResolvePeriodWindowWeekPrevious(t *testing.T) {
	w, err := resolvePeriodWindow("week", mustUTC(t, "2026-09-02T10:30:00Z"), 0, true)
	require.NoError(t, err)
	assert.Equal(t, "2026-W35", w.Label)
	assert.Equal(t, "2026-08-24", w.StartDate)
	require.Len(t, w.Days, 7)
}

func TestResolvePeriodWindowWeekAnchorOnSunday(t *testing.T) {
	// 2026-08-30 是周日，应回到本周周一 2026-08-31？不——周日属于上一周，周一为 2026-08-24
	w, err := resolvePeriodWindow("week", mustUTC(t, "2026-08-30T23:00:00Z"), 0, false)
	require.NoError(t, err)
	assert.Equal(t, "2026-W35", w.Label)
	assert.Equal(t, "2026-08-24", w.StartDate)
}

func TestResolvePeriodWindowWeekTimezoneShiftsWeek(t *testing.T) {
	// 同一 UTC 瞬间 2026-09-07T01:30:00Z：
	// 东八区已是周一 09:30 → 本周从 2026-09-07 开始
	wCN, err := resolvePeriodWindow("week", mustUTC(t, "2026-09-07T01:30:00Z"), 480, false)
	require.NoError(t, err)
	assert.Equal(t, "2026-09-07", wCN.StartDate)
	// 纽约（UTC-5）仍是周日 20:30 → 本周从 2026-08-31 开始
	wNY, err := resolvePeriodWindow("week", mustUTC(t, "2026-09-07T01:30:00Z"), -300, false)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-31", wNY.StartDate)
	// 东八区周一 00:00 的 UTC 边界是前一天 16:00
	assert.Equal(t, time.Date(2026, 9, 6, 16, 0, 0, 0, time.UTC).Unix(), wCN.Days[0].StartUTC)
}

func TestResolvePeriodWindowMonth(t *testing.T) {
	w, err := resolvePeriodWindow("month", mustUTC(t, "2026-08-15T08:00:00Z"), 0, false)
	require.NoError(t, err)
	assert.Equal(t, "2026-08", w.Label)
	assert.Equal(t, "2026-08-01", w.StartDate)
	assert.Equal(t, "2026-08-31", w.EndDate)
	require.Len(t, w.Days, 31)
}

func TestResolvePeriodWindowMonthPreviousCrossYear(t *testing.T) {
	// 上期跨年：2026-01 的上期是 2025-12
	w, err := resolvePeriodWindow("month", mustUTC(t, "2026-01-15T08:00:00Z"), 0, true)
	require.NoError(t, err)
	assert.Equal(t, "2025-12", w.Label)
	assert.Equal(t, "2025-12-01", w.StartDate)
	require.Len(t, w.Days, 31)
}

func TestResolvePeriodWindowMonthFebruary(t *testing.T) {
	w, err := resolvePeriodWindow("month", mustUTC(t, "2027-02-10T08:00:00Z"), 0, false)
	require.NoError(t, err)
	require.Len(t, w.Days, 28)

	wLeap, err := resolvePeriodWindow("month", mustUTC(t, "2028-02-10T08:00:00Z"), 0, false)
	require.NoError(t, err)
	require.Len(t, wLeap.Days, 29)
}

func TestResolvePeriodWindowInvalidPeriod(t *testing.T) {
	_, err := resolvePeriodWindow("day", mustUTC(t, "2026-09-02T10:30:00Z"), 0, false)
	require.Error(t, err)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOWORK=off go test ./model/ -run TestResolvePeriodWindow -v`
Expected: 编译失败 `undefined: resolvePeriodWindow`

- [ ] **Step 3: 最小实现**

`model/usage_report_window.go`：

```go
package model

import (
	"errors"
	"fmt"
	"time"
)

var errInvalidUsageReportPeriod = errors.New("invalid usage report period")

// dayWindow 报告时区下的一个自然天查询窗口，边界为 UTC 秒级时间戳。
type dayWindow struct {
	StartUTC int64  // 含
	EndUTC   int64  // 不含
	Date     string // 报告时区自然日 "2006-01-02"
}

// periodWindow 一个报告周期（今期或上期）的窗口信息。
type periodWindow struct {
	Label     string // 周 "2026-W36"；月 "2026-08"
	StartDate string // 周期首日 "2006-01-02"（报告时区）
	EndDate   string // 周期末日 "2006-01-02"（报告时区）
	Days      []dayWindow
}

// resolvePeriodWindow 依据 anchor 瞬时与分钟级固定时区偏移计算周期窗口。
// 周为自然周（周一起始），月为自然月；previous=true 返回紧邻的上一个周期。
func resolvePeriodWindow(period string, anchor time.Time, tzOffsetMin int, previous bool) (periodWindow, error) {
	loc := time.FixedZone("usage-report", tzOffsetMin*60)
	local := anchor.In(loc)

	var start time.Time
	switch period {
	case "week":
		offsetDays := (int(local.Weekday()) + 6) % 7 // 周一=0
		start = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).
			AddDate(0, 0, -offsetDays)
		if previous {
			start = start.AddDate(0, 0, -7)
		}
	case "month":
		month := local.Month()
		if previous {
			month--
		}
		// Month(0) 会被 time.Date 归一化为上一年 12 月
		start = time.Date(local.Year(), month, 1, 0, 0, 0, 0, loc)
	default:
		return periodWindow{}, errInvalidUsageReportPeriod
	}

	var end time.Time
	var label string
	if period == "week" {
		end = start.AddDate(0, 0, 7)
		isoYear, isoWeek := start.ISOWeek()
		label = fmt.Sprintf("%04d-W%02d", isoYear, isoWeek)
	} else {
		end = start.AddDate(0, 1, 0)
		label = fmt.Sprintf("%04d-%02d", start.Year(), int(start.Month()))
	}

	window := periodWindow{
		Label:     label,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.AddDate(0, 0, -1).Format("2006-01-02"),
	}
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		window.Days = append(window.Days, dayWindow{
			StartUTC: day.Unix(),
			EndUTC:   day.AddDate(0, 0, 1).Unix(),
			Date:     day.Format("2006-01-02"),
		})
	}
	return window, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `GOWORK=off go test ./model/ -run TestResolvePeriodWindow -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
git add model/usage_report_window.go model/usage_report_window_test.go
git commit -m "feat(model): add usage report period window resolution"
```

---

### Task 2: DTO + 逐天聚合查询 + 报告组装

**Files:**
- Create: `dto/usage_report.go`
- Create: `model/usage_report.go`
- Test: `model/usage_report_test.go`

**Interfaces:**
- Consumes: Task 1 的 `resolvePeriodWindow`/`periodWindow`/`dayWindow`
- Produces: `dto.UsageReport`（含 `UsageReportPeriodInfo`/`UsageReportOverview`/`UsageReportDaily`/`UsageReportUser`，JSON 小写下划线字段）；`model.GetUsageReport(period string, anchor time.Time, tzOffsetMin int) (*dto.UsageReport, error)`（Task 3/4 依赖）；内部 `assembleUsageReport(period string, curWindow, prevWindow periodWindow, curRows, prevRows [][]userDayAggRow) *dto.UsageReport`（本任务测试）

- [ ] **Step 1: 写失败测试（组装纯逻辑，不依赖 DB）**

`model/usage_report_test.go`：

```go
package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func usageReportTestWindow(t *testing.T, previous bool) periodWindow {
	t.Helper()
	w, err := resolvePeriodWindow("week", time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), 0, previous)
	require.NoError(t, err)
	return w
}

func TestAssembleUsageReport(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)

	// 本期：alice 活跃 2 天（D1/D2），bob 活跃 1 天（D2）
	// 上期：alice 1 天，gone 1 天（本期流失）
	curRows := [][]userDayAggRow{
		{{UserId: 1, Username: "alice", Requests: 3, Quota: 300, Tokens: 30}},
		{
			{UserId: 1, Username: "alice", Requests: 2, Quota: 200, Tokens: 20},
			{UserId: 2, Username: "bob", Requests: 1, Quota: 100, Tokens: 10},
		},
		nil, nil, nil, nil, nil,
	}
	prevRows := [][]userDayAggRow{
		nil,
		{{UserId: 1, Username: "alice", Requests: 5, Quota: 500, Tokens: 50}},
		nil, nil, nil,
		{{UserId: 3, Username: "gone", Requests: 2, Quota: 200, Tokens: 20}},
		nil,
	}

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows)

	assert.Equal(t, "week", report.Period)
	assert.Equal(t, "2026-W36", report.Current.Label)
	assert.Equal(t, "2026-W35", report.Previous.Label)

	// 总览
	assert.Equal(t, int64(6), report.Overview.Requests)
	assert.Equal(t, int64(7), report.Overview.PrevRequests)
	assert.Equal(t, int64(600), report.Overview.Quota)
	assert.Equal(t, int64(700), report.Overview.PrevQuota)
	assert.Equal(t, int64(60), report.Overview.Tokens)
	assert.Equal(t, int64(70), report.Overview.PrevTokens)
	assert.Equal(t, int64(2), report.Overview.ActiveUsers)
	assert.Equal(t, int64(2), report.Overview.PrevActiveUsers)

	// 逐天：7 项，空天为 0
	require.Len(t, report.Daily, 7)
	assert.Equal(t, "2026-08-31", report.Daily[0].Date)
	assert.Equal(t, int64(3), report.Daily[0].Requests)
	assert.Equal(t, int64(1), report.Daily[0].ActiveUsers)
	assert.Equal(t, int64(0), report.Daily[6].Requests)

	// 用户：按本期 Requests 降序；仅上期活跃的 gone 以 0 保留
	require.Len(t, report.Users, 3)
	assert.Equal(t, "alice", report.Users[0].Username)
	assert.Equal(t, int64(5), report.Users[0].Requests)
	assert.Equal(t, 2, report.Users[0].ActiveDays)
	assert.Equal(t, int64(5), report.Users[0].PrevRequests)
	assert.Equal(t, 1, report.Users[0].PrevActiveDays)
	assert.Equal(t, "bob", report.Users[1].Username)
	assert.Equal(t, int64(1), report.Users[1].Requests)
	assert.Equal(t, "gone", report.Users[2].Username)
	assert.Equal(t, int64(0), report.Users[2].Requests)
	assert.Equal(t, int64(2), report.Users[2].PrevRequests)
	assert.Equal(t, 1, report.Users[2].PrevActiveDays)
}

func TestAssembleUsageReportBothEmpty(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)
	emptyCur := make([][]userDayAggRow, 7)
	emptyPrev := make([][]userDayAggRow, 7)

	report := assembleUsageReport("week", curWindow, prevWindow, emptyCur, emptyPrev)
	assert.Equal(t, int64(0), report.Overview.Requests)
	assert.Len(t, report.Daily, 7)
	assert.Empty(t, report.Users)
}

func TestAssembleUsageReportSortStableByUsername(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)
	// 两个用户本期都 0 请求，按 username 升序
	curRows := make([][]userDayAggRow, 7)
	prevRows := make([][]userDayAggRow, 7)
	prevRows[0] = []userDayAggRow{
		{UserId: 2, Username: "zoe", Requests: 1, Quota: 10, Tokens: 1},
		{UserId: 1, Username: "adam", Requests: 1, Quota: 10, Tokens: 1},
	}

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows)
	require.Len(t, report.Users, 2)
	assert.Equal(t, "adam", report.Users[0].Username)
	assert.Equal(t, "zoe", report.Users[1].Username)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOWORK=off go test ./model/ -run TestAssembleUsageReport -v`
Expected: 编译失败 `undefined: assembleUsageReport` / `undefined: userDayAggRow`

- [ ] **Step 3: 实现 DTO**

`dto/usage_report.go`：

```go
package dto

type UsageReportPeriodInfo struct {
	Label     string `json:"label"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type UsageReportOverview struct {
	Requests        int64 `json:"requests"`
	PrevRequests    int64 `json:"prev_requests"`
	Quota           int64 `json:"quota"`
	PrevQuota       int64 `json:"prev_quota"`
	Tokens          int64 `json:"tokens"`
	PrevTokens      int64 `json:"prev_tokens"`
	ActiveUsers     int64 `json:"active_users"`
	PrevActiveUsers int64 `json:"prev_active_users"`
}

type UsageReportDaily struct {
	Date        string `json:"date"`
	Requests    int64  `json:"requests"`
	Quota       int64  `json:"quota"`
	ActiveUsers int64  `json:"active_users"`
}

type UsageReportUser struct {
	UserId         int    `json:"user_id"`
	Username       string `json:"username"`
	Requests       int64  `json:"requests"`
	PrevRequests   int64  `json:"prev_requests"`
	ActiveDays     int    `json:"active_days"`
	PrevActiveDays int    `json:"prev_active_days"`
	Quota          int64  `json:"quota"`
	PrevQuota      int64  `json:"prev_quota"`
	Tokens         int64  `json:"tokens"`
	PrevTokens     int64  `json:"prev_tokens"`
}

type UsageReport struct {
	Period   string                 `json:"period"`
	Current  UsageReportPeriodInfo  `json:"current"`
	Previous UsageReportPeriodInfo  `json:"previous"`
	Overview UsageReportOverview    `json:"overview"`
	Daily    []UsageReportDaily     `json:"daily"`
	Users    []UsageReportUser      `json:"users"`
}
```

- [ ] **Step 4: 实现聚合与组装**

`model/usage_report.go`：

```go
package model

import (
	"errors"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// userDayAggRow 一天内单用户的聚合行。
type userDayAggRow struct {
	UserId   int    `json:"user_id"`
	Username string `json:"username"`
	Requests int64  `json:"requests"`
	Quota    int64  `json:"quota"`
	Tokens   int64  `json:"tokens"`
}

// sumLogsByUserInRange 查询 [startUTC, endUTC) 内消费日志按用户的聚合。
// 纯参数化条件 + COALESCE，SQLite/MySQL/PostgreSQL 行为一致。
func sumLogsByUserInRange(startUTC int64, endUTC int64) ([]userDayAggRow, error) {
	var rows []userDayAggRow
	err := LOG_DB.Table("logs").
		Select("user_id, username, count(*) as requests, COALESCE(sum(quota), 0) as quota, "+
			"COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) as tokens").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, startUTC, endUTC).
		Group("user_id, username").
		Scan(&rows).Error
	if err != nil {
		common.SysError("failed to query usage report rows: " + err.Error())
		return nil, errors.New("查询使用报告数据失败")
	}
	return rows, nil
}

// periodUserAgg 单用户在一个周期内的累计。
type periodUserAgg struct {
	username   string
	requests   int64
	quota      int64
	tokens     int64
	activeDays int
}

// aggregateDays 把逐天聚合行（rowsByDay 与 window.Days 一一对应）累计为周期数据。
func aggregateDays(window periodWindow, rowsByDay [][]userDayAggRow) (
	daily []dto.UsageReportDaily,
	users map[int]*periodUserAgg,
	totalRequests, totalQuota, totalTokens int64,
) {
	users = make(map[int]*periodUserAgg)
	for i, day := range window.Days {
		item := dto.UsageReportDaily{Date: day.Date}
		dayUsers := make(map[int]bool)
		for _, row := range rowsByDay[i] {
			user := users[row.UserId]
			if user == nil {
				user = &periodUserAgg{username: row.Username}
				users[row.UserId] = user
			}
			user.requests += row.Requests
			user.quota += row.Quota
			user.tokens += row.Tokens
			item.Requests += row.Requests
			item.Quota += row.Quota
			dayUsers[row.UserId] = true
		}
		item.ActiveUsers = int64(len(dayUsers))
		daily = append(daily, item)
		totalRequests += item.Requests
		totalQuota += item.Quota
		totalTokens += 0 // 占位，下面按天补齐
	}
	return daily, users, totalRequests, totalQuota, totalTokens
}

// assembleUsageReport 组装最终报告（纯函数，便于单测）。
// 仅上期活跃的用户也保留，本期各值为 0（可反映流失）。
func assembleUsageReport(period string, curWindow, prevWindow periodWindow, curRows, prevRows [][]userDayAggRow) *dto.UsageReport {
	curDaily, curUsers, curRequests, curQuota, _ := aggregateDays(curWindow, curRows)
	prevDaily, prevUsers, prevRequests, prevQuota, _ := aggregateDays(prevWindow, prevRows)

	report := &dto.UsageReport{
		Period:   period,
		Current:  dto.UsageReportPeriodInfo{Label: curWindow.Label, StartDate: curWindow.StartDate, EndDate: curWindow.EndDate},
		Previous: dto.UsageReportPeriodInfo{Label: prevWindow.Label, StartDate: prevWindow.StartDate, EndDate: prevWindow.EndDate},
		Overview: dto.UsageReportOverview{
			Requests: curRequests, PrevRequests: prevRequests,
			Quota: curQuota, PrevQuota: prevQuota,
			ActiveUsers:     int64(len(curUsers)),
			PrevActiveUsers: int64(len(prevUsers)),
		},
		Daily: curDaily,
		Users: make([]dto.UsageReportUser, 0, len(curUsers)+len(prevUsers)),
	}

	tokensOf := func(users map[int]*periodUserAgg, userId int) (int64, int) {
		if user, ok := users[userId]; ok {
			return user.tokens, user.activeDays
		}
		return 0, 0
	}

	seen := make(map[int]bool, len(curUsers)+len(prevUsers))
	for userId, user := range curUsers {
		seen[userId] = true
		prevUser := prevUsers[userId]
		entry := dto.UsageReportUser{
			UserId: userId, Username: user.username,
			Requests: user.requests, ActiveDays: user.activeDays,
			Quota: user.quota, Tokens: user.tokens,
		}
		if prevUser != nil {
			entry.PrevRequests = prevUser.requests
			entry.PrevActiveDays = prevUser.activeDays
			entry.PrevQuota = prevUser.quota
			entry.PrevTokens = prevUser.tokens
		}
		report.Users = append(report.Users, entry)
	}
	for userId, user := range prevUsers {
		if seen[userId] {
			continue
		}
		report.Users = append(report.Users, dto.UsageReportUser{
			UserId: userId, Username: user.username,
			PrevRequests: user.requests, PrevActiveDays: user.activeDays,
			PrevQuota: user.quota, PrevTokens: user.tokens,
		})
	}

	sort.Slice(report.Users, func(i, j int) bool {
		if report.Users[i].Requests != report.Users[j].Requests {
			return report.Users[i].Requests > report.Users[j].Requests
		}
		return report.Users[i].Username < report.Users[j].Username
	})

	// tokens 总量按天累计（aggregateDays 的 totalTokens 由调用方补齐）
	for _, day := range curDaily {
		_ = day
	}
	report.Overview.Tokens = sumTokens(curUsers)
	report.Overview.PrevTokens = sumTokens(prevUsers)
	return report
}

func sumTokens(users map[int]*periodUserAgg) int64 {
	var total int64
	for _, user := range users {
		total += user.tokens
	}
	return total
}

// GetUsageReport 生成周/月使用报告。anchor 为任意瞬时，tzOffsetMin 为报告时区偏移。
func GetUsageReport(period string, anchor time.Time, tzOffsetMin int) (*dto.UsageReport, error) {
	curWindow, err := resolvePeriodWindow(period, anchor, tzOffsetMin, false)
	if err != nil {
		return nil, err
	}
	prevWindow, err := resolvePeriodWindow(period, anchor, tzOffsetMin, true)
	if err != nil {
		return nil, err
	}

	fetchRows := func(window periodWindow) ([][]userDayAggRow, error) {
		rows := make([][]userDayAggRow, len(window.Days))
		for i, day := range window.Days {
			dayRows, err := sumLogsByUserInRange(day.StartUTC, day.EndUTC)
			if err != nil {
				return nil, err
			}
			rows[i] = dayRows
		}
		return rows, nil
	}

	curRows, err := fetchRows(curWindow)
	if err != nil {
		return nil, err
	}
	prevRows, err := fetchRows(prevWindow)
	if err != nil {
		return nil, err
	}
	return assembleUsageReport(period, curWindow, prevWindow, curRows, prevRows), nil
}
```

**注意**：上面 `aggregateDays` 里 `totalTokens` 是占位写法——实现时把逐天 `item` 增加一个 `dayTokens` 局部累计或在 `assembleUsageReport` 用 `sumTokens`（最终版以 `sumTokens` 为准，删掉 `aggregateDays` 的 `totalTokens` 返回值与占位循环，函数签名改为不返回 tokens）。**最终签名**：`aggregateDays(window periodWindow, rowsByDay [][]userDayAggRow) (daily []dto.UsageReportDaily, users map[int]*periodUserAgg, totalRequests, totalQuota int64)`。

- [ ] **Step 5: 运行确认通过（含 Task 1 回归）**

Run: `GOWORK=off go test ./model/ -run 'TestResolvePeriodWindow|TestAssembleUsageReport' -v`
Expected: 全部 PASS

- [ ] **Step 6: 提交**

```bash
git add dto/usage_report.go model/usage_report.go model/usage_report_test.go
git commit -m "feat(model): assemble usage report from per-day log aggregation"
```

---

### Task 3: controller + 路由注册

**Files:**
- Create: `controller/usage_report.go`
- Modify: `router/api-router.go`（dataRoute 组，约 316-322 行处）
- Test: `controller/usage_report_test.go`

**Interfaces:**
- Consumes: `model.GetUsageReport(period string, anchor time.Time, tzOffsetMin int) (*dto.UsageReport, error)`（Task 2）
- Produces: `GET /api/data/usage_report?period=week|month&anchor=YYYY-MM-DD&tz_offset=<min>`，成功 `common.ApiSuccess`，参数错误 `common.ApiErrorMsg`（Task 4/10 依赖该端点）

- [ ] **Step 1: 写失败测试（只测参数拒绝路径，不触 DB）**

`controller/usage_report_test.go`：

```go
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUsageReportRejectsInvalidParams(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantMsg string
	}{
		{"missing period", "", "invalid period, must be week or month"},
		{"unknown period", "period=day", "invalid period, must be week or month"},
		{"bad anchor format", "period=week&anchor=2026/08/31", "invalid anchor, expect YYYY-MM-DD"},
		{"tz not numeric", "period=week&tz_offset=abc", "invalid tz_offset"},
		{"tz out of range", "period=week&tz_offset=900", "invalid tz_offset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)

			GetUsageReport(c)

			var body struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			assert.False(t, body.Success)
			assert.Equal(t, tt.wantMsg, body.Message)
		})
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `GOWORK=off go test ./controller/ -run TestGetUsageReport -v`
Expected: 编译失败 `undefined: GetUsageReport`

- [ ] **Step 3: 实现 controller**

`controller/usage_report.go`：

```go
package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const maxUsageReportTzOffsetMinutes = 840

// GetUsageReport 处理 GET /api/data/usage_report。
// period=week|month（必填）；anchor=YYYY-MM-DD（可选，默认今天，按 tz_offset 时区解释）；
// tz_offset=分钟（可选，默认 0，范围 [-840, 840]）。
func GetUsageReport(c *gin.Context) {
	period := c.Query("period")
	if period != "week" && period != "month" {
		common.ApiErrorMsg(c, "invalid period, must be week or month")
		return
	}

	tzOffsetMinutes := 0
	if raw := c.Query("tz_offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value > maxUsageReportTzOffsetMinutes || value < -maxUsageReportTzOffsetMinutes {
			common.ApiErrorMsg(c, "invalid tz_offset")
			return
		}
		tzOffsetMinutes = value
	}

	loc := time.FixedZone("usage-report", tzOffsetMinutes*60)
	anchor := time.Now().In(loc)
	if raw := c.Query("anchor"); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			common.ApiErrorMsg(c, "invalid anchor, expect YYYY-MM-DD")
			return
		}
		anchor = parsed
	}

	report, err := model.GetUsageReport(period, anchor, tzOffsetMinutes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}
```

- [ ] **Step 4: 注册路由**

`router/api-router.go` dataRoute 组（`dataRoute := apiRouter.Group("/data")` 之后，与其他 `dataRoute.GET` 并列）：

```go
dataRoute.GET("/usage_report", middleware.AdminAuth(), controller.GetUsageReport)
```

- [ ] **Step 5: 运行测试 + 构建确认**

Run: `GOWORK=off go test ./controller/ -run TestGetUsageReport -v && GOWORK=off go build ./...`
Expected: 测试 PASS；构建可能因缺少 `web/dist` 报 embed 错误——先执行 `mkdir -p web/dist && touch web/dist/index.html` 再构建。PASS

- [ ] **Step 6: 提交**

```bash
git add controller/usage_report.go controller/usage_report_test.go router/api-router.go
git commit -m "feat(controller): expose admin usage report aggregation endpoint"
```

---

### Task 4: 三数据库真机验证（AGENTS.md 硬性要求）

**Files:**
- Create（临时，验证后删除、不提交）: `model/usage_report_verify_test.go`

**Interfaces:**
- Consumes: `model.GetUsageReport`（Task 2）；`Log` 模型与 `LOG_DB`（既有）

- [ ] **Step 1: 写 env 门控的集成验证测试**

`model/usage_report_verify_test.go`（与 `model/task_cas_test.go` 同款 sqlite 驱动 `github.com/glebarez/sqlite`；mysql/postgres 驱动已在 go.mod，import 别名与 `model/main.go` 一致）：

```go
package model

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 用法：UR_VERIFY_DRIVER=sqlite|mysql|postgres UR_VERIFY_DSN=... go test ./model/ -run TestUsageReportIntegration -v
// 验证后删除本文件，不提交。
func TestUsageReportIntegration(t *testing.T) {
	dsn := os.Getenv("UR_VERIFY_DSN")
	if dsn == "" {
		t.Skip("UR_VERIFY_DSN not set")
	}

	var dialector gorm.Dialector
	switch os.Getenv("UR_VERIFY_DRIVER") {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	default:
		dialector = sqlite.Open(dsn)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))

	original := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = original })

	require.NoError(t, db.Exec("DELETE FROM logs").Error)
	// anchor 2026-09-02（周三，tz=0）：本期 08-31~09-06，上期 08-24~08-30
	tuesday := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC).Unix()
	insert := func(userID int, username string, ts int64, quota, pt, ct int64) {
		require.NoError(t, db.Exec(
			"INSERT INTO logs (user_id, username, created_at, type, quota, prompt_tokens, completion_tokens, model_name) "+
				"VALUES (?, ?, ?, 2, ?, ?, ?, 'verify-model')",
			userID, username, ts, quota, pt, ct).Error)
	}
	insert(1, "alice", tuesday, 100, 10, 5)        // 本期 D2
	insert(1, "alice", tuesday+86400, 200, 20, 10) // 本期 D3
	insert(2, "bob", tuesday-7*86400, 50, 5, 5)    // 上期
	insert(3, "gone", tuesday-7*86400, 30, 3, 3)   // 上期 only

	report, err := GetUsageReport("week", time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), 0)
	require.NoError(t, err)

	assert.Equal(t, "2026-W36", report.Current.Label)
	assert.Equal(t, "2026-W35", report.Previous.Label)
	require.Len(t, report.Daily, 7)
	assert.Equal(t, int64(2), report.Overview.Requests)
	assert.Equal(t, int64(2), report.Overview.PrevRequests)
	assert.Equal(t, int64(300), report.Overview.Quota)
	assert.Equal(t, int64(80), report.Overview.PrevQuota)
	assert.Equal(t, int64(45), report.Overview.Tokens)
	assert.Equal(t, int64(25), report.Overview.PrevTokens)
	require.Len(t, report.Users, 3)
	assert.Equal(t, "alice", report.Users[0].Username)
	assert.Equal(t, 2, report.Users[0].ActiveDays)
	assert.Equal(t, int64(0), report.Users[1].Requests) // bob/gone 本期为 0
}
```

- [ ] **Step 2: SQLite 真机验证**

Run:
```bash
rm -f /tmp/ur_verify.db
UR_VERIFY_DRIVER=sqlite UR_VERIFY_DSN=/tmp/ur_verify.db GOWORK=off go test ./model/ -run TestUsageReportIntegration -v
```
Expected: PASS（记录 sqlite 驱动为 glebarez/sqlite 纯 Go 实现，版本见 go.mod）

- [ ] **Step 3: MySQL 真机验证**

```bash
docker run -d --name ur-mysql -e MYSQL_ROOT_PASSWORD=urpass -e MYSQL_DATABASE=ur_verify -p 3307:3306 mysql:8.0
# 等 30 秒就绪后：
UR_VERIFY_DRIVER=mysql UR_VERIFY_DSN='root:urpass@tcp(127.0.0.1:3307)/ur_verify?parseTime=true' \
  GOWORK=off go test ./model/ -run TestUsageReportIntegration -v
```
Expected: PASS。再用 `mysql:5.7` 重复一次（AGENTS.md 最低支持 5.7.8）：`docker run ... mysql:5.7`（注意 5.7 需要加 `--default-authentication-plugin=mysql_native_password`）。

- [ ] **Step 4: PostgreSQL 真机验证**

```bash
docker run -d --name ur-pg -e POSTGRES_PASSWORD=urpass -e POSTGRES_DB=ur_verify -p 5433:5432 postgres:15
UR_VERIFY_DRIVER=postgres UR_VERIFY_DSN='postgres://postgres:urpass@127.0.0.1:5433/ur_verify?sslmode=disable' \
  GOWORK=off go test ./model/ -run TestUsageReportIntegration -v
```
Expected: PASS

- [ ] **Step 5: 记录验证矩阵并清理**

把每个数据库的 `版本 / 启动命令 / 测试命令 / 结果` 记入一个临时笔记（PR body 要用）：

```
| 数据库 | 版本 | 结果 |
| SQLite | glebarez/sqlite (纯 Go, go.mod 版本) | PASS |
| MySQL  | 8.0.x / 5.7.x | PASS |
| PostgreSQL | 15.x | PASS |
```

删除临时测试文件与容器：
```bash
rm model/usage_report_verify_test.go
docker rm -f ur-mysql ur-pg
```

**注意**：本任务无代码提交（验证性任务），验证记录随 Task 11 的 PR 呈现。若任何方言失败，**停止**，按 AGENTS.md 不得宣称数据库兼容。

---

### Task 5: 前端依赖 + 类型 + 环比纯函数

**Files:**
- Modify: `web/package.json`（bun add）
- Create: `web/src/features/dashboard/usage-report/types.ts`
- Create: `web/src/features/dashboard/usage-report/lib/stats.ts`
- Test: `web/src/features/dashboard/usage-report/__tests__/stats.test.ts`

**Interfaces:**
- Produces: `UsageReport`/`UsageReportOverview`/`UsageReportDaily`/`UsageReportUser`/`UsageReportPeriod`（TS 类型，镜像 dto JSON）；`percentChange(current: number, previous: number): number | null`；`formatPercentChange(value: number | null): string | null`。Task 7/8/9 依赖。

- [ ] **Step 1: 安装 pptxgenjs**

```bash
cd web && bun add pptxgenjs
```

- [ ] **Step 2: 写失败测试**

`__tests__/stats.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { formatPercentChange, percentChange } from '../lib/stats'

describe('percentChange', () => {
  it('returns percentage growth relative to previous', () => {
    expect(percentChange(150, 100)).toBe(50)
  })

  it('returns negative percentage for decline', () => {
    expect(percentChange(80, 100)).toBe(-20)
  })

  it('returns null when previous is zero (no baseline)', () => {
    expect(percentChange(10, 0)).toBeNull()
  })

  it('returns null when previous is negative', () => {
    expect(percentChange(10, -1)).toBeNull()
  })
})

describe('formatPercentChange', () => {
  it('formats with explicit plus sign and one decimal', () => {
    expect(formatPercentChange(12.34)).toBe('+12.3%')
  })

  it('keeps negative sign', () => {
    expect(formatPercentChange(-8)).toBe('-8.0%')
  })

  it('returns null text placeholder handled by caller', () => {
    expect(formatPercentChange(null)).toBeNull()
  })
})
```

- [ ] **Step 3: 运行确认失败**

Run: `cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/stats.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 4: 实现类型与纯函数**

`types.ts`：

```ts
export type UsageReportPeriod = 'week' | 'month'

export interface UsageReportPeriodInfo {
  label: string
  start_date: string
  end_date: string
}

export interface UsageReportOverview {
  requests: number
  prev_requests: number
  quota: number
  prev_quota: number
  tokens: number
  prev_tokens: number
  active_users: number
  prev_active_users: number
}

export interface UsageReportDaily {
  date: string
  requests: number
  quota: number
  active_users: number
}

export interface UsageReportUser {
  user_id: number
  username: string
  requests: number
  prev_requests: number
  active_days: number
  prev_active_days: number
  quota: number
  prev_quota: number
  tokens: number
  prev_tokens: number
}

export interface UsageReport {
  period: UsageReportPeriod
  current: UsageReportPeriodInfo
  previous: UsageReportPeriodInfo
  overview: UsageReportOverview
  daily: UsageReportDaily[]
  users: UsageReportUser[]
}
```

`lib/stats.ts`：

```ts
/**
 * 环比变化百分比：(current - previous) / previous * 100。
 * 上期为 0 或负数时返回 null（界面显示 "—"）。
 */
export function percentChange(current: number, previous: number): number | null {
  if (previous <= 0) return null
  return ((current - previous) / previous) * 100
}

/** 格式化为 "+12.3%" / "-8.0%"；null 返回 null，由调用方渲染占位符 */
export function formatPercentChange(value: number | null): string | null {
  if (value == null) return null
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(1)}%`
}
```

- [ ] **Step 5: 运行确认通过 + 版权头 + 提交**

```bash
cd web && bun run copyright && bun run vitest run src/features/dashboard/usage-report/__tests__/stats.test.ts
git add web/package.json web/bun.lock web/src/features/dashboard/usage-report/
git commit -m "feat(web): add usage report types and period-over-period stats helpers"
```

---

### Task 6: 数据获取（api + useQuery hook）

**Files:**
- Create: `web/src/features/dashboard/usage-report/api.ts`
- Create: `web/src/features/dashboard/usage-report/hooks/use-usage-report.ts`

**Interfaces:**
- Consumes: Task 5 的 `UsageReport`/`UsageReportPeriod`；`@/lib/api` 的 `api` 实例
- Produces: `getUsageReport(params: { period: UsageReportPeriod; anchor?: string; tzOffsetMinutes: number }): Promise<UsageReport>`；`useUsageReport(period: UsageReportPeriod, anchor: string | undefined)`（Task 7 依赖）

- [ ] **Step 1: 实现 api 函数（先读 `features/dashboard/api.ts` 任一函数，对齐其取数写法）**

`api.ts`：

```ts
import { api } from '@/lib/api'

import type { UsageReport, UsageReportPeriod } from './types'

export async function getUsageReport(params: {
  period: UsageReportPeriod
  anchor?: string
  tzOffsetMinutes: number
}): Promise<UsageReport> {
  const res = await api.get('/api/data/usage_report', {
    params: {
      period: params.period,
      anchor: params.anchor,
      tz_offset: params.tzOffsetMinutes,
    },
  })
  return res.data.data as UsageReport
}
```

（若 `@/lib/api` 拦截器已解包 `data`，以 `features/dashboard/api.ts` 现有写法为准调整。）

- [ ] **Step 2: 实现 hook**

`hooks/use-usage-report.ts`：

```ts
import { useQuery } from '@tanstack/react-query'

import { getUsageReport } from '../api'
import type { UsageReportPeriod } from '../types'

export function useUsageReport(period: UsageReportPeriod, anchor: string | undefined) {
  // 报告天边界跟随浏览器时区（getTimezoneOffset 对东八区返回 -480，取负得 +480）
  const tzOffsetMinutes = -new Date().getTimezoneOffset()
  return useQuery({
    queryKey: ['usage-report', period, anchor ?? '', tzOffsetMinutes],
    queryFn: () => getUsageReport({ period, anchor, tzOffsetMinutes }),
  })
}
```

- [ ] **Step 3: 类型检查 + 版权头 + 提交**

```bash
cd web && bun run copyright && bun run typecheck
git add web/src/features/dashboard/usage-report/
git commit -m "feat(web): add usage report data fetching hook"
```

---

### Task 7: 周期切换 + 总览卡组件

**Files:**
- Create: `web/src/features/dashboard/usage-report/components/period-switcher.tsx`
- Create: `web/src/features/dashboard/usage-report/components/overview-cards.tsx`
- Create: `web/src/features/dashboard/usage-report/components/change-indicator.tsx`
- Test: `web/src/features/dashboard/usage-report/__tests__/overview-cards.test.tsx`

**Interfaces:**
- Consumes: Task 5/6 的类型与 `percentChange`/`formatPercentChange`；`@/components/ui/tabs`、`@/components/ui/button`（以仓库现有组件路径为准）
- Produces: `PeriodSwitcher(props: { period: UsageReportPeriod; anchor: string; onChange: (next: { period?: UsageReportPeriod; anchor?: string }) => void })`；`OverviewCards(props: { overview: UsageReportOverview })`；`ChangeIndicator(props: { value: number | null })`（Task 10 依赖渲染）

- [ ] **Step 1: 写失败测试**

`__tests__/overview-cards.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { OverviewCards } from '../components/overview-cards'
import type { UsageReportOverview } from '../types'

const baseOverview: UsageReportOverview = {
  requests: 120,
  prev_requests: 100,
  quota: 600000,
  prev_quota: 500000,
  tokens: 9000,
  prev_tokens: 0,
  active_users: 8,
  prev_active_users: 4,
}

describe('OverviewCards', () => {
  it('renders four metric cards with current values and changes', () => {
    render(<OverviewCards overview={baseOverview} />)

    expect(screen.getByRole('listitem', { name: /requests/i })).toHaveTextContent('120')
    expect(screen.getByRole('listitem', { name: /requests/i })).toHaveTextContent('+20.0%')
    expect(screen.getByRole('listitem', { name: /active users/i })).toHaveTextContent('8')
    expect(screen.getByRole('listitem', { name: /active users/i })).toHaveTextContent('+100.0%')
  })

  it('shows placeholder dash when previous period has no baseline', () => {
    render(<OverviewCards overview={baseOverview} />)

    expect(screen.getByRole('listitem', { name: /tokens/i })).toHaveTextContent('—')
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/overview-cards.test.tsx`
Expected: FAIL（组件不存在）

- [ ] **Step 3: 实现组件**

`components/change-indicator.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

import { formatPercentChange, percentChange } from '../lib/stats'

interface ChangeIndicatorProps {
  current: number
  previous: number
}

export function ChangeIndicator(props: ChangeIndicatorProps) {
  const { t } = useTranslation()
  const formatted = formatPercentChange(percentChange(props.current, props.previous))
  if (formatted == null) {
    return <span title={t('No usage in previous period')}>—</span>
  }
  const direction = formatted.startsWith('+') ? '↑' : '↓'
  return (
    <span aria-label={t('Change') + ' ' + formatted}>
      {direction} {formatted}
    </span>
  )
}
```

`components/overview-cards.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

import { formatQuota } from '@/lib/format'

import { ChangeIndicator } from './change-indicator'
import type { UsageReportOverview } from '../types'

interface OverviewCardsProps {
  overview: UsageReportOverview
}

export function OverviewCards(props: OverviewCardsProps) {
  const { t } = useTranslation()
  const cards = [
    { label: t('Requests'), current: props.overview.requests, previous: props.overview.prev_requests, value: props.overview.requests.toLocaleString() },
    { label: t('Cost'), current: props.overview.quota, previous: props.overview.prev_quota, value: formatQuota(props.overview.quota) },
    { label: t('Tokens'), current: props.overview.tokens, previous: props.overview.prev_tokens, value: props.overview.tokens.toLocaleString() },
    { label: t('Active users'), current: props.overview.active_users, previous: props.overview.prev_active_users, value: props.overview.active_users.toLocaleString() },
  ]

  return (
    <ul className="grid grid-cols-2 gap-4 md:grid-cols-4">
      {cards.map((card) => (
        <li key={card.label} aria-label={card.label} className="rounded-lg border p-4">
          <div className="text-muted-foreground text-sm">{card.label}</div>
          <div className="text-2xl font-semibold">{card.value}</div>
          <ChangeIndicator current={card.current} previous={card.previous} />
        </li>
      ))}
    </ul>
  )
}
```

`components/period-switcher.tsx`：

```tsx
import { ChevronLeft, ChevronRight } from 'lucide-react'
import dayjs from 'dayjs'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { UsageReportPeriod } from '../types'

interface PeriodSwitcherProps {
  period: UsageReportPeriod
  anchor: string
  onChange: (next: { period?: UsageReportPeriod; anchor?: string }) => void
}

export function PeriodSwitcher(props: PeriodSwitcherProps) {
  const { t } = useTranslation()

  const shiftAnchor = (direction: 1 | -1) => {
    const base = dayjs(props.anchor)
    const next =
      props.period === 'week'
        ? base.add(7 * direction, 'day')
        : base.add(direction, 'month')
    props.onChange({ anchor: next.format('YYYY-MM-DD') })
  }

  return (
    <div className="flex items-center gap-2">
      <Button
        variant="outline"
        size="icon"
        aria-label={t('Previous period')}
        onClick={() => shiftAnchor(-1)}
      >
        <ChevronLeft aria-hidden="true" />
      </Button>
      <Tabs
        value={props.period}
        onValueChange={(value) => props.onChange({ period: value as UsageReportPeriod })}
      >
        <TabsList>
          <TabsTrigger value="week">{t('Weekly')}</TabsTrigger>
          <TabsTrigger value="month">{t('Monthly')}</TabsTrigger>
        </TabsList>
      </Tabs>
      <Button
        variant="outline"
        size="icon"
        aria-label={t('Next period')}
        onClick={() => shiftAnchor(1)}
      >
        <ChevronRight aria-hidden="true" />
      </Button>
    </div>
  )
}
```

（`@/components/ui/button`、`size="icon"` 若与仓库现有 Button API 不符，以既有用法为准，如 `size="sm"`。）

- [ ] **Step 4: 运行确认通过 + lint/typecheck + 提交**

```bash
cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/overview-cards.test.tsx \
  && bun run typecheck && bun run lint && bun run copyright
git add web/src/features/dashboard/usage-report/
git commit -m "feat(web): add usage report period switcher and overview cards"
```

---

### Task 8: 趋势图 + 用户明细表

**Files:**
- Create: `web/src/features/dashboard/usage-report/components/daily-trend-chart.tsx`
- Create: `web/src/features/dashboard/usage-report/components/user-table.tsx`
- Test: `web/src/features/dashboard/usage-report/__tests__/user-table.test.tsx`

**Interfaces:**
- Consumes: Task 5 类型；`@/lib/format` 的 `formatQuota`/`quotaUnitsToDollars`；`@/lib/vchart`（先读 `features/dashboard/components/users/user-charts.tsx` 与 `lib/vchart.ts` 对齐主题包装）
- Produces: `DailyTrendChart(props: { daily: UsageReportDaily[] })`；`UserTable(props: { users: UsageReportUser[] })`（Task 10 依赖渲染）

- [ ] **Step 1: 写失败测试（表格行为）**

`__tests__/user-table.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { UserTable } from '../components/user-table'
import type { UsageReportUser } from '../types'

const users: UsageReportUser[] = [
  { user_id: 1, username: 'alice', requests: 50, prev_requests: 40, active_days: 6, prev_active_days: 5, quota: 600000, prev_quota: 500000, tokens: 900, prev_tokens: 800 },
  { user_id: 2, username: 'bob', requests: 10, prev_requests: 30, active_days: 2, prev_active_days: 5, quota: 100000, prev_quota: 300000, tokens: 100, prev_tokens: 300 },
]

describe('UserTable', () => {
  it('renders one row per user with requests and active days', () => {
    render(<UserTable users={users} />)

    expect(screen.getByRole('cell', { name: 'alice' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'bob' })).toBeInTheDocument()
    expect(screen.getByText('50')).toBeInTheDocument()
    expect(screen.getByText('6')).toBeInTheDocument()
  })

  it('renders empty state when no users', () => {
    render(<UserTable users={[]} />)

    expect(screen.getByText('No usage records in this period')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/user-table.test.tsx`
Expected: FAIL

- [ ] **Step 3: 实现表格**

`components/user-table.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

import { formatQuota } from '@/lib/format'

import { ChangeIndicator } from './change-indicator'
import type { UsageReportUser } from '../types'

interface UserTableProps {
  users: UsageReportUser[]
}

export function UserTable(props: UserTableProps) {
  const { t } = useTranslation()

  if (props.users.length === 0) {
    return (
      <div className="text-muted-foreground py-8 text-center" role="status">
        {t('No usage records in this period')}
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-muted-foreground border-b text-left">
            <th className="p-2">{t('User')}</th>
            <th className="p-2">{t('Requests')}</th>
            <th className="p-2">{t('Change')}</th>
            <th className="p-2">{t('Active days')}</th>
            <th className="p-2">{t('Cost')}</th>
            <th className="p-2">{t('Tokens')}</th>
          </tr>
        </thead>
        <tbody>
          {props.users.map((user) => (
            <tr key={user.user_id} className="border-b">
              <td className="p-2 font-medium">{user.username}</td>
              <td className="p-2">{user.requests.toLocaleString()}</td>
              <td className="p-2">
                <ChangeIndicator current={user.requests} previous={user.prev_requests} />
              </td>
              <td className="p-2">
                {user.active_days}
                <span className="text-muted-foreground"> / {user.prev_active_days}</span>
              </td>
              <td className="p-2">
                {formatQuota(user.quota)}
                <span className="text-muted-foreground"> / {formatQuota(user.prev_quota)}</span>
              </td>
              <td className="p-2">{user.tokens.toLocaleString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
```

- [ ] **Step 4: 实现趋势图**

`components/daily-trend-chart.tsx`（先读 `features/dashboard/components/users/user-charts.tsx` 与 `web/src/lib/vchart.ts`，若项目有统一的 VChart 封装/主题（`VCHART_OPTION`），用相同方式包装 spec）：

```tsx
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { VChart } from '@visactor/react-vchart'
import { quotaUnitsToDollars } from '@/lib/format'

import type { UsageReportDaily } from '../types'

interface DailyTrendChartProps {
  daily: UsageReportDaily[]
}

export function DailyTrendChart(props: DailyTrendChartProps) {
  const { t } = useTranslation()

  const spec = useMemo(
    () => ({
      type: 'common',
      data: [
        {
          id: 'usage',
          values: props.daily.map((item) => ({
            date: item.date.slice(5),
            requests: item.requests,
            cost: quotaUnitsToDollars(item.quota),
          })),
        },
      ],
      series: [
        { type: 'bar', xField: 'date', yField: 'requests', data: { id: 'usage' } },
        { type: 'line', xField: 'date', yField: 'cost', data: { id: 'usage' } },
      ],
      axes: [
        { orient: 'left', title: { visible: true, text: t('Requests') } },
        { orient: 'right', title: { visible: true, text: t('Cost') } },
        { orient: 'bottom', label: { autoRotate: true } },
      ],
    }),
    [props.daily, t],
  )

  return (
    <div className="h-80" role="img" aria-label={t('Daily trend')}>
      <VChart spec={spec} />
    </div>
  )
}
```

- [ ] **Step 5: 运行测试 + lint/typecheck + 版权头 + 提交**

```bash
cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/user-table.test.tsx \
  && bun run typecheck && bun run lint && bun run copyright
git add web/src/features/dashboard/usage-report/
git commit -m "feat(web): add usage report daily trend chart and user table"
```

---

### Task 9: PPT 导出

**Files:**
- Create: `web/src/features/dashboard/usage-report/lib/export-ppt.ts`
- Test: `web/src/features/dashboard/usage-report/__tests__/export-ppt.test.ts`

**Interfaces:**
- Consumes: Task 5 类型与 `percentChange`/`formatPercentChange`；`@/lib/format` 的 `quotaUnitsToDollars`
- Produces: `buildReportSlides(report: UsageReport, t: TFunction): ReportSlides`（纯组装，测试用）；`exportUsageReportPptx(report: UsageReport, t: TFunction): Promise<string>`（返回文件名，Task 10 依赖）

- [ ] **Step 1: 写失败测试**

`__tests__/export-ppt.test.ts`：

```ts
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import { buildReportSlides } from '../lib/export-ppt'
import type { UsageReport } from '../types'

const identityT = ((key: string) => key) as unknown as TFunction

function buildReport(userCount: number): UsageReport {
  const users = Array.from({ length: 20 }, (_, i) => ({
    user_id: i + 1,
    username: `user-${String(i + 1).padStart(2, '0')}`,
    requests: 100 - i,
    prev_requests: 50,
    active_days: 5,
    prev_active_days: 4,
    quota: 100000 * (userCount - i),
    prev_quota: 50000,
    tokens: 1000,
    prev_tokens: 500,
  })).slice(0, userCount)

  return {
    period: 'week',
    current: { label: '2026-W36', start_date: '2026-08-31', end_date: '2026-09-06' },
    previous: { label: '2026-W35', start_date: '2026-08-24', end_date: '2026-08-30' },
    overview: {
      requests: 300, prev_requests: 200, quota: 600000, prev_quota: 500000,
      tokens: 9000, prev_tokens: 0, active_users: 8, prev_active_users: 4,
    },
    daily: Array.from({ length: 7 }, (_, i) => ({
      date: `2026-09-0${i + 1}`,
      requests: 10 * (i + 1),
      quota: 1000,
      active_users: 2,
    })),
    users,
  }
}

describe('buildReportSlides', () => {
  it('caps user table at top 15 and aggregates the rest into one row', () => {
    const slides = buildReportSlides(buildReport(20), identityT)
    const usersSlide = slides.find((slide) => slide.kind === 'users')

    expect(usersSlide?.rows).toHaveLength(16)
    const lastRow = usersSlide?.rows.at(-1)
    expect(lastRow?.[0]).toBe('Others')
  })

  it('builds trend chart from daily entries', () => {
    const slides = buildReportSlides(buildReport(3), identityT)
    const trendSlide = slides.find((slide) => slide.kind === 'trend')

    expect(trendSlide?.labels).toHaveLength(7)
    expect(trendSlide?.requestsValues[0]).toBe(10)
  })

  it('caps cost share pie at top 10 users', () => {
    const slides = buildReportSlides(buildReport(20), identityT)
    const costSlide = slides.find((slide) => slide.kind === 'cost-share')

    expect(costSlide?.labels).toHaveLength(10)
  })
})
```

- [ ] **Step 2: 运行确认失败**

Run: `cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/export-ppt.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 实现**

`lib/export-ppt.ts`：

```ts
import type { TFunction } from 'i18next'
import pptxgenjs from 'pptxgenjs'

import { quotaUnitsToDollars } from '@/lib/format'

import { formatPercentChange, percentChange } from './stats'
import type { UsageReport } from '../types'

const TOP_TABLE_USERS = 15
const TOP_PIE_USERS = 10

export interface ReportSlides {
  kind: 'title' | 'overview' | 'trend' | 'users' | 'cost-share'
  title: string
  // overview/users 页的表格行；trend 页为 labels/requestsValues/costValues；cost-share 页为 labels/values
  rows?: string[][]
  labels?: string[]
  requestsValues?: number[]
  costValues?: number[]
  values?: number[]
}

function changeText(current: number, previous: number): string {
  const formatted = formatPercentChange(percentChange(current, previous))
  return formatted ?? '—'
}

export function buildReportSlides(report: UsageReport, t: TFunction): ReportSlides[] {
  const periodName =
    report.period === 'week' ? t('Usage Weekly Report') : t('Usage Monthly Report')

  const overviewRows = [
    [t('Metric'), report.current.label, report.previous.label, t('Change')],
    [t('Requests'), String(report.overview.requests), String(report.overview.prev_requests), changeText(report.overview.requests, report.overview.prev_requests)],
    [t('Cost'), `$${quotaUnitsToDollars(report.overview.quota).toFixed(2)}`, `$${quotaUnitsToDollars(report.overview.prev_quota).toFixed(2)}`, changeText(report.overview.quota, report.overview.prev_quota)],
    [t('Tokens'), String(report.overview.tokens), String(report.overview.prev_tokens), changeText(report.overview.tokens, report.overview.prev_tokens)],
    [t('Active users'), String(report.overview.active_users), String(report.overview.prev_active_users), changeText(report.overview.active_users, report.overview.prev_active_users)],
  ]

  const topUsers = report.users.slice(0, TOP_TABLE_USERS)
  const restUsers = report.users.slice(TOP_TABLE_USERS)
  const userRows: string[][] = [
    [t('User'), t('Requests'), t('Change'), t('Active days'), t('Cost'), t('Change')],
    ...topUsers.map((user) => [
      user.username,
      String(user.requests),
      changeText(user.requests, user.prev_requests),
      `${user.active_days} / ${user.prev_active_days}`,
      `$${quotaUnitsToDollars(user.quota).toFixed(2)}`,
      changeText(user.quota, user.prev_quota),
    ]),
  ]
  if (restUsers.length > 0) {
    const restSum = restUsers.reduce(
      (acc, user) => ({
        requests: acc.requests + user.requests,
        prevRequests: acc.prevRequests + user.prev_requests,
        quota: acc.quota + user.quota,
      }),
      { requests: 0, prevRequests: 0, quota: 0 },
    )
    userRows.push([
      `${t('Others')} (${restUsers.length})`,
      String(restSum.requests),
      changeText(restSum.requests, restSum.prevRequests),
      '—',
      `$${quotaUnitsToDollars(restSum.quota).toFixed(2)}`,
      changeText(restSum.quota, 0) === '—' ? '—' : changeText(restSum.quota, 0),
    ])
  }

  return [
    {
      kind: 'title',
      title: `${periodName} ${report.current.label}`,
    },
    { kind: 'overview', title: t('Overview'), rows: overviewRows },
    {
      kind: 'trend',
      title: t('Daily trend'),
      labels: report.daily.map((item) => item.date.slice(5)),
      requestsValues: report.daily.map((item) => item.requests),
      costValues: report.daily.map((item) => quotaUnitsToDollars(item.quota)),
    },
    { kind: 'users', title: t('Top users'), rows: userRows },
    {
      kind: 'cost-share',
      title: t('Cost distribution'),
      labels: report.users.slice(0, TOP_PIE_USERS).map((user) => user.username),
      values: report.users.slice(0, TOP_PIE_USERS).map((user) => quotaUnitsToDollars(user.quota)),
    },
  ]
}

export async function exportUsageReportPptx(report: UsageReport, t: TFunction): Promise<string> {
  const pptx = new pptxgenjs()
  pptx.layout = 'LAYOUT_16x9'
  const slides = buildReportSlides(report, t)

  for (const slide of slides) {
    const page = pptx.addSlide()
    page.addText(slide.title, { x: 0.5, y: 0.3, fontSize: 24, bold: true })
    if (slide.kind === 'title') {
      page.addText(
        `${t('Current period')}: ${report.current.start_date} ~ ${report.current.end_date}\n${t('Generated at')}: ${new Date().toLocaleString()}`,
        { x: 0.5, y: 2.0, fontSize: 16 },
      )
    } else if (slide.kind === 'overview' || slide.kind === 'users') {
      page.addTable(slide.rows ?? [], {
        x: 0.5, y: 1.2, w: 9.0, fontSize: 12,
        border: { pt: 0.5, color: 'CCCCCC' },
      })
    } else if (slide.kind === 'trend') {
      page.addChart(
        pptx.ChartType.bar,
        [
          { name: t('Requests'), labels: slide.labels ?? [], values: slide.requestsValues ?? [] },
        ],
        { x: 0.4, y: 1.1, w: 9.0, h: 4.6, showLegend: true },
      )
    } else if (slide.kind === 'cost-share') {
      page.addChart(
        pptx.ChartType.pie,
        [{ name: t('Cost'), labels: slide.labels ?? [], values: slide.values ?? [] }],
        { x: 2.0, y: 1.1, w: 6.0, h: 4.6, showLegend: true, showPercent: true },
      )
    }
  }

  const fileName = `usage-report-${report.current.label}.pptx`
  await pptx.writeFile({ fileName })
  return fileName
}
```

- [ ] **Step 4: 运行确认通过 + lint/typecheck + 版权头 + 提交**

```bash
cd web && bun run vitest run src/features/dashboard/usage-report/__tests__/export-ppt.test.ts \
  && bun run typecheck && bun run lint && bun run copyright
git add web/src/features/dashboard/usage-report/
git commit -m "feat(web): add pptx export for usage report"
```

---

### Task 10: 分节入口 + 注册 + i18n

**Files:**
- Create: `web/src/features/dashboard/usage-report/index.tsx`
- Modify: `web/src/features/dashboard/section-registry.tsx`
- Modify: `web/src/features/dashboard/index.tsx`（约 179 行 SECTION_META、约 267 行起的分节渲染分支）
- Modify: `web/src/i18n/locales/zh.json`（及其他语言文件，经 i18n:sync 后补译）

**Interfaces:**
- Consumes: Task 6-9 全部组件与函数
- Produces: `UsageReportSection`（导出，供 dashboard 分节渲染）

- [ ] **Step 1: 实现分节入口**

`usage-report/index.tsx`：

```tsx
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import dayjs from 'dayjs'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { DailyTrendChart } from './components/daily-trend-chart'
import { OverviewCards } from './components/overview-cards'
import { PeriodSwitcher } from './components/period-switcher'
import { UserTable } from './components/user-table'
import { useUsageReport } from './hooks/use-usage-report'
import { exportUsageReportPptx } from './lib/export-ppt'
import type { UsageReportPeriod } from './types'

export function UsageReportSection() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<UsageReportPeriod>('week')
  const [anchor, setAnchor] = useState<string>(() => dayjs().format('YYYY-MM-DD'))
  const [exporting, setExporting] = useState(false)
  const { data, isLoading, isError } = useUsageReport(period, anchor)

  const handleExport = async () => {
    if (!data) return
    setExporting(true)
    try {
      await exportUsageReportPptx(data, t)
    } finally {
      setExporting(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <PeriodSwitcher
          period={period}
          anchor={anchor}
          onChange={(next) => {
            if (next.period) setPeriod(next.period)
            if (next.anchor) setAnchor(next.anchor)
          }}
        />
        <Button onClick={handleExport} disabled={!data || exporting}>
          {exporting ? t('Exporting...') : t('Export PPT')}
        </Button>
      </div>

      {isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : isError || !data ? (
        <div className="text-muted-foreground py-8 text-center" role="alert">
          {t('Failed to load usage report')}
        </div>
      ) : (
        <>
          <div className="text-muted-foreground text-sm">
            {data.current.start_date} ~ {data.current.end_date}
            {t('vs previous period')} {data.previous.label}
          </div>
          <OverviewCards overview={data.overview} />
          <DailyTrendChart daily={data.daily} />
          <UserTable users={data.users} />
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 2: 注册分节**

`section-registry.tsx` 的 `DASHBOARD_SECTIONS` 数组追加（并保持 `ADMIN_ONLY_SECTIONS` 同步）：

```tsx
{
  id: 'usage-report',
  titleKey: 'Usage Report',
  adminOnly: true,
  build: () => null,
},
```

同文件 `ADMIN_ONLY_SECTIONS` 改为 `new Set<string>(['users', 'usage-report'])`。

- [ ] **Step 3: 接入 dashboard 渲染分支**

`features/dashboard/index.tsx`：
1. `SECTION_META`（约 179 行）加 `usage-report: { titleKey: 'Usage Report' }`
2. 约 267 行的分节三元链加一支：`activeSection === 'usage-report' ? <UsageReportSection /> : ...`（顶部 import；具体接入点以该文件现有 models/flow 分支写法为准）

- [ ] **Step 4: i18n 文案**

涉及键（英文原文即键）：`Usage Report`、`Weekly`、`Monthly`、`Previous period`、`Next period`、`Requests`、`Cost`、`Tokens`、`Active users`、`Active days`、`Change`、`User`、`Others`、`No usage records in this period`、`No usage in previous period`、`Export PPT`、`Exporting...`、`Failed to load usage report`、`vs previous period`、`Daily trend`、`Top users`、`Cost distribution`、`Overview`、`Usage Weekly Report`、`Usage Monthly Report`、`Current period`、`Generated at`

```bash
cd web && bun run i18n:sync
# 然后手工在 zh.json（以及 zh-TW/fr/ru/ja/vi 可先复用英文，zh 必须翻译）填入中文译文，例如：
# "Usage Report": "使用报告"，"Weekly": "周报"，"Monthly": "月报"，"Export PPT": "导出 PPT"，
# "Active days": "活跃天数"，"Active users": "活跃用户"，"No usage records in this period": "本期无消费记录"
```

- [ ] **Step 5: 全量前端检查 + 提交**

```bash
cd web && bun run typecheck && bun run lint && bun run test && bun run i18n:sync && bun run copyright && bun run format
git add web/src/features/dashboard/ web/src/i18n/
git commit -m "feat(web): register usage report dashboard section with i18n"
```

---

### Task 11: 端到端联调 + 后端全量 + PR 准备

**Files:**
- 无新文件（验证任务）

- [ ] **Step 1: 后端全量测试**

```bash
mkdir -p web/dist && touch web/dist/index.html
make test
```
Expected: 根模块与 relaykit 全部 PASS（本功能不碰 relaykit，但 make test 覆盖两模块）

- [ ] **Step 2: 本地联调冒烟**

```bash
make dev-api        # docker 起 PG+Redis+后端 :3000
make dev-web        # 前端 :5173
```
手工验证清单（浏览器）：
1. 管理员登录 → 数据看板出现「使用报告」分节（非管理员不可见）
2. 周报/月报切换、上一期/下一期翻页，周期标签与日期范围正确
3. 有数据时：总览卡环比、趋势图、用户表数值正确；无数据时显示「本期无消费记录」
4. 点「导出 PPT」→ 下载 `usage-report-<label>.pptx`，用 WPS/Office 打开：图表为原生对象（双击可编辑数据）、文字为文本框、Top 15 + 其余汇总行、共 5 页

- [ ] **Step 3: 三库验证记录确认**

确认 Task 4 的验证矩阵（SQLite/MySQL 8.0+5.7/PG 15）记录完整，包含版本、命令、结果。

- [ ] **Step 4: 准备 PR body**

复制 `.agents/github/PR.md` 为 PR body 全文（**不要**用 `.github/PULL_REQUEST_TEMPLATE.md`），填入：
- 变更说明（后端聚合 API + 前端分节 + PPT 导出）
- 三数据库验证矩阵（版本/命令/结果）
- 冒烟验证清单结果
- 当前 git 用户（`git config user.name`）若非仓库历史核心开发者（`git log` 高频作者），在 body 中声明代码为 AI 生成/AI 协助

- [ ] **Step 5: 最终提交（如有冒烟修复）**

```bash
git add -A && git commit -m "fix(web): usage report smoke test fixes"
```

---

## 计划自审记录

- **Spec 覆盖**：spec §5（API/DTO/model）→ Task 1-3；§5.4 跨方言与 §9 三库验证 → Task 4；§6 前端 → Task 5-8、10；§7 PPT → Task 9；§8 错误边界 → Task 3（参数）、Task 2 测试（空数据/上期无数据）、Task 8（空状态）、Task 10（加载/错误态）；§9 检查命令 → 各任务 Step 与 Task 11。无缺口
- **占位符**：Task 2 Step 4 中 `aggregateDays` 的 tokens 占位已在同 Step 明确最终签名与删除要求；其余步骤均含完整代码
- **类型一致性**：`resolvePeriodWindow`/`periodWindow`（Task 1↔2↔4）、`GetUsageReport`（Task 2↔3↔4）、TS `UsageReport*`（Task 5↔6↔7↔8↔9↔10）、`buildReportSlides`/`exportUsageReportPptx`（Task 9↔10）签名一致
