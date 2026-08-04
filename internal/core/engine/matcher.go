// internal/core/engine/matcher.go
//
// 规则匹配系统：被动规则批量执行、主动规则互斥匹配
//
// 设计原则：
//   - 两阶段匹配：被动规则（副作用）批量执行 + 主动规则（输出）互斥匹配
//   - 被动规则不产生输出，多条同时匹配时全部执行
//   - 主动规则只产生输出，只执行第一个匹配的（同一互斥组内）
package engine

import (
	"fmt"
	"os"
	"strings"

	"mephisto/internal/domain"
)

// debugPrint 打印调试信息到 stderr（仅在调试模式下输出）。
//
// 惰性求值说明：调用方应将 format 与 args 直接传入本函数，
// 而非先 fmt.Sprintf 再传递。fmt.Fprintf 的字符串格式化
// 只在 debug=true 时才执行，debug=false（默认）时零开销。
func debugPrint(debug bool, format string, args ...any) {
	if !debug {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// previewAction 截断动作文本用于调试展示（超过 60 字符时省略）。
// 仅调试模式调用，开销为 len + 切片，可忽略。
func previewAction(action string) string {
	if len(action) > 60 {
		return action[:60] + "..."
	}
	return action
}

// ============================================================
// 两阶段规则匹配：被动规则（批量）+ 主动规则（互斥）
// ============================================================

// isPassiveAction 判断动作是否为被动动作。
//
// 被动动作包括：
//   - 状态修改：以 "状态." 开头
//   - 注入记忆：以 "注入 " 开头
//
// 被动动作的特点是：只产生副作用，不直接输出文本。
// 它们可以被批量执行，多个规则同时匹配也不会冲突。
func isPassiveAction(action string) bool {
	return strings.HasPrefix(action, actionStatePrefix) || strings.HasPrefix(action, actionInjectPrefix)
}

// matchPassiveRules 批量执行所有满足条件的被动规则。
//
// 设计目的：状态修改和注入记忆是"副作用"操作，不会直接产生输出。
// 多条这样的规则同时匹配时应该全部执行，而不是只执行第一条。
//
// 执行流程：
//  1. 遍历所有规则，筛选出被动规则（状态修改/注入记忆）
//  2. 对每条规则评估条件
//  3. 如果条件满足且互斥组未触发，立即执行动作
//  4. 互斥组：同一组内只触发第一个匹配的规则
//
// 参数：
//   - rules: 规则列表
//   - input: 当前用户输入
//   - runtime: 运行时（用于读取最新状态 + 执行动作）
//   - debug: 是否输出调试信息
//
// 返回值：
//   - bool: 是否有被动规则被触发
//   - string: 骰子结果描述（所有触发的被动规则中的 roll() 结果拼接）
func matchPassiveRules(rules []*domain.Rule, input string, runtime *Runtime, debug bool) (bool, string) {
	triggeredGroups := make(map[string]bool)
	var rollParts []string
	anyMatched := false

	debugPrint(debug, "🔍 阶段一：被动规则批量执行")
	debugPrint(debug, "----------------------------------------")

	for _, rule := range rules {
		if !isPassiveAction(rule.Action) {
			continue
		}

		// 惰性求值：debug=false 时不构造调试字符串
		debugPrint(debug, "📌 检查被动规则 [%s] (行 %d)", rule.Name, rule.Line)
		debugPrint(debug, "   条件: %s", rule.Cond)

		// 互斥组检查
		if rule.Group != "" && triggeredGroups[rule.Group] {
			debugPrint(debug, "   ⏭️  跳过: 组 [%s] 已触发", rule.Group)
			debugPrint(debug, "")
			continue
		}

		// 每次从 runtime 读取最新状态（前面的被动规则可能已修改状态）
		state := runtime.State()
		// 创建骰子结果存储，确保条件判定与提取使用同一骰值
		rs := NewRollStore()
		result := evalCondition(rule.Cond, input, state, rs)
		if result {
			debugPrint(debug, "   结果: true")
			debugPrint(debug, "   ✦ 触发 → %s", previewAction(rule.Action))
			anyMatched = true

			if rule.Group != "" {
				debugPrint(debug, "   🔒 锁定组 [%s]", rule.Group)
				triggeredGroups[rule.Group] = true
			}

			// 提取骰子信息（使用同一 RollStore）
			// 仅规则真正触发时展示骰子结果（对齐 Flutter：未匹配的规则不显示）
			if hasRoll(rule.Cond) {
				if ri := extractRollInfo(rule.Name, rule.Cond, rs); ri != "" {
					rollParts = append(rollParts, ri)
				}
			}

			// 执行动作（被动动作没有直接输出，传递 nil onChunk）
			ExecuteAction(rule.Action, input, runtime, nil, nil, "")
			debugPrint(debug, "")
		} else {
			debugPrint(debug, "   结果: false")
			debugPrint(debug, "   ╳ 未触发")
			debugPrint(debug, "")
		}
	}

	rollInfo := strings.Join(rollParts, "\n")
	debugPrint(debug, "----------------------------------------")
	matchedCount := 0
	if anyMatched {
		matchedCount = 1
	}
	debugPrint(debug, "📊 被动规则执行完成: %d 条触发\n", matchedCount)
	return anyMatched, rollInfo
}

// matchActiveRule 匹配第一条满足条件的主动规则（互斥匹配）。
//
// 主动规则是产生直接输出的规则（LLM 指令、静态文本等）。
// 同一轮对话中，只触发第一个匹配的主动规则。
//
// 参数：
//   - rules: 规则列表
//   - input: 当前用户输入
//   - state: 当前状态快照（由调用方传入，可能是被动规则更新后的状态）
//   - debug: 是否输出调试信息
//
// 返回值：
//   - *domain.Rule: 匹配到的规则（nil 表示无匹配）
//   - bool: 是否匹配成功
//   - string: 骰子结果描述（仅匹配规则触发时返回，未匹配的规则不显示骰子）
func matchActiveRule(rules []*domain.Rule, input string, state map[string]any, debug bool) (*domain.Rule, bool, string) {
	triggeredGroups := make(map[string]bool)
	var rollInfo string

	debugPrint(debug, "🔍 阶段二：主动规则互斥匹配")
	debugPrint(debug, "----------------------------------------")

	for _, rule := range rules {
		// 跳过被动规则（已在阶段一执行）
		if isPassiveAction(rule.Action) {
			continue
		}

		// 惰性求值：debug=false 时不构造调试字符串
		debugPrint(debug, "📌 检查主动规则 [%s] (行 %d)", rule.Name, rule.Line)
		debugPrint(debug, "   条件: %s", rule.Cond)

		// 互斥组检查
		if rule.Group != "" && triggeredGroups[rule.Group] {
			debugPrint(debug, "   ⏭️  跳过: 组 [%s] 已触发", rule.Group)
			debugPrint(debug, "")
			continue
		}

		// 创建骰子结果存储，确保条件判定与信息提取使用同一骰值
		rs := NewRollStore()
		result := evalCondition(rule.Cond, input, state, rs)
		if result {
			debugPrint(debug, "   结果: true")
			debugPrint(debug, "   ✦ 触发 → %s", previewAction(rule.Action))
			if rule.Group != "" {
				debugPrint(debug, "   🔒 锁定组 [%s]", rule.Group)
				triggeredGroups[rule.Group] = true
			}
			debugPrint(debug, "")
			// 提取骰子结果信息（使用同一 RollStore）
			// 仅规则真正触发时展示骰子结果（对齐 Flutter：未匹配的规则不显示）
			rollInfo = extractRollInfo(rule.Name, rule.Cond, rs)
			return rule, true, rollInfo
		}

		debugPrint(debug, "   结果: false")
		debugPrint(debug, "   ╳ 未触发")
		debugPrint(debug, "")
	}

	debugPrint(debug, "----------------------------------------")
	debugPrint(debug, "📊 共检查 %d 条主动规则，未匹配到任何规则\n", len(rules))
	// 无匹配时返回空骰子信息（对齐 Flutter：未匹配的规则不显示骰子）
	return nil, false, ""
}

