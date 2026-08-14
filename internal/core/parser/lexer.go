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
// 草稿宽容策略（对齐 Flutter meph_lexer.dart）：
//   - 未知区块（如【草稿】【设定集】）同样切分但标记 IsKnown=false。
//   - 解析层会静默忽略未知区块，方便用户书写备忘/草稿，不会报错。
//
// 已知区块白名单包含 9 个标准区块 + 1 个系统保留区块（@命运，对齐 Flutter）。
// 先前通过环境变量 MEPHISTO_EXTRA_BLOCKS 扩展白名单的机制已移除 ——
// 草稿宽容策略下，未知区块被自动宽容接受，无需再通过环境变量注册。
package parser

import (
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
// 字段说明：
//
//	Title   - 区块标题，如 "角色名"、"锚点"。
//	          由 blockTitle 提取（未知区块同样接受，标记 IsKnown=false）。
//	Content - 区块的内容行列表（不含标题行）。
//	          每行都带有源文件中的绝对行号。
//	Line    - 标题行在源文件中的绝对行号（快速参考）。
//	IsKnown - 是否为已知区块（在白名单中）。
//	          未知区块（如【草稿】【设定集】）同样切分但标记为 false，
//	          解析层会静默忽略未知区块，方便用户书写备忘/草稿，不会报错。
type Block struct {
	Title   string
	Content []Line
	Line    int
	IsKnown bool
}

// knownBlocks 是已知区块名的静态白名单（对齐 Flutter meph_lexer.dart）。
//
// 基础白名单包含 9 个标准区块 + 1 个系统保留区块：
//
//	角色名、锚点、世界观、角色背景、开局场景、状态、规则、记忆、历史、@命运
var knownBlocks = map[string]bool{
	"角色名":  true,
	"锚点":   true,
	"世界观":  true,
	"角色背景": true,
	"开局场景": true,
	"状态":   true,
	"规则":   true,
	"记忆":   true,
	"历史":   true,
	"@命运":  true,
}

// isKnownBlock 检查区块名是否在白名单中。
func isKnownBlock(name string) bool {
	return knownBlocks[name]
}

// Lex 将 .meph 文本切分为区块列表。
//
// 这是解析流程的第一阶段（词法分析 / 区块切分）。
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
//	   - 未知区块同样接受，标记 IsKnown=false（草稿宽容策略）
//
//	3. 当前行不是标题（普通内容行）：
//	   - 如果 inBlock == false：报错（内容出现在任何区块之外）
//	   - 如果 inBlock == true：累加当前行到内容缓存（同时记录行号）
//
//	4. 扫描结束后：
//	   - 如果 inBlock == true，保存最后一个区块
//	   - 检查是否至少有一个区块，没有则报错
//
// 草稿宽容策略（对齐 Flutter meph_lexer.dart）：
//   - 未知区块（如【草稿】【设定集】）同样切分但标记 IsKnown=false。
//   - 解析层会静默忽略未知区块，方便用户书写备忘/草稿，不会报错。
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
		// 标题格式：【标题名】（未知区块同样接受，标记 IsKnown=false）
		if title, ok := blockTitle(trimmed); ok {
			// 如果已经在某个区块中，先保存旧区块
			if inBlock {
				blocks = append(blocks, Block{
					Title:   currentBlockTitle,
					Content: currentBlockContent,
					Line:    currentBlockLine,
					IsKnown: isKnownBlock(currentBlockTitle),
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
			IsKnown: isKnownBlock(currentBlockTitle),
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

// blockTitle 检查一行是否为区块标题（与 Flutter meph_lexer.dart 的 blockTitle 对齐）。
//
// 支持两种形式：
//   - 用户区块：【标题】（如【角色名】）
//   - 系统保留区块：@标题（如 @命运），独立成行
//
// `@` 前缀是「系统生成元数据」命名空间，与用户 `【】` 区块天然区分。
//
// 输入：一行原始文本（可能包含首尾空白）
// 输出：区块标题（如 "角色名"、"@命运"）和一个布尔值表示是否匹配。
//
// 校验规则：
//  1. 去除首尾空白后，必须以 【 开头，以 】 结尾；或以 @ 开头独立成行。
//  2. 提取标题内容，去除首尾空白。
//  3. 标题不能为空字符串。
//
// 不要求标题在白名单中，未知区块作为草稿宽容接受（IsKnown=false 供解析层判断）。
func blockTitle(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	// 系统保留区块：@xxx 独立成行
	if strings.HasPrefix(trimmed, "@") {
		title := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
		if title == "" {
			return "", false
		}
		return "@" + title, true
	}

	// 用户区块：【标题】
	if !strings.HasPrefix(trimmed, "【") || !strings.HasSuffix(trimmed, "】") {
		return "", false
	}

	// 提取区块标题
	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "【"), "】"))

	if title == "" {
		return "", false
	}

	return title, true
}
