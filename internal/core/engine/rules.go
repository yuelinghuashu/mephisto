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
	"os"
	"strconv"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// debugPrint 打印调试信息到 stderr（仅在调试模式下输出）
func debugPrint(debug bool, format string, args ...any) {
	if !debug {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

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
		matched, _ := evalRoll(cond)
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
//
// 返回值：
//   - bool: 掷点结果是否满足条件
//   - int: 实际骰子点数总和（用于传递给 LLM 作为叙事参考）
func evalRoll(cond string) (bool, int) {
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

	// 掷骰子（math/rand/v2 自动随机种子）
	total := 0
	for range count {
		total += rand.IntN(sides) + 1 // 1 ~ sides
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

// parseRollExpr 解析 roll(...) 表达式，提取参数和可选的用户阈值。
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

// formatMaxValue 返回骰子的最大值（count * sides）。
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
// 功能：重新解析条件中的每个 roll(...) 表达式并掷骰，
// 生成给 LLM 阅读的描述文本，如：
//
//	"🎲 骰子结果：roll(1d100) = 87/100（阈值 ≥80）"
//
// 注意：此函数会重新掷骰，与 evalCondition 中的判定骰值不同。
// 这是有意设计——条件判定和叙事参考各掷一次，两者独立。
// 如果条件中有多个 roll(...)，结果会拼接在一起。
//
// 参数：
//   - cond: 条件字符串（如 `包含 "愤怒" && roll(1d100) >= 80`）
//
// 返回值：
//   - string: 骰子结果描述（无 roll 表达式时返回空字符串）
func extractRollInfo(cond string) string {
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

		// 掷骰
		total := 0
		for range re.Count {
			total += rand.IntN(re.Sides) + 1
		}

		desc := fmt.Sprintf("%s = %d/%d", re.RollCore, total, re.maxValue())
		if td := re.thresholdDesc(); td != "" {
			desc = fmt.Sprintf("%s（%s）", desc, td)
		}
		parts = append(parts, desc)
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
	return "🎲 骰子结果：" + strings.Join(parts, ", ")
}

// ============================================================
// 规则匹配（支持互斥组 + 调试输出）
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

		result := evalCondition(rule.Cond, input, state)
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
			// 提取骰子结果信息（用于 LLM 叙事）
			rollInfo := extractRollInfo(rule.Cond)
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
//   - rollInfo: 骰子结果描述（匹配规则时生成，注入 LLM 指令中）
//
// 返回值：
//   - string: 响应文本（空字符串表示无直接输出）
//
// 设计说明：
//   - 注入动作返回空字符串，由上层决定是否继续 LLM 叙事
//   - 状态修改返回带 📊 前缀的确认消息
//   - 普通文本直接返回（无 LLM 时）或作为指令调用 LLM
func ExecuteAction(action, input string, runtime *Runtime, llmClient llm.Client, onChunk func(string), rollInfo string) string {
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
			instruction := action
			if rollInfo != "" {
				instruction = fmt.Sprintf("%s（%s）", action, rollInfo)
			}
			return callLLMInternal(input, instruction, llmClient, runtime, onChunk)
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