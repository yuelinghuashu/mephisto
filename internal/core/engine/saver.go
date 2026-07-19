// internal/core/engine/saver.go
//
// 本文件提供子版存档的保存功能。
// 子版 = 母版所有静态区块 + 更新后的【状态】 + 【记忆】 + 【历史】
//
// 文件格式：.meph（与母版格式一致）
// 命名规则：母版 story.meph → 子版 story_child.meph
//
//	分支 --branch dark → story_dark.meph
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// SaveChild 保存子版存档。
func SaveChild(contract *domain.Contract, state map[string]any, memories []string, history []domain.HistoryEntry, filename string, branch string) error {
	content, err := buildChildContent(contract, state, memories, history)
	if err != nil {
		return fmt.Errorf("构建子版内容失败: %w", err)
	}

	childPath := BuildChildPath(filename, branch)
	dir := filepath.Dir(childPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(childPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// buildChildContent 构建子版文件内容。
func buildChildContent(contract *domain.Contract, state map[string]any, memories []string, history []domain.HistoryEntry) (string, error) {
	var sb strings.Builder

	vars := map[string]string{
		"角色名": contract.RoleName,
	}
	for k, v := range state {
		vars[k] = fmt.Sprintf("%v", v)
	}

	// ---- 1. 角色名（单行文本，无末尾换行） ----
	fmt.Fprintf(&sb, "【角色名】\n%s\n\n", contract.RoleName)

	// ---- 2. 锚点（列表，无末尾换行） ----
	if len(contract.Anchor) > 0 {
		fmt.Fprint(&sb, "【锚点】\n")
		for _, kv := range contract.Anchor {
			value := shared.ReplacePlaceholders(kv.Value, vars)
			fmt.Fprintf(&sb, "- %s: %s\n", kv.Key, value)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 3. 世界观（多行文本，末尾已有换行） ----
	if contract.Worldview != "" {
		content := shared.ReplacePlaceholders(contract.Worldview, vars)
		// 内容末尾可能已有换行，只加一个换行来分隔下一个区块
		fmt.Fprintf(&sb, "【世界观】\n%s\n", content)
	}

	// ---- 4. 角色背景（多行文本，末尾已有换行） ----
	if contract.Background != "" {
		content := shared.ReplacePlaceholders(contract.Background, vars)
		fmt.Fprintf(&sb, "【角色背景】\n%s\n", content)
	}

	// ---- 5. 开局场景（多行文本，末尾已有换行） ----
	if contract.Opening != "" {
		content := shared.ReplacePlaceholders(contract.Opening, vars)
		fmt.Fprintf(&sb, "【开局场景】\n%s\n", content)
	}

	// ---- 6. 状态（列表，无末尾换行） ----
	if len(state) > 0 {
		fmt.Fprint(&sb, "【状态】\n")
		for k, v := range state {
			fmt.Fprintf(&sb, "- %s: %v\n", k, v)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 7. 规则（列表，无末尾换行） ----
	if len(contract.Rules) > 0 {
		fmt.Fprint(&sb, "【规则】\n")
		for _, rule := range contract.Rules {
			action := shared.ReplacePlaceholders(rule.Action, vars)
			if rule.Group != "" {
				fmt.Fprintf(&sb, "[%s] if %s -> [group:%s] %s\n", rule.Name, rule.Cond, rule.Group, action)
			} else {
				fmt.Fprintf(&sb, "[%s] if %s -> %s\n", rule.Name, rule.Cond, action)
			}
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 8. 校验（列表，无末尾换行） ----
	if len(contract.Validation) > 0 {
		fmt.Fprint(&sb, "【校验】\n")
		for _, kv := range contract.Validation {
			fmt.Fprintf(&sb, "- %s: %s\n", kv.Key, kv.Value)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 9. 记忆（列表，无末尾换行） ----
	if len(memories) > 0 {
		fmt.Fprint(&sb, "【记忆】\n")
		for _, mem := range memories {
			fmt.Fprintf(&sb, "- %s\n", mem)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 10. 历史（列表，无末尾换行） ----
	if len(history) > 0 {
		fmt.Fprint(&sb, "【历史】\n")
		for _, entry := range history {
			content := strings.ReplaceAll(entry.Content, "\n", "\\n")
			fmt.Fprintf(&sb, "- %s: %s\n", entry.Role, content)
		}
		fmt.Fprint(&sb, "\n")
	}

	return sb.String(), nil
}
