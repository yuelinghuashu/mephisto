// cmd/mephisto/commands.go
//
// 子命令执行逻辑
package main

import (
	"fmt"

	"mephisto/internal/core/engine"
	"mephisto/internal/core/llm"
	"mephisto/internal/core/parser"
)

// runParse 执行解析命令。
func runParse(cfg *AppConfig) error {
	contract, err := parser.ParseFile(cfg.File)
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
func runInteractive(cfg *AppConfig) error {
	contract, err := parser.ParseFile(cfg.File)
	if err != nil {
		return err
	}

	llmClient, err := createLLMClient(cfg)
	if err != nil {
		return err
	}

	// ---- 加载自定义约束（如果有） ----
	constraints, err := llm.LoadConstraints(cfg.ConstraintsFile)
	if err != nil {
		return err
	}

	// ---- 创建引擎（记忆管理由引擎内部自动处理） ----
	eng := engine.New(
		contract,
		engine.WithLLM(llmClient),
		engine.WithDebug(cfg.Debug),
		engine.WithMemoryConfig(engine.DefaultMemoryConfig),
		engine.WithConstraints(constraints),
	)

	session := NewSession(eng, cfg.File, cfg.Branch, cfg.Reset)
	return session.Start()
}