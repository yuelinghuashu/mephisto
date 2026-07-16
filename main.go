// ============================================================
// main.go - 梅菲斯特程序入口（支持环境变量全覆盖）
// ============================================================

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mephisto/app"
	"mephisto/llm"

	"github.com/joho/godotenv"
)

var (
	apiKey      string
	model       string
	baseURL     string
	debug       bool
	quiet       bool
	maxTokens   int
	temperature float64
	// tempSet 记录温度是否被命令行或环境变量显式设置
	tempSet bool
)

func init() {
	flag.StringVar(&apiKey, "api-key", "", "LLM API Key（或环境变量 MEPHISTO_API_KEY，或 .env）")
	flag.StringVar(&model, "model", "", "LLM 模型名称（或环境变量 MEPHISTO_MODEL）")
	flag.StringVar(&baseURL, "base-url", "", "API Base URL（或环境变量 MEPHISTO_BASE_URL）")
	flag.BoolVar(&debug, "debug", false, "启用规则调试模式")
	flag.BoolVar(&quiet, "quiet", false, "安静模式：隐藏规则注入信息，保持叙事沉浸感")

	flag.IntVar(&maxTokens, "max-tokens", 0, "最大生成 Token 数（或环境变量 MEPHISTO_MAX_TOKENS）")
	flag.Float64Var(&temperature, "temperature", 0.0, "温度值 0.0~2.0（或环境变量 MEPHISTO_TEMPERATURE）")
}

func main() {
	flag.Parse()

	// 1. 加载 .env
	_ = godotenv.Load()

	// 2. 检查命令行是否显式设置了 temperature
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "temperature" {
			tempSet = true
		}
	})

	// 3. 读取环境变量填充未设置的 flag
	resolveConfigFromEnv()

	// 4. 解析文件名
	filename := resolveFilename()
	if filename == "" {
		printUsage()
		os.Exit(1)
	}

	if !isValidMephFile(filename) {
		fmt.Printf("❌ 错误: 不支持的文件类型 %q，请使用 .meph 或 .mephisto 文件\n", filepath.Ext(filename))
		os.Exit(1)
	}

	// 5. 构建 LLM 配置
	var tempPtr *float64
	if tempSet {
		tempPtr = &temperature
	}
	llmCfg := llm.Config{
		APIKey:      apiKey,
		Model:       model,
		BaseURL:     baseURL,
		MaxTokens:   maxTokens,
		Temperature: tempPtr,
	}

	cfg := app.Config{
		LLM:   llmCfg,
		Debug: debug,
		Quiet: quiet,
	}

	if err := app.Run(filename, cfg); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}

// resolveConfigFromEnv 用环境变量填充未设置的 flag
func resolveConfigFromEnv() {
	if apiKey == "" {
		apiKey = os.Getenv("MEPHISTO_API_KEY")
	}
	if model == "" {
		model = os.Getenv("MEPHISTO_MODEL")
	}
	if baseURL == "" {
		baseURL = os.Getenv("MEPHISTO_BASE_URL")
	}
	if maxTokens == 0 {
		if v := os.Getenv("MEPHISTO_MAX_TOKENS"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				maxTokens = i
			}
		}
	}
	// 温度处理：如果未通过命令行设置，尝试环境变量
	if !tempSet {
		if v := os.Getenv("MEPHISTO_TEMPERATURE"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				temperature = f
				tempSet = true
			}
		}
	}
	// 设置默认值
	if model == "" {
		model = "deepseek-chat"
	}
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}
}

// resolveFilename 解析文件名（跳过 flag 参数）
func resolveFilename() string {
	filename := flag.Arg(0)
	if filename == "" && len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if !strings.HasPrefix(arg, "-") {
				return arg
			}
		}
	}
	return filename
}

func isValidMephFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".meph" || ext == ".mephisto"
}

func printUsage() {
	fmt.Println("📖 梅菲斯特（Mephisto）— 长线叙事引擎")
	fmt.Println()
	fmt.Println("用法: go run main.go [选项] <文件.meph>")
	fmt.Println()
	fmt.Println("选项:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("环境变量:")
	fmt.Println("  MEPHISTO_API_KEY       API Key")
	fmt.Println("  MEPHISTO_MODEL         模型名称")
	fmt.Println("  MEPHISTO_BASE_URL      API Base URL")
	fmt.Println("  MEPHISTO_MAX_TOKENS    最大生成 Token 数")
	fmt.Println("  MEPHISTO_TEMPERATURE   温度值 (0.0~2.0)")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  go run main.go data/sample.meph")
	fmt.Println("  go run main.go -quiet data/sample.meph")
	fmt.Println("  go run main.go -debug data/sample.meph")
	fmt.Println("  MEPHISTO_API_KEY=sk-xxx go run main.go data/sample.meph")
	fmt.Println()
	fmt.Println("💡 提示: 在项目根目录创建 .env 文件可永久保存配置")
	fmt.Println("   echo 'MEPHISTO_API_KEY=sk-xxx' > .env")
}
