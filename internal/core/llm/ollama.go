// internal/core/llm/ollama.go
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

// OllamaClient 实现了 Client 接口，用于调用本地 Ollama 服务。
type OllamaClient struct {
	config     OllamaConfig
	httpClient *http.Client
}

// OllamaConfig 配置 Ollama 客户端
type OllamaConfig struct {
	BaseURL string        // 默认 http://localhost:11434
	Model   string        // 模型名称，默认 llama3.2
	Timeout time.Duration // 超时时间，默认 60 秒

}

// NewOllamaClient 创建 Ollama 客户端
func NewOllamaClient(cfg OllamaConfig) *OllamaClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "llama3.2"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &OllamaClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// request 是通用请求函数，处理所有 Ollama API 调用。
// 参数：
//   - ctx: 上下文
//   - prompt: 用户提示
//   - stream: 是否启用流式
//   - handler: 处理响应的函数（同步模式解析 JSON，流式模式逐块读取）
func (c *OllamaClient) request(ctx context.Context, prompt string, stream bool, handler func(resp *http.Response) (string, error)) (string, error) {
	// 构建请求体
	reqBody := map[string]any{
		"model":  c.config.Model,
		"prompt": prompt,
		"stream": stream,
	}

	// 序列化请求体
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama 返回错误: %s", resp.Status)
	}

	// 处理响应
	return handler(resp)
}

// Generate 实现同步生成
func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	return c.request(ctx, prompt, false, func(resp *http.Response) (string, error) {
		var result struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("解析响应失败: %w", err)
		}
		return strings.TrimSpace(result.Response), nil
	})
}

// GenerateStream 流式生成
func (c *OllamaClient) GenerateStream(ctx context.Context, prompt string, callback func(chunk string)) (string, error) {
	return c.request(ctx, prompt, true, func(resp *http.Response) (string, error) {
		var fullResponse strings.Builder
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk struct {
				Response string `json:"response"`
				Done     bool   `json:"done"`
			}
			if err := json.Unmarshal(line, &chunk); err != nil {
				continue
			}

			if chunk.Response != "" {
				fullResponse.WriteString(chunk.Response)
				if callback != nil {
					callback(chunk.Response)
				}
			}

			if chunk.Done {
				break
			}
		}

		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("读取流式响应失败: %w", err)
		}

		return strings.TrimSpace(fullResponse.String()), nil
	})
}

// Close 释放资源（Ollama 客户端无需特殊清理）
func (c *OllamaClient) Close() error {
	return nil
}
