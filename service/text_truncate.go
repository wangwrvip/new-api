package service

import "github.com/QuantumNous/new-api/tokenkit"

// previewHardCapRunes bounds a preview no matter what the token estimate says.
// Some families (tokenkit's gemini weights) count ordinary whitespace as zero
// tokens, so a whitespace-heavy request estimates to zero and a token budget
// alone would let the whole request body through. The cap also bounds the rune
// conversion and the binary search for a large body. 8192 runes covers the
// maximum configurable 2000-token window at an ordinary 4 runes per token and
// keeps an ASCII preview within 8 KiB.
const previewHardCapRunes = 8192

// previewRunesPerToken sizes the search window. Common text is 3-5 runes per
// token; 8x also covers base64 (about 1.5-2).
const previewRunesPerToken = 8

// CountTextToken counts the tokens of text for model. Every model goes through
// tokenkit: OpenAI GPT/o-series names use tiktoken, other models use the
// calibrated weighted estimate. Keeping a single counting path keeps the
// request preview consistent with the prompt estimate.
func CountTextToken(text string, model string) int {
	return tokenkit.Count(model, text)
}

// TruncateTextToTokens returns the prefix of text that stays within maxTokens
// tokens, for the consume log's request preview. It binary-searches rune
// prefixes with CountTextToken, so multi-byte text is never cut mid-character.
// Text already under the token limit is returned unchanged, unless it exceeds
// the previewHardCapRunes hard cap (the estimate can be zero for whitespace).
func TruncateTextToTokens(text string, model string, maxTokens int) string {
	if text == "" || maxTokens <= 0 {
		return ""
	}
	// Callers clamp maxTokens to 2000, but the helper must not let maxTokens*8
	// overflow or promise more runes than the hard cap on its own.
	if maxTokens > previewHardCapRunes {
		maxTokens = previewHardCapRunes
	}
	// The result never exceeds this many runes, so the search never needs more.
	limit := min(maxTokens*previewRunesPerToken, previewHardCapRunes)
	// A rune is at most 4 bytes, so the first 4*limit bytes hold at least limit
	// runes. Converting just that prefix keeps a huge request body from
	// allocating its full []rune; a trailing partial rune lies beyond limit and
	// is never returned.
	if len(text) > 4*limit {
		text = text[:4*limit]
	}
	runes := []rune(text)
	if len(runes) <= maxTokens {
		return text
	}
	// Find the smallest rune prefix whose token count reaches maxTokens. When
	// the estimator never reaches it (zero-weight whitespace), the search
	// converges to upper, which limit caps instead of expanding to the full
	// text.
	upper := min(len(runes), limit)
	lo, hi := 1, upper
	for lo < hi {
		mid := (lo + hi) / 2
		if CountTextToken(string(runes[:mid]), model) >= maxTokens {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return string(runes[:min(lo, previewHardCapRunes)])
}
