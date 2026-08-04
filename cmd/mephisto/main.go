// cmd/mephisto/main.go
//
// Mephisto CLI - 入口
// 职责：初始化、解析参数、调度命令、设置退出码
package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// errUnknownCommand 未知命令错误（run 分发到 default 分支时返回）。
var errUnknownCommand = errors.New("未知的命令")

func main() {
	// ---- 加载 .env 文件（如果存在） ----
	_ = godotenv.Load() // 忽略错误，文件不存在也不影响

	// 解析命令行参数（直接使用 LoadConfig）
	cfg, err := LoadConfig()
	if err != nil {
		printError(err)
		os.Exit(1)
	}

	// 执行命令并设置退出码
	// run 返回 error 时退出码为 1
	if err := run(cfg); err != nil {
		printError(err)
		os.Exit(1)
	}
	os.Exit(0)
}

// run 根据配置执行对应的命令。
//
// 与 main() 分离的可测试性设计：
//   - 所有命令逻辑在此集中分发，不直接调用 os.Exit
//   - 返回 error 时由 main() 统一打印错误并设置退出码 1
//   - 便于将来为 CLI 命令编写单元测试（无需启动进程）
func run(cfg *AppConfig) error {
	switch cfg.Command {
	case CmdVersion:
		// 打印版本信息
		printVersion()
		return nil

	case CmdHelp:
		// 打印帮助信息
		printHelp()
		return nil

	case CmdParse:
		// 解析契约
		return runParse(cfg)

	case CmdInit:
		// 生成示例契约文件
		return runInit(cfg.InitTemplate)

	case CmdRun:
		// 运行交互式会话
		return runInteractive(cfg)

	default:
		printHelp()
		return errUnknownCommand
	}
}