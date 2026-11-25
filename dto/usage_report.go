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
	DisplayName    string `json:"display_name"` // 主库显示名；未设置或日志中已删除的用户为空
	Department     string `json:"department"`   // 主库部门；已从 users 表删除的用户为空
	Requests       int64  `json:"requests"`
	PrevRequests   int64  `json:"prev_requests"`
	ActiveDays     int    `json:"active_days"`
	PrevActiveDays int    `json:"prev_active_days"`
	Quota          int64  `json:"quota"`
	PrevQuota      int64  `json:"prev_quota"`
	Tokens         int64  `json:"tokens"`
	PrevTokens     int64  `json:"prev_tokens"`
	// 归一化 token 维度（本期）：非缓存输入 / 输出 / 缓存读+写合计；
	// 三者之和等于 Tokens 的统一含缓存口径。旧日志维度为 0。
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheTokens  int64 `json:"cache_tokens"`
	// 缓存命中的读分量：命中率 = CacheReadTokens / (InputTokens + CacheTokens)。旧日志为 0。
	CacheReadTokens int64 `json:"cache_read_tokens"`
}

// UsageReportDepartment 单个部门的活跃统计。ActiveRate 为本期活跃用户 / 总用户（0..1）。
// 只统计仍在 users 表中的用户（日志中已删除的用户无部门信息，不参与部门统计）。
type UsageReportDepartment struct {
	Department      string  `json:"department"`
	TotalUsers      int     `json:"total_users"`
	ActiveUsers     int     `json:"active_users"`
	PrevActiveUsers int     `json:"prev_active_users"`
	ActiveRate      float64 `json:"active_rate"`
}

type UsageReport struct {
	Period      string                  `json:"period"`
	Current     UsageReportPeriodInfo   `json:"current"`
	Previous    UsageReportPeriodInfo   `json:"previous"`
	Overview    UsageReportOverview     `json:"overview"`
	Daily       []UsageReportDaily      `json:"daily"`
	Departments []UsageReportDepartment `json:"departments"`
	Users       []UsageReportUser       `json:"users"`
}
