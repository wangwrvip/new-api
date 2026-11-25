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
