// cmd/mephisto/config.go
//
// 配置加载：子命令解析 + 独立 FlagSet。
// 每个子命令使用独立的 flag.FlagSet，互不干扰。
package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

const (
	CmdRun     = "run"
	CmdParse   = "parse"
	CmdVersion = "version"
	CmdHelp    = "help"
)

// AppConfig 是 Mephisto 的全部配置。
type AppConfig struct {
	// ---- 命令与文件 ----
	Command string // 子命令: parse / run / check / version / help
	File    string // .meph 文件路径

	// ---- 运行时行为 ----
	Branch string // 分支名（多分支故事线）
	Reset  bool   // 忽略子版存档
	Debug  bool   // 启用规则调试
	Quiet  bool   // 静默模式
	Output string // 输出文件路径（parse 命令）

	// ---- 约束配置 ----
	ConstraintsFile string // 自定义约束文件路径（空=使用默认）

	// ---- LLM 配置 ----
	Client    string // deepseek / openai / ollama
	Model     string // 模型名称
	APIKey    string // API 密钥
	BaseURL   string // API 基础 URL
	MaxTokens int    // 最大生成 Token 数
}

// LoadConfig 加载配置。
//
// 所有子命令的格式统一为：
//   mephisto <子命令> [选项] <文件>
//
// 简写模式：
//   mephisto <文件>                     (等价于 parse <文件>)
func LoadConfig() *AppConfig {
	args := os.Args[1:] // 跳过程序名

	if len(args) == 0 {
		return &AppConfig{Command: CmdHelp}
	}

	first := args[0]

	// ---- 版本 / 帮助（无文件参数） ----
	if first == "version" || first == "-v" || first == "--version" {
		return &AppConfig{Command: CmdVersion}
	}
	if first == "help" || first == "-h" || first == "--help" {
		return &AppConfig{Command: CmdHelp}
	}

	// ---- 识别子命令 ----
	switch first {
	case CmdParse:
		return parseParseArgs(args[1:])

	case CmdRun:
		return parseRunArgs(args[1:])

	default:
		// 隐式 parse 模式：第一个参数是文件路径
		return &AppConfig{
			Command: CmdParse,
			File:    first,
		}
	}
}

// ============================================================
// 子命令解析
// ============================================================

// 由于 Go 标准 flag 包在遇到位置参数后停止解析 flag，
// 而用户可能将选项放在文件前后任意位置，
// 所以这里先扫描所有参数提取 flag 值，再取最后一个非 flag 参数作为文件路径。

// parseParseArgs 解析 parse 子命令的参数。
//
// 用法：mephisto parse [选项] <文件>
//   mephisto parse -o out.json data/sample.meph
//   mephisto parse data/sample.meph -o out.json
func parseParseArgs(args []string) *AppConfig {
	cfg := &AppConfig{Command: CmdParse}

	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.StringVar(&cfg.Output, "o", "", "输出到文件（默认输出到 stdout）")
	fs.BoolVar(&cfg.Quiet, "q", getEnvBool("MEPHISTO_QUIET"), "静默模式，只输出错误")
	// 忽略未知 flag 和错误输出
	fs.SetOutput(nil)

	remaining := parseFlexible(fs, args)
	if len(remaining) > 0 {
		cfg.File = remaining[len(remaining)-1] // 取最后一个位置参数
	}

	return cfg
}

// parseRunArgs 解析 run 子命令的参数。
//
// 用法：mephisto run [选项] <文件>
//   mephisto run -branch dark -reset data/sample.meph
//   mephisto run data/sample.meph -client ollama
func parseRunArgs(args []string) *AppConfig {
	cfg := &AppConfig{Command: CmdRun}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&cfg.Branch, "branch", getEnv("MEPHISTO_BRANCH", ""), "分支名")
	fs.BoolVar(&cfg.Reset, "reset", getEnvBool("MEPHISTO_RESET"), "忽略子版存档，从母版重新开始")
	fs.BoolVar(&cfg.Debug, "debug", getEnvBool("MEPHISTO_DEBUG"), "启用规则调试")
	fs.BoolVar(&cfg.Quiet, "q", getEnvBool("MEPHISTO_QUIET"), "静默模式")

	// 约束配置
	fs.StringVar(&cfg.ConstraintsFile, "constraints", "", "自定义约束文件（默认使用内置约束）")

	// LLM 配置
	fs.StringVar(&cfg.Client, "client", getEnv("MEPHISTO_CLIENT", "deepseek"), "LLM 客户端: deepseek/openai/ollama")
	fs.StringVar(&cfg.Model, "model", getEnv("MEPHISTO_MODEL", "deepseek-v4-flash"), "模型名称")
	fs.StringVar(&cfg.APIKey, "api-key", getEnv("OPENAI_API_KEY", ""), "API 密钥")
	fs.StringVar(&cfg.BaseURL, "base-url", getEnv("OPENAI_BASE_URL", "https://api.deepseek.com/v1"), "API 基础 URL")
	fs.IntVar(&cfg.MaxTokens, "max-tokens", getEnvInt("MEPHISTO_MAX_TOKENS", 4096), "最大生成 Token 数")
	fs.SetOutput(nil)

	remaining := parseFlexible(fs, args)
	if len(remaining) > 0 {
		cfg.File = remaining[len(remaining)-1] // 取最后一个位置参数
	}

	return cfg
}

// parseFlexible 灵活解析参数：支持选项出现在位置参数前后任意位置。
//
// 策略：通过 fs.Lookup 识别已注册的 flag，正确区分布尔/非布尔 flag 的值消费。
// 已知 flag 及其值被收集到 flagArgs 中，其余作为位置参数返回。
func parseFlexible(fs *flag.FlagSet, args []string) []string {
	var flagArgs []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			// -- 之后全部是位置参数
			positional = append(positional, args[i+1:]...)
			break
		}

		// 非 flag 参数 → 位置参数
		if len(arg) == 0 || arg[0] != '-' {
			positional = append(positional, arg)
			continue
		}

		// 以 - 开头：检查是否已注册的 flag
		flagName := strings.TrimLeft(arg, "-")
		fl := fs.Lookup(flagName)
		if fl == nil {
			// 未注册的 flag → 当作位置参数
			positional = append(positional, arg)
			continue
		}

		// 已注册的 flag：收集 flag 本身
		flagArgs = append(flagArgs, arg)

		// 判断是否为布尔 flag（默认值为 "false" 的即为布尔型）
		isBool := fl.Value.String() == "false"

		// 非布尔 flag 需要消费下一个参数作为值
		if !isBool && i+1 < len(args) {
			next := args[i+1]
			// 值不能以 - 开头（否则是另一个 flag）
			if len(next) > 0 && next[0] != '-' {
				flagArgs = append(flagArgs, next)
				i++
			}
		}
	}

	// 用 FlagSet 解析收集到的 flag 参数
	_ = fs.Parse(flagArgs)

	return positional
}

// ============================================================
// 辅助函数：从环境变量读取
// ============================================================

// getEnv 读取环境变量，不存在时返回默认值。
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvBool 读取布尔型环境变量。
// 支持的值：true, 1, yes（不区分大小写）
func getEnvBool(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1" || v == "yes"
}

// getEnvInt 读取整型环境变量。
// 如果环境变量不存在或解析失败，返回默认值。
func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return fallback
}