// internal/core/engine/llm_helper.go
//
// LLM 调用辅助函数：提取 engine.go 与 executor.go 中重复的 LLM 调用逻辑。
//
// 职责：
//   - 合并用户输入与指令
//   - 构建 Prompt（使用运行时的记忆，而非契约初始值）
//   - 调用 LLM（恒为流式输出）
//   - 失败时降级为默认静态响应
package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mephisto/internal/core/llm"
)

// defaultLLMTimeout 单次 LLM 调用的超时时间。
//
// 防止 LLM API 挂起导致 CLI 无限等待。超时后自动降级为默认静态响应。
const defaultLLMTimeout = 60 * time.Second

// callLLM 调用 LLM 生成响应（恒为流式输出）。
//
// 参数：
//   - input: 当前用户输入
//   - instruction: 额外指令（如 "继续推进剧情"）
//   - runtime: 运行时（提供契约、状态、历史、记忆）
//   - llmClient: LLM 客户端（可为 nil，nil 时返回默认静态响应）
//   - constraints: 自定义输出约束（空=使用默认）
//   - onChunk: 流式输出回调（可为 nil）
//
// 返回值：
//   - string: LLM 生成的完整响应文本（无 LLM 或调用失败时返回默认静态响应）
//
// 设计说明：
//   - 这是 engine.callLLM 与 executor.callLLMInternal 的公共提取，
//     消除了两处几乎完全相同的实现。
//   - 只支持流式输出（与 Flutter 后版对齐），非流式 Generate 已移除。
//   - 带超时控制：LLM API 挂起时自动降级，并在降级时通过 onChunk 提示用户。
func callLLM(input, instruction string, runtime *Runtime, llmClient llm.Client, constraints string, onChunk func(string)) string {
	if llmClient == nil {
		return defaultStaticResponse(runtime.Contract().RoleName, onChunk)
	}

	// 合并用户输入与指令
	combinedInput := input
	if instruction != "" && instruction != input {
		combinedInput = fmt.Sprintf("%s\n（指令：%s）", input, instruction)
	}

	// 构建 Prompt（使用运行时的记忆，而非契约初始值）
	// maxMemories=0 表示不裁剪（CLI 无上下文窗口配置；排序始终生效）
	prompt := llm.BuildPrompt(
		runtime.Contract(),
		runtime.State(),
		runtime.History(),
		runtime.Memories(),
		combinedInput,
		constraints,
		0,
	)

	// 带超时的上下文：防止 LLM API 挂起无限等待
	ctx, cancel := context.WithTimeout(context.Background(), defaultLLMTimeout)
	defer cancel()

	resp, err := llmClient.GenerateStream(ctx, prompt, onChunk)
	if err != nil {
		// 降级前明确提示用户，避免"角色沉默"被误认为剧情设计
		reason := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "请求超时"
		}
		if onChunk != nil {
			onChunk(fmt.Sprintf("（⚠️ LLM 调用失败：%s，已降级为静态响应）\n", reason))
		}
		return defaultStaticResponse(runtime.Contract().RoleName, onChunk)
	}
	return resp
}

// buildInstruction 构建规则触发后的叙事指令。
//
// 参数：
//   - rollInfo: 骰子结果描述（可能为空）
//
// 返回值：
//   - string: 完整的指令文本
func buildInstruction(rollInfo string) string {
	if rollInfo != "" {
		return fmt.Sprintf("继续推进剧情（%s）", rollInfo)
	}
	return "继续推进剧情"
}

// hasRoll 判断条件中是否包含 roll 表达式。
// 提取散落的魔法字符串判断，统一入口。
func hasRoll(cond string) bool {
	return strings.Contains(cond, rollPrefix)
}