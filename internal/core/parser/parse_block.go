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
func parseKeyValue(lines []Line, blockName string) ([]domain.StateItem, error) {
	entries, err := scanEntries(lines, blockName)
	if err != nil {
		return nil, err
	}

	var result []domain.StateItem
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
		result = append(result, domain.StateItem{Key: key, Value: domain.ParseStateValue(value)})
	}
	return result, nil
}

// ============================================================
// 各区块解析函数
// ============================================================

// parsePlainText 解析纯文本系统区块（如 `@命运`）：取首行非空内容作为值；无内容返回空字符串。
//
// 与 Flutter meph_parser.dart 的 _parsePlainText 对齐。
func parsePlainText(lines []Line) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}
	return ""
}

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

// mergeUnknownIntoText 将未知区块合并为文本行（标题行 + 原内容行），
// 供并入前一个文本型区块（对齐 Flutter meph_parser.dart 的 _mergeUnknownIntoText）。
//
// 标题行以 `【标题】` 形式保留在文本中——它本来就是用户散文的一部分。
func mergeUnknownIntoText(block Block) string {
	var sb strings.Builder
	sb.WriteString("【" + block.Title + "】")
	for _, line := range block.Content {
		sb.WriteByte('\n')
		sb.WriteString(line.Text)
	}
	return sb.String()
}

// parsePlainList 解析纯文本列表（【记忆】）。
//
// 格式（与 Flutter meph_parser.dart 的 _parsePlainList 对齐）：
//   - 条目1
//   - 条目2
//   - [4] 带权重前缀的条目
//
// 不解析键值对，整行内容作为字符串值。
//
// 记忆条目前缀（极简设计，方案 B：仅提取内容，不保存权重）：
//   - `[权重] 内容`（如 `[4] 浮士德与梅菲斯特立下赌约`）→ 提取 `]` 后的内容
//     作为记忆条目（权重值内部忽略，保留前缀仅供 Flutter 版/编辑器兼容）
//   - 无前缀的旧格式条目 → 原样保留
//
// 记忆是系统区块，由程序自动生成，但解析器保留读取能力（用于加载子版）。
func parsePlainList(lines []Line, blockName string) ([]string, error) {
	entries, err := scanEntries(lines, blockName)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, stripMemoryWeightPrefix(entry.Raw))
	}
	return result, nil
}

// memoryWeightPrefixPattern 匹配记忆条目开头的 `[权重] ` 前缀（如 `[4] `）。
//
// 权重为 1-5 的单个数字（与 Flutter 版 Memory.maxImportance=5 对齐）。
// 方案 B：CLI 内部不保存权重值，仅提取 `]` 之后的内容作为记忆条目。
var memoryWeightPrefixPattern = regexp.MustCompile(`^\[\d\]\s+(.+)$`)

// stripMemoryWeightPrefix 从记忆条目中提取「去掉 [权重] 前缀后的内容」。
//
// 带 `[权重] ` 前缀：返回 `]` 之后的内容（如 `[4] 记忆内容` → `记忆内容`）。
// 无前缀/无法识别前缀：原样返回（向后兼容旧格式）。
func stripMemoryWeightPrefix(raw string) string {
	if m := memoryWeightPrefixPattern.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	return raw
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

	// 互斥组（可选）：动作以 [group:组名] 开头时剥离
	// 缺少闭合的 "]" 时剥离异常，静默导致组名解析错误，必须尽早报错
	group := ""
	if strings.Contains(action, "[group:") && !strings.Contains(action, "]") {
		return nil, &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "互斥组应写为 [group:组名]（缺少闭合的 \"]\"）",
		}
	}
	groupPrefix := "[group:"
	if strings.HasPrefix(action, groupPrefix) {
		endIndex := strings.Index(action, "]")
		if endIndex != -1 {
			group = action[len(groupPrefix):endIndex]
			action = strings.TrimSpace(action[endIndex+1:])
		}
	}

	// 校验规则语法（运算符空格、关键词空格、roll 格式、括号匹配、&& 分隔符）
	if err := validateRuleSyntax(cond, action, lineNumber, blockName); err != nil {
		return nil, err
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
		State: []domain.StateItem{},
		// 其他切片类型统一为 nil（序列化时会被 omitempty 忽略）
	}

	seenBlocks := make(map[string]struct{})
	// 最近处理的「文本型区块」类型（世界观 / 角色背景 / 开局场景 / 空）。
	// 用于未知区块的内容归并（对齐 Flutter meph_parser.dart 的
	// _mergeUnknownIntoText）：散文区块中的一行 `【传说】` 会被 Lexer 切为
	// 未知区块，若静默忽略，其后的全部文本会从世界观/背景/开局场景中丢失。
	// 归并后保留标题行 + 内容行，用户散文原样不丢。
	lastTextBlock := ""

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
			lastTextBlock = ""
		case "锚点":
			value, err := parseKeyValue(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Anchor = value
			lastTextBlock = ""
		case "世界观":
			contract.Worldview = parseTextBlock(block.Content)
			lastTextBlock = "世界观"
		case "角色背景":
			contract.Background = parseTextBlock(block.Content)
			lastTextBlock = "角色背景"
		case "开局场景":
			contract.Opening = parseTextBlock(block.Content)
			lastTextBlock = "开局场景"
		case "状态":
			value, err := parseKeyValue(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.State = value
			lastTextBlock = ""
		case "规则":
			value, err := parseRules(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Rules = value
			lastTextBlock = ""
		case "记忆":
			value, err := parsePlainList(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Memories = value
			lastTextBlock = ""
		case "@命运":
			contract.BranchTitle = parsePlainText(block.Content)
			lastTextBlock = ""
		case "历史":
			value, err := parseHistory(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.History = value
			lastTextBlock = ""
		default:
			// 未知区块（用户草稿/备忘）：静默忽略（草稿宽容）。
			// 例外：紧跟在文本型区块之后时，把标题行 + 内容行**并入**该区块
			// 文本——散文正文中出现的一行 `【xxx】` 不应导致其后内容丢失
			// （对齐 Flutter _mergeUnknownIntoText）。
			// 结构化区块（规则/记忆/历史等）后的未知区块仍忽略（避免脏行
			// 混入结构化解析导致误报）。
			if lastTextBlock != "" {
				merged := mergeUnknownIntoText(block)
				switch lastTextBlock {
				case "世界观":
					contract.Worldview += "\n" + merged
				case "角色背景":
					contract.Background += "\n" + merged
				default:
					contract.Opening += "\n" + merged
				}
				// lastTextBlock 保持不变：后续未知区块继续并入同一文本区块
			}
		}
	}
	
	return contract, nil
}
