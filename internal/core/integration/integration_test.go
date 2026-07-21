// internal/core/integration/integration_test.go
//
// 本文件包含 Mephisto 核心模块的集成测试。
// 集成测试验证从 .meph 文件解析到引擎运行的完整链路，确保各模块之间的协作正确。
//
// 测试覆盖范围：
//  1. 完整流程测试：解析 → 验证 → 引擎运行，覆盖多条规则匹配
//  2. 错误处理测试：格式错误文件的解析失败
//  3. 业务验证测试：缺少必填区块时的验证失败
//  4. 状态持久化测试：状态在多次对话中的变化
//  5. 历史容量测试：历史记录的自动截断
//
// 与单元测试的区别：
//   - 单元测试（parser_test.go / engine_test.go）验证单个模块的内部逻辑
//   - 集成测试验证模块之间的协作，使用真实的 .meph 文件作为输入
package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"mephisto/internal/core/engine"
	"mephisto/internal/core/parser"
	"mephisto/internal/core/validator"
	"mephisto/internal/testutil"
)

// formatValidationErrors 将验证错误格式化为可读的字符串。
func formatValidationErrors(errs []validator.ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("契约验证失败:\n")
	for _, err := range errs {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}
	return sb.String()
}

// TestFullIntegration 测试完整的解析 → 验证 → 引擎运行流程。
func TestFullIntegration(t *testing.T) {
	contractPath := testutil.GetTestDataPath("test_contract.meph")
	contract, err := parser.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("解析 test_contract.meph 失败: %v", err)
	}

	errs := validator.Validate(contract)
	if len(errs) > 0 {
		t.Fatalf("%s", formatValidationErrors(errs))
	}

	tests := []struct {
		name         string
		input        string
		wantContains string
		wantMemory   string
	}{
		{"触发攻击", "我要攻击！", "攻击响应", ""},
		{"触发防御", "我要防御！", "防御响应", ""},
		{"触发光之国", "光之国是什么？", "", "光之国知识"},
		{"无匹配", "你好", "测试角色 沉默地注视着命运。", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := engine.New(contract)
			response, err := eng.Run(tt.input, nil)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tt.wantContains != "" && !strings.Contains(response, tt.wantContains) {
				t.Errorf("Run() response = %v, want contain %v", response, tt.wantContains)
			}
			if tt.wantMemory != "" {
				memories := eng.Memories()
				if len(memories) == 0 || memories[0] != tt.wantMemory {
					t.Errorf("Memories = %v, want contain %v", memories, tt.wantMemory)
				}
			}
		})
	}
}

// TestIntegrationWithInvalidFile 测试格式错误文件的错误传递。
func TestIntegrationWithInvalidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid_*.meph")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `【锚点】
- 核心信念 "力量"`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	_, err = parser.ParseFile(tmpFile.Name())
	if err == nil {
		t.Error("期望解析失败（格式错误），但实际成功了")
	}
}

// TestIntegrationWithMissingRequiredBlock 测试缺少必填区块时的错误传递。
func TestIntegrationWithMissingRequiredBlock(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "missing_*.meph")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `【状态】
- 情绪: 暴怒`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	contract, err := parser.ParseFile(tmpFile.Name())
	if err == nil {
		errs := validator.Validate(contract)
		if len(errs) == 0 {
			t.Error("期望验证失败（角色名为空），但实际通过了")
		} else {
			t.Logf("验证正确失败: %s", formatValidationErrors(errs))
		}
	} else {
		t.Logf("解析失败（可接受）: %v", err)
	}
}

// TestIntegrationStatePersistence 测试状态在多次对话中的持久性。
func TestIntegrationStatePersistence(t *testing.T) {
	contractPath := testutil.GetTestDataPath("test_contract.meph")
	contract, err := parser.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("解析 test_contract.meph 失败: %v", err)
	}

	errs := validator.Validate(contract)
	if len(errs) > 0 {
		t.Fatalf("%s", formatValidationErrors(errs))
	}

	eng := engine.New(contract)

	state := eng.State()
	if val, ok := state["堕落指数"]; !ok || val != 50 {
		t.Errorf("初始堕落指数 = %v, want 50", val)
	}

	_, err = eng.Run("光之国是什么？", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	memories := eng.Memories()
	if len(memories) == 0 || memories[0] != "光之国知识" {
		t.Errorf("记忆 = %v, want contain [光之国知识]", memories)
	}
}

// TestIntegrationHistoryLimit 测试历史记录的容量限制。
func TestIntegrationHistoryLimit(t *testing.T) {
	contractPath := testutil.GetTestDataPath("test_contract.meph")
	contract, err := parser.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("解析 test_contract.meph 失败: %v", err)
	}

	errs := validator.Validate(contract)
	if len(errs) > 0 {
		t.Fatalf("%s", formatValidationErrors(errs))
	}

	// 设置最大历史保留 2 轮（直接使用 WithMaxHistory）
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
