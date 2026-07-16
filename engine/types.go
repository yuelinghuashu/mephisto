// ============================================================
// types.go - 规则引擎数据结构
// 职责：
// 1. 定义规则引擎的核心数据结构（Rule, Context, ActionResult）
// 2. 定义动作类型枚举
// 3. 这些结构在 engine 包内部使用，与 parser 包无直接依赖
// ============================================================

package engine

import "mephisto/parser"

// ActionType 表示动作的类型
type ActionType int

const (
	ActionPlain  ActionType = iota // 普通消息
	ActionInject                   // 注入内容
	ActionLLM                      // LLM 生成指令
	ActionDice                     // 骰子结果
)

// Rule 表示一条可执行的规则
type Rule struct {
	Name      string      // 规则名称
	Condition string      // 条件字符串（原始，仅用于显示）
	Action    string      // 动作字符串
	Group     string      // 互斥组名
	Line      int         // 来源行号
	Expr      parser.Expr // 预编译的 AST
}

// Context 是规则求值时的上下文
type Context map[string]any

// ActionResult 表示动作执行的结果
type ActionResult struct {
	Success bool       // 是否执行成功
	Type    ActionType // 动作类型
	Data    string     // 数据：注入内容、LLM 指令、普通消息等
	Message string     // 完整描述信息（用于显示）
}
