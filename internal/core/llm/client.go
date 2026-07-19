// internal/core/llm/client.go
package llm

import "context"

// Client 是 LLM 的通用客户端接口。
// 支持流式和非流式两种调用方式。
type Client interface {
	// Generate 生成响应（非流式）
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateStream 生成响应（流式），通过回调逐块返回
	GenerateStream(ctx context.Context, prompt string, callback func(chunk string)) (string, error)

	// Close 释放客户端资源（如有需要）
	Close() error
}
