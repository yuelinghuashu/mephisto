// internal/core/engine/engine_test.go
//
// 本文件包含引擎包的单元测试。
// 测试覆盖：
//  1. 条件评估函数（evalCondition）
//  2. 规则匹配（matchRule，含互斥组）
//  3. 动作执行（ExecuteAction）
//  4. 引擎核心（Engine.Run）
//  5. 状态、历史、记忆的管理
package engine

import (
	"fmt"
	"strings"
	"testing"

	"mephisto/internal/domain"
)

// ============================================================
// 条件评估测试
// ============================================================

func TestEvalCondition(t *testing.T) {
	state := map[string]any{
		"堕落指数": 85,
		"生命值":  30,
		"等级":   5,
		"情绪":   "愤怒",
	}

	tests := []struct {
		name     string
		cond     string
		input    string
		expected bool
	}{
		// ---- 包含/不包含 ----
		{"包含匹配", `包含 "攻击"`, "我要发起攻击！", true},
		{"包含不匹配", `包含 "防御"`, "我要发起攻击！", false},
		{"不包含匹配", `不包含 "光之国"`, "我是贝利亚", true},
		{"不包含不匹配", `不包含 "光之国"`, "光之国是故乡", false},

		// ---- 状态比较 ----
		{"状态 >", `状态.堕落指数 > 80`, "", true},
		{"状态 <", `状态.生命值 < 50`, "", true},
		{"状态 == 数字", `状态.等级 == 5`, "", true},
		{"状态 == 字符串", `状态.情绪 == "愤怒"`, "", true},
		{"状态 != 数字", `状态.生命值 != 100`, "", true},
		{"状态不存在", `状态.不存在 > 10`, "", false},

		// ---- 逻辑组合 ----
		{"与运算 全部满足", `包含 "攻击" && 状态.堕落指数 > 80`, "我要发起攻击！", true},
		{"与运算 部分不满足", `包含 "攻击" && 状态.堕落指数 < 80`, "我要发起攻击！", false},
		{"或运算 任意满足", `包含 "防御" || 状态.堕落指数 > 80`, "我要发起攻击！", true},
		{"或运算 全部不满足", `包含 "防御" || 状态.生命值 > 100`, "我要发起攻击！", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evalCondition(tt.cond, tt.input, state)
			if result != tt.expected {
				t.Errorf("evalCondition(%q) = %v, expected %v", tt.cond, result, tt.expected)
			}
		})
	}
}

// ============================================================
// 规则匹配测试（含互斥组）
// ============================================================

func TestMatchRule(t *testing.T) {
	state := map[string]any{"堕落指数": 85}

	rules := []*domain.Rule{
		{Name: "攻击", Cond: `包含 "攻击"`, Action: "攻击动作", Group: "combat"},
		{Name: "防御", Cond: `包含 "防御"`, Action: "防御动作", Group: "combat"},
		{Name: "光之国", Cond: `包含 "光之国"`, Action: "光之国动作", Group: ""},
	}

	tests := []struct {
		name        string
		input       string
		wantName    string
		wantMatched bool
	}{
		{"触发攻击", "我要攻击！", "攻击", true},
		{"触发防御", "我要防御！", "防御", true},
		{"触发光之国", "光之国是什么？", "光之国", true},
		{"无匹配", "你好", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, matched := matchRule(rules, tt.input, state)
			if matched != tt.wantMatched {
				t.Errorf("matched = %v, want %v", matched, tt.wantMatched)
				return
			}
			if matched && rule.Name != tt.wantName {
				t.Errorf("rule.Name = %v, want %v", rule.Name, tt.wantName)
			}
		})
	}

	// ---- 互斥组测试 ----
	t.Run("互斥组", func(t *testing.T) {
		// 输入同时匹配攻击和防御，但应该只触发攻击（第一个）
		rule, matched := matchRule(rules, "我要攻击和防御！", state)
		if !matched {
			t.Error("期望匹配到规则")
			return
		}
		if rule.Name != "攻击" {
			t.Errorf("期望触发攻击，实际触发 %s", rule.Name)
		}
	})
}

// ============================================================
// 动作执行测试
// ============================================================

func TestExecuteAction(t *testing.T) {
	contract := &domain.Contract{RoleName: "贝利亚奥特曼"}
	runtime := NewRuntime(contract, 20)

	tests := []struct {
		name         string
		action       string
		input        string
		expected     string
		wantMemories []string
	}{
		{
			name:         "注入动作",
			action:       `注入 "光之国是故乡"`,
			input:        "",
			expected:     "",
			wantMemories: []string{"光之国是故乡"},
		},
		{
			name:         "注入动作（占位符替换）",
			action:       `注入 "{角色名}的故乡是光之国"`,
			input:        "",
			expected:     "",
			wantMemories: []string{"贝利亚奥特曼的故乡是光之国"},
		},
		{
			name:     "状态修改（数字）",
			action:   `状态.生命值 = 100`,
			input:    "",
			expected: "📊 生命值 = 100",
		},
		{
			name:     "状态修改（字符串）",
			action:   `状态.情绪 = "愤怒"`,
			input:    "",
			expected: `📊 情绪 = 愤怒`,
		},
		{
			name:     "静态文本（无 LLM）",
			action:   `全力攻击周围目标`,
			input:    "",
			expected: "全力攻击周围目标",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个测试重置 runtime 的记忆
			runtime.ReplaceMemories([]string{})

			result := ExecuteAction(tt.action, tt.input, runtime, nil, nil)
			if result != tt.expected {
				t.Errorf("ExecuteAction() = %v, expected %v", result, tt.expected)
			}

			if tt.wantMemories != nil {
				memories := runtime.Memories()
				if len(memories) != len(tt.wantMemories) {
					t.Errorf("Memories length = %d, want %d", len(memories), len(tt.wantMemories))
					return
				}
				for i, m := range tt.wantMemories {
					if memories[i] != m {
						t.Errorf("Memories[%d] = %v, want %v", i, memories[i], m)
					}
				}
			}
		})
	}
}

// ============================================================
// 引擎核心测试
// ============================================================

func TestEngineRun(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string
		wantMemories []string
		extraRules   []*domain.Rule
		extraState   map[string]any
	}{
		{
			name:         "匹配攻击规则",
			input:        "我要发动攻击！",
			wantContains: "攻击动作",
		},
		{
			name:         "匹配防御规则",
			input:        "我要防御！",
			wantContains: "防御动作",
		},
		{
			name:         "匹配光之国规则（注入）",
			input:        "你知道光之国吗？",
			wantContains: "",
			wantMemories: []string{"贝利亚奥特曼的故乡是光之国"},
		},
		{
			name:         "匹配高堕落规则（状态触发）",
			input:        "我感觉到力量在涌动",
			wantContains: "",
			wantMemories: []string{"堕落指数过高"},
			extraRules: []*domain.Rule{
				{Name: "高堕落", Cond: `状态.堕落指数 > 80`, Action: `注入 "堕落指数过高"`},
			},
			extraState: map[string]any{"堕落指数": 95},
		},
		{
			name:         "无匹配规则，返回默认响应",
			input:        "你好",
			wantContains: "沉默地注视着命运",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []*domain.Rule{
				{Name: "攻击", Cond: `包含 "攻击" || 包含 "战斗"`, Action: "攻击动作", Group: "combat"},
				{Name: "防御", Cond: `包含 "防御" || 包含 "防守"`, Action: "防御动作", Group: "combat"},
				{Name: "光之国", Cond: `包含 "光之国"`, Action: `注入 "{角色名}的故乡是光之国"`},
			}
			rules = append(rules, tt.extraRules...)

			state := []domain.KeyValue{{Key: "堕落指数", Value: "50"}}
			for k, v := range tt.extraState {
				state = append(state, domain.KeyValue{Key: k, Value: fmt.Sprintf("%v", v)})
			}

			contract := &domain.Contract{
				RoleName: "贝利亚奥特曼",
				Rules:    rules,
				State:    state,
			}

			eng := New(contract)
			response, err := eng.Run(tt.input, nil)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tt.wantContains != "" && !strings.Contains(response, tt.wantContains) {
				t.Errorf("Run() response = %v, want contain %v", response, tt.wantContains)
			}

			if tt.wantMemories != nil {
				memories := eng.Memories()
				if len(memories) != len(tt.wantMemories) {
					t.Errorf("Memories length = %d, want %d", len(memories), len(tt.wantMemories))
					return
				}
				for i, m := range tt.wantMemories {
					if memories[i] != m {
						t.Errorf("Memories[%d] = %v, want %v", i, memories[i], m)
					}
				}
			}
		})
	}
}

func TestEngineHistory(t *testing.T) {
	contract := &domain.Contract{
		RoleName: "测试角色",
		State:    []domain.KeyValue{},
		Rules:    []*domain.Rule{},
	}

	eng := New(contract, WithMaxHistory(2))

	inputs := []string{"第一轮", "第二轮", "第三轮"}
	for _, input := range inputs {
		_, err := eng.Run(input, nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}

	history := eng.History()
	expectedLen := 4
	if len(history) != expectedLen {
		t.Errorf("History length = %d, want %d", len(history), expectedLen)
	}

	if len(history) > 0 {
		last := history[len(history)-1]
		if last.Role != "assistant" {
			t.Errorf("Last history role = %s, want assistant", last.Role)
		}
	}
}

func TestEngineStateAndMemories(t *testing.T) {
	contract := &domain.Contract{
		RoleName: "测试角色",
		State: []domain.KeyValue{
			{Key: "初始状态", Value: "初始值"},
		},
		Memories: []string{"初始记忆"},
		Rules:    []*domain.Rule{},
	}

	eng := New(contract)

	state := eng.State()
	if state["初始状态"] != "初始值" {
		t.Errorf("State['初始状态'] = %v, want 初始值", state["初始状态"])
	}
	state["初始状态"] = "被修改"
	if eng.State()["初始状态"] != "初始值" {
		t.Errorf("Engine state was mutated by copy")
	}

	memories := eng.Memories()
	if len(memories) != 1 || memories[0] != "初始记忆" {
		t.Errorf("Memories = %v, want [初始记忆]", memories)
	}
	memories[0] = "被修改"
	if len(eng.Memories()) == 0 || eng.Memories()[0] != "初始记忆" {
		t.Errorf("Engine memories was mutated by copy")
	}
}

func TestEngineDefaultResponse(t *testing.T) {
	contract := &domain.Contract{
		RoleName: "贝利亚奥特曼",
		State:    []domain.KeyValue{},
		Rules:    []*domain.Rule{},
	}
	eng := New(contract)

	response, err := eng.Run("你好", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	expected := "贝利亚奥特曼 沉默地注视着命运。"
	if response != expected {
		t.Errorf("default response = %v, want %v", response, expected)
	}

	contract2 := &domain.Contract{
		RoleName: "",
		State:    []domain.KeyValue{},
		Rules:    []*domain.Rule{},
	}
	eng2 := New(contract2)
	response2, err := eng2.Run("你好", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	expected2 := "角色 沉默地注视着命运。"
	if response2 != expected2 {
		t.Errorf("default response = %v, want %v", response2, expected2)
	}
}
