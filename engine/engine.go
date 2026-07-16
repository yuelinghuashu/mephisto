// ============================================================
// engine.go - 规则引擎主入口
// 职责：
// 1. 接收预编译好的规则（含 AST）
// 2. 在运行时直接执行 Eval()，无需重新解析
// 3. 支持互斥组（同一组只执行第一个匹配的）
// 4. 支持变量替换（动作中的 {变量} 插值）
// 5. 调试日志
// 6. 在 init 中注册 ParseExpression 到 parser.ParseExprFunc
// ============================================================

package engine

import (
	"fmt"
	"regexp"
	"strings"

	"mephisto/parser"
	"mephisto/utils"
)

// groupRegex 匹配互斥组标记 [group:xxx]
// 使用全局变量缓存，避免每次 extractGroup 都重新编译
var groupRegex = regexp.MustCompile(`^\[group:([^\]]+)\]\s*`)

// ============================================================
// 初始化：注册解析函数到 parser 包
// 这样 parser 包可以调用 engine.ParseExpression 而无需导入 engine
// ============================================================

func init() {
	parser.ParseExprFunc = ParseExpression
}

// ============================================================
// 规则引擎数据结构
// ============================================================

// RuleEngine 规则引擎
type RuleEngine struct {
	Rules   []Rule
	Context Context
	Debug   bool
}

// NewRuleEngine 创建规则引擎
func NewRuleEngine(ctx Context) *RuleEngine {
	return &RuleEngine{
		Rules:   []Rule{},
		Context: ctx,
		Debug:   false,
	}
}

// SetDebug 设置调试模式
func (e *RuleEngine) SetDebug(debug bool) {
	e.Debug = debug
}

// AddRule 添加规则
func (e *RuleEngine) AddRule(name, condition, action string, expr parser.Expr, line int) {
	// 提取互斥组标记 [group:xxx]
	action, group := extractGroup(action)

	e.Rules = append(e.Rules, Rule{
		Name:      name,
		Condition: condition,
		Action:    action,
		Group:     group,
		Line:      line,
		Expr:      expr,
	})
}

// extractGroup 从动作中提取互斥组标记
// 使用全局正则表达式 groupRegex
func extractGroup(action string) (string, string) {
	action = strings.TrimSpace(action)
	matches := groupRegex.FindStringSubmatch(action)
	if len(matches) >= 2 {
		group := matches[1]
		cleaned := strings.TrimSpace(groupRegex.ReplaceAllString(action, ""))
		return cleaned, group
	}
	return action, ""
}

// AddRules 批量添加规则（从解析器生成的 BlockEntry 列表）
func (e *RuleEngine) AddRules(entries []*parser.BlockEntry) {
	for _, entry := range entries {
		if entry.Type == "rule" {
			// 直接使用预编译好的 RuleExpr 和 RuleAction
			e.AddRule(
				entry.Key,
				entry.Value,      // 条件字符串（仅用于显示）
				entry.RuleAction, // 预编译的动作
				entry.RuleExpr,   // 预编译的 AST（parser.Expr 类型）
				entry.Line,
			)
		}
	}
}

// Execute 执行所有匹配的规则
func (e *RuleEngine) Execute() ([]ActionResult, error) {
	var results []ActionResult
	triggeredGroups := make(map[string]bool)

	if e.Debug {
		fmt.Println("\n🔍 规则调试模式")
		fmt.Println(strings.Repeat("-", 40))
	}

	for _, rule := range e.Rules {
		if e.Debug {
			fmt.Printf("\n📌 检查规则 [%s] (行 %d)\n", rule.Name, rule.Line)
			fmt.Printf("   条件: %s\n", rule.Condition)
		}

		// 互斥组检查
		if rule.Group != "" && triggeredGroups[rule.Group] {
			if e.Debug {
				fmt.Printf("   ⏭️  跳过: 组 [%s] 已触发\n", rule.Group)
			}
			continue
		}

		// 直接执行预编译的 AST，无需重新解析
		if rule.Expr == nil {
			return nil, fmt.Errorf("规则 [%s] 未预编译 (行 %d)", rule.Name, rule.Line)
		}

		matched, err := rule.Expr.Eval(e.Context)
		if err != nil {
			return nil, fmt.Errorf("求值规则 [%s] 失败 (行 %d): %w", rule.Name, rule.Line, err)
		}

		matchedBool, ok := matched.(bool)
		if !ok {
			return nil, fmt.Errorf("规则 [%s] 条件结果不是布尔值 (行 %d): %v", rule.Name, rule.Line, matched)
		}

		if e.Debug {
			fmt.Printf("   结果: %v\n", matchedBool)
		}

		if matchedBool {
			// 执行动作（支持变量替换）
			action := utils.ReplaceVariables(rule.Action, e.Context)

			result := e.executeAction(action, rule.Name, rule.Line)

			if e.Debug {
				fmt.Printf("   ✅ 触发 → %s\n", rule.Action)
			}
			results = append(results, result)

			if rule.Group != "" {
				triggeredGroups[rule.Group] = true
				if e.Debug {
					fmt.Printf("   🔒 锁定组 [%s]\n", rule.Group)
				}
			}
		} else {
			if e.Debug {
				fmt.Printf("   ❌ 未触发\n")
			}
		}
	}

	if e.Debug {
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("📊 共匹配 %d 条规则，触发 %d 条\n", len(e.Rules), len(results))
	}

	return results, nil
}

// executeAction 执行动作，返回 ActionResult
// 现在会填充 Type 字段
func (e *RuleEngine) executeAction(action, ruleName string, line int) ActionResult {
	_ = line // 保留

	action = strings.TrimSpace(action)

	// 注入动作
	if content, ok := strings.CutPrefix(action, "注入 "); ok {
		content = strings.TrimSpace(content)
		content = strings.Trim(content, `"'`)
		return ActionResult{
			Success: true,
			Type:    ActionInject,
			Data:    content,
			Message: fmt.Sprintf("触发规则 [%s]: 注入内容「%s」", ruleName, content),
		}
	}

	// LLM 生成动作
	if prompt, ok := strings.CutPrefix(action, "LLM: "); ok {
		prompt = strings.TrimSpace(prompt)
		prompt = strings.Trim(prompt, `"'`)
		return ActionResult{
			Success: true,
			Type:    ActionLLM,
			Data:    prompt,
			Message: fmt.Sprintf("触发规则 [%s]: LLM 生成", ruleName),
		}
	}

	// 骰子动作（如果动作中直接包含 roll 并返回结果）
	if strings.Contains(action, "roll(") {
		if result, err := RollDice(action); err == nil && result > 0 {
			return ActionResult{
				Success: true,
				Type:    ActionDice,
				Data:    fmt.Sprintf("%d", result),
				Message: fmt.Sprintf("触发规则 [%s]: %s | 掷骰结果: %d", ruleName, action, result),
			}
		}
	}

	// 普通动作
	return ActionResult{
		Success: true,
		Type:    ActionPlain,
		Data:    action,
		Message: fmt.Sprintf("触发规则 [%s]: %s", ruleName, action),
	}
}
