// internal/core/engine/condition.go
//
// 条件评估系统：评估规则条件表达式
//
// 支持语法：
//   - 包含/不包含 "关键词"    → 文本匹配
//   - 状态.键 操作符 值      → 状态比较
//   - roll(1d100)            → 骰子判定
//   - 条件1 && 条件2         → 与运算
//   - 条件1 || 条件2         → 或运算
package engine

import (
	"fmt"
	"strconv"
	"strings"

	"mephisto/internal/shared"
)

// ============================================================
// 条件评估（直接函数，无接口）
// ============================================================

// evalCondition 评估条件表达式。
//
// 支持语法：
//   - 包含 "关键词"      → 用户输入包含指定文本
//   - 不包含 "关键词"    → 用户输入不包含指定文本
//   - 状态.键 > 值      → 状态值大于指定值
//   - 状态.键 >= 值     → 状态值大于等于指定值
//   - 状态.键 < 值      → 状态值小于指定值
//   - 状态.键 <= 值     → 状态值小于等于指定值
//   - 状态.键 == 值     → 状态值等于指定值
//   - 状态.键 != 值     → 状态值不等于指定值
//   - 条件1 && 条件2    → 两个条件都满足（与运算）
//   - 条件1 || 条件2    → 任意条件满足（或运算）
//   - roll(1d100)       → 掷骰子，结果 >= 默认阈值时返回 true
//   - roll(1d100) >= 80 → 掷骰子，结果 >= 80 时返回 true（自定义阈值）
//
// 参数：
//   - cond: 条件字符串（如 `包含 "攻击"`）
//   - input: 当前用户输入
//   - state: 当前状态 map
//   - rs: 骰子结果存储（可为 nil，为 nil 时独立掷骰，但不会与叙事信息同步）
//
// 返回值：
//   - bool: 条件是否满足
//
// 注意事项：
//   - 逻辑运算符优先级：&& 高于 ||
//   - 状态不存在时返回 false（而非报错）
//   - 类型不匹配时尝试转换后再比较
func evalCondition(cond, input string, state map[string]any, rs *RollStore) bool {
	cond = strings.TrimSpace(cond)

	// ---- 逻辑运算符（优先级：&& > ||） ----
	// 先处理 ||，再处理 &&
	if strings.Contains(cond, "||") {
		for _, p := range strings.Split(cond, "||") {
			if evalCondition(strings.TrimSpace(p), input, state, rs) {
				return true
			}
		}
		return false
	}

	if strings.Contains(cond, "&&") {
		for _, p := range strings.Split(cond, "&&") {
			if !evalCondition(strings.TrimSpace(p), input, state, rs) {
				return false
			}
		}
		return true
	}

	// ---- 原子条件 ----
	switch {
	case strings.HasPrefix(cond, "包含 "):
		keyword := shared.Unquote(strings.TrimPrefix(cond, "包含 "))
		return strings.Contains(input, keyword)

	case strings.HasPrefix(cond, "不包含 "):
		keyword := shared.Unquote(strings.TrimPrefix(cond, "不包含 "))
		return !strings.Contains(input, keyword)

	case strings.HasPrefix(cond, "状态."):
		return evalStateCondition(cond, state)

	case strings.HasPrefix(cond, "roll("):
		matched, _ := evalRoll(cond, rs)
		return matched

	default:
		return false
	}
}

// evalStateCondition 评估状态条件。
//
// 格式：状态.键 操作符 值
// 支持的操作符：>=, <=, !=, ==, >, <
//
// 类型处理策略：
//   - 状态值总是以存储的类型进行比较
//   - 如果状态值和比较值类型不一致，尝试转换
//   - 转换失败时返回 false
func evalStateCondition(cond string, state map[string]any) bool {
	rest := strings.TrimPrefix(cond, "状态.")
	rest = strings.TrimSpace(rest)

	// 查找操作符（优先匹配多字符）
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
		return false
	}

	key := strings.TrimSpace(rest[:idx])
	valStr := strings.TrimSpace(rest[idx+len(op):])

	stateVal, ok := state[key]
	if !ok {
		return false
	}

	switch op {
	case "==":
		return equalValue(stateVal, valStr)
	case "!=":
		return !equalValue(stateVal, valStr)
	case ">", ">=", "<", "<=":
		return compareNumeric(stateVal, valStr, op)
	}
	return false
}

// equalValue 比较两个值是否相等（支持跨类型）。
func equalValue(a any, b string) bool {
	switch v := a.(type) {
	case int, int64, float64:
		// 数字类型：统一转为 float64 比较
		left := toFloat(v)
		right, err := strconv.ParseFloat(b, 64)
		return err == nil && left == right
	case string:
		return v == shared.Unquote(b)
	case bool:
		if bv, err := strconv.ParseBool(b); err == nil {
			return v == bv
		}
		return false
	default:
		return fmt.Sprintf("%v", a) == b
	}
}

// compareNumeric 比较两个数字值。
//
// 操作符：>, >=, <, <=
// 要求：
//   - 状态值必须是数字类型（int、int64 或 float64）
//   - 比较值必须是有效的数字字符串
//   - 不满足上述条件时返回 false
func compareNumeric(a any, b string, op string) bool {
	// 将状态值转为 float64
	var left float64
	switch v := a.(type) {
	case int:
		left = float64(v)
	case int64:
		left = float64(v)
	case float64:
		left = v
	default:
		return false
	}

	// 解析比较值
	right, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return false
	}

	// 执行比较
	switch op {
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	}
	return false
}

// toFloat 将数字类型转换为 float64。
// 用于 equalValue 中对数字类型做统一比较。
func toFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	default:
		return 0
	}
}
