// ============================================================
// validate.go - 类型验证器（声明式）
// 职责：
// 1. 根据区块注册表（BlockRegistry）检查每个区块的内容类型是否正确
// 2. 对文本区块（SingleLineText, MultiLineText）检查是否混入列表或规则
// 3. 对结构化区块（KeyValueList, RuleList）检查是否只有对应类型的条目
// 4. 【记忆】区块特殊处理：允许纯文本列表（无冒号）
// 这是解析的第三阶段：结构化条目 → 类型验证
// ============================================================

package parser

import (
	"fmt"
	"strings"
)

// Validate 验证所有区块的类型是否正确
func (pf *ParsedFile) Validate() error {
	for _, block := range pf.Blocks {
		if err := validateBlock(block); err != nil {
			return fmt.Errorf("【%s】区块验证失败 (起始于第 %d 行): %w",
				block.Name, block.Line, err)
		}
	}
	return nil
}

// validateBlock 根据注册表校验单个区块的内容
// 注意：这里只做语义约束检查，不重复解析器已保证的格式细节
func validateBlock(block *ParsedBlock) error {
	spec, ok := BlockRegistry[block.Name]
	if !ok {
		return fmt.Errorf("未知区块类型: %s", block.Name)
	}

	switch spec.Type {
	// ---- 【角色名】：必须恰好一行非空文本 ----
	case SingleLineText:
		var nonEmpty int
		for _, e := range block.Entries {
			if e.Type != "text" {
				return fmt.Errorf("只能包含文本，不能包含列表项或规则")
			}
			if strings.TrimSpace(e.Value) != "" {
				nonEmpty++
			}
		}
		if nonEmpty != 1 {
			return fmt.Errorf("必须恰好有一行非空文本")
		}
		return nil

	// ---- 【世界观】、【角色背景】、【开局场景】：只能包含文本（可以有多个非空行） ----
	case MultiLineText:
		for _, e := range block.Entries {
			if e.Type != "text" {
				return fmt.Errorf("只能包含文本，不能包含列表项或规则")
			}
		}
		return nil

	// ---- 【状态】、【锚点】、【校验】：只允许列表项（必须包含键值对） ----
	// ---- 【记忆】：只允许列表项（允许纯文本，键可以为空） ----
	case KeyValueList:
		for _, e := range block.Entries {
			if e.Type != "list" {
				return fmt.Errorf("只允许列表项")
			}
		}

		// 记忆区块允许纯文本（Key 为空），其他区块必须包含键值对
		if block.Name != KeyMemory {
			for _, e := range block.Entries {
				if e.Key == "" {
					return fmt.Errorf("列表项必须包含键值对（- 键: 值），不允许纯文本")
				}
			}
		}
		return nil

	// ---- 【规则】：只允许规则条目 ----
	case RuleList:
		for _, e := range block.Entries {
			if e.Type != "rule" {
				return fmt.Errorf("只允许规则条目（[名称] if 条件 -> 动作）")
			}
		}
		return nil

	default:
		return fmt.Errorf("未知的区块类型编号: %d", spec.Type)
	}
}
