package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTruncateTextToTokens 验证请求预览截断的边界：
// 空文本/上限为 0、短文本原样返回、超长文本截到约 maxTokens、全文不足时返回全文。
func TestTruncateTextToTokens(t *testing.T) {
	const model = "claude-3-5-sonnet-20241022" // 统一走 cl100k tokenizer，结果确定

	t.Run("empty text or non-positive limit returns empty", func(t *testing.T) {
		assert.Equal(t, "", TruncateTextToTokens("", model, 200))
		assert.Equal(t, "", TruncateTextToTokens("some text", model, 0))
		assert.Equal(t, "", TruncateTextToTokens("some text", model, -1))
	})

	t.Run("short text returned unchanged", func(t *testing.T) {
		// 4 runes <= 200 → 原样返回，不做任何 token 计算
		text := "你好世界"
		assert.Equal(t, text, TruncateTextToTokens(text, model, 200))
	})

	t.Run("long CJK text truncated to about maxTokens", func(t *testing.T) {
		text := strings.Repeat("字", 300) // CJK 每 rune 约 1 token → 300 tokens
		got := TruncateTextToTokens(text, model, 100)
		require.NotEqual(t, text, got)
		assert.True(t, strings.HasPrefix(text, got), "result must be a prefix of input")
		tokens := CountTextToken(got, model)
		assert.GreaterOrEqual(t, tokens, 100, "preview should reach the limit")
		assert.LessOrEqual(t, tokens, 105, "preview should not overshoot much")
	})

	t.Run("text under limit in tokens returns full text even with many runes", func(t *testing.T) {
		// 英文 500 runes 的 token 数远小于 200（约 130），应返回全文
		text := strings.Repeat("abc ", 125) // 500 runes
		assert.Less(t, CountTextToken(text, model), 200)
		assert.Equal(t, text, TruncateTextToTokens(text, model, 200))
	})

	t.Run("long Latin text truncated to about maxTokens", func(t *testing.T) {
		text := strings.Repeat("word ", 600) // 2400 runes，token 数远超 200
		got := TruncateTextToTokens(text, model, 200)
		require.NotEqual(t, text, got)
		assert.True(t, strings.HasPrefix(text, got), "result must be a prefix of input")
		tokens := CountTextToken(got, model)
		assert.GreaterOrEqual(t, tokens, 200)
		assert.LessOrEqual(t, tokens, 210)
	})
}
