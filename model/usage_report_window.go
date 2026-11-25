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
