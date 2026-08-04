// cmd/mephisto/config.go
//
// 配置加载：子命令解析 + 独立 FlagSet。
// 每个子命令使用独立的 flag.FlagSet，互不干扰。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// errShowHelp 请求显示帮助信息的哨兵错误。
// parseFlexible 遇到子命令的 -h/--help 时返回，由 LoadConfig 捕获并转换为帮助命令。
var errShowHelp = errors.New("show help")

const (
	CmdRun     = "run"
	CmdParse   = "parse"
	CmdInit    = "init"
	CmdVersion = "version"
	CmdHelp    = "help"
)

// AppConfig 是 Mephisto 的全部配置。
type AppConfig struct {
	// ---- 命令与文件 ----
	Command string // 子命令: parse / run / init / version / help
	File    string // .meph 文件路径

	// ---- 运行时行为 ----
	InitTemplate string // 初始化模板名（init 命令）
	Branch       string // 分支名（多分支故事线）
	Reset        bool   // 忽略子版存档
	Debug        bool   // 启用规则调试
	Quiet        bool   // 静默模式
	Output       string // 输出文件路径（parse 命令）

	// ---- 约束配置 ----
	ConstraintsFile string // 自定义约束文件路径（空=使用默认）

	// ---- LLM 配置 ----
	Client    string // openai / ollama（deepseek 为兼容别名）
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
func LoadConfig() (*AppConfig, error) {
	args := os.Args[1:] // 跳过程序名

	if len(args) == 0 {
		return &AppConfig{Command: CmdHelp}, nil
	}

	first := args[0]

	// ---- 版本 / 帮助（无文件参数） ----
	if first == "version" || first == "-v" || first == "--version" {
		return &AppConfig{Command: CmdVersion}, nil
	}
	if first == "help" || first == "-h" || first == "--help" {
		return &AppConfig{Command: CmdHelp}, nil
	}

	// ---- 识别子命令 ----
	var cfg *AppConfig
	var err error
	switch first {
	case CmdParse:
		cfg, err = parseParseArgs(args[1:])
	case CmdInit:
		cfg, err = parseInitArgs(args[1:])
	case CmdRun:
		cfg, err = parseRunArgs(args[1:])
	default:
		// 隐式 parse 模式：第一个参数是文件路径
		cfg = &AppConfig{
			Command: CmdParse,
			File:    first,
		}
	}

	// 子命令请求帮助（run -h / parse --help）→ 显示帮助
	if errors.Is(err, errShowHelp) {
		return &AppConfig{Command: CmdHelp}, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// ============================================================
// 子命令解析
// ============================================================

// parseParseArgs 解析 parse 子命令的参数。
//
// 用法：mephisto parse [选项] <文件>
//   mephisto parse -o out.json data/sample.meph
//   mephisto parse data/sample.meph -o out.json
func parseParseArgs(args []string) (*AppConfig, error) {
	cfg := &AppConfig{Command: CmdParse}

	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	addStringPair(fs, "o", "output", "", "输出到文件（默认输出到 stdout）", &cfg.Output)
	addBoolPair(fs, "q", "quiet", getEnvBool("MEPHISTO_QUIET"), "静默模式，只输出错误", &cfg.Quiet)
	fs.SetOutput(nil)

	remaining, err := parseFlexible(fs, args)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 {
		cfg.File = remaining[len(remaining)-1] // 取最后一个位置参数
	}

	return cfg, nil
}

// parseInitArgs 解析 init 子命令的参数。
//
// 用法：mephisto init [模板名]
//   mephisto init          → 生成 faust.meph（默认）
//   mephisto init dantes   → 生成 dantes.meph
func parseInitArgs(args []string) (*AppConfig, error) {
	cfg := &AppConfig{Command: CmdInit}

	// 支持 mephisto init -h / --help
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return &AppConfig{Command: CmdHelp}, nil
		}
	}

	// init 命令不支持任何选项
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("init 命令不支持选项：%s（用法：mephisto init [模板名]）", arg)
		}
	}

	if len(args) > 0 {
		cfg.InitTemplate = args[0]
	}
	if len(args) > 1 {
		return nil, fmt.Errorf("init 命令只接受一个模板名参数（支持：faust、dantes）")
	}

	return cfg, nil
}

// parseRunArgs 解析 run 子命令的参数。
//
// 用法：mephisto run [选项] <文件>
//   mephisto run --branch dark --reset data/sample.meph
//   mephisto run data/sample.meph --client ollama
func parseRunArgs(args []string) (*AppConfig, error) {
	cfg := &AppConfig{Command: CmdRun}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	addStringPair(fs, "b", "branch", getEnv("MEPHISTO_BRANCH", ""), "分支名", &cfg.Branch)
	addBoolPair(fs, "r", "reset", getEnvBool("MEPHISTO_RESET"), "忽略子版存档，从母版重新开始", &cfg.Reset)
	addBoolPair(fs, "d", "debug", getEnvBool("MEPHISTO_DEBUG"), "启用规则调试", &cfg.Debug)
	addBoolPair(fs, "q", "quiet", getEnvBool("MEPHISTO_QUIET"), "静默模式", &cfg.Quiet)

	// 约束配置
	fs.StringVar(&cfg.ConstraintsFile, "constraints", "", "自定义约束文件（默认使用内置约束）")

	// LLM 配置
	// 客户端类型：openai（OpenAI 兼容，含 DeepSeek）+ ollama，与 Flutter 版对齐。
	// "deepseek" 保留为 openai 兼容分支的历史别名（默认值兼容老配置）。
	addStringPair(fs, "c", "client", getEnv("MEPHISTO_CLIENT", "openai"), "LLM 客户端: openai/ollama", &cfg.Client)
	addStringPair(fs, "m", "model", getEnv("MEPHISTO_MODEL", "deepseek-v4-flash"), "模型名称", &cfg.Model)
	fs.StringVar(&cfg.APIKey, "api-key", getEnv("OPENAI_API_KEY", ""), "API 密钥")
	fs.StringVar(&cfg.BaseURL, "base-url", getEnv("OPENAI_BASE_URL", "https://api.deepseek.com/v1"), "API 基础 URL")
	fs.IntVar(&cfg.MaxTokens, "max-tokens", getEnvInt("MEPHISTO_MAX_TOKENS", 4096), "最大生成 Token 数")
	fs.SetOutput(nil)

	remaining, err := parseFlexible(fs, args)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 {
		cfg.File = remaining[len(remaining)-1] // 取最后一个位置参数
	}

	return cfg, nil
}

// addStringPair 注册短/长两个形式的字符串选项。
func addStringPair(fs *flag.FlagSet, short, long, def, usage string, target *string) {
	fs.StringVar(target, short, def, usage)
	fs.StringVar(target, long, def, usage)
}

// addBoolPair 注册短/长两个形式的布尔选项。
func addBoolPair(fs *flag.FlagSet, short, long string, def bool, usage string, target *bool) {
	fs.BoolVar(target, short, def, usage)
	fs.BoolVar(target, long, def, usage)
}

// parseFlexible 灵活解析参数：支持选项出现在位置参数前后任意位置。
//
// 策略：通过 fs.Lookup 识别已注册的 flag，正确区分布尔/非布尔 flag 的值消费。
// 已知 flag 及其值被收集到 flagArgs 中，其余作为位置参数返回。
// 未知的 flag（未注册的选项）直接返回错误，避免拼写错误被静默忽略。
func parseFlexible(fs *flag.FlagSet, args []string) ([]string, error) {
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

		// 以 - 开头：检查是否已注册的 flag。
		// 支持 -flag=value 和 --flag=value 两种内联值形式。
		flagName := strings.TrimLeft(arg, "-")
		if eq := strings.Index(flagName, "="); eq != -1 {
			flagName = flagName[:eq]
		}

		// 子命令级帮助（run -h / parse --help）
		if flagName == "h" || flagName == "help" {
			return nil, errShowHelp
		}

		// 废弃单横线长选项：单横线仅允许单字母短选项（如 -b），
		// 长选项必须使用双横线（如 --branch）。
		// 示例：-branch / -reset / -client 等旧写法全部拒绝并提示迁移。
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(flagName) > 1 {
			return nil, fmt.Errorf("长选项请使用 --%s（单横线 - 仅用于短选项，如 -b）", flagName)
		}

		fl := fs.Lookup(flagName)
		if fl == nil {
			// 未注册的 flag → 报错，而不是静默当作位置参数
			return nil, fmt.Errorf("未知的选项：%s（使用 'mephisto help' 查看可用选项）", arg)
		}

		// 已注册的 flag：收集 flag 本身
		flagArgs = append(flagArgs, arg)

		// 布尔 flag 或已带内联值（-flag=value）的 flag 不消费下一个参数
		hasInlineValue := strings.Contains(arg, "=")
		if isBoolFlag(fl) || hasInlineValue {
			continue
		}

		// 非布尔 flag 需要消费下一个参数作为值
		if i+1 < len(args) {
			next := args[i+1]
			// 值不能以 - 开头（否则是另一个 flag）
			if len(next) > 0 && next[0] != '-' {
				flagArgs = append(flagArgs, next)
				i++
				continue
			}
		}
		return nil, fmt.Errorf("选项 %s 缺少参数值", arg)
	}

	// 用 FlagSet 解析收集到的 flag 参数
	if err := fs.Parse(flagArgs); err != nil {
		return nil, fmt.Errorf("参数解析失败：%v", err)
	}

	return positional, nil
}

// isBoolFlag 判断 flag 是否为布尔类型。
//
// 标准库的布尔 flag（flag.Bool / flag.BoolVar）实现了带 IsBoolFlag() 方法的内部接口，
// 通过类型断言判断比比较默认值字符串（"false"）更可靠。
func isBoolFlag(fl *flag.Flag) bool {
	bf, ok := fl.Value.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
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