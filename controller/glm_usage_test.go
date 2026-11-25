package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
			wantErrText: "upstream rejected the request: invalid api key",
		},
		{
			name:        "non json body is surfaced as an error",
			body:        `<html>gateway timeout</html>`,
			wantErrText: "upstream response invalid",
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

func setupGlmUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "glm-usage.db")+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func callGlmChannelsUsage(t *testing.T, ids string) map[string]glmUsageChannelResult {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/glm/usage?ids="+ids, nil)

	GetGlmChannelsUsage(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                             `json:"success"`
		Data    map[string]glmUsageChannelResult `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response.Data
}

func TestFetchGlmChannelUsageUsesChannelTransportSettings(t *testing.T) {
	var requests atomic.Int32
	var authHeader atomic.Value
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		authHeader.Store(r.Header.Get("Authorization"))
		if r.URL.Path != glmUsagePath {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(glmUsageLivePayload))
	}))
	defer server.Close()

	// Self-signed httptest certificate: the request only succeeds when the
	// channel's tls_insecure_skip_verify setting reaches the HTTP client.
	setting := `{"tls_insecure_skip_verify":true}`
	channel := &model.Channel{
		Id:      1,
		Type:    constant.ChannelTypeZhipu,
		Key:     "sk-glm-transport-settings",
		BaseURL: &server.URL,
		Setting: &setting,
	}

	result := fetchGlmChannelUsage(context.Background(), channel)

	require.Empty(t, result.Message)
	assert.True(t, result.Success)
	assert.Equal(t, "MAX", result.Level)
	assert.Equal(t, int32(1), requests.Load())
	assert.Equal(t, "sk-glm-transport-settings", authHeader.Load())
}

func TestFetchGlmChannelUsageDistinguishesFailureMessages(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{
			name:    "upstream business rejection",
			body:    `{"success":false,"code":401,"msg":"invalid api key"}`,
			message: "upstream rejected the request",
		},
		{
			name:    "malformed response body",
			body:    `<html>gateway timeout</html>`,
			message: "upstream response invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			channel := &model.Channel{
				Id:      1,
				Type:    constant.ChannelTypeZhipu,
				Key:     "sk-glm-parse-error",
				BaseURL: &server.URL,
			}

			result := fetchGlmChannelUsage(context.Background(), channel)

			assert.False(t, result.Success)
			assert.Equal(t, test.message, result.Message)
		})
	}
}

func TestFetchGlmChannelUsageTransportFailureUsesUpstreamKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	channel := &model.Channel{
		Id:      1,
		Type:    constant.ChannelTypeZhipu,
		Key:     "sk-glm",
		BaseURL: &server.URL,
	}

	result := fetchGlmChannelUsage(context.Background(), channel)

	require.False(t, result.Success)
	assert.Equal(t, "Upstream request failed", result.Message)
}

func TestGetGlmChannelsUsageRejectsMultiKeyChannel(t *testing.T) {
	db := setupGlmUsageTestDB(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(glmUsageLivePayload))
	}))
	defer server.Close()

	channel := &model.Channel{
		Name:        "glm-multi-key",
		Type:        constant.ChannelTypeZhipu,
		Key:         "sk-multi-key-test",
		BaseURL:     &server.URL,
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2},
	}
	require.NoError(t, db.Create(channel).Error)
	id := strconv.Itoa(channel.Id)

	data := callGlmChannelsUsage(t, id)
	result, ok := data[id]
	require.True(t, ok)
	assert.False(t, result.Success)
	assert.Equal(t, "multi-key channel is not supported", result.Message)
	assert.Zero(t, requests.Load(), "multi-key channels must never reach the upstream")
}

func TestGetGlmChannelsUsageRejectsNonZhipuChannel(t *testing.T) {
	db := setupGlmUsageTestDB(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(glmUsageLivePayload))
	}))
	defer server.Close()

	channel := &model.Channel{
		Name:    "glm-openai",
		Type:    constant.ChannelTypeOpenAI,
		Key:     "sk-openai",
		BaseURL: &server.URL,
	}
	require.NoError(t, db.Create(channel).Error)
	id := strconv.Itoa(channel.Id)

	data := callGlmChannelsUsage(t, id)
	result, ok := data[id]
	require.True(t, ok)
	assert.Equal(t, "channel type is not Zhipu", result.Message)
	assert.Zero(t, requests.Load())
}

func TestGetGlmChannelsUsageKeepsFirstFiftyIDsOnly(t *testing.T) {
	db := setupGlmUsageTestDB(t)
	// P4.3 moves the frontend to batches; until then the handler silently
	// truncates at 50, and this test pins that behavior.
	const requestedIDs = 51
	ids := make([]string, 0, requestedIDs)
	for range requestedIDs {
		channel := &model.Channel{Name: "glm-batch-cap", Type: constant.ChannelTypeOpenAI, Key: "sk-openai"}
		require.NoError(t, db.Create(channel).Error)
		ids = append(ids, strconv.Itoa(channel.Id))
	}

	data := callGlmChannelsUsage(t, strings.Join(ids, ","))

	require.Len(t, data, 50)
	for _, id := range ids[:50] {
		assert.Contains(t, data, id)
	}
	assert.NotContains(t, data, ids[requestedIDs-1])
}
