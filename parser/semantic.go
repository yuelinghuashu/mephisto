// ============================================================
// semantic.go - 语义解析器
// 职责：
// 1. 将每个区块的 Raw 文本按行解析成结构化条目（Entries）
// 2. 识别列表项、规则、普通文本
// 3. 对文本区块保留空行，对结构化区块过滤空行
// 4. 绝对行号（与文件物理行对齐）
// 5. 在解析阶段预编译规则 AST（性能优化）
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
		// 从注册表判断是否为文本区块（SingleLineText 或 MultiLineText）
		spec, ok := BlockRegistry[block.Name]
		if !ok {
			return fmt.Errorf("未知区块类型: %s", block.Name)
		}
		isTextBlock := spec.Type == SingleLineText || spec.Type == MultiLineText

		entries, err := parseBlockContent(block.Raw, block.Line, isTextBlock)
		if err != nil {
			return fmt.Errorf("解析 【%s】 区块失败: %w", block.Name, err)
		}
		block.Entries = entries
	}
	// 扫描外部引用
	pf.ScanReferences()
	return nil
}

// parseBlockContent 解析区块的原始文本（raw），生成条目列表
// 参数：
//   - raw: 区块原始内容（包含注释、空行）
//   - startLine: 区块标题所在行号（用于计算绝对行号）
//   - keepEmptyLines: 是否保留空行（文本区块为 true，结构化区块为 false）
func parseBlockContent(raw string, startLine int, keepEmptyLines bool) ([]*BlockEntry, error) {
	var entries []*BlockEntry
	i := 0

	// strings.Lines 返回一个迭代器，按需生成每一行（Go 1.23+）
	for line := range strings.Lines(raw) {
		// 绝对行号 = 标题行号 + 1（标题行不占 Raw）+ i
		absoluteLineNum := startLine + 1 + i
		trimmed := strings.TrimSpace(line)

		// 跳过注释行（行号已保留）
		if strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		// 空行处理
		if trimmed == "" {
			if keepEmptyLines {
				// 文本区块：保留空行（作为 Value="" 的 text 条目）
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

		// 规则项：以 "[" 开头且包含 "if" 和 "->"
		if strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "if") && strings.Contains(trimmed, "->") {
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
// 优化：取所有分隔符中索引最小的，解决中英文冒号混用问题
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

	// 优化后的 splitKeyValue：取所有分隔符中索引最小的
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
// 优化：取所有分隔符中首次出现位置最靠左的那个
// 这样即使 Value 中包含冒号，也不会误分割
func splitKeyValue(s string) (key, value string, ok bool) {
	seps := []string{": ", ":", "："}
	firstIdx := -1
	sepLen := 0

	for _, sep := range seps {
		idx := strings.Index(s, sep)
		if idx != -1 {
			if firstIdx == -1 || idx < firstIdx {
				firstIdx = idx
				sepLen = len(sep)
			}
		}
	}

	if firstIdx != -1 {
		return s[:firstIdx], s[firstIdx+sepLen:], true
	}
	return "", "", false
}

// parseRuleLine 解析规则行（格式：[名称] if 条件 -> 动作）
// 在解析阶段预编译 AST，避免运行时重新解析
// 这样每次对话执行规则时，直接执行预编译好的表达式树，性能提升巨大
// 注意：使用全局 ParseExprFunc（由 engine 包注册），避免循环依赖
func parseRuleLine(line string, lineNum int) (*BlockEntry, error) {
	line = strings.TrimSpace(line)

	// 检查是否以 "[" 开头
	if !strings.HasPrefix(line, "[") {
		return nil, fmt.Errorf("第 %d 行: 规则必须以 '[' 开头", lineNum)
	}
	// 查找 "]"
	endIdx := strings.Index(line, "]")
	if endIdx == -1 {
		return nil, fmt.Errorf("第 %d 行: 规则缺少闭合的 ']'", lineNum)
	}

	// 提取规则名
	name := strings.TrimSpace(line[1:endIdx])
	if name == "" {
		return nil, fmt.Errorf("第 %d 行: 规则名不能为空", lineNum)
	}

	// 提取剩余部分
	rest := strings.TrimSpace(line[endIdx+1:])

	// 使用 CutPrefix 替代 HasPrefix + TrimPrefix
	condPart, ok := strings.CutPrefix(rest, "if ")
	if !ok {
		return nil, fmt.Errorf("第 %d 行: 规则条件必须以 'if ' 开头", lineNum)
	}
	rest = strings.TrimSpace(condPart)

	// 分割条件和动作（取第一个 "->"）
	// 用 Cut 替代 SplitN
	condStr, actionStr, ok := strings.Cut(rest, "->")
	if !ok {
		return nil, fmt.Errorf("第 %d 行: 规则格式错误，缺少 '->'", lineNum)
	}
	condStr = strings.TrimSpace(condStr)
	actionStr = strings.TrimSpace(actionStr)

	if condStr == "" || actionStr == "" {
		return nil, fmt.Errorf("第 %d 行: 规则的条件或动作不能为空", lineNum)
	}

	// 检查 ParseExprFunc 是否已注册
	if ParseExprFunc == nil {
		return nil, fmt.Errorf("第 %d 行: 表达式解析函数未注册（engine 包未初始化）", lineNum)
	}

	// 预编译 AST：调用注册的解析函数
	expr, err := ParseExprFunc(condStr)
	if err != nil {
		return nil, fmt.Errorf("第 %d 行: 编译条件失败: %w", lineNum, err)
	}

	return &BlockEntry{
		Type:       "rule",
		Key:        name,
		Value:      condStr, // 保留原始条件字符串，供展示/调试
		Line:       lineNum,
		RuleExpr:   expr,      // 预编译的 AST 表达式树（parser.Expr 类型）
		RuleAction: actionStr, // 预编译的动作字符串
	}, nil
}

// ScanReferences 扫描所有区块内容中的 @别名 引用
// 使用 strings.FieldsSeq 迭代字段，避免分配临时切片（Go 1.23+）
// 优化：增加非空和非重复校验
func (pf *ParsedFile) ScanReferences() {
	seen := make(map[string]bool)
	for _, block := range pf.Blocks {
		for w := range strings.FieldsSeq(block.Raw) {
			if strings.HasPrefix(w, "@") && len(w) > 1 {
				ref := strings.TrimLeft(w, "@")
				// 去除常见的标点符号
				ref = strings.TrimFunc(ref, func(r rune) bool {
					return r == ',' || r == '.' || r == '?' || r == '!' ||
						r == ';' || r == '"' || r == '\'' || r == ')' ||
						r == ']' || r == '}' || r == '：'
				})
				// 非空且未重复才添加
				if ref != "" && !seen[ref] {
					seen[ref] = true
					pf.References = append(pf.References, ref)
				}
			}
		}
	}
}
