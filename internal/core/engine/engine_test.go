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
// 骰子表达式测试（含自定义阈值）
// ============================================================

func TestEvalRoll(t *testing.T) {
	tests := []struct {
		name    string
		cond    string
		wantMin int
		wantMax int
	}{
		// ---- 默认阈值 ----
		{"默认 1d100 范围 [1,100]", "roll(1d100)", 1, 100},
		{"默认 2d6 范围 [2,12]", "roll(2d6)", 2, 12},

		// ---- 自定义阈值（只验证返回值的数值范围，是否满足阈值由条件逻辑保证） ----
		{"自定义 >=80", "roll(1d100) >= 80", 1, 100},
		{"自定义 >80", "roll(1d100) > 80", 1, 100},
		{"自定义 <=30", "roll(1d100) <= 30", 1, 100},
		{"自定义 <30", "roll(1d100) < 30", 1, 100},
		{"自定义 ==50", "roll(1d100) == 50", 1, 100},
		{"自定义 !=50", "roll(1d100) != 50", 1, 100},
		{"自定义 2d6 >=10", "roll(2d6) >= 10", 2, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 多次运行，验证骰值始终在范围内
			for i := 0; i < 100; i++ {
				_, val := evalRoll(tt.cond)
				if val < tt.wantMin || val > tt.wantMax {
					t.Errorf("evalRoll(%q) = %d, want in range [%d, %d]", tt.cond, val, tt.wantMin, tt.wantMax)
					break
				}
			}
		})
	}
}

func TestEvalRollWithCustomThreshold(t *testing.T) {
	// 自定义阈值 >=80：用高阈值测试时，多次运行应至少有一次 false
	t.Run("roll(1d100) >= 80 应该有失败的可能", func(t *testing.T) {
		hasFalse := false
		hasTrue := false
		for i := 0; i < 200; i++ {
			matched, _ := evalRoll("roll(1d100) >= 80")
			if matched {
				hasTrue = true
			} else {
				hasFalse = true
			}
			if hasTrue && hasFalse {
				break
			}
		}
		if !hasTrue || !hasFalse {
			t.Errorf("roll(1d100) >= 80: 200 次运行中应有 true 和 false 出现，但 got true=%v, false=%v", hasTrue, hasFalse)
		}
	})

	// roll(1d100) >= 0 应该总是 true
	t.Run("roll(1d100) >= 0 总是成功", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			matched, _ := evalRoll("roll(1d100) >= 0")
			if !matched {
				t.Errorf("roll(1d100) >= 0 应该总是 true，但第 %d 次返回 false", i)
				break
			}
		}
	})

	// roll(1d100) > 100 应该总是 false
	t.Run("roll(1d100) > 100 总是失败", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			matched, _ := evalRoll("roll(1d100) > 100")
			if matched {
				t.Errorf("roll(1d100) > 100 应该总是 false，但第 %d 次返回 true", i)
				break
			}
		}
	})
}

func TestParseRollExpr(t *testing.T) {
	tests := []struct {
		name         string
		cond         string
		wantOk       bool
		wantCount    int
		wantSides    int
		wantOp       string
		wantThreshold int
		wantRollCore string
		wantMaxValue int
	}{
		{"默认无阈值", "roll(1d100)", true, 1, 100, "", 0, "roll(1d100)", 100},
		{"默认 2d6", "roll(2d6)", true, 2, 6, "", 0, "roll(2d6)", 12},
		{"自定义 >=80", "roll(1d100) >= 80", true, 1, 100, ">=", 80, "roll(1d100)", 100},
		{"自定义 >80", "roll(1d100) > 80", true, 1, 100, ">", 80, "roll(1d100)", 100},
		{"自定义 <=30", "roll(1d100) <= 30", true, 1, 100, "<=", 30, "roll(1d100)", 100},
		{"自定义 <30", "roll(1d100) < 30", true, 1, 100, "<", 30, "roll(1d100)", 100},
		{"自定义 ==50", "roll(1d100) == 50", true, 1, 100, "==", 50, "roll(1d100)", 100},
		{"自定义 !=50", "roll(1d100) != 50", true, 1, 100, "!=", 50, "roll(1d100)", 100},
		{"非法格式", "xxx", false, 0, 0, "", 0, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, ok := parseRollExpr(tt.cond)
			if ok != tt.wantOk {
				t.Errorf("parseRollExpr(%q).ok = %v, want %v", tt.cond, ok, tt.wantOk)
				return
			}
			if !ok {
				return
			}
			if re.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", re.Count, tt.wantCount)
			}
			if re.Sides != tt.wantSides {
				t.Errorf("Sides = %d, want %d", re.Sides, tt.wantSides)
			}
			if re.Op != tt.wantOp {
				t.Errorf("Op = %q, want %q", re.Op, tt.wantOp)
			}
			if re.UserThreshold != tt.wantThreshold {
				t.Errorf("UserThreshold = %d, want %d", re.UserThreshold, tt.wantThreshold)
			}
			if re.RollCore != tt.wantRollCore {
				t.Errorf("RollCore = %q, want %q", re.RollCore, tt.wantRollCore)
			}
			if re.maxValue() != tt.wantMaxValue {
				t.Errorf("maxValue() = %d, want %d", re.maxValue(), tt.wantMaxValue)
			}
			if re.Op != "" {
				expectedDesc := fmt.Sprintf("阈值 %s%d", re.Op, re.UserThreshold)
				if re.thresholdDesc() != expectedDesc {
					t.Errorf("thresholdDesc() = %q, want %q", re.thresholdDesc(), expectedDesc)
				}
			} else {
				if re.thresholdDesc() != "" {
					t.Errorf("thresholdDesc() should be empty, got %q", re.thresholdDesc())
				}
			}
		})
	}
}

func TestExtractRollInfo(t *testing.T) {
	tests := []struct {
		name     string
		cond     string
		wantPre  string // 前缀
	}{
		{"无骰子", "包含 攻击", ""},
		{"只有骰子", "roll(1d100)", "🎲 骰子结果："},
		{"骰子+阈值", "roll(1d100) >= 80", "🎲 骰子结果："},
		{"复合条件", `包含 "愤怒" && roll(1d100) >= 80`, "🎲 骰子结果："},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRollInfo(tt.cond)
			if tt.wantPre == "" {
				if result != "" {
					t.Errorf("extractRollInfo(%q) = %q, want empty", tt.cond, result)
				}
				return
			}
			if !strings.HasPrefix(result, tt.wantPre) {
				t.Errorf("extractRollInfo(%q) = %q, want prefix %q", tt.cond, result, tt.wantPre)
			}
			// 验证包含 = 符号（表示有骰值）
			if !strings.Contains(result, "=") {
				t.Errorf("extractRollInfo(%q) = %q, should contain '='", tt.cond, result)
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
			rule, matched, _ := matchRule(rules, tt.input, state, false)
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
		rule, matched, _ := matchRule(rules, "我要攻击和防御！", state, false)
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

			result := ExecuteAction(tt.action, tt.input, runtime, nil, nil, "")
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

// ============================================================
// 格式化输出测试（用于人工审查变量替换、骰子结果等）
// 运行：go test -v -run TestDumpFormattedMeph
// ============================================================

func TestDumpFormattedMeph(t *testing.T) {
	contract := &domain.Contract{
		RoleName: "贝利亚奥特曼",
		Anchor: []domain.KeyValue{
			{Key: "核心信念", Value: `"力量就是一切"`},
			{Key: "绝对禁忌", Value: "不会承认自己的软弱"},
		},
		Worldview:  "光之国是M78星云中奥特曼的故乡。",
		Background: "{角色名}曾经是光之国最强大的战士之一。",
		Opening:    "宇宙空间站中，{角色名}的记忆开始苏醒。",
		State: []domain.KeyValue{
			{Key: "堕落指数", Value: "85"},
			{Key: "情绪", Value: "暴怒"},
		},
		Rules: []*domain.Rule{
			{Name: "光之国", Cond: `包含 "光之国"`, Action: `注入 "{角色名}的故乡是光之国"`},
			{Name: "暴走", Cond: `roll(1d100) >= 80`, Action: `注入 "{角色名}感到狂暴涌上心头"`},
		},
	}

	eng := New(contract)

	// 第一轮对话：触发光之国注入（变量替换）
	t.Log("===== 第一轮：触发光之国注入（{角色名}→贝利亚奥特曼）=====")
	_, err := eng.Run("你知道光之国吗？", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// 第二轮对话：触发骰子注入（骰子结果随机）
	t.Log("===== 第二轮：触发暴走判定（roll(1d100) >= 80）=====")
	_, err = eng.Run("我感觉力量在涌动！", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// 获取格式化后的 .meph 内容
	content := eng.buildChildContent()

	t.Logf("格式化后的 .meph 文件内容：\n%s", content)

	// 断言：变量替换已生效
	if !strings.Contains(content, "贝利亚奥特曼的故乡是光之国") {
		t.Error("变量替换未生效：{角色名} 应被替换为 贝利亚奥特曼")
	}

	// 断言：roll 骰子结果已包含在记忆描述中
	hasRollResult1 := strings.Contains(content, "感到狂暴涌上心头")
	if !hasRollResult1 {
		t.Log("注：暴走注入未触发（roll 结果未达到阈值 80，属正常随机波动）")
	}

	// 断言：锚点内容已保留
	if !strings.Contains(content, "力量就是一切") {
		t.Error("锚点内容未保留")
	}

	// 断言：状态已更新（历史中记录了状态保持不变）
	state := eng.State()
	if state["堕落指数"] != 85 {
		t.Logf("注意：堕落指数当前值为 %v", state["堕落指数"])
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
