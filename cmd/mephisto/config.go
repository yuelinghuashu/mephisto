// cmd/mephisto/config.go
//
// 配置加载：环境变量 + 命令行参数。
// 所有命令行参数和环境变量都在这里统一管理。
package main

import (
	"flag"
	"os"
	"strconv"
)

const (
	CmdRun     = "run"
	CmdParse   = "parse"
	CmdCheck   = "check"
	CmdVersion = "version"
	CmdHelp    = "help"
)

// AppConfig 是 Mephisto 的全部配置。
// 所有命令行参数和环境变量都在这里统一管理。
type AppConfig struct {
	// ---- 命令与文件 ----
	Command string // 子命令: parse / run / version / help
	File    string // .meph 文件路径

	// ---- 运行时行为 ----
	Branch string // 分支名（多分支故事线）
	Reset  bool   // 忽略子版存档
	Debug  bool   // 启用规则调试
	Quiet  bool   // 静默模式
	Output string // 输出文件路径（parse 命令）

	// ---- LLM 配置 ----
	Client    string // deepseek / openai / ollama
	Model     string // 模型名称
	APIKey    string // API 密钥
	BaseURL   string // API 基础 URL
	MaxTokens int    // 最大生成 Token 数
}

// LoadConfig 加载配置：环境变量 + 命令行参数。
// 优先级：命令行参数 > 环境变量 > 默认值。
func LoadConfig() *AppConfig {
	cfg := &AppConfig{}

	// ---- 1. 定义所有 flags（绑定到 cfg 字段） ----
	// 注意：flag 的默认值从环境变量读取
	flag.StringVar(&cfg.File, "file", "", "契约文件路径")
	flag.StringVar(&cfg.Branch, "branch", getEnv("MEPHISTO_BRANCH", ""), "分支名")
	flag.BoolVar(&cfg.Reset, "reset", getEnvBool("MEPHISTO_RESET"), "忽略子版存档")
	flag.BoolVar(&cfg.Debug, "debug", getEnvBool("MEPHISTO_DEBUG"), "启用规则调试")
	flag.BoolVar(&cfg.Quiet, "q", getEnvBool("MEPHISTO_QUIET"), "静默模式")
	flag.StringVar(&cfg.Output, "o", "", "输出文件路径")

	// LLM 配置
	flag.StringVar(&cfg.Client, "client", getEnv("MEPHISTO_CLIENT", "deepseek"), "LLM 客户端: deepseek/openai/ollama")
	flag.StringVar(&cfg.Model, "model", getEnv("MEPHISTO_MODEL", "deepseek-v4-flash"), "模型名称")
	flag.StringVar(&cfg.Model, "m", getEnv("MEPHISTO_MODEL", "deepseek-v4-flash"), "模型名称（缩写）")
	flag.StringVar(&cfg.APIKey, "api-key", getEnv("OPENAI_API_KEY", ""), "API 密钥")
	flag.StringVar(&cfg.BaseURL, "base-url", getEnv("OPENAI_BASE_URL", "https://api.deepseek.com/v1"), "API 基础 URL")
	flag.IntVar(&cfg.MaxTokens, "max-tokens", getEnvInt("MEPHISTO_MAX_TOKENS", 4096), "最大生成 Token 数")

	// ---- 2. 解析命令行 ----
	flag.Parse()

	// ---- 3. 处理子命令（位置参数） ----
	args := flag.Args()
	if len(args) == 0 {
		cfg.Command = "help"
		return cfg
	}

	cfg.Command = args[0]
	switch cfg.Command {
	case "parse", "run", "check":
		if len(args) >= 2 {
			cfg.File = args[1]
		}
	case "version":
		// version 不需要文件
	case "help":
		// help 不需要文件
	default:
		// 隐式 parse 模式：直接传入文件名
		cfg.Command = "parse"
		cfg.File = args[0]
	}

	return cfg
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
