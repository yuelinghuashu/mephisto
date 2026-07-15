// ============================================================
// block.go - 区块分割器
// 职责：
// 1. 读取文件并处理 BOM
// 2. 按 【区块名】 分割成独立区块（保留注释行以维持行号对齐）
// 3. 验证标题格式、白名单、重复和必填区块
// 这是解析的第一阶段：文件 → 区块列表（Raw 文本）
// ============================================================

package parser

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode"
)

// ParseFile 是解析器的入口函数，串联整个流程
func ParseFile(filename string) (*ParsedFile, error) {
	lines, err := readLines(filename)
	if err != nil {
		return nil, err
	}

	blocks, err := splitBlocks(lines)
	if err != nil {
		return nil, err
	}

	if err := validateBlocks(blocks); err != nil {
		return nil, err
	}

	pf := &ParsedFile{Blocks: blocks}

	// 语义解析
	if err := pf.ParseSemantics(); err != nil {
		return nil, err
	}
	// 类型验证
	if err := pf.Validate(); err != nil {
		return nil, err
	}

	return pf, nil
}

// readLines 读取文件所有行，并处理 UTF-8 BOM
func readLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件：%w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()
		if firstLine {
			// 去掉 UTF-8 BOM (EF BB BF)
			if len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
				line = line[3:]
			}
			firstLine = false
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件时出错：%w", err)
	}
	return lines, nil
}

// normalizeBlockTitle 检查一行是否为有效的区块标题，并返回清理后的标题行
func normalizeBlockTitle(line string) (clean string, ok bool) {
	if !strings.HasPrefix(line, "【") {
		return "", false
	}
	before, after, found := strings.Cut(line, "】")
	if !found {
		return "", false
	}
	// 清理零宽空格等不可见字符
	after = strings.ReplaceAll(after, "\u200B", "")
	after = strings.ReplaceAll(after, "\uFEFF", "")
	if strings.TrimFunc(after, unicode.IsSpace) != "" {
		return "", false
	}
	return before + "】", true
}

// splitBlocks 将行列表分割成区块列表
// 注释行被保留在 Raw 中以保证行号对齐
func splitBlocks(lines []string) ([]*ParsedBlock, error) {
	var blocks []*ParsedBlock
	var currentBlock *ParsedBlock
	var contentLines []string

	for lineNum, line := range lines {
		cleanTitle, isTitle := normalizeBlockTitle(line)
		if isTitle {
			if currentBlock != nil {
				currentBlock.Raw = strings.Join(contentLines, "\n")
				blocks = append(blocks, currentBlock)
			}
			block, err := validateAndCreateBlock(cleanTitle, lineNum+1)
			if err != nil {
				return nil, err
			}
			currentBlock = block
			contentLines = []string{}
			continue
		}

		// ---- 非标题行 ----
		trimmed := strings.TrimSpace(line)

		// 注释行：原样保留（维持行号对齐）
		if strings.HasPrefix(trimmed, "#") {
			contentLines = append(contentLines, line)
			continue
		}

		// 只有以 "【" 开头的行才检查格式错误
		if strings.HasPrefix(line, "【") {
			return nil, fmt.Errorf("第 %d 行：格式错误的标题，请检查是否缺少 '】' 或标题后有文字", lineNum+1)
		}

		if currentBlock == nil {
			return nil, fmt.Errorf("第 %d 行: 内容出现在任何区块之外", lineNum+1)
		}
		contentLines = append(contentLines, line)
	}

	if currentBlock != nil {
		currentBlock.Raw = strings.Join(contentLines, "\n")
		blocks = append(blocks, currentBlock)
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("文件中没有区块")
	}
	return blocks, nil
}

// validateAndCreateBlock 验证标题行格式并创建 ParsedBlock
func validateAndCreateBlock(line string, lineNum int) (*ParsedBlock, error) {
	if strings.Count(line, "【") > 1 || strings.Count(line, "】") > 1 {
		return nil, fmt.Errorf("第 %d 行：一行内只能有一个区块标题", lineNum)
	}

	_, after, found := strings.Cut(line, "】")
	if found {
		after = strings.ReplaceAll(after, "\u200B", "")
		after = strings.ReplaceAll(after, "\uFEFF", "")
		after = strings.TrimFunc(after, unicode.IsSpace)
		if after != "" {
			return nil, fmt.Errorf("第 %d 行：区块标题后面不能跟其他文字", lineNum)
		}
	}

	name := strings.TrimPrefix(line, "【")
	name = strings.TrimSuffix(name, "】")
	if name == "" {
		return nil, fmt.Errorf("第 %d 行：区块名不能为空", lineNum)
	}

	// 白名单检查（手动收集 keys 再排序，兼容 Go 1.21+）
	if _, ok := BlockRegistry[name]; !ok {
		allowed := make([]string, 0, len(BlockRegistry))
		for k := range BlockRegistry {
			allowed = append(allowed, k)
		}
		slices.Sort(allowed)
		return nil, fmt.Errorf("第 %d 行：未知区块名 %q，允许的名称有：%s",
			lineNum, name, strings.Join(allowed, "、"))
	}

	return &ParsedBlock{Name: name, Line: lineNum}, nil
}

// validateBlocks 执行区块级别的全局检查：必填、重复
func validateBlocks(blocks []*ParsedBlock) error {
	seen := make(map[string]int)

	for _, b := range blocks {
		if firstLine, ok := seen[b.Name]; ok {
			return fmt.Errorf("重复区块: 【%s】（第一次出现于第 %d 行，第二次出现于第 %d 行）",
				b.Name, firstLine, b.Line)
		}
		seen[b.Name] = b.Line
	}

	// 对特定区块检查内容是否为空
	// 这些区块是“必须有内容”的
	mustHaveContent := map[string]bool{
		"角色名":  true,
		"世界观":  true,
		"角色背景": true,
	}

	// 检查所有必填区块是否存在
	for _, b := range blocks {
		// 检查区块是否为空
		if mustHaveContent[b.Name] {
			hasContent := false
			for _, line := range strings.Split(b.Raw, "\n") {
				if strings.TrimSpace(line) != "" {
					hasContent = true
					break
				}
			}
			if !hasContent {
				return fmt.Errorf("【%s】区块内容不能为空（起始于第 %d 行）", b.Name, b.Line)
			}
		}
	}
	return nil
}
