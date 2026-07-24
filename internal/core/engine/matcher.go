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

// debugPrint 打印调试信息到 stderr（仅在调试模式下输出）
func debugPrint(debug bool, format string, args ...any) {
	if !debug {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
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
	return strings.HasPrefix(action, "状态.") || strings.HasPrefix(action, "注入 ")
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

		ruleInfo := fmt.Sprintf("📌 检查被动规则 [%s] (行 %d)", rule.Name, rule.Line)
		debugPrint(debug, "%s", ruleInfo)
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
		// 无论条件是否匹配，只要包含 roll(...) 就提取骰子结果展示给用户
		hasRoll := strings.Contains(rule.Cond, "roll(")
		result := evalCondition(rule.Cond, input, state, rs)
		if result {
			debugPrint(debug, "   结果: true")
			actionPreview := rule.Action
			if len(actionPreview) > 60 {
				actionPreview = actionPreview[:60] + "..."
			}
			debugPrint(debug, "   ✅ 触发 → %s", actionPreview)
			anyMatched = true

			if rule.Group != "" {
				debugPrint(debug, "   🔒 锁定组 [%s]", rule.Group)
				triggeredGroups[rule.Group] = true
			}

			// 提取骰子信息（使用同一 RollStore）
			if hasRoll {
				if ri := extractRollInfo(rule.Name, rule.Cond, rs); ri != "" {
					rollParts = append(rollParts, ri)
				}
			}

			// 执行动作（被动动作没有直接输出，传递 nil onChunk）
			ExecuteAction(rule.Action, input, runtime, nil, nil, "")
			debugPrint(debug, "")
		} else {
			debugPrint(debug, "   结果: false")
			debugPrint(debug, "   ❌ 未触发")
			// 失败时也提取骰子信息，让用户看到失败的点数
			if hasRoll {
				if ri := extractRollInfo(rule.Name, rule.Cond, rs); ri != "" {
					rollParts = append(rollParts, ri)
				}
			}
			debugPrint(debug, "")
		}
	}

	rollInfo := strings.Join(rollParts, "\n")
	debugPrint(debug, "----------------------------------------")
	debugPrint(debug, "📊 被动规则执行完成: %d 条触发\n", countMatched(anyMatched))
	return anyMatched, rollInfo
}

// countMatched 辅助函数：将 bool 转为 1/0 用于调试输出。
func countMatched(matched bool) int {
	if matched {
		return 1
	}
	return 0
}

// matchActiveRule 匹配第一条满足条件的主动规则（互斥匹配），
// 同时收集所有主动规则（含未匹配的）的骰子信息。
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
//   - string: 骰子结果描述（所有包含 roll() 的规则结果，每行一个）
func matchActiveRule(rules []*domain.Rule, input string, state map[string]any, debug bool) (*domain.Rule, bool, string) {
	triggeredGroups := make(map[string]bool)
	var allRollParts []string
	var rollInfo string

	debugPrint(debug, "🔍 阶段二：主动规则互斥匹配")
	debugPrint(debug, "----------------------------------------")

	for _, rule := range rules {
		// 跳过被动规则（已在阶段一执行）
		if isPassiveAction(rule.Action) {
			continue
		}

		ruleInfo := fmt.Sprintf("📌 检查主动规则 [%s] (行 %d)", rule.Name, rule.Line)
		debugPrint(debug, "%s", ruleInfo)
		debugPrint(debug, "   条件: %s", rule.Cond)

		// 互斥组检查
		if rule.Group != "" && triggeredGroups[rule.Group] {
			debugPrint(debug, "   ⏭️  跳过: 组 [%s] 已触发", rule.Group)
			debugPrint(debug, "")
			continue
		}

		// 创建骰子结果存储，确保条件判定与信息提取使用同一骰值
		rs := NewRollStore()
		// 先判断是否包含 roll，预先提取骰子结果
		hasRoll := strings.Contains(rule.Cond, "roll(")
		result := evalCondition(rule.Cond, input, state, rs)
		if result {
			debugPrint(debug, "   结果: true")
			actionPreview := rule.Action
			if len(actionPreview) > 60 {
				actionPreview = actionPreview[:60] + "..."
			}
			debugPrint(debug, "   ✅ 触发 → %s", actionPreview)
			if rule.Group != "" {
				debugPrint(debug, "   🔒 锁定组 [%s]", rule.Group)
				triggeredGroups[rule.Group] = true
			}
			debugPrint(debug, "")
			// 提取骰子结果信息（使用同一 RollStore）
			// 合并之前未匹配规则的骰子结果
			matchedRoll := extractRollInfo(rule.Name, rule.Cond, rs)
			if len(allRollParts) > 0 {
				if matchedRoll != "" {
					rollInfo = strings.Join(allRollParts, "\n") + "\n" + matchedRoll
				} else {
					rollInfo = strings.Join(allRollParts, "\n")
				}
			} else {
				rollInfo = matchedRoll
			}
			return rule, true, rollInfo
		}

		debugPrint(debug, "   结果: false")
		debugPrint(debug, "   ❌ 未触发")
		// 失败也收集骰子结果
		if hasRoll {
			if ri := extractRollInfo(rule.Name, rule.Cond, rs); ri != "" {
				allRollParts = append(allRollParts, ri)
			}
		}
		debugPrint(debug, "")
	}

	debugPrint(debug, "----------------------------------------")
	debugPrint(debug, "📊 共检查 %d 条主动规则，未匹配到任何规则\n", len(rules))
	// 无匹配时将收集到的所有骰子结果返回
	rollInfo = strings.Join(allRollParts, "\n")
	return nil, false, rollInfo
}

// matchRule 按顺序匹配第一条满足条件的规则（兼容旧版，不区分主动/被动）。
//
// 匹配流程：
//  1. 遍历规则列表（保持书写顺序）
//  2. 对每条规则评估条件表达式
//  3. 如果条件满足且互斥组未触发，返回该规则
//  4. 互斥组：同一组内只触发第一个匹配的规则
//
// 参数：
//   - rules: 规则列表
//   - input: 当前用户输入
//   - state: 当前状态
//   - debug: 是否输出调试信息
//
// 返回值：
//   - *domain.Rule: 匹配到的规则（nil 表示无匹配）
//   - bool: 是否匹配成功
//   - string: 骰子结果描述（匹配的规则条件中包含 roll() 时返回，否则为空）
//
// 互斥组示例：
//
//	[攻击] if 包含 "攻击" -> [group:combat] 攻击
//	[防御] if 包含 "防御" -> [group:combat] 防御
//	输入包含 "攻击" 时只触发 [攻击]，[防御] 被跳过
//
// 注意：此函数保留用于单元测试向后兼容。新代码请使用 matchActiveRule。
func matchRule(rules []*domain.Rule, input string, state map[string]any, debug bool) (*domain.Rule, bool, string) {
	triggeredGroups := make(map[string]bool)

	debugPrint(debug, "🔍 规则调试模式")
	debugPrint(debug, "----------------------------------------")

	for _, rule := range rules {
		ruleInfo := fmt.Sprintf("📌 检查规则 [%s] (行 %d)", rule.Name, rule.Line)
		debugPrint(debug, "%s", ruleInfo)
		debugPrint(debug, "   条件: %s", rule.Cond)

		// 互斥组检查
		if rule.Group != "" && triggeredGroups[rule.Group] {
			debugPrint(debug, "   ⏭️  跳过: 组 [%s] 已触发", rule.Group)
			debugPrint(debug, "")
			continue
		}

		rs := NewRollStore()
		result := evalCondition(rule.Cond, input, state, rs)
		if result {
			debugPrint(debug, "   结果: true")
			actionPreview := rule.Action
			if len(actionPreview) > 60 {
				actionPreview = actionPreview[:60] + "..."
			}
			debugPrint(debug, "   ✅ 触发 → %s", actionPreview)
			if rule.Group != "" {
				debugPrint(debug, "   🔒 锁定组 [%s]", rule.Group)
				triggeredGroups[rule.Group] = true
			}
			debugPrint(debug, "")
			// 提取骰子结果信息（使用同一 RollStore）
			rollInfo := extractRollInfo(rule.Name, rule.Cond, rs)
			return rule, true, rollInfo
		}

		debugPrint(debug, "   结果: false")
		debugPrint(debug, "   ❌ 未触发")
		debugPrint(debug, "")
	}

	debugPrint(debug, "----------------------------------------")
	debugPrint(debug, "📊 共检查 %d 条规则，未匹配到任何规则\n", len(rules))
	return nil, false, ""
}