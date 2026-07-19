package shared

import (
	"strconv"
	"strings"
)

// parseValue 将字符串转换为合适的类型。
//
// 转换规则：
//   - "true" / "false" → bool
//   - 整数 → int
//   - 浮点数 → float64
//   - 其他 → string
//
func ParseValue(s string) any {
	s = strings.TrimSpace(s)

	// 布尔值
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// 整数（优先）
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}

	// 浮点数
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// 默认字符串
	return s
}
