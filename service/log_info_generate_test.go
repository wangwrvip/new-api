package service

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTextOtherInfoTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	return ctx
}

// newTextOtherInfoTestRelayInfo returns a RelayInfo whose embedded ChannelMeta
// is initialized — GenerateTextOtherInfo reads promoted fields (IsModelMapped)
// that nil-deref without it.
func newTextOtherInfoTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
}

// TestGenerateTextOtherInfoWritesPreviewsTopLevel verifies the request/output
// previews land at the top level of other (log owner + admin visible), NOT
// under admin_info (which the backend strips for non-admin viewers).
func TestGenerateTextOtherInfoWritesPreviewsTopLevel(t *testing.T) {
	ctx := newTextOtherInfoTestContext()
	relayInfo := newTextOtherInfoTestRelayInfo()
	relayInfo.RequestPreview = "请求文本预览"
	relayInfo.RequestPreviewUser = "用户输入"
	relayInfo.ResponsePreview = "模型输出的第一段内容"

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1).Snapshot()

	require.Equal(t, "请求文本预览", other["request_preview"])
	require.Equal(t, "用户输入", other["request_preview_user"])
	require.Equal(t, "模型输出的第一段内容", other["response_preview"])

	adminInfo, ok := other["admin_info"].(map[string]any)
	require.True(t, ok, "admin_info should exist")
	_, hasReq := adminInfo["request_preview"]
	require.False(t, hasReq, "request_preview must not be duplicated under admin_info")
	_, hasUser := adminInfo["request_preview_user"]
	require.False(t, hasUser, "request_preview_user must not be duplicated under admin_info")
	_, hasResp := adminInfo["response_preview"]
	require.False(t, hasResp, "response_preview must not be duplicated under admin_info")
}

// TestGenerateTextOtherInfoPreviewsRespectPluginMask verifies a request whose
// plugin masked content writes no previews: they are captured before the
// plugins run and may still hold the text the plugin removed. A plugin record
// without masking keeps the previews as before.
func TestGenerateTextOtherInfoPreviewsRespectPluginMask(t *testing.T) {
	tests := []struct {
		name   string
		record []relaycommon.RequestPluginRecord
		hidden bool
	}{
		{
			name:   "masked content hides every preview",
			record: []relaycommon.RequestPluginRecord{{Plugin: "rp-reply", Stage: "reply", Masked: 1}},
			hidden: true,
		},
		{
			name: "record without masking keeps previews",
			record: []relaycommon.RequestPluginRecord{{Plugin: "rp-outbound", Stage: "outbound",
				Changes: []relaycommon.RequestPluginChange{{Op: "add", Path: "temperature"}}}},
			hidden: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTextOtherInfoTestContext()
			relayInfo := newTextOtherInfoTestRelayInfo()
			relayInfo.RequestPreview = "请求文本预览"
			relayInfo.RequestPreviewUser = "用户输入"
			relayInfo.ResponsePreview = "模型输出的第一段内容"
			relayInfo.RequestPluginRecord = tt.record

			other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1).Snapshot()

			for key, want := range map[string]string{
				"request_preview":      "请求文本预览",
				"request_preview_user": "用户输入",
				"response_preview":     "模型输出的第一段内容",
			} {
				got, exists := other[key]
				if tt.hidden {
					require.False(t, exists, "%s must not be written when a plugin masked content", key)
					continue
				}
				require.True(t, exists, "%s should be written when nothing was masked", key)
				require.Equal(t, want, got)
			}
		})
	}
}

// TestGenerateTextOtherInfoWritesEstimatedPromptTokens verifies the local
// input-token estimate lands in the public log scope for billing reconciliation
// against the upstream-reported prompt tokens.
func TestGenerateTextOtherInfoWritesEstimatedPromptTokens(t *testing.T) {
	ctx := newTextOtherInfoTestContext()
	relayInfo := newTextOtherInfoTestRelayInfo()
	relayInfo.SetEstimatePromptTokens(33126)

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1).Snapshot()

	require.Equal(t, 33126, other["estimated_prompt_tokens"])
}

// TestGenerateTextOtherInfoOmitsZeroEstimate verifies no estimate key is
// written when token estimation did not run (CountToken off), so the log never
// claims an estimate of 0.
func TestGenerateTextOtherInfoOmitsZeroEstimate(t *testing.T) {
	ctx := newTextOtherInfoTestContext()
	relayInfo := newTextOtherInfoTestRelayInfo()
	relayInfo.SetEstimatePromptTokens(0)

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1).Snapshot()

	_, exists := other["estimated_prompt_tokens"]
	require.False(t, exists, "estimated_prompt_tokens must not be written when the estimate is 0")
}

// TestGenerateTextOtherInfoOmitsEmptyPreviews verifies no preview keys are
// written when the relay captured no preview text.
func TestGenerateTextOtherInfoOmitsEmptyPreviews(t *testing.T) {
	ctx := newTextOtherInfoTestContext()
	relayInfo := newTextOtherInfoTestRelayInfo()

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1).Snapshot()

	_, hasReq := other["request_preview"]
	require.False(t, hasReq)
	_, hasUser := other["request_preview_user"]
	require.False(t, hasUser)
	_, hasResp := other["response_preview"]
	require.False(t, hasResp)
}
