// ============================================================
// helpers.go - 辅助函数
// 职责：提供通用的工具函数，供 app 包内其他文件使用
// ============================================================

package app

import (
	"fmt"
	"regexp"
	"strings"

	"mephisto/engine"
)

// ReplaceVariables 替换文本中的 {变量}
// 用于 【开局场景】、【世界观】、【角色背景】等文本区块
// 示例："{角色名}的故乡" → "贝利亚奥特曼的故乡"
// 如果变量在上下文中不存在，则保持原样
func ReplaceVariables(text string, ctx engine.Context) string {
	re := regexp.MustCompile(`\{([^{}]+)\}`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		key := strings.Trim(match, "{}")
		if val, ok := ctx[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// 找不到变量，保持原样
		return match
	})
}
