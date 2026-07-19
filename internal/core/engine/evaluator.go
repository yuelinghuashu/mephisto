// internal/core/engine/evaluator.go
//
// 本文件提供默认的条件评估器实现。
// 条件评估器负责解析和评估规则中的条件表达式，
// 决定规则是否应当被触发。
//
// 支持的条件语法：
//   - 包含 "xxx"              → 用户输入包含指定文本
//   - 不包含 "xxx"            → 用户输入不包含指定文本
//   - 状态.键 > 值            → 状态值大于指定值
//   - 状态.键 >= 值           → 状态值大于等于指定值
//   - 状态.键 < 值            → 状态值小于指定值
//   - 状态.键 <= 值           → 状态值小于等于指定值
//   - 状态.键 == 值           → 状态值等于指定值
//   - 状态.键 != 值           → 状态值不等于指定值
//   - 条件1 && 条件2          → 两个条件都满足（与运算）
//   - 条件1 || 条件2          → 任意条件满足（或运算）
//
// 组合规则：
//   - && 优先级高于 ||，但建议使用括号明确优先级
package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultConditionEvaluator 是默认的条件评估器。
//
// 该实现是纯函数式的，不持有任何外部状态，因此可以在多个引擎实例间安全复用。
// 所有条件评估都是确定性的——相同输入总是产生相同输出。
//
// 设计决策：
//   - 状态不存在时，状态条件返回 false（而非报错）
//   - 类型不匹配时，尝试转换后再比较（如字符串 "85" 与数字 85 的比较）
//   - 不支持的语法返回明确的错误信息，便于用户调试
type DefaultConditionEvaluator struct{}

// Evaluate 评估条件表达式。
//
// 处理流程：
//  1. 去除首尾空白，空条件返回 false
//  2. 按优先级拆分逻辑运算符（|| 优先，再拆分 &&）
//  3. 递归评估子条件
//  4. 评估原子条件
//
// 返回值：
//   - bool: 条件是否满足
//   - error: 语法错误、类型错误等
func (e *DefaultConditionEvaluator) Evaluate(cond string, ctx ConditionContext) (bool, error) {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return false, fmt.Errorf("条件为空")
	}

	// 处理逻辑运算符（优先级：&& > ||）
	// 先拆分 ||，再拆分 &&
	if strings.Contains(cond, "||") {
		return e.evaluateLogical(cond, "||", false, ctx)
	}
	if strings.Contains(cond, "&&") {
		return e.evaluateLogical(cond, "&&", true, ctx)
	}
	// 原子条件
	return e.evaluateAtomic(cond, ctx)
}

// evaluateLogical 评估逻辑表达式（&& 或 ||）。
//
// 参数：
//   - cond: 原始条件字符串
//   - sep: 分隔符（"&&" 或 "||"）
//   - requireAll: true 表示需要全部满足（&&），false 表示任意满足（||）
//   - ctx: 评估上下文
//
// 返回值：
//   - bool: 逻辑表达式的评估结果
//   - error: 子条件评估过程中的错误
//
// 示例：
//
//	evaluateLogical("a && b", "&&", true, ctx) → a 和 b 都满足才返回 true
//	evaluateLogical("a || b", "||", false, ctx) → a 或 b 任意满足返回 true
func (e *DefaultConditionEvaluator) evaluateLogical(cond, sep string, requireAll bool, ctx ConditionContext) (bool, error) {
	for part := range strings.SplitSeq(cond, sep) {
		matched, err := e.Evaluate(strings.TrimSpace(part), ctx)
		if err != nil {
			return false, err
		}
		// 与运算：任意 false → 整体 false
		if requireAll && !matched {
			return false, nil
		}
		// 或运算：任意 true → 整体 true
		if !requireAll && matched {
			return true, nil
		}
	}
	if requireAll {
		return true, nil // 全部满足
	}
	return false, nil // 全部不满足
}

// evaluateAtomic 评估单个原子条件。
//
// 原子条件是不能被逻辑运算符（&&/||）拆分的独立条件。
// 当前支持三类原子条件：
//  1. 包含/不包含
//  2. 状态比较
//  3. （未来扩展）骰子、历史检查等
func (e *DefaultConditionEvaluator) evaluateAtomic(cond string, ctx ConditionContext) (bool, error) {
	cond = strings.TrimSpace(cond)

	// ---- 包含/不包含 ----
	if strings.HasPrefix(cond, "包含 ") {
		return e.evaluateContains(cond, "包含 ", false, ctx)
	}
	if strings.HasPrefix(cond, "不包含 ") {
		return e.evaluateContains(cond, "不包含 ", true, ctx)
	}

	// ---- 状态比较 ----
	if strings.HasPrefix(cond, "状态.") {
		return e.evaluateStateCondition(cond, ctx)
	}

	// ---- 不支持的语法 ----
	return false, fmt.Errorf("不支持的条件格式: %s", cond)
}

// evaluateContains 评估包含/不包含条件。
//
// 参数：
//   - cond: 原始条件（如 `包含 "攻击"`）
//   - prefix: 条件前缀（"包含 " 或 "不包含 "）
//   - negate: true 表示取反（不包含），false 表示正常（包含）
//   - ctx: 评估上下文
//
// 返回值：
//   - bool: 条件是否满足
//   - error: 引号解析错误
func (e *DefaultConditionEvaluator) evaluateContains(cond, prefix string, negate bool, ctx ConditionContext) (bool, error) {
	substr, err := unquote(strings.TrimPrefix(cond, prefix))
	if err != nil {
		return false, fmt.Errorf("解析包含条件失败: %w", err)
	}
	result := strings.Contains(ctx.Input, substr)
	if negate {
		return !result, nil
	}
	return result, nil
}

// evaluateStateCondition 评估状态条件。
//
// 格式：状态.键 操作符 值
// 支持的操作符：>=, <=, !=, ==, >, <
//
// 类型处理策略：
//   - 状态值总是优先以存储的类型进行比较
//   - 如果状态值和比较值类型不一致，尝试将比较值转换为状态值的类型
//   - 如果转换失败，返回错误
//
// 示例：
//   - 状态.堕落指数 > 80      → 堕落指数 > 80
//   - 状态.情绪 == "愤怒"     → 情绪等于"愤怒"
//   - 状态.生命值 <= 0        → 生命值 <= 0
func (e *DefaultConditionEvaluator) evaluateStateCondition(cond string, ctx ConditionContext) (bool, error) {
	// 去掉 "状态." 前缀
	rest := strings.TrimPrefix(cond, "状态.")
	rest = strings.TrimSpace(rest)

	// 查找操作符（优先匹配多字符操作符）
	ops := []string{">=", "<=", "!=", "==", ">", "<"}
	var op string
	var idx int
	for _, o := range ops {
		if i := strings.Index(rest, o); i != -1 {
			op = o
			idx = i
			break
		}
	}
	if op == "" {
		return false, fmt.Errorf("状态条件缺少操作符: %s", cond)
	}

	// 提取键和值字符串
	key := strings.TrimSpace(rest[:idx])
	valStr := strings.TrimSpace(rest[idx+len(op):])

	if key == "" {
		return false, fmt.Errorf("状态键不能为空")
	}
	if valStr == "" {
		return false, fmt.Errorf("状态值不能为空")
	}

	// 获取当前状态值
	stateVal, exists := ctx.State[key]
	if !exists {
		// 状态不存在，视为 false
		return false, nil
	}

	// 执行比较
	switch op {
	case "==":
		return compareEqual(stateVal, valStr), nil
	case "!=":
		return !compareEqual(stateVal, valStr), nil
	case ">", ">=", "<", "<=":
		return compareNumeric(stateVal, valStr, op)
	default:
		return false, fmt.Errorf("不支持的操作符: %s", op)
	}
}

// compareEqual 比较两个值是否相等。
//
// 支持跨类型比较：
//   - 数字与数字：统一转为 float64 后按数值比较
//   - 字符串与字符串：按文本比较
//   - 数字与字符串：尝试将字符串转为数字后再比较
//   - 布尔值：按布尔值比较
//   - 其他类型：使用 fmt.Sprintf 转为字符串后再比较
func compareEqual(a any, b string) bool {
	// 数字类型统一用 float64 比较
	if isNumeric(a) {
		left := toFloat64(a)
		right, err := strconv.ParseFloat(b, 64)
		if err != nil {
			return false
		}
		return left == right
	}

	switch v := a.(type) {
	case string:
		// 尝试去除 b 的引号
		unquoted, _ := unquote(b)
		return v == unquoted
	case bool:
		if bv, err := strconv.ParseBool(b); err == nil {
			return v == bv
		}
		return false
	default:
		return fmt.Sprintf("%v", v) == b
	}
}

// compareNumeric 比较两个数字值。
//
// 操作符：>, >=, <, <=
// 要求：
//   - 状态值必须是数字类型（int 或 float64）
//   - 比较值必须是有效的数字字符串
//   - 不满足上述条件时返回错误
func compareNumeric(a any, b string, op string) (bool, error) {
	if !isNumeric(a) {
		return false, fmt.Errorf("状态值不是数字类型: %v (类型: %T)", a, a)
	}

	left := toFloat64(a)
	right, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return false, fmt.Errorf("比较值不是有效数字: %s", b)
	}

	switch op {
	case ">":
		return left > right, nil
	case ">=":
		return left >= right, nil
	case "<":
		return left < right, nil
	case "<=":
		return left <= right, nil
	default:
		return false, fmt.Errorf("不支持的操作符: %s", op)
	}
}

// isNumeric 判断值是否为数字类型（int 或 float64）。
func isNumeric(a any) bool {
	switch a.(type) {
	case int, float64:
		return true
	default:
		return false
	}
}

// toFloat64 将 int 或 float64 统一转为 float64。
func toFloat64(v any) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case float64:
		return val
	default:
		return 0
	}
}

