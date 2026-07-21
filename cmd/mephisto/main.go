// cmd/mephisto/main.go
//
// Mephisto CLI - 入口
// 职责：初始化、解析参数、调度命令、设置退出码
package main

import (
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// ---- 加载 .env 文件（如果存在） ----
	_ = godotenv.Load() // 忽略错误，文件不存在也不影响

	// 解析命令行参数（直接使用 LoadConfig）
	cfg := LoadConfig()

	// 根据命令类型执行
	switch cfg.Command {
	case CmdVersion:
		// 打印版本信息
		printVersion()
		os.Exit(0)

	case CmdHelp:
		// 打印帮助信息
		printHelp()
		os.Exit(0)

	case CmdParse:
		// 解析契约
		if err := runParse(cfg); err != nil {
			printError(err)
			os.Exit(1)
		}
		os.Exit(0)

	case CmdRun:
		// 运行交互式会话
		if err := runInteractive(cfg); err != nil {
			printError(err)
			os.Exit(1)
		}
		os.Exit(0)

	case CmdCheck:
		// 检查契约
		if err := runCheck(cfg); err != nil {
			printError(err)
			os.Exit(1)
		}
		os.Exit(0)

	default:
		printHelp()
		os.Exit(1)
	}
}
