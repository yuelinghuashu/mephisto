// ============================================================
// validate.go - 类型验证器（声明式）
// 职责：
// 1. 根据区块注册表（BlockRegistry）检查每个区块的内容类型是否正确
// 2. 对文本区块（SingleLineText, MultiLineText）检查是否混入列表或规则
// 3. 对结构化区块（KeyValueList, RuleList）检查是否只有对应类型的条目
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
func validateBlock(block *ParsedBlock) error {
	spec, ok := BlockRegistry[block.Name]
	if !ok {
		// 理论上不会触发，因为 block.go 已做白名单检查
		return fmt.Errorf("未知区块类型: %s", block.Name)
	}

	switch spec.Type {
	case SingleLineText:
		// 必须恰好一个非空文本条目，且不能有列表或规则
		nonEmptyTexts := 0
		for _, e := range block.Entries {
			if e.Type == "text" && strings.TrimSpace(e.Value) != "" {
				nonEmptyTexts++
			}
			if e.Type != "text" {
				return fmt.Errorf("【%s】区块只能包含文本，不能包含列表项或规则", block.Name)
			}
		}
		if nonEmptyTexts == 0 {
			return fmt.Errorf("【%s】内容不能为空", block.Name)
		}
		if nonEmptyTexts > 1 {
			return fmt.Errorf("【%s】应为单行文本，不能包含多行", block.Name)
		}
		return nil

	case MultiLineText:
		// 只允许 text 条目（可以有多个）
		for _, e := range block.Entries {
			if e.Type != "text" {
				return fmt.Errorf("【%s】区块只能包含文本，不能包含列表项或规则", block.Name)
			}
		}
		return nil

	case KeyValueList:
		// 只允许 list 条目
		for _, e := range block.Entries {
			if e.Type != "list" {
				return fmt.Errorf("【%s】区块只允许列表项（- 键: 值）", block.Name)
			}
		}
		return nil

	case RuleList:
		// 只允许 rule 条目
		for _, e := range block.Entries {
			if e.Type != "rule" {
				return fmt.Errorf("【%s】区块只允许规则条目（[名称] if 条件 -> 动作）", block.Name)
			}
		}
		return nil

	default:
		return fmt.Errorf("未知的区块类型编号: %d", spec.Type)
	}
}
