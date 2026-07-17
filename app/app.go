// ============================================================
// app.go - 应用主流程编排
// 职责：
// 1. 串联整个应用流程：解析 → 构建上下文 → 加载规则 → 进入对话
// 2. 打印角色信息、开局场景
// 3. 初始化 LLM 客户端（可选）
// 4. 使用 parser 包中的常量，避免硬编码
// 5. 支持子版加载（状态、记忆、历史恢复）
// ============================================================

package app

import (
	"fmt"
	"slices"
	"strings"

	"mephisto/engine"
	"mephisto/llm"
	"mephisto/parser"
)

// Config 应用配置
type Config struct {
	LLM    llm.Config // LLM 配置（Temperature 为 nil 时使用默认值）
	Debug  bool       // 是否启用调试模式
	Quiet  bool       // 安静模式：隐藏规则注入信息
	Retain int        // 对话保留轮数
	Branch string     // 分支名
}

// Run 是应用的入口，编排整个流程
func Run(filename string, cfg Config) error {
	fmt.Printf("📂 正在读取: %s\n", filename)
	fmt.Println(strings.Repeat("━", 50))

	// 1. 解析文件
	pf, err := parser.ParseFile(filename)
	if err != nil {
		return fmt.Errorf("解析 %s 失败: %w", filename, err)
	}

	// 2. 构建上下文（先构建母版上下文）
	ctx, openingText, err := BuildContext(pf)
	if err != nil {
		return fmt.Errorf("构建上下文失败 (文件: %s): %w", filename, err)
	}

	// 3. 尝试加载子版（使用 cfg.Branch）
	//    注意：如果传入的已经是子版且匹配分支，LoadChild 会返回自身
	var startHistory *ConversationHistory
	childCtx, childHistory, err := LoadChild(filename, cfg.Branch)
	if err != nil {
		fmt.Printf("⚠️ 加载子版失败: %v\n", err)
	} else if childCtx != nil {
		// 合并子版数据：用子版的状态和记忆覆盖母版
		for key, val := range childCtx {
			if key == parser.KeyMemory || parser.StateExcludeKeys[key] {
				ctx[key] = val
			}
		}
		// 显示加载信息
		if cfg.Branch != "" {
			fmt.Printf("💾 已加载分支「%s」存档\n", cfg.Branch)
		} else {
			fmt.Printf("💾 已加载默认子版存档\n")
		}

		// 如果有子版历史，准备使用它
		if childHistory != nil && childHistory.GetSize() > 0 {
			startHistory = childHistory
			fmt.Printf("💾 已恢复对话历史 (%d 条)\n", startHistory.GetSize())
		} else {
			fmt.Printf("⚠️ 子版中历史为空\n")
		}
	}

	// 4. 打印角色信息
	printRoleInfo(ctx)
	fmt.Println(strings.Repeat("━", 50))

	// 5. 打印开局场景
	printOpening(openingText)
	fmt.Println(strings.Repeat("━", 50))

	// 6. 初始化规则引擎
	eng := engine.NewRuleEngine(ctx)
	eng.SetDebug(cfg.Debug)

	for _, block := range pf.Blocks {
		if block.Name == parser.KeyRules {
			eng.AddRules(block.Entries)
		}
	}
	fmt.Printf("\n📋 已加载 %d 条规则\n", len(eng.Rules))
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println()

	// 7. 初始化 LLM 客户端（如果配置了 API Key）
	var llmClient *llm.Client
	if cfg.LLM.APIKey != "" {
		llmClient = llm.NewClient(cfg.LLM)
		fmt.Println("🤖 LLM 已启用")
		fmt.Println(strings.Repeat("━", 50))
		fmt.Println()
	} else {
		fmt.Println("ℹ️ LLM 未启用（未配置 API Key）")
		fmt.Println(strings.Repeat("━", 50))
		fmt.Println()
	}

	// 8. 进入对话循环（传递 startHistory）
	StartInteractive(eng, ctx, llmClient, cfg.Quiet, cfg.Retain, filename, cfg.Branch, startHistory)

	return nil
}

// printRoleInfo 打印角色加载信息（支持多行文本缩进）
func printRoleInfo(ctx engine.Context) {
	fmt.Println("\n📊 角色加载完成")

	// 1. 先显示系统固定的核心信息（使用 parser.CoreKeys）
	for _, k := range parser.CoreKeys {
		val, ok := ctx[k]
		if !ok {
			continue
		}
		printContextValue(k, val)
	}

	// 2. 再显示从 【状态】 区块读取的动态键值对（排除已显示的核心键）
	var stateKeys []string
	for k := range ctx {
		// 使用 parser.StateExcludeKeys 排除系统键
		if parser.StateExcludeKeys[k] {
			continue
		}
		stateKeys = append(stateKeys, k)
	}

	// 按字母排序，让输出稳定
	slices.Sort(stateKeys)

	if len(stateKeys) > 0 {
		fmt.Println("\n  📌 状态:")
		for _, k := range stateKeys {
			printContextValue(k, ctx[k])
		}
	}
}

// printContextValue 打印单个上下文值
func printContextValue(key string, val any) {
	strVal := fmt.Sprintf("%v", val)

	// 从注册表判断是否为多行文本区块
	isMultiline := false
	if spec, ok := parser.BlockRegistry[key]; ok {
		isMultiline = spec.Type == parser.MultiLineText
	}

	if isMultiline {
		fmt.Printf("  %s:\n", key)
		for line := range strings.SplitSeq(strVal, "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("    %s\n", line)
			}
		}
	} else {
		fmt.Printf("  %s: %v\n", key, val)
	}
}

// printOpening 打印开局场景
func printOpening(openingText strings.Builder) {
	if openingText.Len() > 0 {
		fmt.Println("\n📖 开局场景:")
		fmt.Println(strings.TrimSpace(openingText.String()))
	}
}
