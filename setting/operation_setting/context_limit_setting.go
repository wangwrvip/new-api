package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// ContextLimitSetting 按模型的输入上下文（prompt tokens）上限配置。
// 超过上限的请求在预扣费和渠道选择之前被直接拒绝（413，不重试）。
type ContextLimitSetting struct {
	// Enabled 总开关，关闭时所有限制不生效
	Enabled bool `json:"enabled"`
	// DefaultMaxPromptTokens 模型未单独配置时的默认上限（token 数），0 = 不限制
	DefaultMaxPromptTokens int `json:"default_max_prompt_tokens"`
	// ModelMaxPromptTokens 每模型上限（token 数），键为模型名（大小写不敏感），0 = 该模型不限制
	ModelMaxPromptTokens map[string]int `json:"model_max_prompt_tokens"`
}

var contextLimitSetting = ContextLimitSetting{
	ModelMaxPromptTokens: make(map[string]int),
}

func init() {
	config.GlobalConfig.Register("context_limit_setting", &contextLimitSetting)
}

func GetContextLimitSetting() *ContextLimitSetting {
	return &contextLimitSetting
}

// MaxPromptTokensForModel 返回模型 context 上限（token 数），0 表示不限制。
// 模型名精确匹配（大小写不敏感），未命中时回退到默认上限。
func MaxPromptTokensForModel(model string) int {
	if !contextLimitSetting.Enabled {
		return 0
	}
	for name, limit := range contextLimitSetting.ModelMaxPromptTokens {
		if strings.EqualFold(name, model) {
			return limit
		}
	}
	return contextLimitSetting.DefaultMaxPromptTokens
}
