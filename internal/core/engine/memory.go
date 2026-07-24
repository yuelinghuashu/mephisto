// internal/core/engine/memory.go
//
// 记忆管理：提取和压缩
//
// 长叙事节省 Token 的核心机制。
//
// 设计原则：
//  1. 记忆提取是自动的，每 N 轮触发一次
//  2. 记忆压缩是自动的，当记忆条数超过阈值时触发
//  3. 记忆是累积的，新记忆追加到已有记忆后面
//  4. 压缩时保留最近的重要记忆，合并旧的相似记忆
//
// 使用流程：
//  1. 每轮对话后，engine.Run 调用 ShouldExtract 判断是否需要提取
//  2. 如果需要，调用 ExtractMemories 提取新记忆
//  3. 去重后追加到已有记忆列表
//  4. 如果记忆超过上限，调用 CompressMemories 压缩
//
// 记忆存储位置：
//   - Runtime.memories 存储当前会话的所有记忆
//   - 子版保存时写入 【记忆】 区块
//   - 子版加载时从 【记忆】 区块恢复
package engine

import (
	"context"
	"fmt"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/domain"
)

// ============================================================
// 配置结构体
// ============================================================

// MemoryConfig 记忆配置。
//
// 字段说明：
//   - ExtractInterval: 每 N 轮提取一次记忆。默认 5。
//   - MaxLimit: 记忆条数上限。超过此数量触发压缩。默认 30。
//   - CompressRetain: 压缩时保留的最新条数。默认 5。
//   - ExtractWindow: 提取时参考的最近对话轮数。默认 10。
type MemoryConfig struct {
	ExtractInterval int // 每 N 轮提取一次，默认 5
	MaxLimit        int // 记忆上限，默认 30
	CompressRetain  int // 压缩后保留的最新条数，默认 5
	ExtractWindow   int // 提取时参考的最近对话轮数，默认 10
}

// DefaultMemoryConfig 默认记忆配置。
var DefaultMemoryConfig = MemoryConfig{
	ExtractInterval: 5,
	MaxLimit:        30,
	CompressRetain:  5,
	ExtractWindow:   10,
}

// ============================================================
// 记忆操作函数
// ============================================================

// ShouldExtract 判断是否应该提取记忆。
//
// 参数：
//   - round: 当前对话轮数（从 1 开始计数）
//   - cfg: 记忆配置
//
// 返回值：
//   - bool: true 表示应该提取记忆
//
// 判断逻辑：轮数 > 0 且 轮数 % 提取间隔 == 0
//
// 示例：ExtractInterval = 5 → 第 5、10、15、20... 轮提取
func ShouldExtract(round int, cfg MemoryConfig) bool {
	return round > 0 && round%cfg.ExtractInterval == 0
}

// ExtractMemories 从对话中提取记忆摘要。
//
// 流程：
//  1. 从历史中提取最近 ExtractWindow 轮对话
//  2. 构建提取提示词
//  3. 调用 LLM 生成摘要
//  4. 解析 LLM 输出为记忆列表
//
// 参数：
//   - ctx: 上下文（用于超时控制）
//   - history: 完整的对话历史
//   - existing: 现有的记忆列表（用于去重参考）
//   - llmClient: LLM 客户端
//   - cfg: 记忆配置
//
// 返回值：
//   - []string: 新提取的记忆列表
//   - error: 提取失败时的错误
//
// 注意：此方法只负责提取，不负责去重和追加。
// 调用方应自行去重后再追加到已有记忆列表。
func ExtractMemories(ctx context.Context, history []domain.HistoryEntry, existing []string, llmClient llm.Client, cfg MemoryConfig) ([]string, error) {
	if llmClient == nil || len(history) == 0 {
		return nil, nil
	}

	// 只取最近 ExtractWindow 轮对话（每轮 2 条消息）
	window := cfg.ExtractWindow * 2
	recent := history
	if len(history) > window {
		recent = history[len(history)-window:]
	}

	// 构建提取提示词
	prompt := buildExtractPrompt(recent, existing)

	// 调用 LLM 提取
	response, err := llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("记忆提取失败：%w", err)
	}

	return parseMemoryLines(response), nil
}

// CompressMemories 压缩记忆列表。
//
// 压缩策略：
//   - 将除了最近 CompressRetain 条之外的所有记忆进行压缩
//   - 压缩结果为 3-5 条摘要
//   - 保留最近 CompressRetain 条记忆不变
//   - 最终结果 = 压缩摘要 + 最近 CompressRetain 条
//
// 参数：
//   - ctx: 上下文（用于超时控制）
//   - memories: 要压缩的记忆列表
//   - llmClient: LLM 客户端
//   - cfg: 记忆配置
//
// 返回值：
//   - []string: 压缩后的记忆列表
//   - error: 压缩失败时的错误
func CompressMemories(ctx context.Context, memories []string, llmClient llm.Client, cfg MemoryConfig) ([]string, error) {
	if llmClient == nil || len(memories) <= cfg.MaxLimit {
		return memories, nil
	}

	// 分离：需要压缩的部分 + 保留的最近 N 条
	toCompress := memories[:len(memories)-cfg.CompressRetain]
	recent := memories[len(memories)-cfg.CompressRetain:]

	prompt := buildCompressPrompt(toCompress)

	response, err := llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("记忆压缩失败：%w", err)
	}

	summary := parseMemoryLines(response)
	if len(summary) == 0 {
		// 压缩结果为空，保留原有记忆（容错）
		return memories, nil
	}

	return append(summary, recent...), nil
}

// ============================================================
// 辅助函数
// ============================================================

// buildExtractPrompt 构建提取提示词。
//
// 参数：
//   - history: 最近 N 轮对话历史
//   - existing: 现有记忆列表
//
// 返回值：
//   - string: 完整的提取提示词
func buildExtractPrompt(history []domain.HistoryEntry, existing []string) string {
	var sb strings.Builder

	// 格式化历史
	for _, entry := range history {
		role := entry.Role
		switch role {
		case "fate":
			role = "命运"
		case "assistant":
			role = "角色"
		}
		fmt.Fprintf(&sb, "%s: %s\n", role, entry.Content) // 直接写入 Builder
	}

	// 格式化现有记忆
	existingStr := "（无）"
	if len(existing) > 0 {
		existingStr = strings.Join(existing, "\n")
	}

	return fmt.Sprintf(extractPromptTemplate, existingStr, sb.String())
}

// buildCompressPrompt 构建压缩提示词。
//
// 参数：
//   - memories: 需要压缩的记忆列表
//
// 返回值：
//   - string: 完整的压缩提示词
func buildCompressPrompt(memories []string) string {
	return fmt.Sprintf(compressPromptTemplate, 5, strings.Join(memories, "\n"))
}

// parseMemoryLines 解析 LLM 输出的记忆行。
//
// 输入格式：
//   - 每行以 "- " 开头的列表
//   - 或者普通文本行
//
// 输出：
//   - []string: 清理后的记忆列表
//
// 示例输入：
//   - 贝利亚在光之国边境遭遇了巡逻队
//   - 贝利亚摧毁了一艘追击飞船
//
// 示例输出：
//
//	[]string{"贝利亚在光之国边境遭遇了巡逻队", "贝利亚摧毁了一艘追击飞船"}
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

// ============================================================
// 提示词模板
// ============================================================

// extractPromptTemplate 是记忆提取的提示词模板。
//
// 参数：
//   - %s: 现有记忆列表
//   - %s: 对话历史
//
// 输出要求：
//   - 每条摘要不超过 20 个字
//   - 以 "- " 开头
const extractPromptTemplate = `从以下对话中提取关键事件摘要。

规则：
1. 每条摘要不超过 20 个字
2. 只提取对角色产生重大影响的事件
3. 忽略日常寒暄和无意义对话
4. 如果事件已经存在于现有记忆中，不要重复提取
5. 禁止修改或重复角色的核心设定（如角色名、锚点内容、状态值等）
6. 输出格式：每行一条，以 "- " 开头

现有记忆：
%s

对话历史：
%s

请输出新提取的记忆：`

// compressPromptTemplate 是记忆压缩的提示词模板。
//
// 参数：
//   - %d: 目标摘要条数
//   - %s: 需要压缩的记忆列表
//
// 输出要求：
//   - 压缩为 3-5 条核心摘要
//   - 每条摘要不超过 20 个字
//   - 以 "- " 开头
const compressPromptTemplate = `以下是一段角色的长期记忆列表。请将这些记忆压缩为不超过 %d 条最重要的摘要。

要求：
1. 保留最关键的 3-5 个核心事件
2. 保留最近发生的重要事件
3. 合并相似或重复的记忆
4. 每条摘要不超过 20 个字
5. 禁止篡改角色的核心设定（如角色名、锚点内容、状态值等）
6. 输出格式：每行一条，以 "- " 开头

现有记忆：
%s

请输出压缩后的记忆：`
