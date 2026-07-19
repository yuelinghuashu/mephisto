// internal/core/engine/engine_test.go
//
// 本文件包含引擎包的单元测试和集成测试。
// 测试覆盖：
//  1. 条件评估器（DefaultConditionEvaluator）
//  2. 动作执行器（DefaultActionExecutor）
//  3. 引擎核心（Engine.Run）
//  4. 状态、历史、记忆的管理
package engine

import (
	"fmt"
	"strings"
	"testing"

	"mephisto/internal/domain"
)

// ============================================================
// 条件评估器测试
// ============================================================

// TestDefaultConditionEvaluator 测试默认条件评估器的各种条件类型。
// 覆盖：包含/不包含、状态比较（> < == !=）、逻辑组合（&& ||）、语法错误处理。
func TestDefaultConditionEvaluator(t *testing.T) {
	evaluator := &DefaultConditionEvaluator{}

	tests := []struct {
		name     string
		cond     string
		ctx      ConditionContext
		expected bool
		wantErr  bool
	}{
		// ---- 包含/不包含 ----
		{
			name:     "包含匹配",
			cond:     `包含 "攻击"`,
			ctx:      ConditionContext{Input: "我要发起攻击！"},
			expected: true,
			wantErr:  false,
		},
		{
			name:     "包含不匹配",
			cond:     `包含 "防御"`,
			ctx:      ConditionContext{Input: "我要发起攻击！"},
			expected: false,
			wantErr:  false,
		},
		{
			name:     "不包含匹配（输入不含关键词）",
			cond:     `不包含 "光之国"`,
			ctx:      ConditionContext{Input: "我是贝利亚奥特曼"},
			expected: true,
			wantErr:  false,
		},
		{
			name:     "不包含不匹配（输入含关键词）",
			cond:     `不包含 "光之国"`,
			ctx:      ConditionContext{Input: "光之国是故乡"},
			expected: false,
			wantErr:  false,
		},
		// ---- 状态比较 ----
		{
			name: "状态比较 >",
			cond: `状态.堕落指数 > 80`,
			ctx: ConditionContext{
				State: map[string]any{"堕落指数": 85},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "状态比较 <",
			cond: `状态.生命值 < 50`,
			ctx: ConditionContext{
				State: map[string]any{"生命值": 30},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "状态比较 == (数字)",
			cond: `状态.等级 == 5`,
			ctx: ConditionContext{
				State: map[string]any{"等级": 5},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "状态比较 == (字符串)",
			cond: `状态.情绪 == "愤怒"`,
			ctx: ConditionContext{
				State: map[string]any{"情绪": "愤怒"},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "状态比较 != (数字)",
			cond: `状态.生命值 != 100`,
			ctx: ConditionContext{
				State: map[string]any{"生命值": 80},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "状态不存在返回 false",
			cond: `状态.不存在 > 10`,
			ctx: ConditionContext{
				State: map[string]any{},
			},
			expected: false,
			wantErr:  false,
		},
		// ---- 逻辑组合 ----
		{
			name: "与运算（全部满足）",
			cond: `包含 "攻击" && 状态.堕落指数 > 80`,
			ctx: ConditionContext{
				Input: "我要发起攻击！",
				State: map[string]any{"堕落指数": 85},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "与运算（部分不满足）",
			cond: `包含 "攻击" && 状态.堕落指数 < 80`,
			ctx: ConditionContext{
				Input: "我要发起攻击！",
				State: map[string]any{"堕落指数": 85},
			},
			expected: false,
			wantErr:  false,
		},
		{
			name: "或运算（任意满足）",
			cond: `包含 "防御" || 状态.堕落指数 > 80`,
			ctx: ConditionContext{
				Input: "我要发起攻击！",
				State: map[string]any{"堕落指数": 85},
			},
			expected: true,
			wantErr:  false,
		},
		{
			name: "或运算（全部不满足）",
			cond: `包含 "防御" || 状态.生命值 > 100`,
			ctx: ConditionContext{
				Input: "我要发起攻击！",
				State: map[string]any{"生命值": 80},
			},
			expected: false,
			wantErr:  false,
		},
		// ---- 语法错误 ----
		{
			name:     "不支持的条件格式",
			cond:     `未知条件 "xxx"`,
			ctx:      ConditionContext{},
			expected: false,
			wantErr:  true,
		},
		{
			name:     "空条件",
			cond:     "",
			ctx:      ConditionContext{},
			expected: false,
			wantErr:  true,
		},
		{
			name:     "状态条件缺少操作符",
			cond:     `状态.生命值 100`,
			ctx:      ConditionContext{State: map[string]any{"生命值": 100}},
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.cond, tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("Evaluate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// ============================================================
// 动作执行器测试
// ============================================================

// TestDefaultActionExecutor 测试默认动作执行器的各种动作类型。
// 覆盖：注入（含占位符替换）、状态修改（数字/字符串/布尔）、普通文本、错误处理。
func TestDefaultActionExecutor(t *testing.T) {
	executor := &DefaultActionExecutor{}

	tests := []struct {
		name     string
		action   string
		ctx      ActionContext
		expected string
		wantErr  bool
	}{
		{
			name:   "注入动作",
			action: `注入 "光之国是故乡"`,
			ctx: ActionContext{
				Contract: &domain.Contract{RoleName: "贝利亚奥特曼"},
				Memories: []string{},
			},
			expected: "💭 光之国是故乡",
			wantErr:  false,
		},
		{
			name:   "注入动作（带角色名占位符）",
			action: `注入 "{角色名}的故乡是光之国"`,
			ctx: ActionContext{
				Contract: &domain.Contract{RoleName: "贝利亚奥特曼"},
				Memories: []string{},
			},
			expected: "💭 贝利亚奥特曼的故乡是光之国",
			wantErr:  false,
		},
		{
			name:   "状态修改（数字）",
			action: `状态.生命值 = 100`,
			ctx: ActionContext{
				State: map[string]any{},
			},
			expected: "📊 生命值 = 100",
			wantErr:  false,
		},
		{
			name:   "状态修改（字符串）",
			action: `状态.情绪 = "愤怒"`,
			ctx: ActionContext{
				State: map[string]any{},
			},
			expected: `📊 情绪 = 愤怒`,
			wantErr:  false,
		},
		{
			name:   "状态修改（布尔值）",
			action: `状态.存活 = true`,
			ctx: ActionContext{
				State: map[string]any{},
			},
			expected: "📊 存活 = true",
			wantErr:  false,
		},
		{
			name:     "普通文本",
			action:   `全力攻击周围目标`,
			ctx:      ActionContext{},
			expected: "全力攻击周围目标",
			wantErr:  false,
		},
		{
			name:     "空动作",
			action:   "",
			ctx:      ActionContext{},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "状态修改缺少 =",
			action:   `状态.生命值 100`,
			ctx:      ActionContext{State: map[string]any{}},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(tt.action, &tt.ctx, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("Execute() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// ============================================================
// 引擎核心测试
// ============================================================

// TestEngineRun 测试引擎的完整运行流程。
// 覆盖：规则匹配、动作执行、注入记忆、状态修改、默认响应。
//
// 设计说明：
//   - 每个测试用例独立构建契约，避免用例间状态污染
//   - 使用 extraRules 和 extraState 支持用例特定的规则和状态
//   - 这样“无匹配规则”用例不会意外触发其他规则
func TestEngineRun(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string         // 响应应包含的子串
		wantMemories []string       // 验证执行后的记忆（可选）
		extraRules   []*domain.Rule // 用例特定的额外规则
		extraState   map[string]any // 用例特定的状态覆盖
	}{
		{
			name:         "匹配攻击规则",
			input:        "我要发动攻击！",
			wantContains: "全力攻击周围目标",
		},
		{
			name:         "匹配防御规则",
			input:        "我要防御！",
			wantContains: "防御姿态",
		},
		{
			name:         "匹配光之国规则（注入）",
			input:        "你知道光之国吗？",
			wantContains: "贝利亚奥特曼的故乡是光之国",
			wantMemories: []string{"贝利亚奥特曼的故乡是光之国"},
		},
		{
			name:         "匹配高堕落规则（状态触发）",
			input:        "我感觉到力量在涌动",
			wantContains: "堕落指数过高",
			wantMemories: []string{"堕落指数过高"},
			extraRules: []*domain.Rule{
				{
					Name:   "高堕落",
					Cond:   `状态.堕落指数 > 80`,
					Action: `注入 "堕落指数过高"`,
				},
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
			// ---- 构建基础规则 ----
			rules := []*domain.Rule{
				{
					Name:   "攻击",
					Cond:   `包含 "攻击" || 包含 "战斗"`,
					Action: "全力攻击周围目标",
					Group:  "combat",
				},
				{
					Name:   "防御",
					Cond:   `包含 "防御" || 包含 "防守"`,
					Action: "防御姿态",
					Group:  "combat",
				},
				{
					Name:   "光之国",
					Cond:   `包含 "光之国"`,
					Action: `注入 "{角色名}的故乡是光之国"`,
				},
			}

			// ---- 合并用例特定的额外规则 ----
			rules = append(rules, tt.extraRules...)

			// ---- 构建初始状态（[]KeyValue 保持顺序） ----
			state := []domain.KeyValue{
				{Key: "堕落指数", Value: "50"},
			}
			// 合并用例特定的状态覆盖
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
			if !strings.Contains(response, tt.wantContains) {
				t.Errorf("Run() response = %v, want contain %v", response, tt.wantContains)
			}

			// ---- 验证记忆 ----
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

// TestEngineHistory 测试历史记录和容量限制。
// 验证：历史记录按照 maxHistory 正确截断，且角色顺序正确。
func TestEngineHistory(t *testing.T) {
	contract := &domain.Contract{
		RoleName: "测试角色",
		State:    []domain.KeyValue{},
		Rules:    []*domain.Rule{},
	}

	// 设置最大历史为 2 轮（即 4 条记录）
	eng := New(contract, WithMaxHistory(2))

	inputs := []string{"第一轮", "第二轮", "第三轮"}
	for _, input := range inputs {
		_, err := eng.Run(input, nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}

	history := eng.History()
	// 三轮对话：每轮 2 条（fate + assistant），共 6 条，但容量为 2 轮=4 条
	// 所以应该只保留最近的 4 条记录（即最后两轮）
	expectedLen := 4
	if len(history) != expectedLen {
		t.Errorf("History length = %d, want %d", len(history), expectedLen)
	}

	// 验证最后一条是第三轮的 assistant
	if len(history) > 0 {
		last := history[len(history)-1]
		if last.Role != "assistant" {
			t.Errorf("Last history role = %s, want assistant", last.Role)
		}
	}
}

// TestEngineStateAndMemories 测试状态和记忆的读写隔离。
// 验证：通过 State() 和 Memories() 返回的是副本，外部修改不影响引擎内部。
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

	// ---- 测试状态读取 ----
	state := eng.State()
	if state["初始状态"] != "初始值" {
		t.Errorf("State['初始状态'] = %v, want 初始值", state["初始状态"])
	}

	// 修改返回的副本，不应影响引擎内部
	state["初始状态"] = "被修改"
	if eng.State()["初始状态"] != "初始值" {
		t.Errorf("Engine state was mutated by copy")
	}

	// ---- 测试记忆读取 ----
	memories := eng.Memories()
	if len(memories) != 1 || memories[0] != "初始记忆" {
		t.Errorf("Memories = %v, want [初始记忆]", memories)
	}

	// 修改返回的副本，不应影响引擎内部
	memories[0] = "被修改"
	if len(eng.Memories()) == 0 || eng.Memories()[0] != "初始记忆" {
		t.Errorf("Engine memories was mutated by copy")
	}
}

// TestEngineDefaultResponse 测试默认响应。
// 验证：无规则匹配时，返回包含角色名的默认响应；
// 无角色名时，返回通用默认响应。
func TestEngineDefaultResponse(t *testing.T) {
	// ---- 有角色名 ----
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

	// ---- 无角色名 ----
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
	expected2 := "角色沉默地注视着命运。"
	if response2 != expected2 {
		t.Errorf("default response = %v, want %v", response2, expected2)
	}
}
