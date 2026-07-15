package app

import (
	"fmt"
	"strings"

	"mephisto/engine"
	"mephisto/parser"
)

// Run 是应用的入口，编排整个流程
func Run(filename string) error {
	fmt.Printf("📂 正在读取: %s\n", filename)
	fmt.Println(strings.Repeat("━", 50))

	// 1. 解析文件
	pf, err := parser.ParseFile(filename)
	if err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}

	// 2. 构建上下文
	ctx, openingText, err := BuildContext(pf)
	if err != nil {
		return fmt.Errorf("构建上下文失败: %w", err)
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

	// 定义打印顺序
	orderedKeys := []string{
		"角色名",
		"位置",
		"情绪",
		"堕落指数",
		"生命值",
		"世界观",
		"角色背景",
	}

	// 多行文本区块
	multilineKeys := map[string]bool{
		"世界观":  true,
		"角色背景": true,
	}

	for _, k := range orderedKeys {
		val, ok := ctx[k]
		if !ok {
			continue
		}
		strVal := fmt.Sprintf("%v", val)

		if multilineKeys[k] {
			fmt.Printf("  %s:\n", k)
			for line := range strings.SplitSeq(strVal, "\n") {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("    %s\n", line)
				}
			}
		} else {
			fmt.Printf("  %s: %v\n", k, val)
		}
	}
}

// printOpening 打印开局场景
func printOpening(openingText strings.Builder) {
	if openingText.Len() > 0 {
		fmt.Println("\n📖 开局场景:")
		fmt.Println(strings.TrimSpace(openingText.String()))
	}
}
