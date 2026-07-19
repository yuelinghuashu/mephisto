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
//
// 测试数据：
//
//	所有测试用例共享 testdata/test_contract.meph 作为基础契约文件。
//	该文件包含最小化的规则集，确保测试结果可预测。
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
//
// 参数：
//   - result: 验证结果
//
// 返回值：
//   - string: 格式化的错误信息。如果验证通过，返回空字符串。
//
// 此函数用于集成测试中统一格式化 validator.Result 的输出，
// 避免在每个测试中重复编写错误拼接逻辑。
func formatValidationErrors(result validator.Result) string {
	if result.IsValid() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("契约验证失败:\n")
	for _, err := range result.List() {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}
	return sb.String()
}

// TestFullIntegration 测试完整的解析 → 验证 → 引擎运行流程。
//
// 测试场景：
//  1. 解析 testdata/test_contract.meph 文件
//  2. 验证契约的完整性
//  3. 使用不同输入触发不同的规则匹配
//  4. 验证每条规则的响应是否符合预期
//
// 覆盖的规则：
//   - 攻击规则：输入包含 "攻击" → 返回 "攻击响应"
//   - 防御规则：输入包含 "防御" → 返回 "防御响应"
//   - 光之国规则：输入包含 "光之国" → 注入 "光之国知识"
//   - 无匹配规则：输入不匹配任何规则 → 返回默认响应
//
// 设计说明：
//
//	每个子测试独立创建引擎实例（engine.New(contract)），
//	避免用例之间的状态污染（如历史记录、记忆等）。
func TestFullIntegration(t *testing.T) {
	contractPath := testutil.GetTestDataPath("test_contract.meph")
	contract, err := parser.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("解析 test_contract.meph 失败: %v", err)
	}

	result := validator.Validate(contract)
	if !result.IsValid() {
		t.Fatalf("%s", formatValidationErrors(result))
	}

	tests := []struct {
		name         string
		input        string
		wantContains string
	}{
		{"触发攻击", "我要攻击！", "攻击响应"},
		{"触发防御", "我要防御！", "防御响应"},
		{"触发光之国", "光之国是什么？", "光之国知识"},
		{"无匹配", "你好", "测试角色 沉默地注视着命运。"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := engine.New(contract)
			response, err := eng.Run(tt.input, nil)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !strings.Contains(response, tt.wantContains) {
				t.Errorf("Run() response = %v, want contain %v", response, tt.wantContains)
			}
		})
	}
}

// TestIntegrationWithInvalidFile 测试格式错误文件的错误传递。
//
// 测试场景：
//
//	创建一个格式错误的 .meph 文件（锚点区块的键值对缺少冒号），
//	验证 parser.ParseFile 返回错误，而不是静默处理。
//
// 预期行为：
//   - parser.ParseFile 应返回非 nil 错误
//   - 错误信息应包含行号和错误描述
//
// 设计说明：
//
//	此测试验证错误传递链是否正确：
//	lexer（发现格式错误）→ parser（上抛）→ 调用方（捕获）。
//	确保格式错误不会被解析器忽略或吞没。
func TestIntegrationWithInvalidFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid_*.meph")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// 格式错误：键值对缺少冒号
	// 正确格式应为：- 核心信念： "力量"
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
//
// 测试场景：
//
//	创建一个只包含【状态】区块的 .meph 文件（缺少【角色名】），
//	验证解析器或验证器能正确报告错误。
//
// 预期行为：
//   - 如果解析器成功解析（取决于 lexer 是否强制要求角色名区块），
//     验证器应检测到角色名为空并返回验证错误。
//   - 如果解析器直接报错，也是可接受的行为（取决于 lexer 实现）。
//
// 设计说明：
//
//	此测试验证“缺少必填字段”的错误在解析-验证链路中能被正确捕获。
//	不同版本的 Mephisto 可能在 lexer 层或 validator 层报告此错误，
//	测试兼容两种行为。
func TestIntegrationWithMissingRequiredBlock(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "missing_*.meph")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// 只有【状态】区块，没有【角色名】
	content := `【状态】
- 情绪: 暴怒`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	contract, err := parser.ParseFile(tmpFile.Name())
	if err == nil {
		// 解析成功，验证应该失败（角色名为空）
		result := validator.Validate(contract)
		if result.IsValid() {
			t.Error("期望验证失败（角色名为空），但实际通过了")
		} else {
			t.Logf("验证正确失败: %s", formatValidationErrors(result))
		}
	} else {
		// 解析失败（lexer 层已报错），也是可接受的行为
		t.Logf("解析失败（可接受）: %v", err)
	}
}

// TestIntegrationStatePersistence 测试状态在多次对话中的持久性。
//
// 测试场景：
//  1. 加载 test_contract.meph（初始堕落指数为 50）
//  2. 触发光之国规则（注入记忆，不修改状态）
//  3. 验证记忆被正确保存
//
// 设计说明：
//
//	当前 test_contract.meph 不包含状态修改规则（状态.xxx = yyy），
//	因此本测试聚焦于验证“注入”动作对记忆的持久化。
//	状态持久化的完整测试需在 test_contract.meph 中添加状态修改规则后再补充。
//
// 注意事项：
//
//	test_contract.meph 中没有状态修改规则，所以状态始终为初始值。
//	如果未来需要测试状态修改的持久性，应在测试契约中添加相应规则。
func TestIntegrationStatePersistence(t *testing.T) {
	contractPath := testutil.GetTestDataPath("test_contract.meph")
	contract, err := parser.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("解析 test_contract.meph 失败: %v", err)
	}

	result := validator.Validate(contract)
	if !result.IsValid() {
		t.Fatalf("%s", formatValidationErrors(result))
	}

	eng := engine.New(contract)

	// 验证初始状态：堕落指数应为 50
	state := eng.State()
	if val, ok := state["堕落指数"]; !ok || val != 50 {
		t.Errorf("初始堕落指数 = %v, want 50", val)
	}

	// 触发光之国规则：注入 "光之国知识" 到记忆
	response, err := eng.Run("光之国是什么？", nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(response, "光之国知识") {
		t.Errorf("期望注入响应，实际: %s", response)
	}

	// 验证记忆已被持久化保存
	memories := eng.Memories()
	if len(memories) != 1 || memories[0] != "光之国知识" {
		t.Errorf("记忆 = %v, want [光之国知识]", memories)
	}
}

// TestIntegrationHistoryLimit 测试历史记录的容量限制。
//
// 测试场景：
//
//	设置最大历史保留轮数为 2（引擎.WithMaxHistory(2)），
//	执行 5 轮对话（每轮产生 2 条记录，共 10 条），
//	验证历史记录被自动截断为最近的 4 条（2 轮 × 2 条/轮）。
//
// 预期行为：
//   - 5 轮对话后，历史记录长度应为 4（而不是 10）
//   - 被截断的是最早轮次的记录，保留的是最近两轮
//
// 设计说明：
//
//	此测试验证引擎的历史容量限制机制正常工作，
//	防止无限增长的历史记录导致内存问题。
func TestIntegrationHistoryLimit(t *testing.T) {
	contractPath := testutil.GetTestDataPath("test_contract.meph")
	contract, err := parser.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("解析 test_contract.meph 失败: %v", err)
	}

	result := validator.Validate(contract)
	if !result.IsValid() {
		t.Fatalf("%s", formatValidationErrors(result))
	}

	// 设置最大历史保留 2 轮
	eng := engine.New(contract, engine.WithMaxHistory(2))

	// 执行 5 轮对话
	for range 5 {
		_, err := eng.Run("你好", nil)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}

	// 验证历史记录被截断为 4 条（2 轮 × 2 条/轮）
	history := eng.History()
	if len(history) != 4 {
		t.Errorf("历史记录长度 = %d, want 4", len(history))
	}
}
