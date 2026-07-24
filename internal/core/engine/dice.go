// internal/core/engine/dice.go
//
// 骰子系统：RollStore、骰子表达式评估、叙事信息提取
//
// 设计原则：
//   - RollStore 确保条件判定与叙事信息使用同一骰值
//   - evalRoll 和 extractRollInfo 共享 RollStore 中的骰值
package engine

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// ============================================================
// RollStore：骰子结果存储，确保条件判定与叙事信息使用同一骰值
// ============================================================

// RollStore 存储一次规则匹配中所有 roll 表达式的结果。
// key 是 roll 表达式原文（如 "roll(1d100)"），value 是骰子点数总和。
//
// 设计意图：
//   - evalCondition 中的 evalRoll 从此处获取骰值进行判定
//   - extractRollInfo 从此处获取骰值生成叙事信息
//   - 两者使用同一骰值，消除结果不一致的问题
type RollStore struct {
	values map[string]int
}

// NewRollStore 创建空的骰子结果存储。
func NewRollStore() *RollStore {
	return &RollStore{values: make(map[string]int)}
}

// Roll 获取或掷骰。如果指定表达式已有结果，直接返回；否则掷骰并缓存。
func (rs *RollStore) Roll(expr string) int {
	if v, ok := rs.values[expr]; ok {
		return v
	}
	// 解析并掷骰
	count, sides := parseRollExprComponents(expr)
	total := 0
	for range count {
		total += rand.IntN(sides) + 1
	}
	rs.values[expr] = total
	return total
}

// Get 获取指定表达式的骰值（不掷骰），不存在时返回 0。
func (rs *RollStore) Get(expr string) int {
	return rs.values[expr]
}

// parseRollExprComponents 从 "roll(1d100)" 中解析出 count 和 sides。
func parseRollExprComponents(expr string) (count, sides int) {
	// 去掉 roll( 和尾部括号
	inner := strings.TrimPrefix(expr, "roll(")
	inner = strings.TrimSuffix(inner, ")")
	parts := strings.Split(inner, "d")
	if len(parts) != 2 {
		return 0, 0
	}
	count, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	sides, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	if count <= 0 || sides <= 0 {
		return 0, 0
	}
	return count, sides
}

// ============================================================
// 骰子表达式评估
// ============================================================

// evalRoll 评估骰子表达式。
//
// 支持的格式：
//   - roll(1d100)          → 掷一个 100 面骰，结果 >= 默认阈值返回 true
//   - roll(1d100) >= 80    → 掷一个 100 面骰，结果 >= 80 返回 true
//   - roll(1d100) > 80     → 掷一个 100 面骰，结果 > 80 返回 true
//   - roll(1d100) <= 30    → 掷一个 100 面骰，结果 <= 30 返回 true
//   - roll(1d100) < 30     → 掷一个 100 面骰，结果 < 30 返回 true
//   - roll(1d100) == 50    → 掷一个 100 面骰，结果 == 50 返回 true
//   - roll(1d100) != 50    → 掷一个 100 面骰，结果 != 50 返回 true
//   - roll(2d6)            → 掷两个 6 面骰，结果 >= 默认阈值返回 true
//
// 默认阈值规则：掷点结果 >= (面数 * 骰子数 / 2)，即中间值（0.5 成功率）
// 例如 1d100 默认阈值 = 50，2d6 默认阈值 = 7
//
// 参数：
//   - cond: 完整条件字符串，如 "roll(1d100)" 或 "roll(1d100) >= 80"
//   - rs: 骰子结果存储（可为 nil，为 nil 时独立掷骰）
//
// 返回值：
//   - bool: 掷点结果是否满足条件
//   - int: 实际骰子点数总和（用于传递给 LLM 作为叙事参考）
//
// 设计说明：
//   如果提供了 RollStore，骰值将从 RollStore 获取/缓存，
//   确保条件判定与叙事信息（extractRollInfo）使用同一骰值。
func evalRoll(cond string, rs *RollStore) (bool, int) {
	cond = strings.TrimSpace(cond)

	// 确保以 roll( 开头
	if !strings.HasPrefix(cond, "roll(") {
		return false, 0
	}

	// 找 roll(...) 的闭合括号
	endExpr := strings.Index(cond, ")")
	if endExpr == -1 || endExpr < 5 {
		return false, 0
	}

	// 提取 roll(...) 括号内的表达式
	start := 5 // len("roll(")
	expr := strings.TrimSpace(cond[start:endExpr])

	// 解析格式：{count}d{sides}
	parts := strings.Split(expr, "d")
	if len(parts) != 2 {
		return false, 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || count <= 0 {
		return false, 0
	}
	sides, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || sides <= 0 {
		return false, 0
	}

	// 从 RollStore 获取或掷骰子
	rollCore := cond[:endExpr+1] // 如 "roll(1d100)"
	var total int
	if rs != nil {
		total = rs.Roll(rollCore)
	} else {
		for range count {
			total += rand.IntN(sides) + 1 // 1 ~ sides
		}
	}

	// 解析自定义阈值（可选）
	// 格式：)... 操作符 数字
	rest := strings.TrimSpace(cond[endExpr+1:])
	var op string
	var thresholdVal int
	useCustomThreshold := false

	if rest != "" {
		ops := []string{">=", "<=", "!=", "==", ">", "<"}
		for _, o := range ops {
			if strings.HasPrefix(rest, o) {
				op = o
				valStr := strings.TrimSpace(rest[len(o):])
				val, err := strconv.Atoi(valStr)
				if err == nil {
					thresholdVal = val
					useCustomThreshold = true
				}
				break
			}
		}
	}

	// 判定
	if useCustomThreshold {
		switch op {
		case ">=":
			return total >= thresholdVal, total
		case ">":
			return total > thresholdVal, total
		case "<=":
			return total <= thresholdVal, total
		case "<":
			return total < thresholdVal, total
		case "==":
			return total == thresholdVal, total
		case "!=":
			return total != thresholdVal, total
		default:
			return false, total
		}
	}

	// 无自定义阈值：使用默认 50% 判定
	threshold := (count * sides) / 2
	if count*sides%2 != 0 {
		threshold++ // 奇数时向上取整
	}

	return total >= threshold, total
}

// ============================================================
// 骰子表达式解析与叙事信息
// ============================================================

// rollExpr 解析 roll(...) 表达式，提取参数和可选的用户阈值。
type rollExpr struct {
	Raw           string // 原始完整条件，如 "roll(1d100) >= 80"
	RollCore      string // roll(...) 核心部分，如 "roll(1d100)"
	Count         int
	Sides         int
	Op            string // 用户阈值操作符（空表示使用默认阈值）
	UserThreshold int    // 用户阈值（仅在 Op 非空时有效）
}

// parseRollExpr 解析条件中的 roll 表达式。
func parseRollExpr(cond string) (rollExpr, bool) {
	cond = strings.TrimSpace(cond)
	if !strings.HasPrefix(cond, "roll(") {
		return rollExpr{}, false
	}

	endExpr := strings.Index(cond, ")")
	if endExpr == -1 || endExpr < 5 {
		return rollExpr{}, false
	}

	rollCore := cond[:endExpr+1]
	expr := strings.TrimSpace(cond[5:endExpr])

	parts := strings.Split(expr, "d")
	if len(parts) != 2 {
		return rollExpr{}, false
	}
	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || count <= 0 {
		return rollExpr{}, false
	}
	sides, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || sides <= 0 {
		return rollExpr{}, false
	}

	re := rollExpr{
		Raw:      cond,
		RollCore: rollCore,
		Count:    count,
		Sides:    sides,
	}

	// 解析自定义阈值
	rest := strings.TrimSpace(cond[endExpr+1:])
	if rest != "" {
		ops := []string{">=", "<=", "!=", "==", ">", "<"}
		for _, o := range ops {
			if strings.HasPrefix(rest, o) {
				valStr := strings.TrimSpace(rest[len(o):])
				val, err := strconv.Atoi(valStr)
				if err == nil {
					re.Op = o
					re.UserThreshold = val
				}
				break
			}
		}
	}

	return re, true
}

// maxValue 返回骰子的最大值（count * sides）。
func (re rollExpr) maxValue() int {
	return re.Count * re.Sides
}

// thresholdDesc 返回阈值描述文本，如 "阈值 ≥80" 或空字符串。
func (re rollExpr) thresholdDesc() string {
	if re.Op == "" {
		return ""
	}
	return fmt.Sprintf("阈值 %s%d", re.Op, re.UserThreshold)
}

// extractRollInfo 从条件字符串中提取所有 roll(...) 表达式的计算结果。
//
// 功能：重新解析条件中的每个 roll(...) 表达式，
// 使用 RollStore 中的骰值（与条件判定使用同一骰值）生成可读的描述文本。
// 如果 rs 为 nil，则独立掷骰（不推荐，会导致判定结果与叙事信息不一致）。
//
// 输出格式示例（单行）：
//
//	[战斗判定] roll(1d100) = 87/100（阈值 ≥80）✅
//	[突然暴怒] roll(1d100) = 97/100 ✅
//
// 参数：
//   - ruleName: 规则名
//   - cond: 条件字符串（如 `包含 "愤怒" && roll(1d100) >= 80`）
//   - rs: 骰子结果存储（应与 evalCondition 使用同一个 RollStore）
//
// 返回值：
//   - string: 骰子结果描述（无 roll 表达式时返回空字符串）
//     如果有多个 roll 表达式，每行一个。
func extractRollInfo(ruleName, cond string, rs *RollStore) string {
	remaining := cond
	var parts []string

	for {
		idx := strings.Index(remaining, "roll(")
		if idx == -1 {
			break
		}
		// 从 idx 开始截取到末尾，用 parseRollExpr 解析
		substr := remaining[idx:]
		re, ok := parseRollExpr(substr)
		if !ok {
			// 解析失败，跳过已处理的 roll( 前缀
			remaining = remaining[idx+5:]
			continue
		}

		// 从 RollStore 获取骰值（不重新掷骰）
		var total int
		if rs != nil {
			total = rs.Get(re.RollCore)
		}
		// 如果 rs 为 nil 或没有值，独立掷骰（兼容旧逻辑，但不推荐）
		if total == 0 && rs == nil {
			for range re.Count {
				total += rand.IntN(re.Sides) + 1
			}
		}

		// 用同一 RollStore 判定该 roll 表达式是否成功，加上 ✅/❌
		matched, _ := evalRoll(re.Raw, rs)
		statusIcon := "❌"
		if matched {
			statusIcon = "✅"
		}

		desc := fmt.Sprintf("%s = %d/%d", re.RollCore, total, re.maxValue())
		if td := re.thresholdDesc(); td != "" {
			desc = fmt.Sprintf("%s（%s）", desc, td)
		}
		// 每个结果带上规则名和成功/失败图标
		parts = append(parts, fmt.Sprintf("[%s] %s %s", ruleName, desc, statusIcon))
		remaining = remaining[len(re.RollCore):] // 移除 roll core

		// 跳过阈值部分（如果有）
		if re.Op != "" {
			// 跳过操作符和数值，如 " >= 80"
			skipLen := len(re.Op) + len(fmt.Sprintf("%d", re.UserThreshold))
			if len(remaining) > skipLen {
				remaining = strings.TrimSpace(remaining[skipLen:])
			} else {
				remaining = ""
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	// 每行一个骰子结果
	return strings.Join(parts, "\n")
}
