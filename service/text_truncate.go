package service

import "github.com/QuantumNous/new-api/tokenkit"

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
// Text already under the limit is returned unchanged.
func TruncateTextToTokens(text string, model string, maxTokens int) string {
	if text == "" || maxTokens <= 0 {
		return ""
	}
	runes := []rune(text)
	// At worst one rune is about one token (CJK), so a rune count under the
	// limit is always under the token limit.
	if len(runes) <= maxTokens {
		return text
	}
	// Common text is about 3-5 runes per token; 8x covers the densest forms
	// (base64 is about 1.5-2) and avoids tokenizing a huge request body over
	// and over during the search.
	upper := len(runes)
	if maxTokens*8 < upper {
		upper = maxTokens * 8
	}
	// Whitespace-heavy text can exceed 8 runes per token; when the fast bound
	// still has too few tokens, search the whole text instead.
	if CountTextToken(string(runes[:upper]), model) < maxTokens {
		upper = len(runes)
	}
	// Find the smallest rune prefix whose token count reaches maxTokens; when
	// the full text is under the limit this converges to the full text.
	lo, hi := 1, upper
	for lo < hi {
		mid := (lo + hi) / 2
		if CountTextToken(string(runes[:mid]), model) >= maxTokens {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return string(runes[:lo])
}
