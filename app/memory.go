// ============================================================
// memory.go - 记忆管理（M5 记忆编织）
// 职责：
// 1. 调用 LLM 从对话中提取关键记忆
// 2. 记忆压缩（摘要生成）
// 3. 阈值控制（提取间隔、上限、压缩后保留条数）
// ============================================================

package app

import (
	"context"
	"fmt"
	"strings"

	"mephisto/engine"
	"mephisto/llm"
	"mephisto/parser"
)

// ============================================================
// 硬编码阈值（可由未来命令行参数覆盖）
// ============================================================

const (
	// DefaultExtractInterval 每 N 轮触发一次记忆提取
	DefaultExtractInterval = 3
	// DefaultMemoryLimit 记忆保留上限（超过后触发压缩）
	DefaultMemoryLimit = 50
	// DefaultCompressRetain 压缩后保留的最新记忆条数
	DefaultCompressRetain = 15
)

// ============================================================
// 提示词模板
// ============================================================

const extractPromptTemplate = `你是一位叙事记忆提取器。请从以下对话中提取关键事件、关系变化和角色发展。

规则：
1. 每条记忆以第三人称、过去时态描述一个具体事件或事实
2. 每条记忆不超过 30 个字
3. 只提取对角色产生重大影响的事件
4. 忽略日常寒暄和无意义对话
5. 输出格式：每行一条，以 "- " 开头

用户指令：%s
角色回应：%s

请提取记忆：`

const compressPromptTemplate = `以下是一段角色的长期记忆列表。请将这些记忆压缩为不超过 %d 条最重要的摘要。

要求：
1. 保留最关键的 3-5 个核心事件
2. 保留最近发生的重要事件
3. 合并相似或重复的记忆
4. 每条摘要不超过 30 个字
5. 输出格式：每行一条，以 "- " 开头

现有记忆：
%s

请输出压缩后的记忆：`

// ============================================================
// 核心函数
// ============================================================

// ExtractMemories 从对话中提取新记忆
// 参数：
//   - ctx: 用于控制 LLM 请求的超时/取消
//   - engineCtx: 规则引擎上下文（用于获取角色信息，当前未使用，预留）
//   - userInput: 用户本轮输入
//   - response: 角色本轮回应（LLM 生成的叙事）
//   - client: LLM 客户端
//
// 返回：
//   - []string: 提取出的记忆列表（已去掉 "- " 前缀）
//   - error: 提取失败时返回错误
func ExtractMemories(ctx context.Context, engineCtx engine.Context,
	userInput, response string, client *llm.Client) ([]string, error) {

	if response == "" {
		return nil, nil
	}

	// 构建提示词
	prompt := fmt.Sprintf(extractPromptTemplate, userInput, response)

	messages := []llm.Message{
		{Role: "system", Content: "你是一个专门从叙事对话中提取关键记忆的助手。"},
		{Role: "user", Content: prompt},
	}

	// 调用 LLM（使用独立 context，但传入的 ctx 已带超时）
	result, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("记忆提取 LLM 调用失败: %w", err)
	}

	// 解析结果
	return parseMemoryLines(result), nil
}

// CompressMemories 压缩记忆列表
// 将超出限制的旧记忆压缩为摘要，保留最近的 N 条
func CompressMemories(ctx context.Context, engineCtx engine.Context, client *llm.Client) ([]string, error) {
	memories, ok := engineCtx[parser.KeyMemory].([]string)
	if !ok || len(memories) <= DefaultCompressRetain {
		return nil, nil // 不需要压缩
	}

	// 分离：需要压缩的部分 + 保留的最近 N 条
	toCompress := memories[:len(memories)-DefaultCompressRetain]
	recent := memories[len(memories)-DefaultCompressRetain:]

	prompt := fmt.Sprintf(compressPromptTemplate, DefaultCompressRetain,
		strings.Join(toCompress, "\n"))

	messages := []llm.Message{
		{Role: "system", Content: "你是一个记忆摘要生成器。"},
		{Role: "user", Content: prompt},
	}

	result, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("记忆压缩 LLM 调用失败: %w", err)
	}

	// 解析摘要 + 保留最近的记忆
	summary := parseMemoryLines(result)
	return append(summary, recent...), nil
}

// ============================================================
// 辅助函数
// ============================================================

// parseMemoryLines 解析 LLM 输出的记忆行
// 输入: "- 记忆内容1\n- 记忆内容2\n" 或 "记忆内容1\n记忆内容2"
// 输出: []string{"记忆内容1", "记忆内容2"}
func parseMemoryLines(text string) []string {
	var result []string
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 去掉 "- " 前缀（如果有）
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// ShouldExtract 判断是否应该执行记忆提取
// round: 当前对话轮数（从 1 开始计数）
// 返回 true 表示应该提取记忆
func ShouldExtract(round int) bool {
	return round%DefaultExtractInterval == 0
}

// shouldCompress 判断是否需要压缩
// 当记忆列表超过 DefaultMemoryLimit 时返回 true
func shouldCompress(ctx engine.Context) bool {
	memories, ok := ctx[parser.KeyMemory].([]string)
	if !ok {
		return false
	}
	return len(memories) > DefaultMemoryLimit
}

// appendMemories 追加记忆到上下文
// 如果上下文中已有记忆，追加到末尾；否则创建新列表
func appendMemories(ctx engine.Context, newMemories []string) {
	existing, ok := ctx[parser.KeyMemory].([]string)
	if !ok {
		existing = []string{}
	}
	ctx[parser.KeyMemory] = append(existing, newMemories...)
}
