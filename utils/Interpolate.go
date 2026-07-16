// ============================================================
// interpolate.go - 变量替换工具
// 职责：
// 1. 替换文本中的 {变量} 为上下文中的值
// 2. 如果变量不存在，保持原样
// 3. 使用全局正则表达式缓存，避免每次调用都重新编译
// ============================================================

package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// variableRegex 匹配 {变量名} 格式的表达式
// 使用全局变量缓存，避免每次调用 ReplaceVariables 都重新编译
var variableRegex = regexp.MustCompile(`\{([^{}]+)\}`)

// ReplaceVariables 替换文本中的 {变量}
// 用于 【开局场景】、【世界观】、【角色背景】等文本区块
// 示例："{角色名}的故乡" → "贝利亚奥特曼的故乡"
// 如果变量在上下文中不存在，则保持原样
func ReplaceVariables(text string, ctx map[string]any) string {
	return variableRegex.ReplaceAllStringFunc(text, func(match string) string {
		key := strings.Trim(match, "{}")
		if val, ok := ctx[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// 找不到变量，保持原样
		return match
	})
}
