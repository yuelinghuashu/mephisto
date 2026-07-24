// internal/core/parser/parse_block.go
//
// 本文件负责将 lexer 输出的原始区块（[]Block）解析为结构化的 domain.Contract。
//
// 设计理念：
//  1. 每个区块的解析逻辑独立为一个函数，职责单一，便于测试和修改。
//  2. 内容行在 lexer 阶段已附带绝对行号（Line.Number），
//     解析器直接使用，无需自行计算。
//  3. 错误信息精确到内容行号，并附带区块名，便于用户快速定位问题。
package parser

import (
	"fmt"
	"regexp"

	"mephisto/internal/domain"
	"mephisto/internal/shared"
	"strings"
)

// Entry 表示一个解析后的列表条目。
type Entry struct {
	Raw  string // 去掉 "- " 后的原始内容
	Line int    // 源文件行号
}

// scanEntries 是通用的列表条目扫描器。
// 负责处理所有 "- " 列表的通用逻辑：去空行、去注释、校验前缀。
// 返回的 Entry 列表供上层进一步解析（键值对、纯文本、历史等）。
func scanEntries(lines []Line, blockName string) ([]Entry, error) {
	var entries []Entry

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)

		// 跳过空行和注释
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 必须以 "-" 开头
		if !strings.HasPrefix(trimmed, "-") {
			return nil, &shared.ParseError{
				Line:      line.Number,
				BlockName: blockName,
				Message:   "列表项必须以 '-' 开头",
			}
		}

		rest := strings.TrimPrefix(trimmed, "-")
		rest = strings.TrimSpace(rest)

		if rest == "" {
			return nil, &shared.ParseError{
				Line:      line.Number,
				BlockName: blockName,
				Message:   "列表项内容为空",
			}
		}

		entries = append(entries, Entry{
			Raw:  rest,
			Line: line.Number,
		})
	}

	return entries, nil
}

// splitKeyValue 将 "key: value" 格式的字符串分割为 key 和 value。
//
// 支持的分隔符：
//   - 中文冒号 "："
//   - 英文冒号 ":"
//
// 冒号前后的空格会被自动去除（通过 TrimSpace）。
// 取最靠左的冒号作为分隔符，避免 value 中的冒号被误分割。
//
// 返回值：(key, value, ok)，ok=false 表示未找到任何分隔符。
func splitKeyValue(s string) (string, string, bool) {
	// 查找第一个中文冒号或英文冒号
	if key, value, ok := strings.Cut(s, "："); ok {
		return strings.TrimSpace(key), strings.TrimSpace(value), true
	}
	if key, value, ok := strings.Cut(s, ":"); ok {
		return strings.TrimSpace(key), strings.TrimSpace(value), true
	}
	return "", "", false
}

// parseKeyValue 是底层通用解析函数。
// 遍历行，提取 "- key: value" 格式的键值对，返回原始字符串键值对列表，且 key 不能为空。
// 格式：
//   - 键: 值
//   - 键：值
//
// 空行和以 # 开头的行会被忽略。
func parseKeyValue(lines []Line, blockName string) ([]domain.KeyValue, error) {
	entries, err := scanEntries(lines, blockName)
	if err != nil {
		return nil, err
	}

	var result []domain.KeyValue
	for _, entry := range entries {
		// 检查行内 #（键值对中不允许包含裸露的 # 符号）
		if strings.Contains(entry.Raw, "#") {
			return nil, &shared.ParseError{
				Line:      entry.Line,
				BlockName: blockName,
				Message:   "键值对中不允许包含 '#' 符号（注释必须位于行首）",
			}
		}

		key, value, ok := splitKeyValue(entry.Raw)
		if !ok {
			return nil, &shared.ParseError{
				Line:      entry.Line,
				BlockName: blockName,
				Message:   "键值对格式错误，缺少 ':' 或 '：'",
			}
		}
		if key == "" {
			return nil, &shared.ParseError{
				Line:      entry.Line,
				BlockName: blockName,
				Message:   "键不能为空",
			}
		}
		result = append(result, domain.KeyValue{Key: key, Value: value})
	}
	return result, nil
}

// ============================================================
// 各区块解析函数
// ============================================================

// parseRoleName 解析【角色名】区块。
// 格式：单行文本，取第一行非空内容。
//
// 注意：如果区块内容包含多行，只取第一行非空，其余忽略。
// 这是有意设计：角色名应该是简单的标识符，不应包含换行。
func parseRoleName(lines []Line, blockLine int) (string, error) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed != "" {
			return trimmed, nil
		}
	}
	return "", &shared.ParseError{
		Line:    blockLine,
		Message: "角色名不能为空",
	}
}

// parseTextBlock 解析纯文本区块（世界观、角色背景、开局场景）。
// 将内容行拼接为单个字符串，原样保留。
//
// 为什么不在这里做变量替换（如 {角色名}）？
//
//	变量替换是运行时行为，因为 {角色名} 的值可能来自子版加载后的动态数据。
//	解析器只负责"读出来"，不负责"解释"。变量替换由 engine 层完成。
func parseTextBlock(lines []Line) string {
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line.Text)
	}
	return sb.String()
}

// parsePlainList 解析纯文本列表（【记忆】）。
// 格式：
//   - 条目1
//   - 条目2
//
// 不解析键值对，整行内容作为字符串值。
// 记忆是系统区块，由程序自动生成，但解析器保留读取能力（用于加载子版）。
func parsePlainList(lines []Line, blockName string) ([]string, error) {
	entries, err := scanEntries(lines, blockName)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Raw)
	}
	return result, nil
}

// parseRules 解析【规则】区块。
// 格式：
//
//	[规则名] if 条件 -> 动作
//	[规则名] if 条件 -> [group:组名] 动作
//
// 设计决策：
//  1. 规则名必须用 [] 包裹，且不能为空。
//  2. 条件和动作之间用 -> 分隔，取第一个 -> 作为分隔符，
//     避免动作中可能出现的 -> 被误分割。
//  3. 互斥组是可选的，格式为 [group:组名]，写在动作最前面。
//  4. 空行和以 # 开头的行被忽略。
func parseRules(lines []Line, blockName string) ([]*domain.Rule, error) {
	var result []*domain.Rule

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)

		// 跳过空行和注释行
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 检查行内 #（规则语法中不应包含裸露的 # 符号）
		if strings.Contains(trimmed, "#") {
			return nil, &shared.ParseError{
				Line:      line.Number,
				BlockName: blockName,
				Message:   "规则行中不允许包含 '#' 符号（注释必须位于行首）",
			}
		}

		// parseRuleLine 解析单行规则
		rule, err := parseRuleLine(trimmed, line.Number, blockName)
		if err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, nil
}

// ruleNamePattern 匹配规则名的闭合位置：] + 任意空白 + if，不包含 if 后续字符。
// 使用 ]\s*if\b 避免匹配条件或动作中的 if。
var ruleNamePattern = regexp.MustCompile(`\]\s*if\b`)

// parseRuleLine 解析单行规则。
// 拆分为独立的函数，便于 parseRules 调用和单元测试。
func parseRuleLine(line string, lineNumber int, blockName string) (*domain.Rule, error) {
	trimmed := strings.TrimSpace(line)

	// 规则必须以 [ 开头
	if !strings.HasPrefix(trimmed, "[") {
		return nil, &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "规则必须以 '[' 开头",
		}
	}

	// 用正则定位 ] + 任意空白 + if 的位置，精确定位规则名闭合符。
	// 这样即使条件中包含 ] 也不会被误匹配，同时兼容 ]if、] if、]  if 等多种写法。
	loc := ruleNamePattern.FindStringIndex(trimmed)
	if loc == nil || loc[0] == 0 {
		return nil, &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "规则格式错误，需要 '[规则名] if 条件 -> 动作'",
		}
	}

	closeBracket := loc[0] // ] 的位置

	// 提取规则名
	name := strings.TrimSpace(trimmed[1:closeBracket])
	if name == "" {
		return nil, &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "规则名不能为空",
		}
	}

	// 提取条件和动作：跳过 ] 和 if（含中间的空白）
	rest := strings.TrimSpace(trimmed[loc[1]:])
	rest = strings.TrimSpace(rest)

	// 取第一个 "->" 分割条件和动作。
	// 动作中可能包含 "->"，取第一个能保证条件完整；
	// 条件中若包含 "->" 则会被误分割，但实际规则中极少出现，这是设计上的取舍。
	cond, action, ok := strings.Cut(rest, "->")
	if !ok {
		return nil, &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "规则缺少 '->'",
		}
	}

	// 提取条件和动作
	cond = strings.TrimSpace(cond)
	action = strings.TrimSpace(action)
	if cond == "" || action == "" {
		return nil, &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "规则的条件或动作不能为空",
		}
	}

	// 提取互斥组（可选）
	group := ""
	if strings.HasPrefix(action, "[group:") {
		endIndex := strings.Index(action, "]")
		if endIndex != -1 {
			group = action[7:endIndex]
			action = strings.TrimSpace(action[endIndex+1:])
		}
	}

	return &domain.Rule{
		Name:   name,
		Cond:   cond,
		Action: action,
		Group:  group,
		Line:   lineNumber,
	}, nil
}

// parseHistory 解析【历史】区块。
// 格式：
//   - fate: 内容       ← 命运的指令
//   - assistant: 内容  ← 角色的回应
//
// 角色必须是 fate 或 assistant。
// 内容支持 \n 转义，会被还原为真正的换行符。
//
// 历史是系统区块，由程序自动记录对话历史。
// 为什么支持 \n 转义？因为历史内容可能包含多行文本，
// 但 .meph 是纯文本格式，用 \n 表示换行是通用的做法。
func parseHistory(lines []Line, blockName string) ([]domain.HistoryEntry, error) {
	entries, err := scanEntries(lines, blockName)
	if err != nil {
		return nil, err
	}

	var result []domain.HistoryEntry
	for _, entry := range entries {
		var role, content string
		var ok bool

		if strings.HasPrefix(entry.Raw, "fate:") || strings.HasPrefix(entry.Raw, "fate：") {
			role = "fate"
			content = strings.TrimPrefix(strings.TrimPrefix(entry.Raw, "fate:"), "fate：")
			content = strings.TrimSpace(content)
			ok = true
		} else if strings.HasPrefix(entry.Raw, "assistant:") || strings.HasPrefix(entry.Raw, "assistant：") {
			role = "assistant"
			content = strings.TrimPrefix(strings.TrimPrefix(entry.Raw, "assistant:"), "assistant：")
			content = strings.TrimSpace(content)
			ok = true
		}

		if !ok {
			return nil, &shared.ParseError{
				Line:      entry.Line,
				BlockName: blockName,
				Message:   "历史条目必须以 'fate:' 或 'assistant:' 开头",
			}
		}

		content = strings.ReplaceAll(content, "\\n", "\n")
		result = append(result, domain.HistoryEntry{Role: role, Content: content})
	}
	return result, nil
}

// ============================================================
// 主解析函数
// ============================================================

// parseBlocks 将 lexer 输出的 []Block 解析为 *domain.Contract。
//
// 路由策略：
//
//	根据 Block.Title 将解析任务分发给对应的解析函数。
//	未知标题（由于 isKnownBlock 已过滤，理论上不会出现）被静默忽略。
func parseBlocks(blocks []Block) (*domain.Contract, error) {
	contract := &domain.Contract{
		State: []domain.KeyValue{},
		// 其他切片类型统一为 nil（序列化时会被 omitempty 忽略）
	}

	seenBlocks := make(map[string]struct{})

	for _, block := range blocks {
		// 检测重复区块
		if _, ok := seenBlocks[block.Title]; ok {
			return nil, &shared.ParseError{
				Line:      block.Line,
				BlockName: block.Title,
				Message:   fmt.Sprintf("重复的区块「%s」", block.Title),
			}
		}
		seenBlocks[block.Title] = struct{}{}

		switch block.Title {
		case "角色名":
			value, err := parseRoleName(block.Content, block.Line)
			if err != nil {
				return nil, err
			}
			contract.RoleName = value
		case "锚点":
			value, err := parseKeyValue(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Anchor = value
		case "世界观":
			contract.Worldview = parseTextBlock(block.Content)
		case "角色背景":
			contract.Background = parseTextBlock(block.Content)
		case "开局场景":
			contract.Opening = parseTextBlock(block.Content)
		case "状态":
			value, err := parseKeyValue(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.State = value
		case "规则":
			value, err := parseRules(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Rules = value
		case "记忆":
			value, err := parsePlainList(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Memories = value
		case "历史":
			value, err := parseHistory(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.History = value
		default:
			// 自定义区块：静默忽略（相当于注释区块）
			// 这些区块通过 MEPHISTO_EXTRA_BLOCKS 环境变量定义，
			// 被 Lexer 识别为合法区块，但不产生任何数据。
			continue
		}
	}
	
	return contract, nil
}
