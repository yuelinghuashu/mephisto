// internal/core/llm/client.go
package llm

import "context"

// Client 是 LLM 的通用客户端接口。
// 只支持流式输出（与 Flutter 对齐）。
type Client interface {
	// GenerateStream 生成响应（流式），通过回调逐块返回
	GenerateStream(ctx context.Context, prompt string, callback func(chunk string)) (string, error)

	// Close 释放客户端资源（如有需要）
	Close() error
}
