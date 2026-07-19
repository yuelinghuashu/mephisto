// cmd/mephisto/commands.go
//
// 子命令执行逻辑
package main

import (
	"fmt"

	"mephisto/internal/core/engine"
)

// runParse 执行解析命令。
//
// 流程：
//  1. 解析 .meph 文件
//  2. 验证契约
//  3. 序列化为 JSON
//  4. 写入输出
func runParse(cfg Config) error {
	contract, err := loadContract(cfg.File)
	if err != nil {
		return err
	}

	data, err := serialize(contract)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	return writeOutput(data, cfg.Output, cfg.Quiet)
}

// runInteractive 启动交互式对话模式。
//
// 流程：
//  1. 加载并验证契约
//  2. 创建 LLM 客户端
//  3. 创建记忆管理器
//  4. 创建引擎（注入 LLM 客户端、记忆管理器、调试模式）
//  5. 启动交互会话（传递 reset 标志）
func runInteractive(cfg Config) error {
	contract, err := loadContract(cfg.File)
	if err != nil {
		return err
	}

	llmClient, err := createLLMClient(cfg)
	if err != nil {
		return err
	}

	// ---- 创建记忆管理器 ----
	memoryConfig := engine.DefaultMemoryConfig
	memoryMgr := engine.NewMemoryManager(llmClient, memoryConfig)

	eng := engine.New(
		contract,
		engine.WithLLMClient(llmClient),
		engine.WithMemoryManager(memoryMgr),
		engine.WithDebug(cfg.Debug), // 传递调试模式标志
	)

	// 将 reset 标志传递给会话
	session := NewSession(eng, cfg.File, cfg.Branch, cfg.Reset)
	return session.Start()
}
