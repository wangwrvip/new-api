package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetQuotaDataCache(t *testing.T) {
	t.Helper()
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()
	t.Cleanup(func() {
		CacheQuotaDataLock.Lock()
		CacheQuotaData = make(map[string]*QuotaData)
		CacheQuotaDataLock.Unlock()
	})
}

func newConsumeLogTestContext() *gin.Context {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Set("username", "alice")
	ctx.Set(common.RequestIdKey, "req-test")
	return ctx
}

// RecordConsumeLog 必须把归一化 token 维度写入 logs 行：
// 新式调用（显式传维度）落库 input/cache；旧式调用（无维度，图片/音频/任务等路径）
// 输入回退为 prompt_tokens、缓存为 0，且总量维持旧口径。
func TestRecordConsumeLogNormalizesTokenDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	truncateTables(t)
	common.DataExportEnabled = true
	t.Cleanup(func() { common.DataExportEnabled = false })
	resetQuotaDataCache(t)
	ctx := newConsumeLogTestContext()

	RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     100,
		CompletionTokens: 20,
		ModelName:        "gpt-test",
		TokenName:        "tk",
		Quota:            50,
		Content:          []*common.Message{common.NewMessage("test")},
		TokenId:          1,
		UseTimeSeconds:   1,
		Group:            "default",
	})

	RecordConsumeLog(ctx, 2, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     1000,
		CompletionTokens: 200,
		InputTokens:      850,
		CacheTokens:      150,
		CacheReadTokens:  100,
		ModelName:        "claude-test",
		TokenName:        "tk",
		Quota:            50,
		Content:          []*common.Message{common.NewMessage("test")},
		TokenId:          2,
		UseTimeSeconds:   1,
		Group:            "default",
	})

	var logs []Log
	require.NoError(t, DB.Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)

	// 旧式调用：维度回退，原有字段不变（无缓存维度，缓存读为 0）
	assert.Equal(t, 100, logs[0].PromptTokens)
	assert.Equal(t, 20, logs[0].CompletionTokens)
	assert.Equal(t, 100, logs[0].InputTokens)
	assert.Equal(t, 0, logs[0].CacheTokens)
	assert.Equal(t, 0, logs[0].CacheReadTokens)

	// 新式调用：归一化维度落库，prompt/completion 原样保留（logs UI 兼容）；
	// 缓存读单列落库（150 = 读 100 + 写 50）
	assert.Equal(t, 1000, logs[1].PromptTokens)
	assert.Equal(t, 200, logs[1].CompletionTokens)
	assert.Equal(t, 850, logs[1].InputTokens)
	assert.Equal(t, 150, logs[1].CacheTokens)
	assert.Equal(t, 100, logs[1].CacheReadTokens)

	// 看板总量口径：旧式 = prompt+completion（120）；
	// 新式 = 输入+缓存+输出（850+150+200=1200，统一含缓存）
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	require.Len(t, CacheQuotaData, 2)
	totalTokenUsed := 0
	for _, qd := range CacheQuotaData {
		totalTokenUsed += qd.TokenUsed
	}
	assert.Equal(t, 120+1200, totalTokenUsed)
}

// 纯缓存请求（InputTokens == 0 且 CacheTokens > 0）不能触发病 fallback，
// 否则会把 prompt_tokens 重复计入非缓存输入。
func TestRecordConsumeLogDoesNotFallBackForPureCacheRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	truncateTables(t)
	common.DataExportEnabled = true
	t.Cleanup(func() { common.DataExportEnabled = false })
	resetQuotaDataCache(t)
	ctx := newConsumeLogTestContext()

	other := NewLogOther()
	other.SetPublic("cache_tokens", 500)
	RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     500,
		CompletionTokens: 0,
		InputTokens:      0,
		CacheTokens:      500,
		CacheReadTokens:  500,
		ModelName:        "claude-test",
		Quota:            1,
		Other:            other,
	})

	var logs []Log
	require.NoError(t, DB.Order("id").Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, 500, logs[0].PromptTokens)
	assert.Equal(t, 0, logs[0].InputTokens)
	assert.Equal(t, 500, logs[0].CacheTokens)
	assert.Equal(t, 500, logs[0].CacheReadTokens)

	CacheQuotaDataLock.Lock()
	require.Len(t, CacheQuotaData, 1)
	var got QuotaData
	for _, qd := range CacheQuotaData {
		got = *qd
	}
	CacheQuotaDataLock.Unlock()
	assert.Equal(t, 0, got.InputTokens)
	assert.Equal(t, 500, got.CacheTokens)
	assert.Equal(t, int64(500), got.CacheReadTokens)
	assert.Equal(t, 500, got.TokenUsed)
}

// 上游异常值（cached_tokens > prompt_tokens 等）不能让负维度写入 logs，
// 且 TokenUsed 必须等于钳制后的 输入+缓存+输出。
func TestRecordConsumeLogClampsNegativeTokenDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	truncateTables(t)
	common.DataExportEnabled = true
	t.Cleanup(func() { common.DataExportEnabled = false })
	resetQuotaDataCache(t)
	ctx := newConsumeLogTestContext()

	RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     1000,
		CompletionTokens: 10,
		InputTokens:      -200,
		CacheTokens:      600,
		CacheReadTokens:  200,
		ModelName:        "claude-test",
		Quota:            1,
		Other:            NewLogOther(),
	})

	var logs []Log
	require.NoError(t, DB.Order("id").Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Equal(t, 0, logs[0].InputTokens)
	assert.Equal(t, 600, logs[0].CacheTokens)
	assert.Equal(t, 200, logs[0].CacheReadTokens)

	CacheQuotaDataLock.Lock()
	require.Len(t, CacheQuotaData, 1)
	var got QuotaData
	for _, qd := range CacheQuotaData {
		got = *qd
	}
	CacheQuotaDataLock.Unlock()
	assert.Equal(t, 0, got.InputTokens)
	assert.Equal(t, 600, got.CacheTokens)
	assert.Equal(t, int64(200), got.CacheReadTokens)
	assert.Equal(t, 10, got.OutputTokens)
	assert.Equal(t, 610, got.TokenUsed)
	assert.GreaterOrEqual(t, got.CacheTokens, int(got.CacheReadTokens))
}

// logs 的 prompt/completion 兼容字段同样不能写入负值：上游异常返回负 token
// 时，logs 必须与钳制后的归一化列、quota_data 输出保持同一口径。
func TestRecordConsumeLogClampsNegativeCompatibilityTokenFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	truncateTables(t)
	common.DataExportEnabled = true
	t.Cleanup(func() { common.DataExportEnabled = false })
	resetQuotaDataCache(t)
	ctx := newConsumeLogTestContext()

	// 仅输出为负：prompt 保持原值，completion 钳到 0。
	RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     100,
		CompletionTokens: -10,
		ModelName:        "claude-test",
		Quota:            1,
		Other:            NewLogOther(),
	})
	// 仅输入为负：prompt 钳到 0，completion 保持原值。
	RecordConsumeLog(ctx, 2, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     -100,
		CompletionTokens: 5,
		ModelName:        "claude-test",
		Quota:            1,
		Other:            NewLogOther(),
	})

	var logs []Log
	require.NoError(t, DB.Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	assert.Equal(t, 100, logs[0].PromptTokens)
	assert.Equal(t, 0, logs[0].CompletionTokens)
	assert.Equal(t, 0, logs[1].PromptTokens)
	assert.Equal(t, 5, logs[1].CompletionTokens)

	byUser := make(map[int]QuotaData)
	CacheQuotaDataLock.Lock()
	require.Len(t, CacheQuotaData, 2)
	for _, qd := range CacheQuotaData {
		byUser[qd.UserID] = *qd
	}
	CacheQuotaDataLock.Unlock()

	// quota_data 的输入/输出与 logs 钳制后的值一致，且 TokenUsed 恒等于三者之和。
	assert.Equal(t, 100, byUser[1].InputTokens)
	assert.Equal(t, 0, byUser[1].OutputTokens)
	assert.Equal(t, 100, byUser[1].TokenUsed)
	assert.Equal(t, 0, byUser[2].InputTokens)
	assert.Equal(t, 5, byUser[2].OutputTokens)
	assert.Equal(t, 5, byUser[2].TokenUsed)
}

// logs 与 quota_data 的缓存维度必须同源：
//   - 上游 cache_tokens / cache_write_tokens 与本地归一化三列同时存在时保持一致；
//   - 渠道测试这类只写 other.cache_tokens、不传维度参数的调用不能产生
//     quota_data.cache_read_tokens > cache_tokens 的越界组合。
func TestRecordConsumeLogKeepsLogAndQuotaDataCacheDimensionsConsistent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	truncateTables(t)
	common.DataExportEnabled = true
	t.Cleanup(func() { common.DataExportEnabled = false })
	resetQuotaDataCache(t)
	ctx := newConsumeLogTestContext()

	other := NewLogOther()
	other.SetPublic("cache_tokens", 100)
	other.SetPublic("cache_write_tokens", 50)
	RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     1000,
		CompletionTokens: 200,
		InputTokens:      850,
		CacheTokens:      150,
		CacheReadTokens:  100,
		ModelName:        "claude-test",
		Quota:            1,
		Other:            other,
	})

	// 渠道测试路径：只写 other.cache_tokens，不传归一化维度。
	cacheOnlyOther := NewLogOther()
	cacheOnlyOther.SetPublic("cache_tokens", 10)
	RecordConsumeLog(ctx, 2, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     500,
		CompletionTokens: 10,
		ModelName:        "gpt-test",
		Quota:            1,
		Other:            cacheOnlyOther,
	})

	var logs []Log
	require.NoError(t, DB.Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	assert.Equal(t, 850, logs[0].InputTokens)
	assert.Equal(t, 150, logs[0].CacheTokens)
	assert.Equal(t, 100, logs[0].CacheReadTokens)
	// 旧口径回退：输入为 prompt_tokens，缓存维度为 0；与看板保持一致。
	assert.Equal(t, 500, logs[1].InputTokens)
	assert.Equal(t, 0, logs[1].CacheTokens)
	assert.Equal(t, 0, logs[1].CacheReadTokens)

	byUser := make(map[int]QuotaData)
	CacheQuotaDataLock.Lock()
	require.Len(t, CacheQuotaData, 2)
	for _, qd := range CacheQuotaData {
		byUser[qd.UserID] = *qd
	}
	CacheQuotaDataLock.Unlock()

	first := byUser[1]
	assert.Equal(t, logs[0].InputTokens, first.InputTokens)
	assert.Equal(t, logs[0].CacheTokens, first.CacheTokens)
	assert.Equal(t, int64(logs[0].CacheReadTokens), first.CacheReadTokens)
	assert.Equal(t, int64(50), first.CacheCreationTokens)
	assert.Equal(t, 1200, first.TokenUsed)
	assert.GreaterOrEqual(t, first.CacheTokens, int(first.CacheReadTokens))

	second := byUser[2]
	assert.Equal(t, logs[1].InputTokens, second.InputTokens)
	assert.Equal(t, logs[1].CacheTokens, second.CacheTokens)
	assert.Equal(t, int64(logs[1].CacheReadTokens), second.CacheReadTokens)
	assert.Equal(t, 510, second.TokenUsed)
	assert.GreaterOrEqual(t, second.CacheTokens, int(second.CacheReadTokens))
}

// quota_data 维度：LogQuotaData 累加输入/输出/缓存三列，落库后按用户查询返回聚合值。
func TestQuotaDataAccumulatesTokenDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	truncateTables(t)
	resetQuotaDataCache(t)

	base := QuotaDataLogParams{
		UserID:    7,
		Username:  "bob",
		ModelName: "claude-test",
		Quota:     10,
		CreatedAt: 1700000000,
		UseGroup:  "default",
		TokenID:   1,
		ChannelID: 1,
		NodeName:  "",
	}

	// 同一小时内的两笔消费，维度应累加（CacheReadTokens 为缓存命中的读分量）
	p1 := base
	p1.InputTokens, p1.OutputTokens, p1.CacheTokens, p1.CacheReadTokens, p1.TokenUsed = 100, 20, 30, 10, 150
	LogQuotaData(p1)
	p2 := base
	p2.InputTokens, p2.OutputTokens, p2.CacheTokens, p2.CacheReadTokens, p2.TokenUsed = 40, 10, 60, 50, 110
	LogQuotaData(p2)

	// 锁内只取值，锁外断言：require 失败会 Goexit，不能带着锁退出（否则死锁后续测试）
	CacheQuotaDataLock.Lock()
	require.Len(t, CacheQuotaData, 1)
	var got QuotaData
	for _, qd := range CacheQuotaData {
		got = *qd
	}
	CacheQuotaDataLock.Unlock()
	assert.Equal(t, 140, got.InputTokens)
	assert.Equal(t, 30, got.OutputTokens)
	assert.Equal(t, 90, got.CacheTokens)
	assert.Equal(t, int64(60), got.CacheReadTokens)
	assert.Equal(t, 260, got.TokenUsed)

	SaveQuotaDataCache()

	rows, err := GetQuotaDataByUserId(7, 0, 1700000000+7200)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 140, rows[0].InputTokens)
	assert.Equal(t, 30, rows[0].OutputTokens)
	assert.Equal(t, 90, rows[0].CacheTokens)
	assert.Equal(t, int64(60), rows[0].CacheReadTokens)
	assert.Equal(t, 260, rows[0].TokenUsed)
}
