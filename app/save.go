// ============================================================
// save.go - 子版存档管理（M5 记忆编织）
// 职责：
// 1. 保存子版 .meph 文件（包含运行时状态）
// 2. 加载子版文件并合并到母版上下文
// 3. 支持分支管理
// ============================================================

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mephisto/engine"
	"mephisto/parser"
)

// SaveChild 保存子版存档
func SaveChild(filename, branch string, ctx engine.Context, history *ConversationHistory) error {
	childName := buildChildFilename(filename, branch)

	var sb strings.Builder

	// 1. 先解析母版，复制所有静态区块
	pf, err := parser.ParseFile(filename)
	if err != nil {
		// 如果解析失败（可能 filename 就是子版本身），则只写入运行时区块
		fmt.Printf("⚠️ 无法解析母版，子版将只包含运行时数据: %v\n", err)
	} else {
		for _, block := range pf.Blocks {
			if block.Name == parser.KeyState ||
				block.Name == parser.KeyMemory ||
				block.Name == parser.KeyHistory {
				continue
			}
			fmt.Fprintf(&sb, "【%s】\n", block.Name)
			sb.WriteString(block.Raw)
			if !strings.HasSuffix(block.Raw, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// 2. 写入【状态】区块
	sb.WriteString("【状态】\n")
	for key, val := range ctx {
		if parser.StateExcludeKeys[key] {
			continue
		}
		if val == nil {
			continue
		}
		fmt.Fprintf(&sb, "- %s: %v\n", key, val)
	}
	sb.WriteString("\n")

	// 3. 写入【记忆】区块
	sb.WriteString("【记忆】\n")
	if memories, ok := ctx[parser.KeyMemory].([]string); ok {
		for _, mem := range memories {
			sb.WriteString("- ")
			sb.WriteString(mem)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	// 4. 写入【历史】区块
	if history != nil && history.GetSize() > 0 {
		sb.WriteString("【历史】\n")
		for _, msg := range history.GetMessages() {
			// 转义换行符，确保一行一条消息
			content := strings.ReplaceAll(msg.Content, "\n", "\\n")
			fmt.Fprintf(&sb, "- %s: %s\n", msg.Role, content)
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(childName, []byte(sb.String()), 0644)
}

// LoadChild 加载子版存档
func LoadChild(filename, branch string) (engine.Context, *ConversationHistory, error) {
	childName := buildChildFilename(filename, branch)
	if _, err := os.Stat(childName); os.IsNotExist(err) {
		return nil, nil, nil
	}

	fmt.Printf("📂 加载子版: %s\n", childName)

	pf, err := parser.ParseFile(childName)
	if err != nil {
		return nil, nil, fmt.Errorf("加载子版失败: %w", err)
	}

	ctx, _, err := BuildContext(pf)
	if err != nil {
		return nil, nil, fmt.Errorf("构建子版上下文失败: %w", err)
	}

	// ============================================================
	// 从【历史】区块恢复对话历史（增强版）
	// ============================================================
	history := NewConversationHistory(100) // 先用大容量防止截断
	historyLoaded := 0

	for _, block := range pf.Blocks {
		if block.Name == parser.KeyHistory {
			for _, entry := range block.Entries {
				if entry.Type != "list" {
					continue
				}

				// ✅ 直接使用 entry.Key 和 entry.Value
				// entry.Key 已经是 "user" 或 "assistant"，entry.Value 是内容
				role := strings.TrimSpace(entry.Key)
				content := strings.TrimSpace(entry.Value)

				// 反转义换行符
				content = strings.ReplaceAll(content, "\\n", "\n")

				// 验证 role 是否有效
				if role != "user" && role != "assistant" && role != "system" {
					fmt.Printf("⚠️ 跳过无效角色 '%s' 的历史条目\n", role)
					continue
				}

				history.Add(role, content)
				historyLoaded++
				fmt.Printf("  📝 恢复 [%d] %s: %.50s...\n", historyLoaded, role, content)
			}
		}
	}

	if historyLoaded > 0 {
		fmt.Printf("✅ 从子版恢复 %d 条对话历史\n", historyLoaded)
	} else {
		fmt.Printf("📝 子版中没有历史记录\n")
	}

	return ctx, history, nil
}

// ============================================================
// 辅助函数
// ============================================================

func isChildFile(filename string) bool {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	if strings.HasSuffix(name, "_child") {
		return true
	}
	lastUnderscore := strings.LastIndex(name, "_")
	if lastUnderscore != -1 && lastUnderscore < len(name)-1 {
		return true
	}
	return false
}

func getBranchSuffix(branch string) string {
	if branch != "" {
		return "_" + branch
	}
	return "_child"
}

func extractBranchFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filepath.Base(filename), ext)

	lastUnderscore := strings.LastIndex(base, "_")
	if lastUnderscore == -1 {
		return ""
	}
	suffix := base[lastUnderscore+1:]
	if suffix == "child" {
		return ""
	}
	return suffix
}

func buildChildFilename(filename, branch string) string {
	if isChildFile(filename) {
		return filename
	}

	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	suffix := getBranchSuffix(branch)
	return filepath.Join(dir, name+suffix+ext)
}
