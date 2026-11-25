package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountRequestTokenIgnoresBillingSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalCountToken := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() {
		constant.CountToken = originalCountToken
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o-mini")
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}
	meta := &types.TokenCountMeta{
		TokenType:   types.TokenTypeTokenizer,
		CombineText: "count this prompt even though billing estimation is disabled",
	}

	count, err := CountRequestToken(c, meta, info)
	require.NoError(t, err)
	assert.Greater(t, count, 0)

	estimated, err := EstimateRequestToken(c, meta, info)
	require.NoError(t, err)
	assert.Zero(t, estimated)
}
