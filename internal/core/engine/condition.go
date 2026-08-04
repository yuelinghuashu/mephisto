// internal/core/engine/condition.go
//
// 条件评估系统：评估规则条件表达式
//
// 支持语法：
//   - 包含/不包含 "关键词"    → 文本匹配
//   - 状态.键 操作符 值       → 状态比较
//   - roll(1d100)            → 骰子判定
//   - 条件1 && 条件2         → 与运算（优先级高于 ||）
//   - 条件1 || 条件2         → 或运算
//
// 性能优化：条件字符串在首次求值时编译为 CondNode AST 并缓存在包级
// map，后续评估直接复用已编译结构，避免每轮对话对「规则名 if 条件」反复
// 做字符串拆分（split || / &&）与子串判断（对齐 Flutter meph 的编译缓存）。
package engine

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"mephisto/internal/domain"
)

// ============================================================
// 编译缓存：同一条件字符串跨引擎实例共享编译结果
// ============================================================

// conditionCacheMax 条件编译缓存的最大条目数。
//
// 超过上限时清空重建（简单 FAV 淘汰策略）：
//   - 契约文件的条件数量级通常为几十到几百，1024 远超实际需求
//   - 防仓库长期运行（如作为库嵌入服务）时无界增长
//   - 清空重建正确性无影响——缓存只是性能优化，重建后首次求值重新编译即可
const conditionCacheMax = 1024

// conditionCache 是模块级条件编译缓存（key = 条件字符串原文）。
//
// 设计取舍：
//   - 同一个 .meph 契约的规则在每轮对话中都会被同一组 Rule 复用，
//     缓存可显著减少重复字符串拆分（split || / &&）的 CPU 开销
//   - 有容量上限（conditionCacheMax），防止长期运行的仓库/服务无界增长。
//     超过上限时整体清空，简单且无内存泄漏风险。
var (
	conditionCacheMu sync.RWMutex
	conditionCache   = make(map[string]CondNode)
)

// cacheCondition 缓存编译后的条件节点。
// 超过容量上限时清空重建（FAV：First-Access Victim，最早进入的最先被淘汰）。
func cacheCondition(cond string, node CondNode) CondNode {
	conditionCacheMu.Lock()
	if len(conditionCache) >= conditionCacheMax {
		conditionCache = make(map[string]CondNode, conditionCacheMax)
	}
	conditionCache[cond] = node
	conditionCacheMu.Unlock()
	return node
}

// cachedCondition 从缓存获取编译后的条件节点。
func cachedCondition(cond string) (CondNode, bool) {
	conditionCacheMu.RLock()
	node, ok := conditionCache[cond]
	conditionCacheMu.RUnlock()
	return node, ok
}

// ============================================================
// AST 节点定义
// ============================================================

// CondNode 是编译后的条件节点接口。
type CondNode interface {
	// eval 使用当前输入/状态/骰子存储求值。
	eval(input string, state map[string]any, rs *RollStore) bool
}

// orNode 或运算：任一子条件满足即通过。
type orNode struct {
	children []CondNode
}

func (n *orNode) eval(input string, state map[string]any, rs *RollStore) bool {
	for _, c := range n.children {
		if c.eval(input, state, rs) {
			return true
		}
	}
	return false
}

// andNode 与运算：所有子条件满足才通过。
type andNode struct {
	children []CondNode
}

func (n *andNode) eval(input string, state map[string]any, rs *RollStore) bool {
	for _, c := range n.children {
		if !c.eval(input, state, rs) {
			return false
		}
	}
	return true
}

// containsNode 文本匹配：`包含 "关键词"`
type containsNode struct {
	keyword string
}

func (n *containsNode) eval(input string, state map[string]any, rs *RollStore) bool {
	return strings.Contains(input, n.keyword)
}

// notContainsNode 文本匹配：`不包含 "关键词"`
type notContainsNode struct {
	keyword string
}

func (n *notContainsNode) eval(input string, state map[string]any, rs *RollStore) bool {
	return !strings.Contains(input, n.keyword)
}

// stateCondNode 状态比较：`状态.键 操作符 值`（eval 时即时查状态 Map）
type stateCondNode struct {
	cond string
}

func (n *stateCondNode) eval(input string, state map[string]any, rs *RollStore) bool {
	return evalStateCondition(n.cond, state)
}

// rollCondNode 骰子判定：`roll(1d100)` / `roll(1d100) >= 80`（eval 时即时掷骰）
type rollCondNode struct {
	cond string
}

func (n *rollCondNode) eval(input string, state map[string]any, rs *RollStore) bool {
	matched, _ := evalRoll(n.cond, rs)
	return matched
}

// ============================================================
// 编译
// ============================================================

// compileAtom 编译原子条件为 CondNode；无法编译时返回 nil。
//
// 编译失败的原子条件在评估时视为不匹配（与旧逻辑 `return false` 一致）。
func compileAtom(c string) CondNode {
	if strings.HasPrefix(c, "包含 ") {
		return &containsNode{keyword: domain.Unquote(strings.TrimPrefix(c, "包含 "))}
	}
	if strings.HasPrefix(c, "不包含 ") {
		return &notContainsNode{keyword: domain.Unquote(strings.TrimPrefix(c, "不包含 "))}
	}
	if strings.HasPrefix(c, "状态.") {
		return &stateCondNode{cond: c}
	}
	if strings.HasPrefix(c, "roll(") {
		return &rollCondNode{cond: c}
	}
	return nil
}

// compileConditionUncached 编译条件表达式（递归处理逻辑运算符与括号分组；无缓存版本）。
//
// 括号分组支持（对齐 Flutter meph 的 _parseCondition / _splitTopLevel）：
//   - 整个条件被一对括号完整包裹时，剥掉后递归编译（如 `(a || b)`）
//   - 逻辑运算符只在**括号外**分割，括号内的表达式作为一个整体处理
//     （如 `x && (a || b)` 中 `(a || b)` 不会被 `||` 分割）
func compileConditionUncached(cond string) CondNode {
	c := strings.TrimSpace(cond)
	if c == "" {
		return nil
	}

	// ---- 括号分组：整个条件被一对括号完整包裹时，剥掉后递归编译 ----
	// 例如 `(包含 "a" || 包含 "b")` → 编译内部 `包含 "a" || 包含 "b"`
	if inner, ok := unwrapOuterParens(c); ok {
		return compileConditionUncached(inner)
	}

	// ---- 逻辑运算符（优先级：&& > ||，均只在括号外分割） ----
	// 先处理 ||，再处理 &&
	if hasTopLevelOperator(c, "||") {
		var children []CondNode
		for _, p := range splitTopLevel(c, "||") {
			if child := compileConditionUncached(p); child != nil {
				children = append(children, child)
			}
		}
		if len(children) == 0 {
			return nil
		}
		return &orNode{children: children}
	}

	if hasTopLevelOperator(c, "&&") {
		var children []CondNode
		for _, p := range splitTopLevel(c, "&&") {
			if child := compileConditionUncached(p); child != nil {
				children = append(children, child)
			}
		}
		if len(children) == 0 {
			return nil
		}
		return &andNode{children: children}
	}

	// ---- 原子条件 ----
	return compileAtom(c)
}

// unwrapOuterParens 如果整个条件被一对括号完整包裹（如 `(a || b)`），剥掉并返回内部内容。
// 返回 (inner, true) 表示找到完整的括号包裹；否则返回 ("", false)。
func unwrapOuterParens(c string) (string, bool) {
	if !strings.HasPrefix(c, "(") {
		return "", false
	}
	end := findMatchingParen(c, 0)
	if end == -1 || end != len(c)-1 {
		return "", false
	}
	inner := strings.TrimSpace(c[1:end])
	if inner == "" {
		return "", false
	}
	return inner, true
}

// findMatchingParen 查找 start 处左括号对应的右括号下标；找不到返回 -1。
func findMatchingParen(c string, start int) int {
	depth := 0
	for i := start; i < len(c); i++ {
		switch c[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// hasTopLevelOperator 判断指定运算符是否出现在顶层（括号外）。
func hasTopLevelOperator(c, op string) bool {
	depth := 0
	for i := 0; i < len(c); i++ {
		switch c[i] {
		case '(':
			depth++
		case ')':
			depth--
		default:
			if depth == 0 && i+len(op) <= len(c) && c[i:i+len(op)] == op {
				return true
			}
		}
	}
	return false
}

// splitTopLevel 在顶层（括号外）按运算符切分条件字符串，返回切分片段（已 trim）。
func splitTopLevel(c, op string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(c); i++ {
		switch c[i] {
		case '(':
			depth++
		case ')':
			depth--
		default:
			if depth == 0 && i+len(op) <= len(c) && c[i:i+len(op)] == op {
				parts = append(parts, strings.TrimSpace(c[start:i]))
				i += len(op) - 1
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(c[start:]))
	return parts
}

// compileCondition 编译条件表达式（结果写入模块级缓存）。
func compileCondition(cond string) CondNode {
	if node, ok := cachedCondition(cond); ok {
		return node
	}
	node := compileConditionUncached(cond)
	if node != nil {
		node = cacheCondition(cond, node)
	}
	return node
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
//   - rs: 骰子结果存储（可为 nil，为 nil 时独立掷骰，但不会与叙事信息同步）
//
// 返回值：
//   - bool: 条件是否满足
//
// 注意事项：
//   - 逻辑运算符优先级：&& 高于 ||
//   - 状态不存在时返回 false（而非报错）
//   - 类型不匹配时尝试转换后再比较
//
// 性能说明：
//   优先走编译缓存（首次编译后复用 AST），避免每轮对话对同一条件反复做
//   字符串拆分与子串判断。
func evalCondition(cond, input string, state map[string]any, rs *RollStore) bool {
	cond = strings.TrimSpace(cond)
	if node := compileCondition(cond); node != nil {
		return node.eval(input, state, rs)
	}
	return false
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

	// 查找操作符（优先匹配多字符；复用公共常量避免重复定义）
	var op string
	var idx int
	for _, o := range comparisonOperators {
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
		return v == domain.Unquote(b)
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