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
	"mephisto/internal/domain"
	"strings"
)

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

// parseKeyValuePairs 是底层通用解析函数。
// 遍历行，提取 "- key: value" 格式的键值对，返回原始字符串键值对列表，且 key 不能为空。
// 格式：
//   - 键: 值
//   - 键：值
//
// 空行和以 # 开头的行会被忽略。
func parseKeyValuePairs(lines []Line, blockName string) ([]domain.KeyValue, error) {
	var result []domain.KeyValue

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)

		// 跳过空行和注释行
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 必须以 "-" 开头
		if !strings.HasPrefix(trimmed, "-") {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：列表项必须以 '-' 开头", line.Number, blockName)
		}

		rest := strings.TrimPrefix(trimmed, "-")
		rest = strings.TrimSpace(rest)

		if rest == "" {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：列表项内容为空", line.Number, blockName)
		}

		key, value, ok := splitKeyValue(rest)
		if !ok {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：键值对格式错误，缺少 ':' 或 '：'", line.Number, blockName)
		}
		if key == "" {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：键不能为空", line.Number, blockName)
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
	return "", fmt.Errorf("第 %d 行：角色名不能为空", blockLine)
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

// parseKeyValueList 解析键值对列表（锚点、校验）。
// 直接复用 parseKeyValuePairs。
func parseKeyValueList(lines []Line, blockName string) ([]domain.KeyValue, error) {
	return parseKeyValuePairs(lines, blockName)
}

// parseStateBlock 解析【状态】区块。
// 复用 parseKeyValuePairs，然后对值自动转换为合适的类型（bool/int/float/string）。
//
// 为什么状态需要类型转换？
//
//	状态的值是动态变量（如情绪、生命值、位置），
//	在引擎中需要参与逻辑判断（如 生命值 > 0），
//	因此需要以正确的类型存储，而不是全部作为字符串。
func parseStateBlock(lines []Line, blockName string) ([]domain.KeyValue, error) {
	pairs, err := parseKeyValuePairs(lines, blockName)
	if err != nil {
		return nil, err
	}
	return pairs, nil
}

// parsePlainList 解析纯文本列表（【记忆】）。
// 格式：
//   - 条目1
//   - 条目2
//
// 不解析键值对，整行内容作为字符串值。
// 记忆是系统区块，由程序自动生成，但解析器保留读取能力（用于加载子版）。
func parsePlainList(lines []Line, blockName string) ([]string, error) {
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)

		// 跳过空行和注释行
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 必须以 "-" 开头
		if !strings.HasPrefix(trimmed, "-") {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：列表项必须以 '-' 开头", line.Number, blockName)
		}

		value := strings.TrimPrefix(trimmed, "-")
		value = strings.TrimSpace(value)

		if value == "" {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：列表项内容为空", line.Number, blockName)
		}

		result = append(result, value)
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

		// parseRuleLine 解析单行规则
		rule, err := parseRuleLine(trimmed, line.Number, blockName)
		if err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, nil
}

// parseRuleLine 解析单行规则。
// 拆分为独立的函数，便于 parseRules 调用和单元测试。
func parseRuleLine(line string, lineNumber int, blockName string) (*domain.Rule, error) {
	trimmed := strings.TrimSpace(line)

	// 规则必须以 [ 开头
	if !strings.HasPrefix(trimmed, "[") {
		return nil, fmt.Errorf("第 %d 行（区块「%s」）：规则必须以 '[' 开头", lineNumber, blockName)
	}

	// 找闭合的 ]
	index := strings.Index(trimmed, "]")
	if index == -1 {
		return nil, fmt.Errorf("第 %d 行（区块「%s」）：规则缺少闭合的 ']'", lineNumber, blockName)
	}

	// 提取规则名
	name := strings.TrimSpace(trimmed[1:index])
	if name == "" {
		return nil, fmt.Errorf("第 %d 行（区块「%s」）：规则名不能为空", lineNumber, blockName)
	}

	// 提取条件和动作
	rest := strings.TrimSpace(trimmed[index+1:])
	if !strings.HasPrefix(rest, "if ") {
		return nil, fmt.Errorf("第 %d 行（区块「%s」）：规则条件必须以 'if ' 开头", lineNumber, blockName)
	}

	// 移除 if 前缀
	rest = strings.TrimPrefix(rest, "if")
	rest = strings.TrimSpace(rest)

	// 取第一个 "->" 分割条件和动作。
	// 动作中可能包含 "->"，取第一个能保证条件完整；
	// 条件中若包含 "->" 则会被误分割，但实际规则中极少出现，这是设计上的取舍。
	cond, action, ok := strings.Cut(rest, "->")
	if !ok {
		return nil, fmt.Errorf("第 %d 行（区块「%s」）：规则缺少 '->'", lineNumber, blockName)
	}

	// 提取条件和动作
	cond = strings.TrimSpace(cond)
	action = strings.TrimSpace(action)
	if cond == "" || action == "" {
		return nil, fmt.Errorf("第 %d 行（区块「%s」）：规则的条件或动作不能为空", lineNumber, blockName)
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
	var result []domain.HistoryEntry

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)

		// 跳过空行和注释行
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 必须以 "-" 开头
		if !strings.HasPrefix(trimmed, "-") {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：列表项必须以 '-' 开头", line.Number, blockName)
		}

		// 去掉 "- " 前缀
		rest := strings.TrimPrefix(trimmed, "-")
		rest = strings.TrimSpace(rest)

		if rest == "" {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：历史条目内容为空", line.Number, blockName)
		}

		// ---- 关键修复：硬匹配 fate: 或 assistant: 前缀 ----
		// 而不是使用 splitKeyValue（会被内容中的冒号干扰）
		var role, contentText string
		var ok bool

		if strings.HasPrefix(rest, "fate:") || strings.HasPrefix(rest, "fate：") {
			role = "fate"
			contentText = strings.TrimPrefix(strings.TrimPrefix(rest, "fate:"), "fate：")
			contentText = strings.TrimSpace(contentText)
			ok = true
		} else if strings.HasPrefix(rest, "assistant:") || strings.HasPrefix(rest, "assistant：") {
			role = "assistant"
			contentText = strings.TrimPrefix(strings.TrimPrefix(rest, "assistant:"), "assistant：")
			contentText = strings.TrimSpace(contentText)
			ok = true
		}

		if !ok {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：历史条目必须以 'fate:' 或 'assistant:' 开头", line.Number, blockName)
		}

		if role == "" || contentText == "" {
			return nil, fmt.Errorf("第 %d 行（区块「%s」）：角色或内容不能为空", line.Number, blockName)
		}

		// 反转义换行符
		contentText = strings.ReplaceAll(contentText, "\\n", "\n")

		result = append(result, domain.HistoryEntry{
			Role:    role,
			Content: contentText,
		})
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

	for _, block := range blocks {
		switch block.Title {
		case "角色名":
			value, err := parseRoleName(block.Content, block.Line)
			if err != nil {
				return nil, err
			}
			contract.RoleName = value
		case "锚点":
			value, err := parseKeyValueList(block.Content, block.Title)
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
			value, err := parseStateBlock(block.Content, block.Title)
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
		case "校验":
			value, err := parseKeyValueList(block.Content, block.Title)
			if err != nil {
				return nil, err
			}
			contract.Validation = value
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
			// 未知区块：理论上不会发生，因为 isKnownBlock 已过滤。
			// 静默忽略，保持向前兼容。
			continue
		}
	}
	return contract, nil
}
