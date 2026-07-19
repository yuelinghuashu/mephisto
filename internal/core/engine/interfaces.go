// internal/core/engine/interfaces.go
//
// 本文件定义引擎的可扩展接口。
// 通过将条件评估和动作执行抽象为接口，引擎核心保持稳定，
// 具体实现可以按需替换或扩展。
package engine

import (
	"mephisto/internal/core/llm"
	"mephisto/internal/domain"
)

// ConditionEvaluator 评估条件表达式的接口。
//
// 设计目的：
//   - 将条件语法与引擎核心解耦
//   - 便于单元测试（可注入 mock）
//   - 允许未来替换为更强大的表达式引擎
//
// 默认实现：DefaultConditionEvaluator
//
//	支持：包含/不包含、状态比较（> < == !=）、&& / || 组合
type ConditionEvaluator interface {
	// Evaluate 评估条件表达式。
	// 参数：
	//   - cond: 条件字符串（如 `包含 "攻击"`）
	//   - ctx:  评估上下文（输入、状态、历史、记忆）
	// 返回值：
	//   - bool: 条件是否满足
	//   - error: 解析或评估过程中的错误
	Evaluate(cond string, ctx ConditionContext) (bool, error)
}

// ActionExecutor 执行动作的接口。
//
// 设计目的：
//   - 将动作语法与引擎核心解耦
//   - 便于单元测试（可注入 mock）
//   - 允许未来扩展新动作类型（如 LLM 调用、骰子等）
//
// 默认实现：DefaultActionExecutor
//
//	支持：注入、状态修改、普通文本
type ActionExecutor interface {
	// Execute 执行动作。
	// 参数：
	//   - action: 动作字符串（如 `注入 "背景信息"`）
	//   - ctx:    执行上下文（输入、状态、历史、记忆、契约）
	//   - onChunk: 流式回调，逐块输出响应内容（可为 nil，表示非流式）
	// 返回值：
	//   - string: 执行结果（通常为引擎的响应文本）
	//   - error:  执行过程中的错误
	Execute(action string, ctx *ActionContext, onChunk func(string)) (string, error)
}

// ConditionContext 条件评估的上下文。
// 包含评估条件所需的所有运行时数据。
type ConditionContext struct {
	Input    string                // 当前用户输入
	State    map[string]any        // 当前状态
	History  []domain.HistoryEntry // 对话历史
	Memories []string              // 长期记忆
}

// ActionContext 动作执行的上下文。
// 包含执行动作所需的所有运行时数据及契约信息。
type ActionContext struct {
	Input     string                // 当前用户输入
	State     map[string]any        // 当前状态（可修改）
	History   []domain.HistoryEntry // 对话历史
	Memories  []string              // 长期记忆（可追加）
	Contract  *domain.Contract      // 完整契约（只读）
	LLMClient llm.Client            // LLM 客户端（可调用）
}
