package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanUserPreviewText(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text unchanged", input: "你是什么模型", want: "你是什么模型"},
		{name: "only system reminder", input: "<system-reminder>injected context</system-reminder>", want: ""},
		{name: "reminder then question", input: "<system-reminder>injected\ncontext</system-reminder>\n你是什么模型", want: "你是什么模型"},
		{name: "case insensitive", input: "<SYSTEM-REMINDER>ctx</System-Reminder>", want: ""},
		{name: "multiline reminder", input: "<system-reminder>\nline one\nline two\n</system-reminder>", want: ""},
		{name: "tools used before summary", input: "<tools-used-before-summary>old tools</tools-used-before-summary>continue", want: "continue"},
		{name: "multiple reminders keep middle text", input: "<system-reminder>a</system-reminder>keep<system-reminder>b</system-reminder>", want: "keep"},
		{name: "trims surrounding whitespace", input: "  answer  ", want: "answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CleanUserPreviewText(tc.input))
		})
	}
}
