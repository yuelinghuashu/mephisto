// cmd/mephisto/utils.go
//
// CLI 辅助函数：创建 LLM 客户端
//
// 从 commands.go 中拆离，保持 commands.go 的简洁。
package main

import (
	"fmt"

	"mephisto/internal/core/llm"
)

// createLLMClient 根据配置创建对应的 LLM 客户端。
//
// 支持的客户端类型（与 Flutter 版对齐）：
//   - openai: OpenAI 兼容 API（含 DeepSeek 等一切兼容服务，通过 BaseURL 区分）
//   - ollama: 使用本地 Ollama 服务
//
// 兼容性说明：
//   - "deepseek" 是 openai 兼容分支的旧别名（默认 BaseURL 指向 DeepSeek），保留以兼容老配置
//
// 参数：
//   - cfg: 应用配置（包含 Client、Model、APIKey、BaseURL、MaxTokens）
//
// 返回值：
//   - llm.Client: 可用的 LLM 客户端
//   - error: 未知客户端类型时的错误
func createLLMClient(cfg *AppConfig) (llm.Client, error) {
	switch cfg.Client {
	case "deepseek", "openai":
		client := llm.NewOpenAIClient(llm.OpenAIConfig{
			APIKey:    cfg.APIKey,
			BaseURL:   cfg.BaseURL,
			Model:     cfg.Model,
			MaxTokens: cfg.MaxTokens,
		})
		displayURL := cfg.BaseURL
		if displayURL == "" {
			displayURL = "https://api.deepseek.com/v1"
		}
		fmt.Printf("  LLM 后端：OpenAI 兼容\n")
		fmt.Printf("  模型：%s\n", cfg.Model)
		fmt.Printf("  API：%s\n", displayURL)
		return client, nil

	case "ollama":
		client := llm.NewOllamaClient(llm.OllamaConfig{
			Model: cfg.Model,
		})
		fmt.Printf("  LLM 后端：Ollama（%s）\n", cfg.Model)
		return client, nil

	default:
		return nil, fmt.Errorf("未知的客户端类型：%s（支持：openai、ollama）", cfg.Client)
	}
}
