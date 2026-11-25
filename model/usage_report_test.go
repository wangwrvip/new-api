package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
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

	allUsers := map[int]userReportIdentity{
		1: {username: "alice", displayName: "Alice A", department: "公共部门"},
		2: {username: "bob", department: "公共部门"},
		3: {username: "gone", department: "公共部门"},
	}

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows, allUsers)

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

	// 用户：按本期 Quota（费用）降序；仅上期活跃的 gone 以 0 保留
	require.Len(t, report.Users, 3)
	assert.Equal(t, "alice", report.Users[0].Username)
	assert.Equal(t, "Alice A", report.Users[0].DisplayName)
	assert.Equal(t, int64(5), report.Users[0].Requests)
	assert.Equal(t, 2, report.Users[0].ActiveDays)
	assert.Equal(t, int64(5), report.Users[0].PrevRequests)
	assert.Equal(t, 1, report.Users[0].PrevActiveDays)
	assert.Equal(t, "bob", report.Users[1].Username)
	assert.Equal(t, int64(1), report.Users[1].Requests)
	// bob 未设置显示名，DisplayName 为空
	assert.Empty(t, report.Users[1].DisplayName)
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

	report := assembleUsageReport("week", curWindow, prevWindow, emptyCur, emptyPrev, nil)
	assert.Equal(t, int64(0), report.Overview.Requests)
	assert.Len(t, report.Daily, 7)
	assert.Empty(t, report.Users)
}

func TestAssembleUsageReportIncludesAllUsers(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)
	// 本期 alice 有访问；上期 bob 有访问；ghost 已从 users 表删除（不在 allUsers）但上期有日志
	curRows := make([][]userDayAggRow, 7)
	prevRows := make([][]userDayAggRow, 7)
	curRows[0] = []userDayAggRow{{UserId: 1, Username: "alice", Requests: 3, Quota: 300, Tokens: 30}}
	prevRows[0] = []userDayAggRow{
		{UserId: 2, Username: "bob", Requests: 1, Quota: 100, Tokens: 10},
		{UserId: 99, Username: "ghost", Requests: 1, Quota: 100, Tokens: 10},
	}
	allUsers := map[int]userReportIdentity{
		1: {username: "alice", displayName: "Alice A", department: "公共部门"},
		2: {username: "bob", department: "公共部门"},
		3: {username: "charlie", displayName: "Charlie C", department: "公共部门"},
		4: {username: "dee", department: "公共部门"},
	}

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows, allUsers)

	// 有访问用户在前，无访问用户补 0 后按 username 升序
	require.Len(t, report.Users, 5)
	assert.Equal(t, "alice", report.Users[0].Username)
	assert.Equal(t, "Alice A", report.Users[0].DisplayName)
	assert.Equal(t, int64(3), report.Users[0].Requests)
	// 上期有访问、本期为 0 的 bob 保留
	assert.Equal(t, "bob", report.Users[1].Username)
	assert.Equal(t, int64(0), report.Users[1].Requests)
	assert.Equal(t, int64(1), report.Users[1].PrevRequests)
	// 从未有访问数据的用户补 0，带显示名
	assert.Equal(t, "charlie", report.Users[2].Username)
	assert.Equal(t, "Charlie C", report.Users[2].DisplayName)
	assert.Equal(t, int64(0), report.Users[2].Requests)
	assert.Equal(t, 0, report.Users[2].ActiveDays)
	assert.Equal(t, int64(0), report.Users[2].Quota)
	assert.Equal(t, int64(0), report.Users[2].Tokens)
	assert.Zero(t, report.Users[2].PrevRequests)
	assert.Zero(t, report.Users[2].PrevActiveDays)
	assert.Equal(t, "dee", report.Users[3].Username)
	assert.Empty(t, report.Users[3].DisplayName)
	// 已从 users 表删除、仅日志可见的用户仍保留（不丢失历史数据），无显示名
	assert.Equal(t, "ghost", report.Users[4].Username)
	assert.Empty(t, report.Users[4].DisplayName)
	assert.Equal(t, int64(0), report.Users[4].Requests)
	assert.Equal(t, int64(1), report.Users[4].PrevRequests)

	// 活跃用户总数仅统计有访问的用户
	assert.Equal(t, int64(1), report.Overview.ActiveUsers)
}

func TestAssembleUsageReportSortStableByUsername(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)
	// 两个用户本期都 0 费用，按 username 升序
	curRows := make([][]userDayAggRow, 7)
	prevRows := make([][]userDayAggRow, 7)
	prevRows[0] = []userDayAggRow{
		{UserId: 2, Username: "zoe", Requests: 1, Quota: 10, Tokens: 1},
		{UserId: 1, Username: "adam", Requests: 1, Quota: 10, Tokens: 1},
	}

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows, nil)
	require.Len(t, report.Users, 2)
	assert.Equal(t, "adam", report.Users[0].Username)
	assert.Equal(t, "zoe", report.Users[1].Username)
}

func TestAssembleUsageReportSortsUsersByQuota(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)
	// dave 请求数少但费用高，应排在请求数多、费用低的 carol 之前
	curRows := make([][]userDayAggRow, 7)
	curRows[0] = []userDayAggRow{
		{UserId: 1, Username: "carol", Requests: 10, Quota: 100, Tokens: 100},
		{UserId: 2, Username: "dave", Requests: 2, Quota: 5000, Tokens: 20},
	}
	prevRows := make([][]userDayAggRow, 7)

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows, nil)
	require.Len(t, report.Users, 2)
	assert.Equal(t, "dave", report.Users[0].Username)
	assert.Equal(t, "carol", report.Users[1].Username)
}

func TestAssembleUsageReportDepartments(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)
	// 本期：视频部 alice、平台部 dana 活跃；上期：视频部 bob、测试组 carol 活跃；
	// ghost 仅上期日志、已从 users 表删除（不在 allUsers），不参与部门统计；
	// erin 未设置部门（空值）与 fred 归属默认部门。
	curRows := make([][]userDayAggRow, 7)
	prevRows := make([][]userDayAggRow, 7)
	curRows[0] = []userDayAggRow{
		{UserId: 1, Username: "alice", Requests: 3, Quota: 300, Tokens: 30},
		{UserId: 4, Username: "dana", Requests: 1, Quota: 100, Tokens: 10},
	}
	prevRows[0] = []userDayAggRow{
		{UserId: 2, Username: "bob", Requests: 1, Quota: 100, Tokens: 10},
		{UserId: 3, Username: "carol", Requests: 1, Quota: 100, Tokens: 10},
		{UserId: 99, Username: "ghost", Requests: 1, Quota: 100, Tokens: 10},
	}
	allUsers := map[int]userReportIdentity{
		1: {username: "alice", department: "视频部"},
		2: {username: "bob", department: "视频部"},
		3: {username: "carol", department: "测试组"},
		4: {username: "dana", department: "平台部"},
		5: {username: "erin", department: "公共部门"}, // 经 getAllUsers 兜底后的空部门用户
		6: {username: "fred", department: "公共部门"},
	}

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows, allUsers)

	// 排序：本期活跃降序 → 上期活跃降序 → 部门名升序
	// 视频部(active1,prev1) / 平台部(active1,prev0) / 测试组(active0,prev1) / 公共部门(active0,prev0)
	require.Len(t, report.Departments, 4)
	video := report.Departments[0]
	assert.Equal(t, "视频部", video.Department)
	assert.Equal(t, 2, video.TotalUsers)
	assert.Equal(t, 1, video.ActiveUsers)
	assert.Equal(t, 1, video.PrevActiveUsers)
	assert.InDelta(t, 0.5, video.ActiveRate, 1e-9)

	platform := report.Departments[1]
	assert.Equal(t, "平台部", platform.Department)
	assert.Equal(t, 1, platform.TotalUsers)
	assert.Equal(t, 1, platform.ActiveUsers)
	assert.Equal(t, 0, platform.PrevActiveUsers)
	assert.InDelta(t, 1.0, platform.ActiveRate, 1e-9)

	testGroup := report.Departments[2]
	assert.Equal(t, "测试组", testGroup.Department)
	assert.Equal(t, 1, testGroup.TotalUsers)
	assert.Equal(t, 0, testGroup.ActiveUsers)
	assert.Equal(t, 1, testGroup.PrevActiveUsers)
	assert.Zero(t, testGroup.ActiveRate)

	// 默认部门：erin（空部门兜底）+ fred
	defaultDept := report.Departments[3]
	assert.Equal(t, "公共部门", defaultDept.Department)
	assert.Equal(t, 2, defaultDept.TotalUsers)
	assert.Zero(t, defaultDept.ActiveUsers)
	assert.Zero(t, defaultDept.PrevActiveUsers)

	// 用户条目带部门；软删用户（不在 users 表）部门为空。
	// 排序按本期 Quota 降序：alice(300) dana(100) 之后为 0 费用用户按 username 升序 bob/carol/erin/fred
	assert.Equal(t, "视频部", report.Users[0].Department)
	assert.Equal(t, "平台部", report.Users[1].Department)
	assert.Equal(t, "视频部", report.Users[2].Department)  // bob
	assert.Equal(t, "测试组", report.Users[3].Department)  // carol
	assert.Equal(t, "公共部门", report.Users[4].Department) // erin
	assert.Equal(t, "公共部门", report.Users[5].Department) // fred
	ghostEntry := findUserEntry(report.Users, "ghost")
	require.NotNil(t, ghostEntry)
	assert.Empty(t, ghostEntry.Department)
}

func findUserEntry(users []dto.UsageReportUser, username string) *dto.UsageReportUser {
	for i := range users {
		if users[i].Username == username {
			return &users[i]
		}
	}
	return nil
}

func TestNormalizeDepartment(t *testing.T) {
	assert.Equal(t, "公共部门", normalizeDepartment(""))
	assert.Equal(t, "公共部门", normalizeDepartment(" "))
	assert.Equal(t, "视频部", normalizeDepartment("视频部"))
}

// TestAggregateDepartmentStatsEmptyFallback 直接验证聚合对空部门（跳过 getAllUsers 兜底）的防御处理。
func TestAggregateDepartmentStatsEmptyFallback(t *testing.T) {
	curUsers := map[int]*periodUserAgg{
		1: {username: "a"},
	}
	prevUsers := map[int]*periodUserAgg{
		2: {username: "b"},
	}
	// 用户 1 空部门、用户 2 指定部门
	allUsers := map[int]userReportIdentity{
		1: {username: "a"},
		2: {username: "b", department: "测试组"},
	}

	stats := aggregateDepartmentStats(curUsers, prevUsers, allUsers)
	require.Len(t, stats, 2)
	// 空部门兜底为默认部门：用户 1 本期活跃计入默认部门；上期活跃用户 2 属测试组
	var defaultDept, testGroup *dto.UsageReportDepartment
	for i := range stats {
		switch stats[i].Department {
		case "公共部门":
			defaultDept = &stats[i]
		case "测试组":
			testGroup = &stats[i]
		}
	}
	require.NotNil(t, defaultDept)
	assert.Equal(t, 1, defaultDept.TotalUsers)
	assert.Equal(t, 1, defaultDept.ActiveUsers)
	assert.Zero(t, defaultDept.PrevActiveUsers)
	require.NotNil(t, testGroup)
	assert.Equal(t, 1, testGroup.TotalUsers)
	assert.Zero(t, testGroup.ActiveUsers)
	assert.Equal(t, 1, testGroup.PrevActiveUsers)
}

// 用户维度 token 四分：输入/输出/缓存在周期内逐天累加。
func TestAssembleUsageReportTokenDimensions(t *testing.T) {
	curWindow := usageReportTestWindow(t, false)
	prevWindow := usageReportTestWindow(t, true)

	curRows := [][]userDayAggRow{
		{{UserId: 1, Username: "alice", Requests: 3, Quota: 300, Tokens: 1350,
			InputTokens: 1000, OutputTokens: 200, CacheTokens: 150, CacheReadTokens: 120}},
		{{UserId: 1, Username: "alice", Requests: 2, Quota: 200, Tokens: 120,
			InputTokens: 40, OutputTokens: 10, CacheTokens: 70, CacheReadTokens: 30}},
		nil, nil, nil, nil, nil,
	}
	prevRows := make([][]userDayAggRow, 7)

	report := assembleUsageReport("week", curWindow, prevWindow, curRows, prevRows, nil)

	require.Len(t, report.Users, 1)
	user := report.Users[0]
	assert.Equal(t, int64(1040), user.InputTokens)
	assert.Equal(t, int64(210), user.OutputTokens)
	assert.Equal(t, int64(220), user.CacheTokens)
	assert.Equal(t, int64(150), user.CacheReadTokens)
	assert.Equal(t, int64(1470), user.Tokens)
	// 总览 Tokens 来自用户累加，口径一致
	assert.Equal(t, int64(1470), report.Overview.Tokens)
}

// sumLogsByUserInRange 的总量公式：新日志（有维度列）= 输入+缓存+输出（统一含缓存），
// 旧日志（维度全 0）回退 prompt+completion。
func TestSumLogsByUserInRangeTokenDimensions(t *testing.T) {
	truncateTables(t)
	dayStart := int64(1700000000)
	dayEnd := dayStart + 86400

	rows := []Log{
		// 旧日志：无维度，回退旧口径
		{UserId: 1, Username: "alice", Type: LogTypeConsume, CreatedAt: dayStart + 100,
			PromptTokens: 100, CompletionTokens: 20},
		// 新日志（Claude 渠道）：prompt 不含缓存，总量 = 输入+缓存+输出；
		// 缓存 150 = 读 100 + 写 50
		{UserId: 1, Username: "alice", Type: LogTypeConsume, CreatedAt: dayStart + 200,
			PromptTokens: 1000, CompletionTokens: 200, InputTokens: 850, CacheTokens: 150,
			CacheReadTokens: 100},
		// 新日志（OpenAI 渠道）：prompt 含缓存，公式结果与 prompt+completion 一致；
		// OpenAI 语义缓存全为读
		{UserId: 1, Username: "alice", Type: LogTypeConsume, CreatedAt: dayStart + 300,
			PromptTokens: 2604, CompletionTokens: 383, InputTokens: 172, CacheTokens: 2432,
			CacheReadTokens: 2432},
	}
	for i := range rows {
		require.NoError(t, DB.Create(&rows[i]).Error)
	}

	agg, err := sumLogsByUserInRange(dayStart, dayEnd)
	require.NoError(t, err)
	require.Len(t, agg, 1)

	// 旧 120 + 新 1200 + 新 2987
	assert.Equal(t, int64(120+1200+2987), agg[0].Tokens)
	assert.Equal(t, int64(850+172), agg[0].InputTokens)
	assert.Equal(t, int64(20+200+383), agg[0].OutputTokens)
	assert.Equal(t, int64(150+2432), agg[0].CacheTokens)
	assert.Equal(t, int64(100+2432), agg[0].CacheReadTokens)
}
