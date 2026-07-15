// ============================================================
// expr.go - 表达式解析器
// 职责：将条件字符串解析为 AST（抽象语法树）
// 支持：
//   - 原子条件: 情绪 == "暴怒"
//   - 逻辑运算: 情绪 == "暴怒" && 堕落指数 > 60
//   - 括号分组: (情绪 == "暴怒" || 情绪 == "疯狂") && 堕落指数 > 60
//   - 骰子表达式: roll(1d100) > 80
//   - 短路求值: && 左侧为假时，不再评估右侧
//   - 缺失变量返回 nil（不会导致规则中断）
//   - 实现 parser.Expr 接口，与 parser 包解耦
// ============================================================

package engine

import (
	"fmt"
	"strconv"
	"strings"

	"mephisto/parser"
)

// ============================================================
// 所有 AST 节点实现 parser.Expr 接口
// ============================================================

// VarExpr 表示从上下文中读取变量的表达式
type VarExpr struct {
	Name string // 变量名
}

// Eval 从上下文中获取变量的值
// 如果变量不存在，返回 (nil, nil) 而不是错误
// 这样缺失变量不会导致规则中断，而是优雅降级
func (e *VarExpr) Eval(env map[string]any) (any, error) {
	val, ok := env[e.Name]
	if !ok {
		return nil, nil // 缺失变量返回 nil
	}
	return val, nil
}

// ConstExpr 表示常量值
type ConstExpr struct {
	Val any // 常量值（字符串、数字、布尔值）
}

// Eval 直接返回常量值
func (e *ConstExpr) Eval(env map[string]any) (any, error) {
	return e.Val, nil
}

// BinaryExpr 表示二元运算表达式
type BinaryExpr struct {
	Op    string      // 操作符："&&", "||", "==", "!=", ">", "<", ">=", "<=", "包含"
	Left  parser.Expr // 左子表达式
	Right parser.Expr // 右子表达式
}

// Eval 求值二元表达式
// 实现短路求值（Short-Circuit Evaluation）：
//   - &&：如果左侧为假，直接返回 false，不评估右侧
//   - ||：如果左侧为真，直接返回 true，不评估右侧
//   - 这避免了因右侧变量缺失导致的崩溃
func (e *BinaryExpr) Eval(env map[string]any) (any, error) {
	switch e.Op {
	case "&&":
		// 评估左侧
		leftVal, err := e.Left.Eval(env)
		if err != nil {
			return nil, err
		}
		leftBool, ok := toBool(leftVal)
		if !ok {
			return nil, fmt.Errorf("&& 操作需要布尔值")
		}
		// 短路：左侧为假，直接返回 false
		if !leftBool {
			return false, nil
		}
		// 评估右侧
		rightVal, err := e.Right.Eval(env)
		if err != nil {
			return nil, err
		}
		rightBool, ok := toBool(rightVal)
		if !ok {
			return nil, fmt.Errorf("&& 操作需要布尔值")
		}
		return rightBool, nil

	case "||":
		// 评估左侧
		leftVal, err := e.Left.Eval(env)
		if err != nil {
			return nil, err
		}
		leftBool, ok := toBool(leftVal)
		if !ok {
			return nil, fmt.Errorf("|| 操作需要布尔值")
		}
		// 短路：左侧为真，直接返回 true
		if leftBool {
			return true, nil
		}
		// 评估右侧
		rightVal, err := e.Right.Eval(env)
		if err != nil {
			return nil, err
		}
		rightBool, ok := toBool(rightVal)
		if !ok {
			return nil, fmt.Errorf("|| 操作需要布尔值")
		}
		return rightBool, nil

	default:
		// 比较运算：需要评估左右两边
		leftVal, err := e.Left.Eval(env)
		if err != nil {
			return nil, err
		}
		rightVal, err := e.Right.Eval(env)
		if err != nil {
			return nil, err
		}

		// 如果任一操作数为 nil，返回 false
		if leftVal == nil || rightVal == nil {
			return false, nil
		}

		switch e.Op {
		case "==":
			return compareEqual(leftVal, rightVal), nil
		case "!=":
			return !compareEqual(leftVal, rightVal), nil
		case ">", "<", ">=", "<=":
			return compareNumeric(leftVal, rightVal, e.Op)
		case "包含":
			return compareContains(leftVal, rightVal), nil
		default:
			return nil, fmt.Errorf("不支持的操作符: %s", e.Op)
		}
	}
}

// ============================================================
// 辅助函数
// ============================================================

// toBool 尝试将值转换为布尔值
func toBool(v any) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	default:
		return false, false
	}
}

// compareEqual 比较两个值是否相等
// 类型不同时尝试转换为字符串进行比较
func compareEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// 尝试字符串比较（兜底）
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// compareNumeric 比较两个数值
// 支持 int 和 float64 的混合比较
func compareNumeric(left, right any, op string) (bool, error) {
	// 转换为 float64
	lNum, err := toFloat64(left)
	if err != nil {
		return false, fmt.Errorf("左侧无法转换为数字: %v", left)
	}
	rNum, err := toFloat64(right)
	if err != nil {
		return false, fmt.Errorf("右侧无法转换为数字: %v", right)
	}

	switch op {
	case ">":
		return lNum > rNum, nil
	case "<":
		return lNum < rNum, nil
	case ">=":
		return lNum >= rNum, nil
	case "<=":
		return lNum <= rNum, nil
	default:
		return false, fmt.Errorf("不支持的比较操作: %s", op)
	}
}

// compareContains 检查左侧是否包含右侧（字符串包含）
func compareContains(left, right any) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.Contains(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right))
}

// toFloat64 尝试将 any 转换为 float64
func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("无法转换为数字: %T %v", v, v)
	}
}

// ============================================================
// DiceExprWrapper：骰子表达式包装器（使骰子可以作为 Expr 使用）
// ============================================================

// DiceExprWrapper 包装骰子表达式，使其实现 parser.Expr
type DiceExprWrapper struct {
	ExprStr string // 原始骰子表达式字符串，如 "roll(1d100)"
}

// Eval 执行骰子并返回结果
func (e *DiceExprWrapper) Eval(env map[string]any) (any, error) {
	return RollDice(e.ExprStr)
}

// ============================================================
// ParseExpression 入口函数
// ============================================================

// ParseExpression 解析条件字符串为 AST 表达式，返回 parser.Expr
// 这是 engine 包提供给外部（主要是 parser 包）的解析函数
// 在 init 中注册到 parser.ParseExprFunc
func ParseExpression(cond string) (parser.Expr, error) {
	cond = strings.TrimSpace(cond)
	// 去除最外层括号
	for strings.HasPrefix(cond, "(") && strings.HasSuffix(cond, ")") {
		inner := cond[1 : len(cond)-1]
		if isBalanced(inner) {
			cond = strings.TrimSpace(inner)
		} else {
			break
		}
	}

	// 按优先级分割：先 && 再 ||
	if expr, ok := splitByOperator(cond, "&&"); ok {
		left, err := ParseExpression(expr.Left)
		if err != nil {
			return nil, err
		}
		right, err := ParseExpression(expr.Right)
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Op: "&&", Left: left, Right: right}, nil
	}

	if expr, ok := splitByOperator(cond, "||"); ok {
		left, err := ParseExpression(expr.Left)
		if err != nil {
			return nil, err
		}
		right, err := ParseExpression(expr.Right)
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Op: "||", Left: left, Right: right}, nil
	}

	// 原子条件
	return parsePrimary(cond)
}

// parsePrimary 解析原子表达式（变量、常量、比较运算）
func parsePrimary(s string) (parser.Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("空表达式")
	}

	// 先检测是否为比较运算（优先级高于骰子）
	operators := []string{"!=", "==", ">=", "<=", ">", "<", "包含"}
	for _, op := range operators {
		idx := strings.Index(s, op)
		if idx != -1 {
			left := strings.TrimSpace(s[:idx])
			right := strings.TrimSpace(s[idx+len(op):])
			if left != "" && right != "" {
				leftExpr, err := parsePrimary(left)
				if err != nil {
					return nil, err
				}
				rightExpr, err := parsePrimary(right)
				if err != nil {
					return nil, err
				}
				return &BinaryExpr{Op: op, Left: leftExpr, Right: rightExpr}, nil
			}
		}
	}

	// 再检测骰子表达式
	if IsDiceExpression(s) {
		return &DiceExprWrapper{ExprStr: s}, nil
	}

	// 检测字符串常量（带引号）
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return &ConstExpr{Val: s[1 : len(s)-1]}, nil
	}
	if strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`) {
		return &ConstExpr{Val: s[1 : len(s)-1]}, nil
	}

	// 检测数字常量
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return &ConstExpr{Val: f}, nil
		}
	} else {
		if i, err := strconv.Atoi(s); err == nil {
			return &ConstExpr{Val: i}, nil
		}
	}

	// 检测布尔常量
	if s == "true" {
		return &ConstExpr{Val: true}, nil
	}
	if s == "false" {
		return &ConstExpr{Val: false}, nil
	}

	// 默认作为变量
	return &VarExpr{Name: s}, nil
}

// splitByOperator 按操作符分割表达式，跳过括号内的内容
type splitPair struct {
	Left  string
	Right string
}

func splitByOperator(cond, op string) (*splitPair, bool) {
	cond = strings.TrimSpace(cond)
	parenDepth := 0
	opLen := len(op)

	for i := 0; i < len(cond); i++ {
		switch cond[i] {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		default:
			if parenDepth == 0 && i+opLen <= len(cond) && cond[i:i+opLen] == op {
				left := strings.TrimSpace(cond[:i])
				right := strings.TrimSpace(cond[i+opLen:])
				if left != "" && right != "" {
					return &splitPair{Left: left, Right: right}, true
				}
			}
		}
	}
	return nil, false
}

// isBalanced 检查括号是否平衡
func isBalanced(s string) bool {
	count := 0
	for _, ch := range s {
		if ch == '(' {
			count++
		} else if ch == ')' {
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}
