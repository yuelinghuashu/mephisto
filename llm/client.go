// ============================================================
// client.go - LLM 客户端
// 职责：
// 1. 封装与 LLM API（DeepSeek/OpenAI）的通信
// 2. 支持配置 API Key、模型、温度等参数
// 3. 支持普通请求和流式请求
// 4. 所有请求支持 Context 取消
// ============================================================

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

// Config LLM 客户端配置
type Config struct {
	APIKey      string   // API 密钥
	BaseURL     string   // API 基础 URL，默认 https://api.deepseek.com/v1
	Model       string   // 模型名称，默认 deepseek-chat
	MaxTokens   int      // 最大生成 token 数，默认 4096
	Temperature *float64 // 温度值（0.0-2.0），nil 表示使用默认值 0.7
}

// Client LLM 客户端
type Client struct {
	config Config
	http   *http.Client
}

// NewClient 创建 LLM 客户端
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: 60 * time.Second},
	}
}

// Message 消息结构
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 请求结构（兼容 OpenAI）
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse 非流式响应结构
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// Chat 发送普通（非流式）对话请求，支持 Context 取消
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	reqBody := ChatRequest{
		Model:     c.config.Model,
		Messages:  messages,
		MaxTokens: c.config.MaxTokens,
		Stream:    false,
	}
	if c.config.Temperature != nil {
		reqBody.Temperature = *c.config.Temperature
	} else {
		reqBody.Temperature = 0.7
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.config.BaseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("API 错误: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("无响应内容")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// ChatStream 发送流式请求，通过回调函数逐块传递内容
// onChunk 可能被多次调用，每次传递增量文本（未包含换行）
// 返回 error 表示请求失败或解析错误
func (c *Client) ChatStream(ctx context.Context, messages []Message, onChunk func(string)) error {
	reqBody := ChatRequest{
		Model:     c.config.Model,
		Messages:  messages,
		MaxTokens: c.config.MaxTokens,
		Stream:    true,
	}
	if c.config.Temperature != nil {
		reqBody.Temperature = *c.config.Temperature
	} else {
		reqBody.Temperature = 0.7
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.config.BaseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 尝试读取错误信息
		var errMsg bytes.Buffer
		bufio.NewReader(resp.Body).WriteTo(&errMsg)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(errMsg.String()))
	}

	scanner := bufio.NewScanner(resp.Body)
	// 增加缓冲区以应对长行
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	// 解析 SSE 数据
	var accumulated strings.Builder
	for scanner.Scan() {
		// 对行进行 TrimSpace，处理可能的 \r 等空白字符
		line := strings.TrimSpace(scanner.Text())

		// 忽略空行和注释
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		// 解析 JSON 片段
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 忽略无法解析的行（可能不完整）
			continue
		}
		if chunk.Error != nil {
			return fmt.Errorf("流式错误: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				onChunk(content)
				accumulated.WriteString(content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流式数据失败: %w", err)
	}
	return nil
}
