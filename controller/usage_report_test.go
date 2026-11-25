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
