// internal/core/llm/openai.go
//
// 本文件提供 OpenAI/DeepSeek 兼容的 LLM 客户端实现。
// 支持流式和非流式两种调用方式。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient 实现了 Client 接口，用于调用 OpenAI/DeepSeek 兼容的 API。
type OpenAIClient struct {
	config     OpenAIConfig
	httpClient *http.Client
}

// OpenAIConfig 配置 OpenAI/DeepSeek 客户端。
// 注意：APIKey 由调用方显式提供，不再从环境变量自动读取。
type OpenAIConfig struct {
	APIKey    string        // API 密钥（必填）
	BaseURL   string        // API 基础 URL，默认 https://api.deepseek.com/v1
	Model     string        // 模型名称，默认 deepseek-v4-flash
	MaxTokens int           // 最大生成 Token 数，默认 4096
	Timeout   time.Duration // 超时时间，默认 120 秒
}

// NewOpenAIClient 创建 OpenAI/DeepSeek 客户端。
// 注意：APIKey 必须由调用方提供，不再从环境变量自动回退。
func NewOpenAIClient(cfg OpenAIConfig) *OpenAIClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-v4-flash"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}

	return &OpenAIClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// chatEndpoint 返回完整的 API 端点 URL。
func (c *OpenAIClient) chatEndpoint() string {
	return strings.TrimSuffix(c.config.BaseURL, "/") + "/chat/completions"
}

// Generate 实现同步生成（非流式）。
func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]any{
		"model":      c.config.Model,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"stream":     false,
		"max_tokens": c.config.MaxTokens,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.chatEndpoint(), bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回错误：%s", resp.Status)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败：%w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API 返回空响应")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

// GenerateStream 实现流式生成。
func (c *OpenAIClient) GenerateStream(ctx context.Context, prompt string, callback func(chunk string)) (string, error) {
	reqBody := map[string]any{
		"model":      c.config.Model,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"stream":     true,
		"max_tokens": c.config.MaxTokens,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.chatEndpoint(), bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回错误：%s", resp.Status)
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "data: [DONE]") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullResponse.WriteString(content)
			if callback != nil {
				callback(content)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取流式响应失败：%w", err)
	}

	return strings.TrimSpace(fullResponse.String()), nil
}

// Close 释放资源。
func (c *OpenAIClient) Close() error {
	return nil
}
