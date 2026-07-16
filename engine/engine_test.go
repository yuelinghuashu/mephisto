package engine

import (
	"testing"

	"mephisto/parser"
)

// TestExprEval 测试 AST 求值
func TestExprEval(t *testing.T) {
	tests := []struct {
		name     string
		cond     string
		ctx      Context
		expected bool
	}{
		{
			name:     "变量相等",
			cond:     `情绪 == "暴怒"`,
			ctx:      Context{"情绪": "暴怒"},
			expected: true,
		},
		{
			name:     "变量不等",
			cond:     `情绪 == "暴怒"`,
			ctx:      Context{"情绪": "平静"},
			expected: false,
		},
		{
			name:     "数字比较",
			cond:     `堕落指数 > 70`,
			ctx:      Context{"堕落指数": 85},
			expected: true,
		},
		{
			name:     "数字比较失败",
			cond:     `堕落指数 > 70`,
			ctx:      Context{"堕落指数": 50},
			expected: false,
		},
		{
			name:     "AND 短路：左侧为假不评估右侧",
			cond:     `情绪 == "暴怒" && 堕落指数 > 70`,
			ctx:      Context{"情绪": "平静"},
			expected: false,
		},
		{
			name:     "OR 短路：左侧为真不评估右侧",
			cond:     `情绪 == "暴怒" || 堕落指数 > 70`,
			ctx:      Context{"情绪": "暴怒"},
			expected: true,
		},
		{
			name:     "包含操作符",
			cond:     `输入包含 "光之国"`,
			ctx:      Context{"输入": "光之国是我的故乡"},
			expected: true,
		},
		{
			name:     "括号分组",
			cond:     `(情绪 == "暴怒" || 情绪 == "疯狂") && 堕落指数 > 70`,
			ctx:      Context{"情绪": "暴怒", "堕落指数": 85},
			expected: true,
		},
		{
			name:     "变量缺失返回 false",
			cond:     `缺失变量 > 10`,
			ctx:      Context{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseExpression(tt.cond)
			if err != nil {
				t.Fatalf("解析条件失败: %v", err)
			}
			result, err := expr.Eval(tt.ctx)
			if err != nil {
				t.Fatalf("求值失败: %v", err)
			}
			got, ok := result.(bool)
			if !ok {
				t.Fatalf("结果不是布尔值: %v", result)
			}
			if got != tt.expected {
				t.Errorf("条件 %q 结果错误: got %v, want %v", tt.cond, got, tt.expected)
			}
		})
	}
}

// TestDiceRoll 测试骰子表达式
func TestDiceRoll(t *testing.T) {
	tests := []struct {
		name string
		expr string
		min  int
		max  int
	}{
		{"1d6", "roll(1d6)", 1, 6},
		{"2d6", "roll(2d6)", 2, 12},
		{"1d100", "roll(1d100)", 1, 100},
		{"带空格", "roll( 1d20 )", 1, 20},
		{"大小写", "ROLL(2d6)", 2, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RollDice(tt.expr)
			if err != nil {
				t.Fatalf("掷骰失败: %v", err)
			}
			if result < tt.min || result > tt.max {
				t.Errorf("掷骰结果 %d 超出范围 [%d, %d]", result, tt.min, tt.max)
			}
		})
	}
}

// TestEngineMutualExclusion 测试互斥组
func TestEngineMutualExclusion(t *testing.T) {
	ctx := Context{
		"情绪":   "暴怒",
		"堕落指数": 85,
	}
	eng := NewRuleEngine(ctx)

	entries := []*parser.BlockEntry{
		{
			Type:       "rule",
			Key:        "攻击1",
			Value:      `情绪 == "暴怒" && 堕落指数 > 70`,
			RuleAction: "[group:combat] 全力攻击",
			Line:       1,
		},
		{
			Type:       "rule",
			Key:        "攻击2",
			Value:      `情绪 == "暴怒"`,
			RuleAction: "[group:combat] 普通攻击",
			Line:       2,
		},
		{
			Type:       "rule",
			Key:        "嘲讽",
			Value:      `情绪 == "暴怒"`,
			RuleAction: "嘲讽对手",
			Line:       3,
		},
	}

	// 预编译 AST
	for _, entry := range entries {
		expr, _ := ParseExpression(entry.Value)
		entry.RuleExpr = expr
	}

	eng.AddRules(entries)

	results, err := eng.Execute()
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("预期触发 2 条规则，实际触发 %d 条", len(results))
	}

	foundAttack := false
	foundTaunt := false
	for _, r := range results {
		if r.Message == "触发规则 [攻击1]: 全力攻击" {
			foundAttack = true
		}
		if r.Message == "触发规则 [嘲讽]: 嘲讽对手" {
			foundTaunt = true
		}
	}
	if !foundAttack {
		t.Error("互斥组第一条规则未触发")
	}
	if !foundTaunt {
		t.Error("非互斥组规则未触发")
	}
}

func TestEngineActionExecution(t *testing.T) {
	ctx := Context{
		"角色名":  "贝利亚",
		"堕落指数": 85,
	}
	eng := NewRuleEngine(ctx)

	// 构造规则：包含注入和普通动作
	entries := []*parser.BlockEntry{
		{
			Type:       "rule",
			Key:        "注入测试",
			Value:      `堕落指数 > 70`,
			RuleAction: `注入 "堕落指数过高，{角色名}"`,
			Line:       1,
		},
		{
			Type:       "rule",
			Key:        "普通动作",
			Value:      `堕落指数 > 70`,
			RuleAction: `警告：状态异常`,
			Line:       2,
		},
	}

	// 预编译 AST
	for _, entry := range entries {
		expr, _ := ParseExpression(entry.Value)
		entry.RuleExpr = expr
	}

	eng.AddRules(entries)
	results, err := eng.Execute()
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}

	// 验证结果
	if len(results) != 2 {
		t.Errorf("预期 2 个结果，实际 %d", len(results))
	}

	// 验证注入内容的变量替换
	foundInject := false
	foundAction := false
	for _, r := range results {
		if r.Message == `触发规则 [注入测试]: 注入内容「堕落指数过高，贝利亚」` {
			foundInject = true
		}
		if r.Message == `触发规则 [普通动作]: 警告：状态异常` {
			foundAction = true
		}
	}
	if !foundInject {
		t.Error("注入动作未正确执行或变量未替换")
	}
	if !foundAction {
		t.Error("普通动作未执行")
	}
}
