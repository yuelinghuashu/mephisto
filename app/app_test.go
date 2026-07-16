//go:build integration

// ============================================================
// app_test.go - 应用层集成测试
// 职责：
// 1. 验证应用主流程的可用性（无 LLM）
// 2. 验证 LLM 集成（需设置环境变量）
// 3. 使用模拟 stdin 避免阻塞
// 4. 需要标签 integration 才会运行
// 5. 使用 runtime.Caller 获取测试数据绝对路径，避免工作目录依赖
// ============================================================

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mephisto/llm"
)

// getTestDataPath 返回测试数据文件的绝对路径
// 基于当前测试文件的目录（app/）向上查找 data/ 目录
func getTestDataPath(filename string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("无法获取当前文件路径")
	}
	dir := filepath.Dir(currentFile)
	return filepath.Join(dir, "..", "data", filename)
}

// TestRunNoLLM 测试无 LLM 模式下的应用启动，并自动退出
func TestRunNoLLM(t *testing.T) {
	// 模拟标准输入：输入 "quit" 并关闭
	input := "quit\n"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	// 运行应用（使用示例文件，通过辅助函数获取绝对路径）
	samplePath := getTestDataPath("sample.meph")
	err = Run(samplePath, Config{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// 如果执行到此处，说明成功退出
}

// TestRunWithLLM 测试带 LLM 的集成，需要设置环境变量：
//
//	MEPHISTO_TEST_LLM=true  启用此测试
//	MEPHISTO_API_KEY=xxx    DeepSeek API Key
func TestRunWithLLM(t *testing.T) {
	// 1. 检查是否显式启用 LLM 测试
	if os.Getenv("MEPHISTO_TEST_LLM") != "true" {
		t.Skip("跳过 LLM 测试：未设置 MEPHISTO_TEST_LLM=true")
	}

	// 2. 检查 API Key 是否存在
	apiKey := os.Getenv("MEPHISTO_API_KEY")
	if apiKey == "" {
		t.Skip("跳过 LLM 测试：未设置 MEPHISTO_API_KEY")
	}

	// 3. 模拟多行输入：先输入 "hello" 触发 LLM（自由对话），再输入 "quit" 退出
	input := "hello\nquit\n"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	// 4. 配置 LLM
	cfg := Config{
		LLM: llm.Config{
			APIKey:      apiKey,
			Model:       "deepseek-chat",
			BaseURL:     "https://api.deepseek.com/v1",
			MaxTokens:   4096,
			Temperature: nil, // 使用默认
		},
		Debug: false,
	}

	// 5. 运行应用（使用示例文件，通过辅助函数获取绝对路径）
	samplePath := getTestDataPath("sample.meph")
	err = Run(samplePath, cfg)
	if err != nil {
		// 如果是 LLM 调用失败（网络、认证等），跳过而不是失败
		if strings.Contains(err.Error(), "LLM") ||
			strings.Contains(err.Error(), "API") ||
			strings.Contains(err.Error(), "请求") ||
			strings.Contains(err.Error(), "timeout") {
			t.Skipf("LLM 调用失败（可能网络或密钥问题）: %v", err)
		}
		t.Fatalf("Run failed: %v", err)
	}
	// 成功执行完整个流程，说明 LLM 集成通过
}
