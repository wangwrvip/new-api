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
