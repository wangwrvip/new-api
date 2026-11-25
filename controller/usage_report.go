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
