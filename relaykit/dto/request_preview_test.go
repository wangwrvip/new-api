package dto

import (
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 验证日志请求预览的 LastUserMessage：取最后一条 user 消息，多文本段换行连接。

func TestGeneralOpenAIRequestLastUserMessage(t *testing.T) {
	raw := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "system prompt"},
			{"role": "user", "content": "first question"},
			{"role": "assistant", "content": "first answer"},
			{"role": "user", "content": "你是什么模型"}
		]
	}`)
	var req GeneralOpenAIRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
	assert.Contains(t, meta.CombineText, "system prompt")
}

func TestGeneralOpenAIRequestLastUserMessageMultipleSegments(t *testing.T) {
	raw := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "part one"}, {"type": "text", "text": "part two"}]}
		]
	}`)
	var req GeneralOpenAIRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "part one\npart two", meta.LastUserMessage)
}

func TestClaudeRequestLastUserMessage(t *testing.T) {
	// 字符串内容 + 数组内容混合，最后一条 user 生效。
	raw := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"system": "system prompt",
		"messages": [
			{"role": "user", "content": "first question"},
			{"role": "assistant", "content": [{"type": "text", "text": "first answer"}]},
			{"role": "user", "content": [{"type": "text", "text": "最后一条"}, {"type": "text", "text": "问题"}]}
		]
	}`)
	var req ClaudeRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "最后一条\n问题", meta.LastUserMessage)
}

func TestClaudeRequestNoUserMessage(t *testing.T) {
	raw := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"system": "system prompt",
		"messages": [
			{"role": "assistant", "content": "hello"}
		]
	}`)
	var req ClaudeRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "", meta.LastUserMessage)
}

func TestOpenAIResponsesRequestLastUserMessage(t *testing.T) {
	raw := []byte(`{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "你是什么模型"}]}
		]
	}`)
	var req OpenAIResponsesRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
}

func TestOpenAIResponsesRequestStringInput(t *testing.T) {
	raw := []byte(`{"model": "gpt-4o", "input": "你是什么模型"}`)
	var req OpenAIResponsesRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
}

// A trailing user turn that only carries a client-injected system reminder must
// not replace the last real user message in the preview, while the reminder
// still counts toward the prompt estimate.
func TestGeneralOpenAIRequestLastUserMessageSkipsSystemReminder(t *testing.T) {
	raw := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "你是什么模型"},
			{"role": "assistant", "content": "我是 AI"},
			{"role": "user", "content": "<system-reminder>injected context</system-reminder>"}
		]
	}`)
	var req GeneralOpenAIRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
	assert.Contains(t, meta.CombineText, "injected context")
}

func TestGeneralOpenAIRequestLastUserMessageKeepsTextAfterSystemReminder(t *testing.T) {
	raw := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "<system-reminder>injected\ncontext</system-reminder>"},
				{"type": "text", "text": "你是什么模型"}
			]}
		]
	}`)
	var req GeneralOpenAIRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
}

func TestClaudeRequestLastUserMessageSkipsSystemReminder(t *testing.T) {
	raw := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "user", "content": "你是什么模型"},
			{"role": "assistant", "content": [{"type": "text", "text": "我是 AI"}]},
			{"role": "user", "content": [{"type": "text", "text": "<system-reminder>injected context</system-reminder>"}]}
		]
	}`)
	var req ClaudeRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
	assert.Contains(t, meta.CombineText, "injected context")
}

func TestClaudeRequestLastUserMessageKeepsTextAfterSystemReminder(t *testing.T) {
	raw := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "user", "content": "<system-reminder>injected\ncontext</system-reminder>\n你是什么模型"}
		]
	}`)
	var req ClaudeRequest
	require.NoError(t, kitutil.Unmarshal(raw, &req))

	meta := req.GetTokenCountMeta()
	assert.Equal(t, "你是什么模型", meta.LastUserMessage)
}
