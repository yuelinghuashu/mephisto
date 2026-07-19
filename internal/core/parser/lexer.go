// internal/core/parser/lexer.go
//
// 本文件负责将 .meph 格式的原始文本切分为区块（Block）列表。
// 这是解析流程的第一阶段（词法分析 / 区块切分）。
//
// 设计理念：
//  1. 区块以 【标题】 格式的行作为分隔符。
//  2. 每个区块包含标题、内容行列表（带绝对行号）、标题行号。
//  3. 区块外的内容被视为格式错误，只有空行被允许作为视觉分隔。
//  4. 已知区块名被严格限制（白名单），避免拼写错误导致的隐式 bug。
//  5. 内容行在 lexer 阶段就带上绝对行号，避免 parser 重复计算。
//
// 与 parse_block.go 的分工：
//
//	Lexer（本文件）  ：负责"切分"——识别标题，将内容按行分组。
//	Parser（parse_block.go）：负责"解析"——将原始行内容转换为结构化数据。
//
// 这种分层设计使得"切分"和"解析"职责分离，便于独立测试和修改。
package parser

import (
	"fmt"
	"strings"
)

// Line 表示带行号的内容行。
//
// 为什么要在 lexer 阶段就记录行号？
//
//	在重构前，行号由 parser 自行计算（baseLine + 偏移量），
//	导致 parser 需要维护复杂的迭代器状态。
//	现在由 lexer 在扫描时直接记录绝对行号，parser 拿到 Line 后
//	直接使用 Line.Number 报告错误，无需任何计算。
//
// 字段说明：
//
//	Text   - 行的原始文本（保留缩进和空格）
//	Number - 该行在源文件中的绝对行号（从 1 开始）
type Line struct {
	Text   string
	Number int
}

// Block 表示一个切分后的区块（未解析内容）。
//
// 区块是 .meph 文件的基本组织单位：
//
//	【标题】      ← 标题行（本行不进入 Content）
//	内容行1      ← 进入 Content[0]
//	内容行2      ← 进入 Content[1]
//	【下一个标题】 ← 结束当前区块
//
// 与重构前的区别：
//
//	重构前 Content 是 string，行号由 parser 计算。
//	重构后 Content 是 []Line，每行自带绝对行号。
//
// 字段说明：
//
//	Title   - 区块标题，如 "角色名"、"锚点"。
//	          由 isBlockTitle 提取并经过白名单校验。
//	Content - 区块的内容行列表（不含标题行）。
//	          每行都带有源文件中的绝对行号。
//	Line    - 标题行在源文件中的绝对行号（快速参考）。
//	          等于 Content[0].Number - 1（如果 Content 非空）。
type Block struct {
	Title   string
	Content []Line
	Line    int
}

// knownBlocks 是已知区块名的白名单集合。
//
// 为什么使用 map[string]bool 作为白名单？
//  1. 显式列出所有合法区块名，防止拼写错误导致的隐式 bug。
//     （例如用户写了 "脚色名" 而不是 "角色名"，会被识别为普通内容而非区块）
//  2. O(1) 查找，性能优异。
//  3. 便于维护：新增区块只需在此添加一行。
//
// 如果需要扩展为"注册表模式"（支持自定义区块），
// 可以在此处引入插件机制，但当前需求下白名单已足够。
var knownBlocks = map[string]bool{
	"角色名":  true,
	"锚点":   true,
	"世界观":  true,
	"角色背景": true,
	"开局场景": true,
	"状态":   true,
	"规则":   true,
	"校验":   true,
	"记忆":   true,
	"历史":   true,
}

// Lex 将 .meph 文本切分为区块列表。
//
// 输入：.meph 文件的完整文本内容（UTF-8 编码的字符串）。
// 输出：区块列表 []Block，以及可能的错误。
//
// 处理逻辑（按行扫描状态机）：
//
//	状态：inBlock = false（不在区块内）或 true（正在收集区块内容）
//
//	1. 当前行是标题（【xxx】）：
//	   - 如果 inBlock == true，先保存当前区块
//	   - 开始新区块：记录标题、清空内容缓存、inBlock = true
//	   - 继续下一行
//
//	2. 当前行不是标题：
//	   - 如果 inBlock == false（在区块外）：
//	     - 空行 → 忽略（允许文件前后的空行）
//	     - 非空行 → 报错（内容出现在任何区块之外）
//	   - 如果 inBlock == true（在区块内）：
//	     - 累加当前行到内容缓存（同时记录行号）
//
//	3. 扫描结束后：
//	   - 如果 inBlock == true，保存最后一个区块
//	   - 检查是否至少有一个区块，没有则报错
//
// 边界情况处理：
//   - 文件开头有空行 → 跳过，不报错
//   - 区块之间有多个空行 → 跳过，不影响区块内容
//   - 最后一个区块后有空行 → 跳过，不影响
//   - 标题行后立即跟下一个标题 → 第一个区块的 Content 为空（合法）
//   - 文件中没有任何区块 → 报错 "没有有效区块"
//
// 与重构前的重要变化：
//
//	每个内容行在存储时都附带绝对行号（Line.Number），
//	这使得 parser 无需自行计算行号，简化了 parse_block.go 的逻辑。
func Lex(text string) ([]Block, error) {
	lines := strings.Split(text, "\n")

	var blocks []Block             // 所有已完成的区块
	var currentBlockTitle string   // 当前正在收集的区块标题
	var currentBlockContent []Line // 当前正在收集的区块内容（带行号）
	var currentBlockLine int       // 当前区块标题的行号

	inBlock := false

	for i, line := range lines {
		lineNumber := i + 1

		// ---- 处理区块外的空行 ----
		// 如果不在任何区块内，且当前行是空行（或仅空白），则跳过。
		// 这样允许文件开头、区块之间、文件末尾存在空行作为视觉分隔。
		if !inBlock && strings.TrimSpace(line) == "" {
			continue
		}

		// ---- 检查是否为区块标题 ----
		if title, ok := isBlockTitle(line); ok {
			// 如果已经在某个区块中，先保存旧区块
			if inBlock {
				blocks = append(blocks, Block{
					Title:   currentBlockTitle,
					Content: currentBlockContent,
					Line:    currentBlockLine,
				})
			}

			// 开始新的区块：记录标题、清空内容、标记 inBlock
			currentBlockTitle = title
			currentBlockContent = []Line{} // 清空
			currentBlockLine = lineNumber
			inBlock = true
			continue
		}

		// ---- 非标题行处理 ----
		if !inBlock {
			// 在区块外遇到非空内容：这是格式错误
			return nil, fmt.Errorf("第 %d 行：内容出现在任何区块之外", lineNumber)
		}

		// 在区块内：累加当前行到内容缓存
		// 注意：这里保存的是原始行（包括缩进和空格），
		// 同时记录该行的绝对行号，供 parser 错误报告使用。
		currentBlockContent = append(currentBlockContent, Line{
			Text:   line,
			Number: lineNumber,
		})
	}

	// ---- 保存最后一个区块 ----
	// 循环结束后，如果 inBlock == true，说明最后一个区块尚未保存。
	if inBlock {
		blocks = append(blocks, Block{
			Title:   currentBlockTitle,
			Content: currentBlockContent,
			Line:    currentBlockLine,
		})
	}

	// ---- 校验：至少有一个区块 ----
	// 一个合法的 .meph 文件至少应该包含一个区块（通常是"角色名"）。
	if len(blocks) == 0 {
		return nil, fmt.Errorf("没有有效区块")
	}

	return blocks, nil
}

// isBlockTitle 检查一行是否为有效的区块标题。
//
// 输入：一行原始文本（可能包含首尾空白）
// 输出：区块标题（如 "角色名"）和一个布尔值表示是否匹配。
//
// 校验规则（必须同时满足）：
//  1. 去除首尾空白后，必须以 【 开头，以 】 结尾。
//  2. 提取 【 和 】 之间的内容，去除首尾空白。
//  3. 标题不能为空字符串。
//  4. 标题必须在 knownBlocks 白名单中。
//
// 为什么同时检查格式和白名单？
//   - 格式检查（【】）确保标题行的基本形态正确。
//   - 白名单检查（knownBlocks）确保标题是预定义的合法值。
//   - 两者结合，既避免了"伪标题"（如普通文本中出现的 【xxx】）被误判，
//     也避免了拼写错误导致的不可预期行为。
//
// 示例：
//
//	"【角色名】"    → ("角色名", true)
//	"【锚点】"      → ("锚点", true)
//	"【未知区块】"  → ("", false)  // 不在白名单中
//	"【角色名"      → ("", false)  // 缺少闭合】
//	"角色名"        → ("", false)  // 缺少【】
func isBlockTitle(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	// 必须以 【 开头，以 】 结尾
	if !strings.HasPrefix(trimmed, "【") || !strings.HasSuffix(trimmed, "】") {
		return "", false
	}

	// 提取区块标题（去掉前后的 【 和 】）
	title := strings.TrimPrefix(trimmed, "【")
	title = strings.TrimSuffix(title, "】")
	title = strings.TrimSpace(title)

	// 标题不能为空
	if title == "" {
		return "", false
	}

	// 必须在白名单中
	if !isKnownBlock(title) {
		return "", false
	}

	return title, true
}

// isKnownBlock 检查区块名是否在已知列表中。
//
// 这是白名单检查的具体实现。
// 将白名单独立为函数的好处：
//  1. 便于 isBlockTitle 调用，语义清晰。
//  2. 如果未来需要从配置文件加载白名单，只需修改此函数。
//  3. 便于单元测试（可以单独测试白名单逻辑）。
func isKnownBlock(name string) bool {
	return knownBlocks[name]
}
