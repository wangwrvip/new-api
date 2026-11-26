package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResponsePreviewTestContext(t *testing.T, stream bool) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	previousLogConsume := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		constant.StreamingTimeout = previousTimeout
		common.LogConsumeEnabled = previousLogConsume
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName:     "test-model",
		RelayFormat:         types.RelayFormatOpenAI,
		RelayMode:           relayconstant.RelayModeChatCompletions,
		IsStream:            stream,
		DisablePing:         true,
		StartTime:           time.Now(),
		ChannelMeta:         &relaycommon.ChannelMeta{UpstreamModelName: "test-model"},
		ThinkingContentInfo: relaycommon.ThinkingContentInfo{IsFirstThinkingContent: true},
	}
	return c, info
}

func previewTestResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestOaiStreamResponsePreviewPrefersVisibleContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "reasoning first then content",
			body: "data: " + `{"id":"c1","model":"m","choices":[{"index":0,"delta":{"reasoning_content":"thinking"}}]}` + "\n\n" +
				"data: " + `{"id":"c1","model":"m","choices":[{"index":0,"delta":{"content":"visible"}}]}` + "\n\n" +
				"data: " + `{"id":"c1","choices":[],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}` + "\n\n" +
				"data: [DONE]\n\n",
			want: "visible",
		},
		{
			name: "reasoning only falls back",
			body: "data: " + `{"id":"c1","model":"m","choices":[{"index":0,"delta":{"reasoning_content":"thinking"}}]}` + "\n\n" +
				"data: " + `{"id":"c1","choices":[],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}` + "\n\n" +
				"data: [DONE]\n\n",
			want: "thinking",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info := newResponsePreviewTestContext(t, true)
			usage, apiErr := OaiStreamHandler(c, info, previewTestResponse(tc.body))
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, tc.want, info.ResponsePreview)
		})
	}
}

func TestOpenaiHandlerResponsePreviewPrefersVisibleContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "content and reasoning",
			body: `{"id":"c1","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"visible","reasoning_content":"thinking"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
			want: "visible",
		},
		{
			name: "reasoning only falls back",
			body: `{"id":"c1","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"","reasoning_content":"thinking"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
			want: "thinking",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, info := newResponsePreviewTestContext(t, false)
			usage, apiErr := OpenaiHandler(c, info, previewTestResponse(tc.body))
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, tc.want, info.ResponsePreview)
		})
	}
}
