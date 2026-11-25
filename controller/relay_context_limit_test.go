package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const contextLimitTestModel = "context-limit-test-model"

func TestContextLimitErrorReturns413WithCode(t *testing.T) {
	err := contextLimitError("glm-4.7", 210001, 210000)

	require.NotNil(t, err)
	assert.Equal(t, http.StatusRequestEntityTooLarge, err.StatusCode)
	assert.Equal(t, types.ErrorCodeContextLimitExceeded, err.GetErrorCode())
	assert.Equal(t, "context_limit_exceeded", string(err.GetErrorCode()))
	assert.True(t, types.IsSkipRetryError(err))
}

func TestContextLimitErrorOtherIncludesRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	other := contextLimitErrorOther(c, relayInfo, 210001, 210000)
	snapshot := other.Snapshot()

	assert.Equal(t, "/v1/chat/completions", snapshot["request_path"])
	assert.Equal(t, types.ErrorTypeNewAPIError, snapshot["error_type"])
	assert.Equal(t, string(types.ErrorCodeContextLimitExceeded), snapshot["error_code"])
	assert.Equal(t, http.StatusRequestEntityTooLarge, snapshot["status_code"])
	assert.Equal(t, 210001, snapshot["estimated_prompt_tokens"])
	assert.Equal(t, 210000, snapshot["context_limit"])
	assert.Equal(t, string(relayInfo.RelayFormat), snapshot["relay_format"])
}

func TestContextLimitErrorOtherKeepsChannelInfoAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("use_channel", []string{"7", "9"})
	relayInfo := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	other := contextLimitErrorOther(c, relayInfo, 210001, 210000)
	snapshot := other.Snapshot()

	assert.NotContains(t, snapshot, "use_channel")
	adminInfo, ok := snapshot["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"7", "9"}, adminInfo["use_channel"])
}

// TestRelayContextLimitRejectsBeforePreConsumeWithBillingSwitchOff guards the
// core contract: the limit uses CountRequestToken, not EstimateRequestToken, so
// disabling the billing estimation switch must not silently disable it. The
// user has no quota and a wallet-only preference, so if billing ran before the
// limit check the response would be 403 insufficient_user_quota; receiving 413
// is itself proof that no pre-consume was attempted. The zero-quota fixture
// plus the 413/403 status assertions are what actually guard the ordering: a
// quota re-read cannot distinguish it, because the fixture starts at zero and a
// pre-consume followed by a refund would leave the stored balance at zero too.
func TestRelayContextLimitRejectsBeforePreConsumeWithBillingSwitchOff(t *testing.T) {
	_, user := setupContextLimitRelayTest(t)
	withContextLimitBillingConfig(t)
	withContextLimitSetting(t, contextLimitTestModel, 1)
	withCountTokenDisabled(t)

	recorder := runContextLimitRelay(t, user)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "context_limit_exceeded", response.Error.Code)
}

// TestRelayContextLimitAllowsUnderLimitIntoBillingWithBillingSwitchOff proves
// that an under-limit request still reaches PrepareRequestBilling: with a
// wallet-only user that has no quota, the pre-consume check returns 403 instead
// of the 413 context-limit rejection.
func TestRelayContextLimitAllowsUnderLimitIntoBillingWithBillingSwitchOff(t *testing.T) {
	_, user := setupContextLimitRelayTest(t)
	withContextLimitBillingConfig(t)
	withContextLimitSetting(t, contextLimitTestModel, 1_000_000)
	withCountTokenDisabled(t)

	recorder := runContextLimitRelay(t, user)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "insufficient_user_quota", response.Error.Code)
}

// TestEnforceContextLimitFailsOpenOnCountError covers prompt metadata that
// cannot be measured (for example an audio duration or a remote image). The
// limit must be skipped and the request must continue instead of failing with a
// count-token error.
func TestEnforceContextLimitFailsOpenOnCountError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, OriginModelName: contextLimitTestModel}

	err := enforceContextLimit(c, relayInfo, 0, 1, errors.New("error getting audio duration: unsupported format"))
	assert.Nil(t, err)
}

func setupContextLimitRelayTest(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// Billing errors are translated; initialize the bundle the production
	// entrypoint would otherwise set up.
	require.NoError(t, i18n.Init())

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousCache := common.RedisEnabled, common.MemoryCacheEnabled
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousCache
	})

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = database, database
	t.Cleanup(func() {
		sqlDB, err := database.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Log{}))

	user := &model.User{
		Username: "context-limit-test-user",
		Quota:    0,
		Group:    "default",
		AffCode:  "context-limit-test-aff",
	}
	require.NoError(t, database.Create(user).Error)
	return database, user
}

// withContextLimitBillingConfig makes the test model priceable, so a wrongly
// ordered billing call would reach the wallet's quota check instead of failing
// earlier while resolving the price.
func withContextLimitBillingConfig(t *testing.T) {
	t.Helper()
	previous := config.GlobalConfig.ExportAllConfigs()
	t.Cleanup(func() { require.NoError(t, config.GlobalConfig.LoadFromDB(previous)) })
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode":         `{"context-limit-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr":         `{"context-limit-test-model":"tier(\"base\", p * 3)"}`,
		"group_ratio_setting.group_ratio":      `{"default":1}`,
		"quota_setting.pre_consume_multiplier": "1",
	}))
}

func withContextLimitSetting(t *testing.T, modelName string, limit int) {
	t.Helper()
	setting := operation_setting.GetContextLimitSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })
	setting.Enabled = true
	setting.DefaultMaxPromptTokens = 0
	setting.ModelMaxPromptTokens = map[string]int{modelName: limit}
}

func withCountTokenDisabled(t *testing.T) {
	t.Helper()
	previous := constant.CountToken
	constant.CountToken = false
	t.Cleanup(func() { constant.CountToken = previous })
}

func runContextLimitRelay(t *testing.T, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"model":"` + contextLimitTestModel + `","messages":[{"role":"user","content":"` +
		strings.Repeat("measure this prompt context ", 50) + `"}]}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, contextLimitTestModel)
	common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})

	Relay(c, types.RelayFormatOpenAI)
	return recorder
}
