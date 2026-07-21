// cmd/mephisto/utils.go
//
// CLI 辅助函数
// 职责：
//  1. 加载契约文件
//  2. 创建 LLM 客户端
//
// 这两个函数从 commands.go 中拆离，保持 commands.go 的简洁。
package main

import (
	"fmt"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/core/parser"
	"mephisto/internal/core/validator"
	"mephisto/internal/domain"
)

// loadContract 加载并验证契约文件。
//
// 流程：
//  1. 解析 .meph 文件为 domain.Contract
//  2. 验证契约的完整性
//
// 参数：
//   - filePath: .meph 文件的路径
//
// 返回值：
//   - *domain.Contract: 已验证的契约
//   - error: 加载或验证失败时的错误
func loadContract(filePath string) (*domain.Contract, error) {
	contract, err := parser.ParseFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("加载契约失败：%w", err)
	}

	errs := validator.Validate(contract)
	if len(errs) > 0 {
		var sb strings.Builder
		for _, e := range errs {
			sb.WriteString("\n  - ")
			sb.WriteString(e.Error())
		}
		return nil, fmt.Errorf("契约验证失败：%s", sb.String())
	}

	return contract, nil
}

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
