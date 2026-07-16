package app

import (
	"fmt"
	"slices"
	"strings"

	"mephisto/engine"
	"mephisto/parser"
)

type Config struct {
	LLM struct {
		APIKey  string
		Model   string
		BaseURL string
	}
	Debug bool
}

// Run 是应用的入口，编排整个流程
func Run(filename string) error {
	fmt.Printf("📂 正在读取: %s\n", filename)
	fmt.Println(strings.Repeat("━", 50))

	// 1. 解析文件
	pf, err := parser.ParseFile(filename)
	if err != nil {
		return fmt.Errorf("解析 %s 失败: %w", filename, err)
	}

	// 2. 构建上下文
	ctx, openingText, err := BuildContext(pf)
	if err != nil {
		return fmt.Errorf("构建上下文失败 (文件: %s): %w", filename, err)
	}

	// 3. 打印角色信息
	printRoleInfo(ctx)
	fmt.Println(strings.Repeat("━", 50))

	// 4. 打印开局场景
	printOpening(openingText)
	fmt.Println(strings.Repeat("━", 50))

	// 5. 初始化规则引擎
	eng := engine.NewRuleEngine(ctx)
	eng.SetDebug(false)

	for _, block := range pf.Blocks {
		if block.Name == "规则" {
			eng.AddRules(block.Entries)
		}
	}
	fmt.Printf("\n📋 已加载 %d 条规则\n", len(eng.Rules))
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println()

	// 6. 进入对话循环
	StartInteractive(eng, ctx)

	return nil
}

// printRoleInfo 打印角色加载信息（支持多行文本缩进）
func printRoleInfo(ctx engine.Context) {
	fmt.Println("\n📊 角色加载完成")

	// 1. 先显示系统固定的核心信息（按固定顺序）
	coreKeys := []string{
		"角色名",
		"世界观",
		"角色背景",
	}

	for _, k := range coreKeys {
		val, ok := ctx[k]
		if !ok {
			continue
		}
		printContextValue(k, val)
	}

	// 2. 再显示从 【状态】 区块读取的动态键值对（排除已显示的核心键）
	// 收集所有状态键
	var stateKeys []string
	for k := range ctx {
		// 排除核心键和系统内部键
		if k == "角色名" || k == "世界观" || k == "角色背景" || k == "开局场景" || k == "输入" {
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
