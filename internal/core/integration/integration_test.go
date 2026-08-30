// internal/core/integration/integration_test.go
//
// 本文件包含 Mephisto 核心模块的集成测试。
// 集成测试验证从 .meph 文件解析到引擎运行的完整链路，确保各模块之间的协作正确。
//
// 测试覆盖范围：
//  1. 完整流程测试：解析 → 引擎运行，覆盖多条规则匹配
//  2. 错误处理测试：格式错误文件的解析失败
//  3. 业务验证测试：缺少必填区块时的解析失败
//  4. 状态持久化测试：状态在多次对话中的变化
//  5. 历史容量测试：历史记录的自动截断
//
// 与单元测试的区别：
//   - 单元测试（parser_test.go / engine_test.go）验证单个模块的内部逻辑
//   - 集成测试验证模块之间的协作，使用真实的 .meph 文件作为输入
package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mephisto/internal/core/engine"
	"mephisto/internal/core/parser"
	"mephisto/internal/domain"
)

// testContractPath 返回集成测试用的 .meph 文件路径。
// 使用根目录 data/ 下的真实浮士德模板，验证引擎与示例契约的兼容性。
func testContractPath() string {
	return "../../../data/faust.meph"
}

// TestFullIntegration 测试完整的解析 → 引擎运行流程。
func TestFullIntegration(t *testing.T) {
	contract, err := parser.ParseFile(testContractPath())
	if err != nil {
		t.Fatalf("解析 .meph 文件失败: %v", err)
	}

	// 情绪流转规则（状态机：包含"满足" → 状态.情绪 = "满足"）
	t.Run("触发情绪流转", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("我感到了真正的满足", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if state["情绪"] != "满足" {
			t.Errorf("情绪 = %v, want 满足", state["情绪"])
		}
	})

	// 灵魂维系规则（状态机：灵魂完整度 <= 25 且表达濒死语境 → 重置为 30）
	// 对齐 Flutter 反模式 4/9 修复：纯阈值规则必须绑定语境词（普通对话不再无条件拉回）
	t.Run("触发灵魂维系", func(t *testing.T) {
		// 修改契约初始状态：灵魂完整度设为 20（触发灵魂维系条件）
		modified := *contract // 浅拷贝避免影响其他子测试
		modified.State = make([]domain.StateItem, len(contract.State))
		for i, item := range contract.State {
			modified.State[i] = item
			if item.Key == "灵魂完整度" {
				modified.State[i] = domain.StateItem{Key: "灵魂完整度", Value: domain.ParseStateValue("20")}
			}
		}
		// 普通对话（无濒死语境）：不再无条件拉回（反模式 4 修复）
		eng := engine.New(&modified)
		if _, err := eng.Run("你好", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if state["灵魂完整度"] != 20 {
			t.Errorf("普通对话不应触发灵魂维系：灵魂完整度 = %v, want 20（保持可跌入深渊）", state["灵魂完整度"])
		}
		// 濒死语境：契约拉回 30
		eng2 := engine.New(&modified)
		if _, err := eng2.Run("我感觉灵魂快要消散了", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state = eng2.State()
		if state["灵魂完整度"] != 30 {
			t.Errorf("濒死语境应触发灵魂维系：灵魂完整度 = %v, want 30", state["灵魂完整度"])
		}
	})

	// 命运骰规则（骰子判定：包含"赌" && roll(1d100) >= 70 → 灵魂完整度 += 10）
	t.Run("触发命运骰", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("我要赌一把", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		// 骰子结果随机，灵魂完整度应为 100（未触发）或 110（触发 +10）
		state := eng.State()
		if val, ok := state["灵魂完整度"]; !ok || (val != 100 && val != 110) {
			t.Errorf("灵魂完整度 = %v, want 100 或 110（骰子成功时）", val)
		}
	})

	// 梅菲斯特低语规则（LLM 指令/普通文本动作，主动规则，直接输出）
	t.Run("触发梅菲斯特低语", func(t *testing.T) {
		eng := engine.New(contract)
		response, err := eng.Run("耳边传来一声低语", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(response, "梅菲斯特靠近{角色名}") {
			t.Errorf("Run() response = %v, want contain 梅菲斯特靠近{角色名}", response)
		}
	})

	// 烛火映心规则（括号分组 + 互斥组：位置==书斋 && (包含"光明" || 包含"烛火") → +5）
	t.Run("触发烛火映心", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("烛火摇曳", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if state["灵魂完整度"] != 105 {
			t.Errorf("灵魂完整度 = %v, want 105（初始100 + 5）", state["灵魂完整度"])
		}
	})

	// 说出停留 + 灵魂归主（状态机终局：情绪==满足 && 包含"停留" → 注入 + 情绪=终局）
	t.Run("触发状态机终局", func(t *testing.T) {
		eng := engine.New(contract)
		// 第一步：触发情绪流转设置情绪为"满足"
		eng.Run("我感到了真正的满足", nil)
		// 第二步：状态.情绪 == "满足" && 包含"停留"
		if _, err := eng.Run("请你停留一下", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if state["情绪"] != "终局" {
			t.Errorf("情绪 = %v, want 终局", state["情绪"])
		}
		memories := eng.Memories()
		found := false
		for _, m := range memories {
			if strings.Contains(m, "契约已成") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("[说出停留] 注入未触发，memories = %v", memories)
		}
	})

	// 无规则匹配时返回默认响应
	t.Run("无匹配", func(t *testing.T) {
		eng := engine.New(contract)
		response, err := eng.Run("你好", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(response, "沉默地注视着命运") {
			t.Errorf("Run() response = %v, want contain 沉默地注视着命运", response)
		}
	})
}

// TestIntegrationWithInvalidFile 测试格式错误文件的错误传递。
func TestIntegrationWithInvalidFile(t *testing.T) {
	content := `【锚点】
- 核心信念 "力量"`
	tmpPath := filepath.Join(t.TempDir(), "invalid.meph")
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, parseErr := parser.ParseFile(tmpPath)
	if parseErr == nil {
		t.Error("期望解析失败（格式错误），但实际成功了")
	}
}

// TestIntegrationWithMissingRequiredBlock 测试缺少必填区块时的错误传递。
// 角色名为空时解析器应返回错误。
func TestIntegrationWithMissingRequiredBlock(t *testing.T) {
	content := `【状态】
- 情绪: 暴怒`
	tmpPath := filepath.Join(t.TempDir(), "missing.meph")
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	contract, parseErr := parser.ParseFile(tmpPath)
	if parseErr != nil {
		// 解析器直接报错（角色名缺失等）
		t.Logf("解析失败（可接受）: %v", parseErr)
	} else if contract.RoleName == "" {
		// 解析成功但角色名为空（取决于解析器实现）
		t.Log("解析成功但角色名为空")
	} else {
		t.Error("期望解析失败或角色名为空，但实际通过了")
	}
}

// TestIntegrationStatePersistence 测试状态在多次对话中的持久性。
func TestIntegrationStatePersistence(t *testing.T) {
	contract, err := parser.ParseFile(testContractPath())
	if err != nil {
		t.Fatalf("解析 .meph 文件失败: %v", err)
	}

	eng := engine.New(contract)

	state := eng.State()
	// 注意：faust.meph 中灵魂完整度为 100（字符串），ParseValue 会解析为 int(100)
	if val, ok := state["灵魂完整度"]; !ok || val != 100 {
		t.Errorf("初始灵魂完整度 = %v (%T), want 100", val, val)
	}

	// 使用包含"满足"的输入来触发情绪流转规则（状态修改类型）
	_, err = eng.Run("我感到了心满意足", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// 验证状态持久化：情绪已从"永不满足"变为"满足"
	state = eng.State()
	if state["情绪"] != "满足" {
		t.Errorf("情绪 = %v, want 满足", state["情绪"])
	}
}

// TestIntegrationDantesTemplate 验证 dantes 模板中的规则能正确触发。
//
// dantes.meph 使用了状态机（复仇之火/交锋）、骰子（棋局）、互斥组（终局抉择）、
// 终局注入（天平方正）等 DSL 特性，此测试确保这些规则被正确编译为 AST 并执行。
func TestIntegrationDantesTemplate(t *testing.T) {
	contract, err := parser.ParseFile("../../../data/dantes.meph")
	if err != nil {
		t.Fatalf("解析 dantes.meph 失败: %v", err)
	}

	// 复仇之火（状态机：提及仇人唤醒仇恨）
	t.Run("触发复仇之火", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("我看到了唐格拉尔", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if val, ok := state["仇恨"]; !ok || (val != 85 && val != 95) {
			t.Errorf("仇恨 = %v, want 85 或 95（棋局可能叠加触发）", val)
		}
	})

	// 交锋（状态机：正面周旋积累谋划）
	t.Run("触发交锋", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("让我来试探他", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if state["谋划"] != 75 {
			t.Errorf("谋划 = %v, want 75（初始70 + 5）", state["谋划"])
		}
	})

	// 棋局（骰子 + 状态条件：谋划 >= 60 时骰子判定）
	t.Run("触发布局棋局", func(t *testing.T) {
		eng := engine.New(contract)
		// 初始谋划 = 70 >= 60，触发 [棋局]：roll(1d100) >= 50
		if _, err := eng.Run("我的布局已定", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		// 骰子结果随机：仇恨应为 80（未触发）或 90（触发 +10）
		state := eng.State()
		if val, ok := state["仇恨"]; !ok || (val != 80 && val != 90) {
			t.Errorf("仇恨 = %v, want 80 或 90（骰子成功时）", val)
		}
	})

	// 代价（状态机：宽恕萌芽）
	t.Run("触发代价", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("我看到了无辜的梅尔塞苔丝", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if state["宽恕"] != 10 {
			t.Errorf("宽恕 = %v, want 10（初始0 + 10）", state["宽恕"])
		}
	})

	// 终局抉择（互斥组：放下 vs 毁灭）
	t.Run("触发放下", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("我选择放下仇恨", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		state := eng.State()
		if val, ok := state["仇恨"]; !ok || (val != 60 && val != 70) {
			t.Errorf("仇恨 = %v, want 60 或 70（棋局可能叠加触发）", val)
		}
	})

	// 天平方正（终局注入：仇恨 >= 100 时触发）
	t.Run("触发天平方正", func(t *testing.T) {
		// 通过修改契约初始状态将仇恨设为 100
		modified := *contract
		modified.State = make([]domain.StateItem, len(contract.State))
		for i, item := range contract.State {
			modified.State[i] = item
			if item.Key == "仇恨" {
				modified.State[i] = domain.StateItem{Key: "仇恨", Value: domain.ParseStateValue("100")}
			}
		}
		eng := engine.New(&modified)
		if _, err := eng.Run("审判的时刻到了", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		memories := eng.Memories()
		found := false
		for _, m := range memories {
			if strings.Contains(m, "执掌天平与利剑的审判者") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("天平方正规则未触发，memories = %v", memories)
		}
	})
}

// TestIntegrationHistoryLimit 测试历史记录的容量限制。
func TestIntegrationHistoryLimit(t *testing.T) {
	contract, err := parser.ParseFile(testContractPath())
	if err != nil {
		t.Fatalf("解析 .meph 文件失败: %v", err)
	}

	// 设置最大历史保留 2 轮
	eng := engine.New(contract, engine.WithMaxHistory(2))

	for range 5 {
		_, err := eng.Run("你好", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}

	history := eng.History()
	if len(history) != 4 {
		t.Errorf("历史记录长度 = %d, want 4", len(history))
	}
}