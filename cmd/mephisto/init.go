// cmd/mephisto/init.go
//
// mephisto init 命令：生成示例契约文件
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// runInit 生成示例契约文件。
//
// 用法：
//   mephisto init faust     → 生成 faust.meph
//   mephisto init dantes    → 生成 dantes.meph
//   mephisto init           → 生成 faust.meph（默认）
func runInit(template string) error {
	name := template
	if name == "" {
		name = "faust"
	}

	if name != "faust" && name != "dantes" {
		return fmt.Errorf("未知的模板：%s（支持：faust、dantes）", name)
	}

	// 读取 data/<模板名>.meph 模板
	data, err := os.ReadFile("data/" + name + ".meph")
	if err != nil {
		return fmt.Errorf("读取模板失败：%w", err)
	}

	path := filepath.Join(".", name+".meph")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("文件已存在：%s", path)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败：%w", err)
	}

	fmt.Printf("✅ 已生成契约文件：%s\n", path)
	fmt.Println()
	fmt.Println("运行方式：")
	fmt.Printf("  mephisto parse %s     # 验证契约格式\n", path)
	fmt.Printf("  mephisto run %s       # 启动交互式对话\n", path)
	return nil
}