// internal/core/engine/executor.go
//
// 动作执行系统：执行规则动作（注入、状态修改、LLM 调用、静态文本）
//
// 支持的动作类型：
//   - 注入：追加到记忆，无直接输出
//   - 状态修改：更新运行时状态，返回确认消息
//   - LLM 调用：将动作文本作为指令调用 LLM 生成叙事
//   - 静态文本：直接返回文本（无 LLM 时降级）
//   - 复合动作：多个子动作用 " && " 串联
package engine

import (
	"context"
	"fmt"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/shared"
)

// ExecuteAction 执行规则动作。
//
// 支持的动作类型（通过前缀识别）：
//   - "注入 " → 将消息追加到记忆（不直接输出）
//   - "状态." → 更新状态值，返回确认消息
//   - 其他文本 → 若有 LLM 则作为指令调用，否则直接返回
//
// 复合动作：
//
//	多个子动作可以用 " && " 串联，依次执行：
//	  注入 "描述" && 状态.堕落指数 += 5
//	  注入 "描述A" && 注入 "描述B" && 状态.情绪 = "愤怒"
//
// 参数：
//   - action: 动作字符串
//   - input: 当前用户输入
//   - runtime: 运行时（用于修改状态和记忆）
//   - llmClient: LLM 客户端（可选）
//   - onChunk: 流式输出回调
//   - rollInfo: 骰子结果描述（匹配规则时生成，注入 LLM 指令中）
//
// 返回值：
//   - string: 响应文本（空字符串表示无直接输出）
//
// 设计说明：
//   - 注入动作返回空字符串，由上层决定是否继续 LLM 叙事
//   - 状态修改返回带 📊 前缀的确认消息
//   - 普通文本直接返回（无 LLM 时）或作为指令调用 LLM
//   - 复合动作的输出是各子动作输出的拼接（非空输出之间用换行分隔）
func ExecuteAction(action, input string, runtime *Runtime, llmClient llm.Client, onChunk func(string), rollInfo string) string {
	action = strings.TrimSpace(action)

	// 支持复合动作：用 " && " 串联多个子动作
	if strings.Contains(action, " && ") {
		return executeCompoundAction(action, input, runtime, llmClient, onChunk, rollInfo)
	}

	switch {
	case strings.HasPrefix(action, "注入 "):
		return executeInject(action, runtime)

	case strings.HasPrefix(action, "状态."):
		return setState(action, runtime, onChunk)

	default:
		// ---- 普通文本 ----
		if llmClient != nil {
			instruction := action
			if rollInfo != "" {
				instruction = fmt.Sprintf("%s（%s）", action, rollInfo)
			}
			return callLLMInternal(input, instruction, llmClient, runtime, onChunk)
		}
		if onChunk != nil {
			for _, ch := range action {
				onChunk(string(ch))
			}
		}
		return action
	}
}

// executeCompoundAction 执行复合动作（用 " && " 分隔的多个子动作）。
func executeCompoundAction(action, input string, runtime *Runtime, llmClient llm.Client, onChunk func(string), rollInfo string) string {
	parts := strings.Split(action, " && ")
	var outputs []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 递归调用 ExecuteAction 执行每个子动作
		result := ExecuteAction(part, input, runtime, llmClient, onChunk, rollInfo)
		if result != "" {
			outputs = append(outputs, result)
		}
	}
	return strings.Join(outputs, "\n")
}

// executeInject 执行注入动作。
func executeInject(action string, runtime *Runtime) string {
	msg := shared.Unquote(strings.TrimPrefix(action, "注入 "))
	msg = shared.ReplacePlaceholders(msg, map[string]string{
		"角色名": runtime.Contract().RoleName,
	})
	runtime.AppendMemory(msg)
	return ""
}

// setState 修改状态。
//
// 支持的格式：
//   - 状态.键 = 值           → 简单赋值（支持 bool/int/float64/string）
//   - 状态.键 += 值          → 复合加法赋值（仅数字类型）
//   - 状态.键 -= 值          → 复合减法赋值
//   - 状态.键 *= 值          → 复合乘法赋值
//   - 状态.键 /= 值          → 复合除法赋值
//
// 复合赋值的类型规则：
//   - 如果当前状态值是 int，运算结果保持 int（直接舍去小数，避免意外类型变更）
//   - 如果当前状态值是 float64，运算结果保持 float64
//
// 返回值：
//   - string: 带 📊 前缀的确认消息
func setState(action string, runtime *Runtime, onChunk func(string)) string {
	rest := strings.TrimPrefix(action, "状态.")
	rest = strings.TrimSpace(rest)

	// 复合赋值运算符（优先级高于简单赋值）
	compoundOps := []string{"+=", "-=", "*=", "/="}
	var compoundOp string
	var compoundIdx int
	for _, op := range compoundOps {
		if i := strings.Index(rest, op); i != -1 {
			compoundOp = op
			compoundIdx = i
			break
		}
	}

	var key, valStr string
	var ok bool

	if compoundOp != "" {
		// ---- 复合赋值 ----
		key = strings.TrimSpace(rest[:compoundIdx])
		valStr = strings.TrimSpace(rest[compoundIdx+len(compoundOp):])
	} else {
		// ---- 简单赋值 ----
		key, valStr, ok = strings.Cut(rest, "=")
		if !ok {
			return "格式错误"
		}
		key, valStr = strings.TrimSpace(key), strings.TrimSpace(valStr)
	}

	// 去除字符串值的引号
	if strings.HasPrefix(valStr, `"`) && strings.HasSuffix(valStr, `"`) {
		valStr = valStr[1 : len(valStr)-1]
	}

	val := shared.ParseValue(valStr)

	// 处理复合赋值
	if compoundOp != "" {
		currentState := runtime.State()
		currentVal, exists := currentState[key]
		if !exists {
			return fmt.Sprintf("📊 状态「%s」不存在", key)
		}

		// 将当前值转为 float64
		var currentFloat float64
		switch v := currentVal.(type) {
		case int:
			currentFloat = float64(v)
		case int64:
			currentFloat = float64(v)
		case float64:
			currentFloat = v
		default:
			return fmt.Sprintf("📊 %s：当前值类型 %T 不支持算术运算", key, currentVal)
		}

		// 将新值转为 float64
		var valFloat float64
		switch v := val.(type) {
		case int:
			valFloat = float64(v)
		case float64:
			valFloat = v
		default:
			return fmt.Sprintf("📊 %s：赋值类型 %T 不支持算术运算", key, val)
		}

		// 执行运算
		var resultFloat float64
		switch compoundOp {
		case "+=":
			resultFloat = currentFloat + valFloat
		case "-=":
			resultFloat = currentFloat - valFloat
		case "*=":
			resultFloat = currentFloat * valFloat
		case "/=":
			if valFloat == 0 {
				return "📊 除数不能为0"
			}
			resultFloat = currentFloat / valFloat
		}

		// 根据原始值的类型决定结果类型，避免 int → float64 的类型跃迁
		switch currentVal.(type) {
		case int:
			runtime.SetState(key, int(resultFloat))
		default:
			runtime.SetState(key, resultFloat)
		}

		resultMsg := fmt.Sprintf("📊 %s %s %v", key, compoundOp, val)
		if onChunk != nil {
			for _, ch := range resultMsg {
				onChunk(string(ch))
			}
		}
		return resultMsg
	}

	// ---- 简单赋值 ----
	runtime.SetState(key, val)

	result := fmt.Sprintf("📊 %s = %v", key, val)
	if onChunk != nil {
		for _, ch := range result {
			onChunk(string(ch))
		}
	}
	return result
}

// callLLMInternal 内部 LLM 调用函数。
//
// 与 engine.go 中的 callLLM 不同，此函数不依赖 Engine 实例，
// 而是通过参数传入所有必要数据，供 ExecuteAction 使用。
//
// 参数：
//   - input: 当前用户输入
//   - instruction: 额外指令
//   - llmClient: LLM 客户端
//   - runtime: 运行时
//   - onChunk: 流式回调
//
// 返回值：
//   - string: LLM 生成的响应
func callLLMInternal(input, instruction string, llmClient llm.Client, runtime *Runtime, onChunk func(string)) string {
	// 合并用户输入与指令
	combinedInput := input
	if instruction != "" && instruction != input {
		combinedInput = fmt.Sprintf("%s\n（指令：%s）", input, instruction)
	}

	// 构建 Prompt（使用运行时的记忆，而非契约初始值）
	prompt := llm.BuildPrompt(
		runtime.Contract(),
		runtime.State(),
		runtime.History(),
		runtime.Memories(),
		combinedInput,
		llm.NarrativeConstraints,
	)

	ctx := context.Background()

	// 调用 LLM（流式或非流式）
	if onChunk != nil {
		resp, err := llmClient.GenerateStream(ctx, prompt, onChunk)
		if err != nil {
			return defaultStaticResponse(runtime.Contract().RoleName, onChunk)
		}
		return resp
	}

	resp, err := llmClient.Generate(ctx, prompt)
	if err != nil {
		return defaultStaticResponse(runtime.Contract().RoleName, onChunk)
	}
	return resp
}

// defaultStaticResponse 生成默认静态响应文本。
func defaultStaticResponse(roleName string, onChunk func(string)) string {
	if roleName == "" {
		roleName = "角色"
	}
	msg := fmt.Sprintf("%s 沉默地注视着命运。", roleName)
	if onChunk != nil {
		for _, ch := range msg {
			onChunk(string(ch))
		}
	}
	return msg
}
