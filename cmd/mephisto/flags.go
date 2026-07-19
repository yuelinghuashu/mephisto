// cmd/mephisto/flags.go
//
// 命令行参数定义、解析和数据结构
package main

import (
	"flag"
	"fmt"
	"os"
)

// Command 类型常量
const (
	CmdRun     = "run"
	CmdParse   = "parse"
	CmdVersion = "version"
	CmdHelp    = "help"
)

// Config 存储解析后的命令行配置
type Config struct {
	Command string // 子命令：parse / run / version / help
	File    string // 要解析的文件路径
	Output  string // 输出文件路径
	Quiet   bool   // 静默模式
	Reset   bool   // 忽略子版存档，从母版重新开始
	Debug   bool   // 启用规则调试模式
	Model   string // LLM 模型名称
	Client  string // LLM 客户端类型: deepseek, openai, ollama
	APIKey  string // API 密钥
	BaseURL string // API 基础 URL
	Branch  string // 分支名
}

// parseFlags 解析命令行参数，支持混合模式：
//   - mephisto file.meph        → 隐式 parse
//   - mephisto parse file.meph  → 显式 parse
//   - mephisto run file.meph    → 交互式对话
//   - mephisto version
//   - mephisto help
func parseFlags() Config {
	var (
		output  = flag.String("o", "", "输出文件路径")
		quiet   = flag.Bool("q", false, "静默模式")
		help    = flag.Bool("h", false, "显示帮助")
		version = flag.Bool("version", false, "显示版本")
		branch  = flag.String("branch", "", "分支名（用于多分支故事线）")
		reset   = flag.Bool("reset", false, "忽略子版存档，从母版重新开始")
		debug   = flag.Bool("debug", false, "启用规则调试模式（显示规则匹配过程）")

		// 统一从环境变量读取默认值
		model   = flag.String("model", getEnv("MEPHISTO_MODEL", "deepseek-v4-flash"), "LLM 模型名称")
		client  = flag.String("client", getEnv("MEPHISTO_CLIENT", "deepseek"), "LLM 客户端: deepseek, openai, ollama")
		apiKey  = flag.String("api-key", getEnv("OPENAI_API_KEY", ""), "API 密钥（优先使用环境变量）")
		baseURL = flag.String("base-url", getEnv("OPENAI_BASE_URL", ""), "API 基础 URL")
	)
	// 添加 -m 作为 -model 的缩写，与 -model 绑定同一变量，默认值一致
	flag.StringVar(model, "m", getEnv("MEPHISTO_MODEL", "deepseek-v4-flash"), "LLM 模型名称（缩写）")

	flag.Usage = printHelp
	flag.Parse()

	if *help {
		return Config{Command: CmdHelp}
	}
	if *version {
		return Config{Command: CmdVersion}
	}

	args := flag.Args()
	if len(args) == 0 {
		return Config{Command: CmdHelp}
	}

	first := args[0]

	switch first {
	case "parse":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "❌ 错误: parse 需要指定文件路径")
			os.Exit(1)
		}
		return Config{
			Command: CmdParse,
			File:    args[1],
			Output:  *output,
			Quiet:   *quiet,
			Model:   *model,
			Branch:  *branch,
		}
	case "run":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "❌ 错误: run 需要指定文件路径")
			os.Exit(1)
		}
		return Config{
			Command: CmdRun,
			File:    args[1],
			Output:  *output,
			Quiet:   *quiet,
			Reset:   *reset,
			Debug:   *debug,
			Model:   *model,
			Client:  *client,
			APIKey:  *apiKey,
			BaseURL: *baseURL,
			Branch:  *branch,
		}
	case "version":
		return Config{Command: CmdVersion}
	case "help":
		return Config{Command: CmdHelp}
	default:
		// 直接文件名模式（默认执行 parse）
		return Config{
			Command: CmdParse,
			File:    first,
			Output:  *output,
			Quiet:   *quiet,
			Model:   *model,
			Branch:  *branch,
		}
	}
}

// getEnv 读取环境变量，如果不存在则返回默认值
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
