// internal/shared/convert.go
//
// 工具函数：类型转换、字符串处理、模板替换。
package shared

import (
	"fmt"
	"mephisto/internal/domain"
	"strconv"
	"strings"
)

// ============================================================
// 值解析与转换
// ============================================================

// ParseValue 将字符串转换为合适的类型。
//
// 转换规则：
//   - "true" / "false" → bool
//   - 整数 → int
//   - 浮点数 → float64
//   - 其他 → string
func ParseValue(s string) any {
	s = strings.TrimSpace(s)

	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	if i, err := strconv.Atoi(s); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return s
}

// KeyValuesToMap 将契约中的键值对列表转换为运行时 map。
func KeyValuesToMap(kvs []domain.KeyValue) map[string]any {
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = ParseValue(kv.Value)
	}
	return m
}

// MapToKeyValues 将运行时 map 转回契约的键值对列表。
func MapToKeyValues(m map[string]any, orderKeys []string) []domain.KeyValue {
	result := make([]domain.KeyValue, 0, len(m))
	if len(orderKeys) > 0 {
		for _, key := range orderKeys {
			if v, ok := m[key]; ok {
				result = append(result, domain.KeyValue{Key: key, Value: fmt.Sprintf("%v", v)})
			}
		}
	} else {
		for k, v := range m {
			result = append(result, domain.KeyValue{Key: k, Value: fmt.Sprintf("%v", v)})
		}
	}
	return result
}

// ============================================================
// 字符串工具
// ============================================================

// Unquote 去除字符串首尾的引号（英文双引号或中文双引号）。
func Unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
			(strings.HasPrefix(s, "“") && strings.HasSuffix(s, "”")) {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ============================================================
// 模板替换
// ============================================================

// ReplacePlaceholders 替换字符串中的占位符 {key}。
func ReplacePlaceholders(template string, vars map[string]string) string {
	for k, v := range vars {
		template = strings.ReplaceAll(template, "{"+k+"}", v)
	}
	return template
}