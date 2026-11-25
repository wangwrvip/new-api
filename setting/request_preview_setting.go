package setting

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

const (
	// RequestPreviewEnabledKey 请求预览开关：开启后，消费日志在 other 顶层
	// request_preview 记录请求内容的前 N 个 token（日志所有者与管理员可见）。
	RequestPreviewEnabledKey = "RequestPreviewEnabled"
	// RequestPreviewMaxTokensKey 请求预览截断的 token 数上限。
	RequestPreviewMaxTokensKey = "RequestPreviewMaxTokens"
	// defaultRequestPreviewMaxTokens 默认预览长度。
	defaultRequestPreviewMaxTokens = 200
	// maxRequestPreviewTokens 预览长度上界，防止日志表无限膨胀。
	maxRequestPreviewTokens = 2000
)

// RequestPreviewEnabled 返回请求预览是否开启；未配置或解析失败时默认开启。
func RequestPreviewEnabled() bool {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[RequestPreviewEnabledKey]
	}
	common.OptionMapRWMutex.RUnlock()

	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return true
	}
	return enabled
}

// RequestPreviewMaxTokens 返回请求预览截断的 token 数上限，钳制到 [1, maxRequestPreviewTokens]。
func RequestPreviewMaxTokens() int {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[RequestPreviewMaxTokensKey]
	}
	common.OptionMapRWMutex.RUnlock()

	maxTokens, err := strconv.Atoi(raw)
	if err != nil || maxTokens < 1 {
		return defaultRequestPreviewMaxTokens
	}
	if maxTokens > maxRequestPreviewTokens {
		return maxRequestPreviewTokens
	}
	return maxTokens
}
