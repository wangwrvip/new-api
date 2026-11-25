package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Live upstream payload shape (probed 2026-09-10):
// TOKENS_LIMIT unit=3 number=5 -> 5-hour rolling window;
// TOKENS_LIMIT unit=6 number=1 -> weekly window; TIME_LIMIT is a
// per-call dimension and must be ignored.
const glmUsageLivePayload = `{"code":200,"msg":"Operation successful","data":{` +
	`"limits":[` +
	`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":4,"nextResetTime":1789009045416},` +
	`{"type":"TOKENS_LIMIT","unit":6,"number":1,"percentage":70,"nextResetTime":1789374107996},` +
	`{"type":"TIME_LIMIT","unit":5,"number":1,"usage":4000,"currentValue":170,"remaining":3830,"percentage":4,"nextResetTime":1790408762998}` +
	`],"level":"max"},"success":true}`

func TestParseGlmUsageResponse(t *testing.T) {
	const dayMs = int64(24 * 3600 * 1000)
	const nowMs = int64(1_700_000_000_000)

	tests := []struct {
		name        string
		body        string
		want        glmUsageData
		wantErrText string
	}{
		{
			name: "live payload classifies 5h and weekly windows and ignores time limit",
			body: glmUsageLivePayload,
			want: glmUsageData{
				Success: true,
				Level:   "MAX",
				FiveH:   &glmUsageSlot{UsedPercent: 4, RemainingPercent: 96, NextResetMs: 1789009045416},
				Weekly:  &glmUsageSlot{UsedPercent: 70, RemainingPercent: 30, NextResetMs: 1789374107996},
			},
		},
		{
			name: "unknown unit falls back to 24h heuristic for slot classification",
			body: `{"success":true,"data":{"level":"pro","limits":[` +
				`{"type":"TOKENS_LIMIT","percentage":10,"nextResetTime":1700000000000}` +
				`,{"type":"TOKENS_LIMIT","percentage":80,"nextResetTime":1700259200000}` +
				`]}}`,
			want: glmUsageData{
				Success: true,
				Level:   "PRO",
				FiveH:   &glmUsageSlot{UsedPercent: 10, RemainingPercent: 90, NextResetMs: nowMs},
				Weekly:  &glmUsageSlot{UsedPercent: 80, RemainingPercent: 20, NextResetMs: nowMs + 3*dayMs},
			},
		},
		{
			name: "weekly unit wins over heuristic regardless of reset distance",
			// Reset is now + 1h (inside the 24h heuristic range): unit 6 must
			// still classify the window as weekly.
			body: `{"success":true,"data":{"limits":[` +
				`{"type":"TOKENS_LIMIT","unit":6,"number":1,"percentage":50,"nextResetTime":1700003600000}` +
				`]}}`,
			want: glmUsageData{
				Success: true,
				Weekly:  &glmUsageSlot{UsedPercent: 50, RemainingPercent: 50, NextResetMs: 1700003600000},
			},
		},
		{
			name: "out of range percentages are clamped to 0-100",
			body: `{"success":true,"data":{"limits":[` +
				`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":120,"nextResetTime":1700000000000},` +
				`{"type":"TOKENS_LIMIT","unit":6,"number":1,"percentage":-5,"nextResetTime":1700259200000}` +
				`]}}`,
			want: glmUsageData{
				Success: true,
				FiveH:   &glmUsageSlot{UsedPercent: 100, RemainingPercent: 0, NextResetMs: nowMs},
				Weekly:  &glmUsageSlot{UsedPercent: 0, RemainingPercent: 100, NextResetMs: nowMs + 3*dayMs},
			},
		},
		{
			name: "first entry wins when several limits map to the same slot",
			body: `{"success":true,"data":{"limits":[` +
				`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":10,"nextResetTime":1700000000000},` +
				`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":99,"nextResetTime":1700001800000}` +
				`]}}`,
			want: glmUsageData{
				Success: true,
				FiveH:   &glmUsageSlot{UsedPercent: 10, RemainingPercent: 90, NextResetMs: nowMs},
			},
		},
		{
			name: "entries without reset time are skipped and empty limits keep nil slots",
			body: `{"success":true,"data":{"level":"lite","limits":[` +
				`{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":10},` +
				`{"type":"TIME_LIMIT","percentage":50,"nextResetTime":1700000000000}` +
				`]}}`,
			want: glmUsageData{Success: true, Level: "LITE"},
		},
		{
			name:        "upstream failure envelope is surfaced as an error",
			body:        `{"success":false,"code":401,"msg":"invalid api key"}`,
			wantErrText: "invalid api key",
		},
		{
			name:        "non json body is surfaced as an error",
			body:        `<html>gateway timeout</html>`,
			wantErrText: "invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseGlmUsageResponse([]byte(test.body), nowMs)

			if test.wantErrText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErrText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}
