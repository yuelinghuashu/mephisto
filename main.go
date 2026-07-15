package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mephisto/app"
)

func main() {
	// 参数检查
	if len(os.Args) < 2 {
		fmt.Println("用法: go run main.go <文件.meph>")
		fmt.Println("示例: go run main.go sample.meph")
		os.Exit(1)
	}
	filename := os.Args[1]

	// 扩展名验证
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".meph" && ext != ".mephisto" {
		fmt.Printf("❌ 错误: 不支持的文件类型 %q，请使用 .meph 或 .mephisto 文件\n", ext)
		os.Exit(1)
	}

	// 运行应用
	if err := app.Run(filename); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}
