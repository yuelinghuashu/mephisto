// ============================================================
// types.go - 规则引擎数据结构
// 职责：
// 1. 定义规则引擎的核心数据结构（Rule, Context, ActionResult）
// 2. 这些结构在 engine 包内部使用，与 parser 包无直接依赖
// ============================================================

package engine

import "mephisto/parser"

// Rule 表示一条可执行的规则
type Rule struct {
	Name      string      // 规则名称
	Condition string      // 条件字符串（原始，仅用于显示）
	Action    string      // 动作字符串
	Group     string      // 互斥组名
	Line      int         // 来源行号
	Expr      parser.Expr // 预编译的 AST（使用 parser.Expr 接口）
}

// Context 是规则求值时的上下文
// 键为变量名，值为任意类型
type Context map[string]any

// ActionResult 表示动作执行的结果
type ActionResult struct {
	Success bool   // 是否执行成功
	Message string // 执行信息
}
