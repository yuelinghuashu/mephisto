package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mephisto/parser"
)

func main() {
	// ----- 1. 参数检查 -----
	if len(os.Args) < 2 {
		fmt.Println("用法: go run main.go <文件.meph>")
		fmt.Println("示例: go run main.go sample.meph")
		os.Exit(1)
	}
	filename := os.Args[1]

	// ----- 2. 扩展名验证 -----
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".meph" && ext != ".mephisto" {
		fmt.Printf("❌ 错误: 不支持的文件类型 %q，请使用 .meph 或 .mephisto 文件\n", ext)
		os.Exit(1)
	}

	// ----- 3. 显示正在读取的文件 -----
	fmt.Printf("📂 正在读取: %s\n", filename)

	// ----- 4. 调用解析器 -----
	pf, err := parser.ParseFile(filename)
	if err != nil {
		fmt.Printf("❌ 解析失败: %v\n", err)
		os.Exit(1)
	}

	// ----- 5. 打印结果 -----
	fmt.Println("\n📋 解析结果")
	fmt.Println(strings.Repeat("=", 60))

	for i, block := range pf.Blocks {
		// 区块标题（带序号和行号）
		fmt.Printf("\n【%s】 (行 %d)\n", block.Name, block.Line)

		if len(block.Entries) == 0 {
			fmt.Println("  (空)")
		} else {
			for _, entry := range block.Entries {
				switch entry.Type {
				case "list":
					// 列表项：- 键: 值
					fmt.Printf("  - %s: %s\n", entry.Key, entry.Value)
				case "rule":
					// 规则：[名称] if 条件 -> 动作
					fmt.Printf("  [%s] if %s\n", entry.Key, entry.Value)
				case "text":
					// 文本内容：保留原样，缩进两个空格
					if entry.Value == "" {
						fmt.Println("  ") // 空行（仅当文本块保留空行时）
					} else {
						for line := range strings.SplitSeq(entry.Value, "\n") {
							fmt.Printf("  %s\n", line)
						}
					}
				}
			}
		}

		// 区块之间加分隔线（最后一个区块不加）
		if i < len(pf.Blocks)-1 {
			fmt.Println(strings.Repeat("-", 60))
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("✅ 解析完成: %d 个区块\n", len(pf.Blocks))

	if len(pf.References) > 0 {
		fmt.Printf("🔗 外部引用: %v\n", pf.References)
	}
}
