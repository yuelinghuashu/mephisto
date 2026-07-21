// cmd/mephisto/commands.go
//
// 子命令执行逻辑
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"mephisto/internal/core/engine"
	"mephisto/internal/core/parser"
	"mephisto/internal/core/validator"
	"mephisto/internal/domain"
)

// CheckResult 是 check 命令的输出结构。
type CheckResult struct {
	Valid   bool               `json:"valid"`
	Errors  []CheckError       `json:"errors"`
	Outline []CheckOutlineItem `json:"outline"`
}

// CheckError 表示一个诊断错误。
type CheckError struct {
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" 或 "warning"
}

// CheckOutlineItem 表示大纲中的一个条目。
type CheckOutlineItem struct {
	Name string `json:"name"`
	Line int    `json:"line"`
	Kind string `json:"kind"` // "block", "rule", "state"
}

// runParse 执行解析命令。
//
// 流程：
//  1. 解析 .meph 文件
//  2. 验证契约
//  3. 序列化为 JSON
//  4. 写入输出
func runParse(cfg *AppConfig) error {
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
//  3. 创建引擎（注入 LLM 客户端、调试模式）
//  4. 启动交互会话（传递 reset 标志）
//
// 注意：记忆管理由引擎内部自动处理，无需外部创建 MemoryManager。
func runInteractive(cfg *AppConfig) error {
	contract, err := loadContract(cfg.File)
	if err != nil {
		return err
	}

	llmClient, err := createLLMClient(cfg)
	if err != nil {
		return err
	}

	// ---- 创建引擎（记忆管理由引擎内部自动处理） ----
	eng := engine.New(
		contract,
		engine.WithLLM(llmClient),
		engine.WithDebug(cfg.Debug),
		engine.WithMemoryConfig(engine.DefaultMemoryConfig),
	)

	session := NewSession(eng, cfg.File, cfg.Branch, cfg.Reset)
	return session.Start()
}

// runCheck 执行检查命令（供 VSCode 插件调用）。
func runCheck(cfg *AppConfig) error {
	// 1. 解析契约
	contract, err := parser.ParseFile(cfg.File)
	if err != nil {
		// 解析失败时，仍然尝试提取部分信息
		return outputCheckError(err)
	}

	// 2. 验证契约
	errs := validator.Validate(contract)

	// 3. 构建输出
	checkResult := CheckResult{
		Valid:   len(errs) == 0,
		Errors:  convertErrors(errs),
		Outline: buildOutline(contract),
	}

	// 4. 输出 JSON
	data, err := json.MarshalIndent(checkResult, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// convertErrors 将验证错误转换为 CheckError 列表。
func convertErrors(errs []validator.ValidationError) []CheckError {
	result := make([]CheckError, 0, len(errs))
	for _, e := range errs {
		line := extractLineFromError(e)
		result = append(result, CheckError{
			Line:     line,
			Message:  e.Error(),
			Severity: "error",
		})
	}
	return result
}

// extractLineFromError 从验证错误中提取行号（简化实现）。
func extractLineFromError(e validator.ValidationError) int {
	msg := e.Error()
	if idx := strings.Index(msg, "第 "); idx != -1 {
		rest := msg[idx+3:]
		if spaceIdx := strings.Index(rest, " "); spaceIdx != -1 {
			var line int
			if _, err := fmt.Sscanf(rest[:spaceIdx], "%d", &line); err == nil {
				return line
			}
		}
	}
	return 0
}

// buildOutline 从契约构建大纲。
func buildOutline(contract *domain.Contract) []CheckOutlineItem {
	var items []CheckOutlineItem

	line := 1

	if contract.RoleName != "" {
		items = append(items, CheckOutlineItem{
			Name: "角色名",
			Line: line,
			Kind: "block",
		})
		line += 2
	}

	if len(contract.Anchor) > 0 {
		items = append(items, CheckOutlineItem{
			Name: "锚点",
			Line: line,
			Kind: "block",
		})
		line += len(contract.Anchor) + 2
	}

	if contract.Worldview != "" {
		items = append(items, CheckOutlineItem{
			Name: "世界观",
			Line: line,
			Kind: "block",
		})
		line += strings.Count(contract.Worldview, "\n") + 2
	}

	if contract.Background != "" {
		items = append(items, CheckOutlineItem{
			Name: "角色背景",
			Line: line,
			Kind: "block",
		})
		line += strings.Count(contract.Background, "\n") + 2
	}

	if contract.Opening != "" {
		items = append(items, CheckOutlineItem{
			Name: "开局场景",
			Line: line,
			Kind: "block",
		})
		line += strings.Count(contract.Opening, "\n") + 2
	}

	if len(contract.State) > 0 {
		items = append(items, CheckOutlineItem{
			Name: "状态",
			Line: line,
			Kind: "block",
		})
		line += len(contract.State) + 2
	}

	if len(contract.Rules) > 0 {
		items = append(items, CheckOutlineItem{
			Name: fmt.Sprintf("规则 (%d 条)", len(contract.Rules)),
			Line: line,
			Kind: "block",
		})
		for _, rule := range contract.Rules {
			items = append(items, CheckOutlineItem{
				Name: rule.Name,
				Line: rule.Line,
				Kind: "rule",
			})
		}
		line += len(contract.Rules) + 2
	}

	if len(contract.Validation) > 0 {
		items = append(items, CheckOutlineItem{
			Name: "校验",
			Line: line,
			Kind: "block",
		})
		line += len(contract.Validation) + 2
	}

	return items
}

// outputCheckError 当解析失败时输出错误 JSON。
func outputCheckError(err error) error {
	result := CheckResult{
		Valid: false,
		Errors: []CheckError{
			{Line: 0, Message: err.Error(), Severity: "error"},
		},
		Outline: []CheckOutlineItem{},
	}
	data, jsonErr := json.MarshalIndent(result, "", "  ")
	if jsonErr != nil {
		return jsonErr
	}
	fmt.Println(string(data))
	return nil
}
