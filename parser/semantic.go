// ============================================================
// semantic.go - 语义解析器
// 职责：
// 1. 将每个区块的 Raw 文本按行解析成结构化条目（Entries）
// 2. 识别列表项、规则、普通文本
// 3. 对文本区块保留空行，对结构化区块过滤空行
// 4. 绝对行号（与文件物理行对齐）
// 这是解析的第二阶段：区块内容 → 结构化条目
// ============================================================

package parser

import (
	"fmt"
	"strings"
)

// ParseSemantics 遍历所有区块，调用 parseBlockContent 填充 Entries
func (pf *ParsedFile) ParseSemantics() error {
	for _, block := range pf.Blocks {
		isTextBlock := block.Name == "角色名" || block.Name == "世界观" || block.Name == "角色背景"
		entries, err := parseBlockContent(block.Raw, block.Line, isTextBlock)
		if err != nil {
			return fmt.Errorf("解析 【%s】 区块失败: %w", block.Name, err)
		}
		block.Entries = entries
	}
	return nil
}

// parseBlockContent 解析区块的原始文本（raw），生成条目列表
// 使用 strings.Lines 按行迭代，避免一次性分配整个行切片
func parseBlockContent(raw string, startLine int, isTextBlock bool) ([]*BlockEntry, error) {
	var entries []*BlockEntry
	i := 0

	// strings.Lines 返回一个迭代器，按需生成每一行（Go 1.23+）
	for line := range strings.Lines(raw) {
		absoluteLineNum := startLine + 1 + i
		trimmed := strings.TrimSpace(line)

		// 跳过注释行（行号已保留）
		if strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		// 空行处理
		if trimmed == "" {
			if isTextBlock {
				entries = append(entries, &BlockEntry{
					Type:  "text",
					Key:   "",
					Value: "",
					Line:  absoluteLineNum,
				})
			}
			i++
			continue
		}

		// 列表项：以 "-" 开头
		if strings.HasPrefix(trimmed, "-") {
			entry, err := parseListLine(trimmed, absoluteLineNum)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry)
			i++
			continue
		}

		// 规则项：以 "[" 开头且包含 "if"
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "if") {
			entry, err := parseRuleLine(trimmed, absoluteLineNum)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry)
			i++
			continue
		}

		// 普通文本（保留原始行，包括缩进）
		entries = append(entries, &BlockEntry{
			Type:  "text",
			Key:   "",
			Value: line, // 保留原样
			Line:  absoluteLineNum,
		})
		i++
	}
	return entries, nil
}

// parseListLine 解析列表行（格式：- 键: 值）
// 值中允许包含冒号（只按第一个冒号分割）
func parseListLine(line string, lineNum int) (*BlockEntry, error) {
	rest := strings.TrimSpace(line)
	if !strings.HasPrefix(rest, "-") {
		return nil, fmt.Errorf("第 %d 行: 列表项必须以 '-' 开头", lineNum)
	}
	rest = strings.TrimPrefix(rest, "-")
	rest = strings.TrimSpace(rest)

	if rest == "" {
		return nil, fmt.Errorf("第 %d 行: 列表项内容为空", lineNum)
	}

	key, value, found := splitKeyValue(rest)
	if !found {
		return nil, fmt.Errorf("第 %d 行: 列表项格式错误，缺少 ':' 或 '：'", lineNum)
	}

	return &BlockEntry{
		Type:  "list",
		Key:   strings.TrimSpace(key),
		Value: strings.TrimSpace(value),
		Line:  lineNum,
	}, nil
}

// splitKeyValue 尝试用多种分隔符分割键值对
// 支持：": "、":"、"："（中文冒号）
func splitKeyValue(s string) (key, value string, ok bool) {
	for _, sep := range []string{": ", ":", "："} {
		if idx := strings.Index(s, sep); idx != -1 {
			key = s[:idx]
			value = s[idx+len(sep):]
			return key, value, true
		}
	}
	return "", "", false
}

// parseRuleLine 解析规则行（格式：[名称] if 条件 -> 动作）
func parseRuleLine(line string, lineNum int) (*BlockEntry, error) {
	line = strings.TrimSpace(line)

	if !strings.HasPrefix(line, "[") {
		return nil, fmt.Errorf("第 %d 行: 规则格式错误，缺少 '['", lineNum)
	}
	endBracket := strings.Index(line, "]")
	if endBracket == -1 {
		return nil, fmt.Errorf("第 %d 行: 规则格式错误，缺少 ']'", lineNum)
	}

	name := line[1:endBracket]
	if name == "" {
		return nil, fmt.Errorf("第 %d 行: 规则名不能为空", lineNum)
	}

	rest := strings.TrimSpace(line[endBracket+1:])
	if !strings.HasPrefix(rest, "if ") {
		return nil, fmt.Errorf("第 %d 行: 规则格式错误，缺少 'if'", lineNum)
	}

	conditionPart := rest[3:]
	arrowIdx := strings.Index(conditionPart, " -> ")
	if arrowIdx == -1 {
		return nil, fmt.Errorf("第 %d 行: 规则格式错误，缺少 '->'", lineNum)
	}

	condition := strings.TrimSpace(conditionPart[:arrowIdx])
	action := strings.TrimSpace(conditionPart[arrowIdx+4:])

	if condition == "" || action == "" {
		return nil, fmt.Errorf("第 %d 行: 规则条件和动作不能为空", lineNum)
	}

	return &BlockEntry{
		Type:  "rule",
		Key:   name,
		Value: condition + " -> " + action,
		Line:  lineNum,
	}, nil
}
