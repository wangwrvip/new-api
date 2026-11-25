package model

import (
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
)

// userDayAggRow 一天内单用户的聚合行。
type userDayAggRow struct {
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	Requests        int64  `json:"requests"`
	Quota           int64  `json:"quota"`
	Tokens          int64  `json:"tokens"`
	InputTokens     int64  `json:"input_tokens"`
	OutputTokens    int64  `json:"output_tokens"`
	CacheTokens     int64  `json:"cache_tokens"`
	CacheReadTokens int64  `json:"cache_read_tokens"`
}

// sumLogsByUserInRange 查询 [startUTC, endUTC) 内消费日志按用户的聚合。
// 纯参数化条件 + COALESCE/CASE，SQLite/MySQL/PostgreSQL 行为一致。
// Tokens 与维度列使用同一回退口径，与写路径（InputTokens/CacheTokens 均未传时
// 输入回退 PromptTokens、缓存为 0）保持一致：
// 新日志（维度列非 0）= 输入+缓存+输出；旧日志（维度列全 0，升级前写入）
// 输入回退 prompt、缓存为 0，总量回退 prompt+completion。
// 因此恒有 InputTokens + CacheTokens + OutputTokens = Tokens。
func sumLogsByUserInRange(startUTC int64, endUTC int64) ([]userDayAggRow, error) {
	var rows []userDayAggRow
	err := LOG_DB.Table("logs").
		Select("user_id, username, count(*) as requests, COALESCE(sum(quota), 0) as quota, "+
			"COALESCE(sum(CASE WHEN input_tokens > 0 OR cache_tokens > 0 "+
			"THEN input_tokens + cache_tokens + completion_tokens "+
			"ELSE prompt_tokens + completion_tokens END), 0) as tokens, "+
			"COALESCE(sum(CASE WHEN input_tokens > 0 OR cache_tokens > 0 "+
			"THEN input_tokens ELSE prompt_tokens END), 0) as input_tokens, "+
			"COALESCE(sum(completion_tokens), 0) as output_tokens, "+
			"COALESCE(sum(cache_tokens), 0) as cache_tokens, "+
			"COALESCE(sum(cache_read_tokens), 0) as cache_read_tokens").
		Where("type = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, startUTC, endUTC).
		Group("user_id, username").
		Scan(&rows).Error
	if err != nil {
		common.SysError(common.LogText("failed to query usage report rows: %s", err.Error()))
		return nil, common.NewMessage("Failed to query usage report data")
	}
	return rows, nil
}

// periodUserAgg 单用户在一个周期内的累计。
type periodUserAgg struct {
	username        string
	requests        int64
	quota           int64
	tokens          int64
	inputTokens     int64
	outputTokens    int64
	cacheTokens     int64
	cacheReadTokens int64
	activeDays      int
}

// aggregateDays 把逐天聚合行（rowsByDay 与 window.Days 一一对应）累计为周期数据。
// 每个有记录的自然天计入该用户 activeDays：同一天同一用户因改名等原因聚合出
// 多行时只计一天，避免活跃天数超过周期天数；dayUsers 统计当天活跃用户数。
func aggregateDays(window periodWindow, rowsByDay [][]userDayAggRow) (
	daily []dto.UsageReportDaily,
	users map[int]*periodUserAgg,
	totalRequests int64,
	totalQuota int64,
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
			user.inputTokens += row.InputTokens
			user.outputTokens += row.OutputTokens
			user.cacheTokens += row.CacheTokens
			user.cacheReadTokens += row.CacheReadTokens
			if !dayUsers[row.UserId] {
				user.activeDays++
				dayUsers[row.UserId] = true
			}
			item.Requests += row.Requests
			item.Quota += row.Quota
		}
		item.ActiveUsers = int64(len(dayUsers))
		daily = append(daily, item)
		totalRequests += item.Requests
		totalQuota += item.Quota
	}
	return daily, users, totalRequests, totalQuota
}

func sumTokens(users map[int]*periodUserAgg) int64 {
	var total int64
	for _, user := range users {
		total += user.tokens
	}
	return total
}

// userReportIdentity 主库用户的身份信息（登录名 + 显示名 + 部门）。
type userReportIdentity struct {
	username    string
	displayName string
	department  string
}

// normalizeDepartment 部门空值（含纯空白）兜底为默认部门，保证部门统计与展示始终有效。
func normalizeDepartment(name string) string {
	if strings.TrimSpace(name) == "" {
		return setting.DefaultDepartment
	}
	return name
}

// getAllUsers 返回主库全部未删除用户（含从未产生消费记录的用户）。
// 部门为空（如存量数据未回填）时按默认部门兜底，保证部门统计始终有效。
func getAllUsers() (map[int]userReportIdentity, error) {
	allUsers := make(map[int]userReportIdentity)
	var rows []struct {
		Id          int
		Username    string
		DisplayName string
		Department  string
	}
	if err := DB.Model(&User{}).Select("id, username, display_name, department").Scan(&rows).Error; err != nil {
		common.SysError(common.LogText("failed to query usage report users: %s", err.Error()))
		return nil, common.NewMessage("Failed to query usage report users")
	}
	for _, row := range rows {
		allUsers[row.Id] = userReportIdentity{username: row.Username, displayName: row.DisplayName, department: normalizeDepartment(row.Department)}
	}
	return allUsers, nil
}

// aggregateDepartmentStats 按部门统计总用户、本期/上期活跃用户与活跃率。
// 总用户来自主库 users 表（allUsers）；活跃用户为周期内有消费记录的、仍存在于 users 表中的用户。
// 日志中已从 users 表删除的用户无部门信息，不参与部门统计，避免活跃数超过总用户数。
// 结果按本期活跃用户数降序，再按上期活跃数降序、部门名升序。
func aggregateDepartmentStats(curUsers, prevUsers map[int]*periodUserAgg, allUsers map[int]userReportIdentity) []dto.UsageReportDepartment {
	type deptStat struct {
		total      int
		active     int
		prevActive int
	}
	stats := make(map[string]*deptStat)
	for _, identity := range allUsers {
		department := normalizeDepartment(identity.department)
		stat := stats[department]
		if stat == nil {
			stat = &deptStat{}
			stats[department] = stat
		}
		stat.total++
	}
	for userId := range curUsers {
		if identity, ok := allUsers[userId]; ok {
			stats[normalizeDepartment(identity.department)].active++
		}
	}
	for userId := range prevUsers {
		if identity, ok := allUsers[userId]; ok {
			stats[normalizeDepartment(identity.department)].prevActive++
		}
	}
	departments := make([]dto.UsageReportDepartment, 0, len(stats))
	for name, stat := range stats {
		rate := 0.0
		if stat.total > 0 {
			rate = float64(stat.active) / float64(stat.total)
		}
		departments = append(departments, dto.UsageReportDepartment{
			Department:      name,
			TotalUsers:      stat.total,
			ActiveUsers:     stat.active,
			PrevActiveUsers: stat.prevActive,
			ActiveRate:      rate,
		})
	}
	sort.Slice(departments, func(i, j int) bool {
		if departments[i].ActiveUsers != departments[j].ActiveUsers {
			return departments[i].ActiveUsers > departments[j].ActiveUsers
		}
		if departments[i].PrevActiveUsers != departments[j].PrevActiveUsers {
			return departments[i].PrevActiveUsers > departments[j].PrevActiveUsers
		}
		return departments[i].Department < departments[j].Department
	})
	return departments
}

// assembleUsageReport 组装最终报告（纯函数，便于单测）。
// Users 为「全量用户」：本期/上期活跃者、仅上期活跃者（本期各值为 0，反映流失）、
// 以及从未有访问数据但仍在 users 表中的用户（各值为 0），按 UserId 去重合并；
// 日志中已从 users 表删除（软删）的用户仍保留，避免丢失历史数据。
// DisplayName 取自主库显示名（未设置或已软删的用户为空，前端回退登录名）。
// Users 按本期 Quota（费用）降序，同费用按 Username 升序。
func assembleUsageReport(period string, curWindow, prevWindow periodWindow, curRows, prevRows [][]userDayAggRow, allUsers map[int]userReportIdentity) *dto.UsageReport {
	curDaily, curUsers, curRequests, curQuota := aggregateDays(curWindow, curRows)
	_, prevUsers, prevRequests, prevQuota := aggregateDays(prevWindow, prevRows)

	report := &dto.UsageReport{
		Period:   period,
		Current:  dto.UsageReportPeriodInfo{Label: curWindow.Label, StartDate: curWindow.StartDate, EndDate: curWindow.EndDate},
		Previous: dto.UsageReportPeriodInfo{Label: prevWindow.Label, StartDate: prevWindow.StartDate, EndDate: prevWindow.EndDate},
		Overview: dto.UsageReportOverview{
			Requests:        curRequests,
			PrevRequests:    prevRequests,
			Quota:           curQuota,
			PrevQuota:       prevQuota,
			Tokens:          sumTokens(curUsers),
			PrevTokens:      sumTokens(prevUsers),
			ActiveUsers:     int64(len(curUsers)),
			PrevActiveUsers: int64(len(prevUsers)),
		},
		Daily: curDaily,
		Users: make([]dto.UsageReportUser, 0, len(curUsers)+len(prevUsers)+len(allUsers)),
	}
	report.Departments = aggregateDepartmentStats(curUsers, prevUsers, allUsers)

	seen := make(map[int]bool, len(curUsers)+len(prevUsers)+len(allUsers))
	for userId, user := range curUsers {
		seen[userId] = true
		entry := dto.UsageReportUser{
			UserId:          userId,
			Username:        user.username,
			DisplayName:     allUsers[userId].displayName,
			Department:      allUsers[userId].department,
			Requests:        user.requests,
			ActiveDays:      user.activeDays,
			Quota:           user.quota,
			Tokens:          user.tokens,
			InputTokens:     user.inputTokens,
			OutputTokens:    user.outputTokens,
			CacheTokens:     user.cacheTokens,
			CacheReadTokens: user.cacheReadTokens,
		}
		if prevUser := prevUsers[userId]; prevUser != nil {
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
		seen[userId] = true
		report.Users = append(report.Users, dto.UsageReportUser{
			UserId:         userId,
			Username:       user.username,
			DisplayName:    allUsers[userId].displayName,
			Department:     allUsers[userId].department,
			PrevRequests:   user.requests,
			PrevActiveDays: user.activeDays,
			PrevQuota:      user.quota,
			PrevTokens:     user.tokens,
		})
	}
	// 补齐从未产生访问数据的用户（各列保持 0）
	for userId, identity := range allUsers {
		if seen[userId] {
			continue
		}
		seen[userId] = true
		report.Users = append(report.Users, dto.UsageReportUser{
			UserId:      userId,
			Username:    identity.username,
			DisplayName: identity.displayName,
			Department:  identity.department,
		})
	}

	sort.Slice(report.Users, func(i, j int) bool {
		if report.Users[i].Quota != report.Users[j].Quota {
			return report.Users[i].Quota > report.Users[j].Quota
		}
		return report.Users[i].Username < report.Users[j].Username
	})
	return report
}

// GetUsageReport 生成周/月使用报告。anchor 为任意瞬时，tzOffsetMin 为报告时区偏移（分钟）。
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
	allUsers, err := getAllUsers()
	if err != nil {
		return nil, err
	}
	return assembleUsageReport(period, curWindow, prevWindow, curRows, prevRows, allUsers), nil
}
