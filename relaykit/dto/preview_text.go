package dto

import (
	"regexp"
	"strings"
)

// Client-injected context blocks that precede the user's actual instruction.
// Claude Code adds them on every turn, so they must not become the log's
// "last user message" preview.
var userPreviewNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<system-reminder>.*?</system-reminder>`),
	regexp.MustCompile(`(?is)<tools-used-before-summary>.*?</tools-used-before-summary>`),
}

// CleanUserPreviewText removes client-injected system reminders from a
// user message before it becomes the log's "last user message" preview.
// It returns an empty string when nothing user-readable remains; callers keep
// the previous user message instead of updating the preview.
func CleanUserPreviewText(text string) string {
	for _, pattern := range userPreviewNoisePatterns {
		text = pattern.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}
