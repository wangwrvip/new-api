package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextLimitErrorReturns413WithCode(t *testing.T) {
	err := contextLimitError("glm-4.7", 210001, 210000)

	require.NotNil(t, err)
	assert.Equal(t, http.StatusRequestEntityTooLarge, err.StatusCode)
	assert.Equal(t, types.ErrorCodeContextLimitExceeded, err.GetErrorCode())
	assert.Equal(t, "context_limit_exceeded", string(err.GetErrorCode()))
	assert.True(t, types.IsSkipRetryError(err))
}

func TestContextLimitErrorOtherIncludesRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	other := contextLimitErrorOther(c, relayInfo, 210001, 210000)
	snapshot := other.Snapshot()

	assert.Equal(t, "/v1/chat/completions", snapshot["request_path"])
	assert.Equal(t, types.ErrorTypeNewAPIError, snapshot["error_type"])
	assert.Equal(t, string(types.ErrorCodeContextLimitExceeded), snapshot["error_code"])
	assert.Equal(t, http.StatusRequestEntityTooLarge, snapshot["status_code"])
	assert.Equal(t, 210001, snapshot["estimated_prompt_tokens"])
	assert.Equal(t, 210000, snapshot["context_limit"])
	assert.Equal(t, string(relayInfo.RelayFormat), snapshot["relay_format"])
}

func TestContextLimitErrorOtherKeepsChannelInfoAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("use_channel", []string{"7", "9"})
	relayInfo := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	other := contextLimitErrorOther(c, relayInfo, 210001, 210000)
	snapshot := other.Snapshot()

	assert.NotContains(t, snapshot, "use_channel")
	adminInfo, ok := snapshot["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"7", "9"}, adminInfo["use_channel"])
}
