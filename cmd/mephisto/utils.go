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
// 支持的客户端类型：
//   - deepseek / openai: 使用 OpenAI/DeepSeek API
//   - ollama: 使用本地 Ollama 服务
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
		fmt.Printf("  LLM 后端：%s\n", cfg.Client)
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
		return nil, fmt.Errorf("未知的客户端类型：%s（支持：deepseek、openai、ollama）", cfg.Client)
	}
}
