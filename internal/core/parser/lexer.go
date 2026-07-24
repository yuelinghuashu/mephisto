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
// 白名单扩展：
//
//	通过环境变量 MEPHISTO_EXTRA_BLOCKS 可以添加额外的区块名，
//	多个区块名用逗号分隔。例如：
//	export MEPHISTO_EXTRA_BLOCKS="自定义区块1,自定义区块2"
package parser

import (
	"os"
	"strings"

	"mephisto/internal/shared"
)

// Line 表示带行号的内容行。
//
// 为什么要在 lexer 阶段就记录行号？
//
// lexer 在扫描时直接记录绝对行号，parser 拿到 Line 后
//
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

// getKnownBlocks 返回已知区块名的白名单。
//
// 基础白名单包含 10 个标准区块：
//
//	角色名、锚点、世界观、角色背景、开局场景、状态、规则、记忆、历史
//
// 扩展方式：
//
//	通过环境变量 MEPHISTO_EXTRA_BLOCKS 添加额外区块名，
//	多个区块名用逗号分隔（会去除首尾空格）。
//
// 为什么使用 map[string]bool 作为白名单？
//  1. 显式列出所有合法区块名，防止拼写错误导致的隐式 bug。
//  2. O(1) 查找，性能优异。
//  3. 环境变量扩展支持自定义区块，无需修改代码。
func getKnownBlocks() map[string]bool {
	// 基础白名单
	base := map[string]bool{
		"角色名":  true,
		"锚点":   true,
		"世界观":  true,
		"角色背景": true,
		"开局场景": true,
		"状态":   true,
		"规则":   true,
		"记忆":   true,
		"历史":   true,
	}

	// 从环境变量加载额外区块
	if extra := os.Getenv("MEPHISTO_EXTRA_BLOCKS"); extra != "" {
		for name := range strings.SplitSeq(extra, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				base[name] = true
			}
		}
	}

	return base
}

// isKnownBlock 检查区块名是否在白名单中。
//
// 每次调用都会重新读取环境变量，支持运行时动态调整。
// 如果性能敏感，可以缓存结果，但考虑到 Lex 只执行一次，当前实现足够。
func isKnownBlock(name string) bool {
	return getKnownBlocks()[name]
}

// Lex 将 .meph 文本切分为区块列表。
//
// 这是解析流程的第一阶段（词法分析 / 区块切分）。
// 输入是 .meph 文件的完整文本内容（UTF-8 编码的字符串），
// 输出是区块列表 []Block，以及可能的错误。
//
// 处理逻辑（按行扫描状态机）：
//
//	状态：inBlock = false（不在区块内）或 true（正在收集区块内容）
//
//	1. 当前行是空行或注释（# 开头）：
//	   - 如果 inBlock == true：将该行记录到当前区块的 Content 中（保留结构）
//	   - 如果 inBlock == false：跳过该行（允许文件顶部有注释/空行）
//
//	2. 当前行是区块标题（【xxx】）：
//	   - 如果 inBlock == true：先保存当前区块
//	   - 开始新区块：记录标题、清空内容缓存、inBlock = true
//	   - 继续下一行
//
//	3. 当前行不是标题（普通内容行）：
//	   - 如果 inBlock == false：报错（内容出现在任何区块之外）
//	   - 如果 inBlock == true：累加当前行到内容缓存（同时记录行号）
//
//	4. 扫描结束后：
//	   - 如果 inBlock == true，保存最后一个区块
//	   - 检查是否至少有一个区块，没有则报错
//
// 为什么在 lexer 阶段就记录行号？
//
//	lexer 在扫描时直接记录绝对行号，parser 拿到 Line 后
//	直接使用 Line.Number 报告错误，无需任何计算。
//	这避免了 parser 维护复杂的迭代器状态。
//
// 空行和注释的处理策略：
//
//	空行和 # 注释在文件任何位置都是合法的：
//	  - 在区块外：作为视觉分隔符或元数据，被跳过
//	  - 在区块内：作为内容的一部分被保留，由上层 Parser 决定如何处理
//
// 边界情况处理：
//   - 文件开头有空行或注释 → 跳过，不报错
//   - 区块之间有多行注释 → 跳过，不影响区块切分
//   - 最后一个区块后有注释 → 跳过，不影响
//   - 标题行后立即跟下一个标题 → 第一个区块的 Content 为空（合法）
//   - 文件中没有任何区块 → 报错 "没有有效区块"
//
// 参数：
//   - text: .meph 文件的完整文本内容
//
// 返回值：
//   - []Block: 切分后的区块列表
//   - error: 解析错误（包含行号和错误描述）
func Lex(text string) ([]Block, error) {
	lines := strings.Split(text, "\n")

	var blocks []Block             // 区块列表
	var currentBlockTitle string   // 当前区块的标题
	var currentBlockContent []Line // 当前区块的内容行列表（不含标题行）
	var currentBlockLine int       // 当前区块的标题行号
	inBlock := false

	for i, line := range lines {
		lineNumber := i + 1
		trimmed := strings.TrimSpace(line)

		// ---- 处理空行和注释（在任何位置都允许） ----
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inBlock {
				// 在区块内：保留空行和注释，保持结构完整性
				currentBlockContent = append(currentBlockContent, Line{
					Text:   line,
					Number: lineNumber,
				})
			}
			// 在区块外：直接跳过
			continue
		}

		// ---- 检查是否为区块标题 ----
		//
		// 标题格式：【标题名】
		// 标题名必须通过白名单校验（isKnownBlock），防止拼写错误
		if title, ok := isBlockTitle(line); ok {
			// 如果已经在某个区块中，先保存旧区块
			if inBlock {
				blocks = append(blocks, Block{
					Title:   currentBlockTitle,
					Content: currentBlockContent,
					Line:    currentBlockLine,
				})
			}

			// 开始新的区块
			currentBlockTitle = title
			currentBlockContent = []Line{}
			currentBlockLine = lineNumber
			inBlock = true
			continue
		}

		// ---- 检查不完整的区块标题格式（以 【 开头但缺少 】，或缺少 【 但有 】） ----
		if strings.HasPrefix(trimmed, "【") || strings.HasSuffix(trimmed, "】") {
			return nil, &shared.ParseError{
				Line:    lineNumber,
				Message: "区块标题格式错误",
			}
		}

		// ---- 非标题行处理 ----
		//
		// 此时已确保：当前行不是空行、不是注释、不是标题
		// 因此只能是普通内容行
		if !inBlock {
			// 区块外的非空、非注释、非标题内容 → 格式错误
			return nil, &shared.ParseError{
				Line:    lineNumber,
				Message: "内容出现在任何区块之外",
			}
		}

		// 在区块内：累加当前行到内容缓存
		// 注意：保留原始文本（包含缩进），不进行任何修剪
		currentBlockContent = append(currentBlockContent, Line{
			Text:   line,
			Number: lineNumber,
		})
	}

	// ---- 保存最后一个区块 ----
	if inBlock {
		blocks = append(blocks, Block{
			Title:   currentBlockTitle,
			Content: currentBlockContent,
			Line:    currentBlockLine,
		})
	}

	// ---- 校验：至少有一个区块 ----
	if len(blocks) == 0 {
		return nil, &shared.ParseError{
			Message: "没有有效区块",
		}
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
//  4. 标题必须在白名单中（基础 + 环境变量扩展）。
//
// 为什么同时检查格式和白名单？
//   - 格式检查（【】）确保标题行的基本形态正确。
//   - 白名单检查确保标题是预定义的合法值。
//   - 两者结合，既避免了"伪标题"被误判，也避免了拼写错误。
func isBlockTitle(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	// 必须以 【 开头，以 】 结尾
	if !strings.HasPrefix(trimmed, "【") || !strings.HasSuffix(trimmed, "】") {
		return "", false
	}

	// 提取区块标题
	title := strings.TrimPrefix(trimmed, "【")
	title = strings.TrimSuffix(title, "】")
	title = strings.TrimSpace(title)

	if title == "" {
		return "", false
	}

	// 必须在白名单中（基础 + 环境变量扩展）
	if !isKnownBlock(title) {
		return "", false
	}

	return title, true
}
