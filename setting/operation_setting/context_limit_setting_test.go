package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaxPromptTokensForModel(t *testing.T) {
	tests := []struct {
		name    string
		setting ContextLimitSetting
		model   string
		want    int
	}{
		{
			name: "disabled returns no limit even when configured",
			setting: ContextLimitSetting{
				Enabled:                false,
				DefaultMaxPromptTokens: 1000,
				ModelMaxPromptTokens:   map[string]int{"glm-4.7": 500},
			},
			model: "glm-4.7",
			want:  0,
		},
		{
			name: "exact model match returns per-model limit",
			setting: ContextLimitSetting{
				Enabled:              true,
				ModelMaxPromptTokens: map[string]int{"glm-4.7": 210000},
			},
			model: "glm-4.7",
			want:  210000,
		},
		{
			name: "model match is case-insensitive",
			setting: ContextLimitSetting{
				Enabled:              true,
				ModelMaxPromptTokens: map[string]int{"GLM-4.7": 210000},
			},
			model: "glm-4.7",
			want:  210000,
		},
		{
			name: "unmatched model falls back to default limit",
			setting: ContextLimitSetting{
				Enabled:                true,
				DefaultMaxPromptTokens: 128000,
				ModelMaxPromptTokens:   map[string]int{"glm-4.7": 210000},
			},
			model: "claude-sonnet-5",
			want:  128000,
		},
		{
			name: "zero default means unlimited",
			setting: ContextLimitSetting{
				Enabled:              true,
				ModelMaxPromptTokens: map[string]int{"glm-4.7": 210000},
			},
			model: "claude-sonnet-5",
			want:  0,
		},
		{
			name: "zero per-model entry overrides default with unlimited",
			setting: ContextLimitSetting{
				Enabled:                true,
				DefaultMaxPromptTokens: 128000,
				ModelMaxPromptTokens:   map[string]int{"unlimited-model": 0},
			},
			model: "unlimited-model",
			want:  0,
		},
		{
			name: "nil per-model map falls back to default",
			setting: ContextLimitSetting{
				Enabled:                true,
				DefaultMaxPromptTokens: 128000,
			},
			model: "glm-4.7",
			want:  128000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orig := contextLimitSetting
			t.Cleanup(func() { contextLimitSetting = orig })
			contextLimitSetting = test.setting

			assert.Equal(t, test.want, MaxPromptTokensForModel(test.model))
		})
	}
}

func TestGetContextLimitSettingReturnsRegisteredConfig(t *testing.T) {
	orig := contextLimitSetting
	t.Cleanup(func() { contextLimitSetting = orig })
	contextLimitSetting = ContextLimitSetting{
		Enabled:                true,
		DefaultMaxPromptTokens: 210000,
		ModelMaxPromptTokens:   map[string]int{"glm-4.7": 210000},
	}

	setting := GetContextLimitSetting()

	require.NotNil(t, setting)
	assert.True(t, setting.Enabled)
	assert.Equal(t, 210000, setting.DefaultMaxPromptTokens)
	assert.Equal(t, map[string]int{"glm-4.7": 210000}, setting.ModelMaxPromptTokens)
}
