// internal/shared/template.go
package shared

import "strings"

// ReplacePlaceholders 替换字符串中的占位符。
// 占位符格式：{key} → 替换为对应的值。
func ReplacePlaceholders(template string, vars map[string]string) string {
	for k, v := range vars {
		template = strings.ReplaceAll(template, "{"+k+"}", v)
	}
	return template
}
