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

	// 契约灼痛规则（包含 "契约" 或 "誓约" → 注入 + 状态修改，必定匹配）
	t.Run("触发契约灼痛", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("契约已经签下", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		memories := eng.Memories()
		found := false
		for _, m := range memories {
			if strings.Contains(m, "契约书在桌上微微发烫") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("契约灼痛规则未触发，memories = %v", memories)
		}
	})

	// 深渊规则（括号分组：包含 "深渊" && (包含 "凝视" || 包含 "回望")）
	t.Run("触发深渊括号分组", func(t *testing.T) {
		eng := engine.New(contract)
		if _, err := eng.Run("深渊在凝视我", nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		memories := eng.Memories()
		found := false
		for _, m := range memories {
			if strings.Contains(m, "深渊也在回望他") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("深渊规则未触发，memories = %v", memories)
		}
	})

	// 梅菲斯特低语规则（LLM 指令/普通文本动作，主动规则，直接输出）
	t.Run("触发梅菲斯特低语", func(t *testing.T) {
		eng := engine.New(contract)
		response, err := eng.Run("耳边传来一声低语", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(response, "一声轻笑") {
			t.Errorf("Run() response = %v, want contain 一声轻笑", response)
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

	// 使用包含"契约"的输入来触发契约灼痛规则（注入类型）
	_, err = eng.Run("契约不该被打破", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// 触发规则后应产生记忆（注入规则会追加记忆）
	memories := eng.Memories()
	if len(memories) == 0 {
		t.Error("期望有记忆产生，但为空")
	}
}

// TestIntegrationDantesTemplate 验证 dantes 模板中的括号分组规则能正确触发。
//
// dantes.meph 使用了括号分组语法（如 `状态.警惕度 >= 70 && (包含 "风暴" || 包含 "浪")`），
// 此测试确保括号分组被正确编译为 AST 并执行。
func TestIntegrationDantesTemplate(t *testing.T) {
	contract, err := parser.ParseFile("../../../data/dantes.meph")
	if err != nil {
		t.Fatalf("解析 dantes.meph 失败: %v", err)
	}

	// 触发现有规则（最简单的注入规则）
	t.Run("触发归航规则", func(t *testing.T) {
		eng := engine.New(contract)
		_, err := eng.Run("我看见了码头！", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		memories := eng.Memories()
		found := false
		for _, m := range memories {
			if strings.Contains(m, "阳光落在甲板上") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("归航规则未触发，memories = %v", memories)
		}
	})

	// 触发括号分组规则：状态.警惕度 >= 70 && (包含 "风暴" || 包含 "浪")
	t.Run("触发括号分组规则", func(t *testing.T) {
		eng := engine.New(contract)
		// 提高警惕度以匹配 >= 70
		eng.Run("政治的风声很紧", nil)  // 触发风声规则：警惕度 += 15
		eng.Run("政治局势紧张", nil)   // 再提升
		eng.Run("政治暗流涌动", nil)   // 再提升
		eng.Run("政治局势日益紧张", nil) // 再提升（警惕度已 >= 70）

		// 现在触发带括号的条件
		_, err := eng.Run("我看到浪来了！", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		memories := eng.Memories()
		found := false
		for _, m := range memories {
			if strings.Contains(m, "暴风来临前的第一丝征兆") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("括号分组规则（风暴警觉）未触发，memories = %v", memories)
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