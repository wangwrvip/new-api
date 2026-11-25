package setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useDepartmentListOption replaces the in-memory option map for one test and
// restores the previous map afterwards, so OptionMap state does not leak
// between tests or packages.
func useDepartmentListOption(t *testing.T, raw string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{}
	if raw != "" {
		common.OptionMap[DepartmentListKey] = raw
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestGetDepartmentsReturnsDefaultsWhenUnset(t *testing.T) {
	useDepartmentListOption(t, "")

	departments := GetDepartments()

	require.NotEmpty(t, departments)
	assert.Equal(t, DefaultDepartment, departments[0])
}

func TestGetDepartmentsFallsBackOnInvalidJSON(t *testing.T) {
	useDepartmentListOption(t, "{")

	departments := GetDepartments()

	require.NotEmpty(t, departments)
	assert.Equal(t, DefaultDepartment, departments[0])
}

func TestNormalizeDepartment(t *testing.T) {
	useDepartmentListOption(t, `["公共部门","视频部"]`)

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "empty falls back to default", value: "", want: DefaultDepartment},
		{name: "whitespace only falls back to default", value: "   ", want: DefaultDepartment},
		{name: "surrounding whitespace trimmed", value: "  视频部  ", want: "视频部"},
		{name: "unlisted rejected", value: "不存在的部门", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDepartment(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDepartmentsJsonStringRoundTrip(t *testing.T) {
	want := []string{DefaultDepartment, "视频部", "单片机", "路由网关", "测试组", "硬件部", "工控机部", "平台部"}

	raw := Departments2JsonString()
	var decoded []string
	require.NoError(t, common.UnmarshalJsonStr(raw, &decoded))
	require.Equal(t, want, decoded)

	useDepartmentListOption(t, raw)
	assert.Equal(t, want, GetDepartments())
}

func TestIsValidDepartment(t *testing.T) {
	useDepartmentListOption(t, `["公共部门","视频部"]`)

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "empty", value: "", valid: false},
		{name: "unlisted", value: "不存在部门", valid: false},
		{name: "listed", value: "视频部", valid: true},
		{name: "surrounding whitespace", value: "  视频部  ", valid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidDepartment(tt.value))
		})
	}
}
