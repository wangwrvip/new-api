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
