// internal/core/engine/rules.go
//
// 规则引擎：条件评估、规则匹配、互斥组、动作执行
//
// 本文件合并了原 evaluator.go 和 executor.go 的功能：
//   - 条件评估：evalCondition 系列函数
//   - 规则匹配：matchRule（支持互斥组）
//   - 动作执行：ExecuteAction（注入/状态修改/LLM/静态文本）
//   - 骰子表达式：evalRoll 支持 roll(1d100) 等格式
//
// 设计原则：
//   - 无接口、无抽象，直接函数实现
//   - 所有函数为纯函数或依赖 Runtime 接口
//   - 互斥组由调用方维护 triggeredGroups map
package engine

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/domain"
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
//   - roll(1d100)       → 掷骰子，结果 >= 阈值时返回 true
//
// 参数：
//   - cond: 条件字符串（如 `包含 "攻击"`）
//   - input: 当前用户输入
//   - state: 当前状态 map
//
// 返回值：
//   - bool: 条件是否满足
//
// 注意事项：
//   - 逻辑运算符优先级：&& 高于 ||
//   - 状态不存在时返回 false（而非报错）
//   - 类型不匹配时尝试转换后再比较
func evalCondition(cond, input string, state map[string]any) bool {
	cond = strings.TrimSpace(cond)

	// ---- 逻辑运算符（优先级：&& > ||） ----
	// 先处理 ||，再处理 &&
	if strings.Contains(cond, "||") {
		for _, p := range strings.Split(cond, "||") {
			if evalCondition(strings.TrimSpace(p), input, state) {
				return true
			}
		}
		return false
	}

	if strings.Contains(cond, "&&") {
		for _, p := range strings.Split(cond, "&&") {
			if !evalCondition(strings.TrimSpace(p), input, state) {
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
		return evalRoll(cond)

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
//
// toFloat 逻辑已内联于此函数，无需额外调用。
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

// ============================================================
// 骰子表达式支持
// ============================================================

// evalRoll 评估骰子表达式。
//
// 支持的格式：
//   - roll(1d100)      → 掷一个 100 面骰，结果 >= 50 返回 true
//   - roll(2d6)        → 掷两个 6 面骰，结果 >= 7 返回 true
//   - roll(1d20)       → 掷一个 20 面骰，结果 >= 10 返回 true
//   - roll(3d10)       → 掷三个 10 面骰，结果 >= 15 返回 true
//
// 设计说明：
//   - 骰子判定规则：掷点结果 >= (面数 * 骰子数 / 2) 时判定为成功
//   - 即：期望值取中间值（0.5），模拟 50% 成功率
//   - 例如 1d100 判定阈值 = 50，2d6 判定阈值 = 7
//
// 返回值：
//   - bool: 掷点结果是否达到阈值
func evalRoll(cond string) bool {
	// 提取 roll(...) 括号内的内容
	start := strings.Index(cond, "(")
	end := strings.LastIndex(cond, ")")
	if start == -1 || end == -1 || end <= start {
		return false
	}
	expr := strings.TrimSpace(cond[start+1 : end])

	// 解析格式：{count}d{sides}
	parts := strings.Split(expr, "d")
	if len(parts) != 2 {
		return false
	}
	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || count <= 0 {
		return false
	}
	sides, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || sides <= 0 {
		return false
	}

	// 掷骰子（math/rand/v2 自动随机种子）
	total := 0
	for i := 0; i < count; i++ {
		total += rand.IntN(sides) + 1 // 1 ~ sides
	}

	// 判定阈值：期望值 >= 中间值（0.5 成功率）
	threshold := (count * sides) / 2
	if count*sides%2 != 0 {
		threshold++ // 奇数时向上取整
	}

	return total >= threshold
}

// ============================================================
// 规则匹配（支持互斥组）
// ============================================================

// matchRule 按顺序匹配第一条满足条件的规则。
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
//
// 返回值：
//   - *domain.Rule: 匹配到的规则（nil 表示无匹配）
//   - bool: 是否匹配成功
//
// 互斥组示例：
//
//	[攻击] if 包含 "攻击" -> [group:combat] 攻击
//	[防御] if 包含 "防御" -> [group:combat] 防御
//	输入包含 "攻击" 时只触发 [攻击]，[防御] 被跳过
func matchRule(rules []*domain.Rule, input string, state map[string]any) (*domain.Rule, bool) {
	triggeredGroups := make(map[string]bool)

	for _, rule := range rules {
		// 互斥组检查
		if rule.Group != "" && triggeredGroups[rule.Group] {
			continue
		}

		if evalCondition(rule.Cond, input, state) {
			if rule.Group != "" {
				triggeredGroups[rule.Group] = true
			}
			return rule, true
		}
	}
	return nil, false
}

// ============================================================
// 动作执行（直接函数，无接口）
// ============================================================

// ExecuteAction 执行规则动作。
//
// 支持的动作类型（通过前缀识别）：
//   - "注入 " → 将消息追加到记忆（不直接输出）
//   - "状态." → 更新状态值，返回确认消息
//   - 其他文本 → 若有 LLM 则作为指令调用，否则直接返回
//
// 参数：
//   - action: 动作字符串
//   - input: 当前用户输入
//   - runtime: 运行时（用于修改状态和记忆）
//   - llmClient: LLM 客户端（可选）
//   - onChunk: 流式输出回调
//
// 返回值：
//   - string: 响应文本（空字符串表示无直接输出）
//
// 设计说明：
//   - 注入动作返回空字符串，由上层决定是否继续 LLM 叙事
//   - 状态修改返回带 📊 前缀的确认消息
//   - 普通文本直接返回（无 LLM 时）或作为指令调用 LLM
func ExecuteAction(action, input string, runtime *Runtime, llmClient llm.Client, onChunk func(string)) string {
	action = strings.TrimSpace(action)

	switch {
	case strings.HasPrefix(action, "注入 "):
		// ---- 注入动作 ----
		msg := shared.Unquote(strings.TrimPrefix(action, "注入 "))
		msg = shared.ReplacePlaceholders(msg, map[string]string{
			"角色名": runtime.Contract().RoleName,
		})
		runtime.AppendMemory(msg)
		return ""

	case strings.HasPrefix(action, "状态."):
		// ---- 状态修改 ----
		return setState(action, runtime, onChunk)

	default:
		// ---- 普通文本 ----
		// 有 LLM：作为指令调用；无 LLM：直接返回静态文本
		if llmClient != nil {
			return callLLMInternal(input, action, llmClient, runtime, onChunk)
		}
		if onChunk != nil {
			for _, ch := range action {
				onChunk(string(ch))
			}
		}
		return action
	}
}

// setState 修改状态。
//
// 格式：状态.键 = 值
// 支持的值类型：数字、布尔值、字符串（自动推断类型）
//
// 返回值：
//   - string: 带 📊 前缀的确认消息
func setState(action string, runtime *Runtime, onChunk func(string)) string {
	rest := strings.TrimPrefix(action, "状态.")
	rest = strings.TrimSpace(rest)

	key, valStr, ok := strings.Cut(rest, "=")
	if !ok {
		return "格式错误"
	}
	key, valStr = strings.TrimSpace(key), strings.TrimSpace(valStr)

	if strings.HasPrefix(valStr, `"`) && strings.HasSuffix(valStr, `"`) {
		valStr = valStr[1 : len(valStr)-1]
	}

	val := shared.ParseValue(valStr)
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

	if onChunk != nil {
		resp, err := llmClient.GenerateStream(ctx, prompt, onChunk)
		if err != nil {
			return ""
		}
		return resp
	}

	resp, err := llmClient.Generate(ctx, prompt)
	if err != nil {
		return ""
	}
	return resp
}
