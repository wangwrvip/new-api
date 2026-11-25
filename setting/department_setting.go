package setting

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	// DepartmentListKey 部门清单设置项，值为 JSON 字符串数组。
	DepartmentListKey = "DepartmentList"
	// DefaultDepartment 未设置部门的用户归属的默认部门。
	DefaultDepartment = "公共部门"
)

// defaultDepartments 默认部门清单：默认部门置首，其余为预定义部门。
func defaultDepartments() []string {
	return []string{DefaultDepartment, "视频部", "单片机", "路由网关", "测试组", "硬件部", "工控机部", "平台部"}
}

// Departments2JsonString 返回默认部门清单的 JSON（供 InitOptionMap 注册默认值）。
func Departments2JsonString() string {
	encoded, err := common.Marshal(defaultDepartments())
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// GetDepartments 返回当前部门清单；未配置或解析失败时回退默认清单。
func GetDepartments() []string {
	common.OptionMapRWMutex.RLock()
	raw := ""
	if common.OptionMap != nil {
		raw = common.OptionMap[DepartmentListKey]
	}
	common.OptionMapRWMutex.RUnlock()

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultDepartments()
	}
	var departments []string
	if err := common.UnmarshalJsonStr(raw, &departments); err != nil {
		return defaultDepartments()
	}
	if len(departments) == 0 {
		return defaultDepartments()
	}
	return departments
}

// IsValidDepartment 判断部门名是否在当前清单中。
func IsValidDepartment(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, item := range GetDepartments() {
		if item == name {
			return true
		}
	}
	return false
}

// NormalizeDepartment 规整用户部门值：空值回退默认部门，非空但不在当前清单时返回错误。
// 兼容存量用户（列默认值回填后非空）；清单变更后旧值仍在库中时，编辑提交会被拒绝，
// 避免静默改写用户数据。
func NormalizeDepartment(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultDepartment, nil
	}
	if !IsValidDepartment(name) {
		return "", errors.New("部门不在当前清单中")
	}
	return name, nil
}
