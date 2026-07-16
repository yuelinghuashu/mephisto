// mephisto/utils/interpolate.go
package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// ReplaceVariables 替换文本中的 {变量}
// 用于 【开局场景】、【世界观】、【角色背景】等文本区块
// 示例："{角色名}的故乡" → "贝利亚奥特曼的故乡"
// 如果变量在上下文中不存在，则保持原样
func ReplaceVariables(text string, ctx map[string]any) string {
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
