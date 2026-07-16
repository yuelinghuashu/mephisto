// ============================================================
// context.go - 上下文构建器
// 职责：
// 1. 从解析结果（ParsedFile）构建规则引擎上下文（Context）
// 2. 从 【状态】 区块提取键值对，自动转换类型
// 3. 从 【角色名】 区块提取角色名
// 4. 从 【锚点】 区块提取核心人格设定
// 5. 支持 【世界观】、【角色背景】、【开局场景】 中的变量替换
// 6. 分阶段构建：先收集所有数据，再处理文本区块
//    这样文本区块可以引用 【状态】 中的任何变量，不受文件顺序影响
// ============================================================

package app

import (
	"fmt"
	"strconv"
	"strings"

	"mephisto/engine"
	"mephisto/parser"
	"mephisto/utils"
)

// BuildContext 从解析结果构建规则引擎上下文
// 返回：上下文（Context）、开局场景文本、错误
func BuildContext(pf *parser.ParsedFile) (engine.Context, strings.Builder, error) {
	ctx := engine.Context{}
	var openingText strings.Builder

	// 用于暂存文本区块，等变量收集完成后再处理
	var textEntries []struct {
		blockName string
		entry     *parser.BlockEntry
	}

	// ============================================================
	// 第一阶段：收集所有数据
	// ============================================================
	for _, block := range pf.Blocks {
		switch block.Name {

		// ---- 从 【状态】 区块提取键值对 ----
		case parser.KeyState:
			for _, entry := range block.Entries {
				if entry.Type == "list" {
					ctx[entry.Key] = parseContextValue(entry.Value)
				}
			}

		// ---- 从 【角色名】 区块提取角色名 ----
		case parser.KeyRoleName:
			for _, entry := range block.Entries {
				if entry.Type == "text" && strings.TrimSpace(entry.Value) != "" {
					ctx[parser.KeyRoleName] = strings.TrimSpace(entry.Value)
					break
				}
			}

		// ---- 从 【锚点】 区块提取核心人格设定 ----
		// 锚点是列表项（- 键: 值），格式化为可读文本
		// 确保在 LLM System Prompt 中作为最高优先级内容
		case parser.KeyAnchor:
			var anchorLines []string
			for _, entry := range block.Entries {
				if entry.Type == "list" {
					anchorLines = append(anchorLines, fmt.Sprintf("- %s: %s", entry.Key, entry.Value))
				}
			}
			if len(anchorLines) > 0 {
				ctx[parser.KeyAnchor] = strings.Join(anchorLines, "\n")
			}

		// ---- 【世界观】、【角色背景】、【开局场景】：暂存，等变量就绪后处理 ----
		case parser.KeyWorldview, parser.KeyBackground, parser.KeyOpening:
			for _, entry := range block.Entries {
				if entry.Type == "text" {
					textEntries = append(textEntries, struct {
						blockName string
						entry     *parser.BlockEntry
					}{block.Name, entry})
				}
			}
		}
	}

	// ============================================================
	// 第二阶段：处理所有文本区块（此时 ctx 已包含所有变量）
	// ============================================================
	for _, te := range textEntries {
		text := utils.ReplaceVariables(te.entry.Value, ctx)

		// 直接拼接，空行也会加入
		if existing, ok := ctx[te.blockName]; ok {
			ctx[te.blockName] = existing.(string) + "\n" + text
		} else {
			ctx[te.blockName] = text
		}
		if te.blockName == parser.KeyOpening {
			openingText.WriteString(text)
			openingText.WriteString("\n")
		}
	}

	// 最后统一 TrimSpace 去掉首尾多余空行
	if openingText.Len() > 0 {
		ctx[parser.KeyOpening] = strings.TrimSpace(openingText.String())
	}

	return ctx, openingText, nil
}

// parseContextValue 将字符串转换为合适的类型
func parseContextValue(s string) any {
	s = strings.TrimSpace(s)

	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	return s
}
