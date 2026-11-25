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
	// 2026-08-30 是周日，属于上一周，周一为 2026-08-24
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
