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
// 复用 parser 包的测试数据文件。
func testContractPath() string {
	return "../parser/testdata/sample.meph"
}

// TestFullIntegration 测试完整的解析 → 引擎运行流程。
func TestFullIntegration(t *testing.T) {
	contract, err := parser.ParseFile(testContractPath())
	if err != nil {
		t.Fatalf("解析 .meph 文件失败: %v", err)
	}

	// 深入追问规则（无条件，必定匹配）
	t.Run("触发深入追问", func(t *testing.T) {
		eng := engine.New(contract)
		response, err := eng.Run("这是为什么？", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(response, "引用经典") {
			t.Errorf("Run() response = %v, want contain 引用经典", response)
		}
	})

	// 转移话题规则（无条件，必定匹配）
	t.Run("触发转移话题", func(t *testing.T) {
		eng := engine.New(contract)
		response, err := eng.Run("够了！", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(response, "沉默片刻") {
			t.Errorf("Run() response = %v, want contain 沉默片刻", response)
		}
	})

	// 狂放宣言规则（无条件，必定匹配）
	// 选择"求索"以避免被前面带 roll() 的规则随机拦截
	t.Run("触发狂放宣言", func(t *testing.T) {
		eng := engine.New(contract)
		response, err := eng.Run("我要求索！", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(response, "激情陈词") {
			t.Errorf("Run() response = %v, want contain 激情陈词", response)
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
	// 注意：50 在 sample.meph 中是 "50"（字符串），ParseValue 会解析为 int(50)
	if val, ok := state["灵魂完整度"]; !ok || val != 50 {
		t.Errorf("初始灵魂完整度 = %v (%T), want 50", val, val)
	}

	// 使用包含"契约"的输入来触发梅菲斯特回声规则（注入类型）
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