// internal/core/engine/executor.go
//
// 本文件提供默认的动作执行器实现。
// 动作执行器负责执行规则匹配后的动作，生成引擎的响应。
//
// 支持的动作语法：
//   - 注入 "消息"              → 将消息注入到记忆（不显示），然后继续走 LLM 叙事
//   - 状态.键 = 值             → 更新状态值，返回带 📊 前缀的确认
//   - LLM: 指令                → 调用 LLM 生成响应（需注入 LLM 客户端）
//   - 普通文本                 → 若有 LLM 客户端则调用 LLM，否则直接返回该文本
//
// 设计原则：
//   - 动作执行器直接修改 ctx.State 和 ctx.Memories，影响引擎状态
//   - 所有修改都是显式的，通过上下文传递
//   - 流式输出由调用方通过 onChunk 回调控制
//   - 注入动作返回空字符串，表示“不产生直接输出”，由上层决定是否继续
package engine

import (
	"context"
	"fmt"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/shared"
)

// DefaultActionExecutor 是默认的动作执行器。
//
// 该实现是纯函数式的，不持有任何外部状态，因此可以在多个引擎实例间安全复用。
// 所有状态修改都通过 ActionContext 传递，执行器本身不存储任何数据。
//
// 执行流程：
//  1. 去除动作字符串首尾空白
//  2. 根据动作类型分发到对应的执行函数
//  3. 返回响应文本（若为空，表示无直接输出）
type DefaultActionExecutor struct{}

// Execute 执行动作。
//
// 参数：
//   - action : 动作字符串
//   - ctx    : 执行上下文（包含状态、历史、记忆、契约、LLM 客户端）
//   - onChunk: 流式回调，逐块输出响应内容（可为 nil，表示非流式）
//
// 返回值：
//   - string: 执行结果（若为空，表示无直接输出，上层应继续调用 LLM）
//   - error : 解析或执行过程中的错误
//
// 处理流程：
//  1. 空动作 → 返回错误
//  2. 注入动作 → executeInject（返回空字符串，不显示）
//  3. 状态修改 → executeStateChange（返回确认消息）
//  4. LLM 动作 → executeLLM（需 LLM 客户端）
//  5. 普通文本 → 有 LLM 则调用 LLM，否则直接返回
//
// 统一流式处理：
//   - 状态修改动作：立即返回结果，然后模拟流式输出（逐字符回调）
//   - LLM 动作：真正的流式输出（逐块回调）
//   - 静态文本：立即返回结果，然后模拟流式输出
func (e *DefaultActionExecutor) Execute(action string, ctx *ActionContext, onChunk func(string)) (string, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return "", fmt.Errorf("动作为空")
	}

	// ---- 动作类型分发 ----
	// 注入动作：只修改记忆，不产生输出
	if strings.HasPrefix(action, "注入 ") {
		return e.executeInject(action, ctx)
	}

	// 状态修改：返回确认消息
	if strings.HasPrefix(action, "状态.") {
		return e.executeWithChunk(e.executeStateChange, action, ctx, onChunk)
	}

	// LLM 动作（需 LLM 客户端）
	if strings.HasPrefix(action, "LLM: ") {
		if ctx.LLMClient == nil {
			return "", fmt.Errorf("LLM 动作需要注入 LLM 客户端")
		}
		return e.executeLLM(action, ctx, onChunk)
	}

	// ---- 普通文本 ----
	// 如果有 LLM 客户端，将其作为指令调用 LLM
	if ctx.LLMClient != nil {
		return e.executeLLM(action, ctx, onChunk)
	}

	// 无 LLM 客户端：直接返回静态文本
	return e.executeStatic(action, onChunk)
}

// executeWithChunk 执行动作并统一处理流式输出。
// 对于同步动作（如状态修改），在返回结果后通过回调模拟逐字符输出。
func (e *DefaultActionExecutor) executeWithChunk(
	fn func(string, *ActionContext) (string, error),
	action string,
	ctx *ActionContext,
	onChunk func(string),
) (string, error) {
	result, err := fn(action, ctx)
	if err != nil {
		return "", err
	}
	if onChunk != nil {
		for _, ch := range result {
			onChunk(string(ch))
		}
	}
	return result, nil
}

// executeInject 执行注入动作。
//
// 注入动作将消息追加到记忆列表，并返回空字符串（表示不直接显示任何内容）。
// 注入的内容会通过记忆进入 LLM 的上下文，由 LLM 自然融入叙事。
//
// 处理步骤：
//  1. 提取引号中的消息内容
//  2. 替换 {角色名} 占位符
//  3. 追加到 ctx.Memories
//  4. 返回空字符串（不显示）
//
// 返回值：总是返回 ("", nil)，表示无直接输出。
func (e *DefaultActionExecutor) executeInject(action string, ctx *ActionContext) (string, error) {
	msg, err := unquote(strings.TrimPrefix(action, "注入 "))
	if err != nil {
		// 解析失败时容错：直接使用原始动作字符串作为消息
		msg = strings.TrimPrefix(action, "注入 ")
	}

	if ctx.Contract != nil {
		msg = shared.ReplacePlaceholders(msg, map[string]string{
			"角色名": ctx.Contract.RoleName,
		})
	}

	// 追加到记忆，供 LLM 使用
	ctx.Memories = append(ctx.Memories, msg)

	// 返回空字符串，表示不直接显示任何内容
	return "", nil
}

// executeStateChange 执行状态修改动作。
//
// 格式：状态.键 = 值
// 支持的值类型：
//   - 数字：状态.生命值 = 100
//   - 布尔值：状态.存活 = true
//   - 字符串：状态.情绪 = "愤怒"
//
// 处理步骤：
//  1. 去掉 "状态." 前缀
//  2. 提取键和值字符串
//  3. 解析值（自动推断类型）
//  4. 更新 ctx.State
//  5. 返回带 📊 前缀的确认
func (e *DefaultActionExecutor) executeStateChange(action string, ctx *ActionContext) (string, error) {
	rest := strings.TrimPrefix(action, "状态.")
	rest = strings.TrimSpace(rest)

	key, valStr, found := strings.Cut(rest, "=")
	if !found {
		return "", fmt.Errorf("状态修改格式错误，缺少 '=': %s", action)
	}

	key = strings.TrimSpace(key)
	valStr = strings.TrimSpace(valStr)

	if key == "" {
		return "", fmt.Errorf("状态键不能为空")
	}
	if valStr == "" {
		return "", fmt.Errorf("状态值不能为空")
	}

	// 去掉值两端的引号（如果有）
	if strings.HasPrefix(valStr, `"`) && strings.HasSuffix(valStr, `"`) {
		valStr = valStr[1 : len(valStr)-1]
	}

	val := shared.ParseValue(valStr)
	ctx.State[key] = val

	return fmt.Sprintf("📊 %s = %v", key, val), nil
}

// executeLLM 调用 LLM 生成动态响应。
//
// 参数：
//   - action : 动作文本，可能以 "LLM: " 开头（显式 LLM 动作）或为普通指令
//   - ctx    : 执行上下文（包含状态、历史、记忆、契约、LLM 客户端）
//   - onChunk: 流式回调，逐块输出响应内容（可为 nil，表示非流式）
//
// 返回值：
//   - string: LLM 生成的完整响应
//   - error : LLM 调用失败时的错误
func (e *DefaultActionExecutor) executeLLM(action string, ctx *ActionContext, onChunk func(string)) (string, error) {
	// ---- 1. 提取指令文本 ----
	// 如果动作以 "LLM: " 开头，则去除该前缀，得到纯净的指令内容。
	instruction := action
	if after, ok := strings.CutPrefix(action, "LLM: "); ok {
		instruction = strings.TrimSpace(after)
	}

	// ---- 2. 构建组合输入 ----
	// 将原始用户输入与动作指令合并，让 LLM 既知道用户说了什么，也知道需要执行什么动作。
	// 如果指令与原始输入相同，则不重复拼接。
	combinedInput := ctx.Input
	if instruction != "" && instruction != ctx.Input {
		combinedInput = fmt.Sprintf("%s\n（动作指令：%s）", ctx.Input, instruction)
	}

	// ---- 3. 使用 BuildPrompt 构建完整 Prompt ----
	// 关键：第五个参数固定为 llm.NarrativeConstraints，确保输出格式和互动要求被严格遵循。
	// 之前错误地将 instruction 作为约束传入，导致 NarrativeConstraints 被覆盖。
	prompt := llm.BuildPrompt(
		ctx.Contract,             // 契约（只读）
		ctx.State,                // 当前状态（动态）
		ctx.History,              // 对话历史
		combinedInput,            // 用户输入 + 动作指令（合并）
		llm.NarrativeConstraints, // 固定的叙事约束（包含多角色互动要求）
	)

	// ---- 4. 调用 LLM（流式或非流式） ----
	if onChunk != nil {
		// 流式调用：逐块输出，同时收集完整响应
		response, err := ctx.LLMClient.GenerateStream(context.Background(), prompt, onChunk)
		if err != nil {
			return "", fmt.Errorf("LLM 流式调用失败: %w", err)
		}
		return response, nil
	}

	// 非流式调用：一次性获取完整响应
	response, err := ctx.LLMClient.Generate(context.Background(), prompt)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}
	return response, nil
}

// executeStatic 执行静态文本动作（无 LLM 客户端时使用）。
//
// 直接返回动作字符串，同时通过回调模拟流式输出。
// 这确保了即使没有 LLM，引擎也能保持一致的交互体验。
func (e *DefaultActionExecutor) executeStatic(action string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		for _, ch := range action {
			onChunk(string(ch))
		}
	}
	return action, nil
}

// unquote 去除字符串首尾的引号。
//
// 支持：
//   - 英文双引号："text" → text
//   - 中文双引号："“text”" → text
//
// 如果字符串不以双引号开头，直接返回原字符串（容错）。
// 如果只有起始引号没有结束引号，返回原字符串（容错，不报错）。
func unquote(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
			(strings.HasPrefix(s, "“") && strings.HasSuffix(s, "”")) {
			return s[1 : len(s)-1], nil
		}
	}
	return s, nil
}
