// cmd/mephisto/help.go
//
// 帮助信息和版本信息
package main

import (
	"errors"
	"fmt"
	"os"

	"mephisto/internal/shared"
)

// 版本信息（构建时注入）
var (
	Version   = "v1.0.1"
	BuildTime = "2026-07-21"
)

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("Mephisto %s (built %s)\n", Version, BuildTime)
}

// progName 返回当前程序名（如 ./mephisto）
// 会自动匹配用户的调用方式，帮助信息和示例中的命令名始终正确。
func progName() string {
	return os.Args[0]
}

// printHelp 打印帮助信息
func printHelp() {
	p := progName()
	fmt.Printf(`Mephisto - 长线叙事引擎

用法:
  %[1]s <子命令> [选项] <文件>
  %[1]s <文件>                        # 简写，等价于 parse

子命令:
  parse <文件> [选项]                  解析 .meph 契约，输出 JSON
  run   <文件> [选项]                  启动交互式对话模式
  version                             显示版本信息
  help                                显示此帮助信息

parse 选项:
  -o <路径>                           输出到文件（默认输出到 stdout）
  -q                                  静默模式，只输出错误

run 选项:
  -branch <分支名>                    分支名（用于多分支故事线）
  -reset                              忽略子版存档，从母版重新开始
  -debug                              启用规则调试模式
  -client <类型>                      LLM 客户端: deepseek, openai, ollama
                                        （默认从 MEPHISTO_CLIENT 环境变量读取）
  -model <模型名>                     模型名称
                                        （默认从 MEPHISTO_MODEL 环境变量读取）
  -api-key <密钥>                     API 密钥
                                        （默认从 OPENAI_API_KEY 环境变量读取）
  -base-url <URL>                     API 基础 URL
                                        （默认从 OPENAI_BASE_URL 环境变量读取）
  -constraints <文件>                 自定义输出约束文件（默认使用内置约束）
  -max-tokens <N>                     最大生成 Token 数（默认 4096）

交互模式 (run) 内置命令:
  /state                   显示当前状态
  /history                 显示对话历史
  /save                    手动保存进度
  exit / quit / q          退出对话

示例:
  %[1]s data/sample.meph                          解析并输出 JSON（简写）
  %[1]s parse data/sample.meph                    同上，完整写法
  %[1]s parse data/sample.meph -o out.json        解析并保存到文件
  %[1]s run data/sample.meph                      启动对话
  %[1]s run data/sample.meph -reset               忽略存档重新开始
  %[1]s run data/sample.meph -branch dark         使用 dark 分支
  %[1]s run data/sample.meph -client ollama       使用 Ollama 运行
  %[1]s run data/sample.meph -debug               启用调试模式
  %[1]s run data/sample.meph -reset -branch dark  组合使用

退出码:
  0  成功
  1  解析失败或参数错误
`, p)
}

// printError 打印错误信息（统一格式）。
//
// 根据错误类型决定输出格式：
//   - ParseError: 包含行号和区块名，输出 "❌ 第 N 行：消息"
//   - EngineError: 包含错误码，输出 "❌ [CODE] 消息"
//   - 其他: 输出 "❌ 消息"
//
// VSCode 插件可以通过 errors.As 提取 ParseError.Line 获取精确行号，
// 无需从字符串中正则提取。
func printError(err error) {
	var parseErr *shared.ParseError
	var engineErr *shared.EngineError

	switch {
	case errors.As(err, &parseErr):
		// ParseError 的 Error() 已经包含行号信息，直接输出
		fmt.Fprintf(os.Stderr, "❌ %v\n", parseErr)
	case errors.As(err, &engineErr):
		// EngineError 的 Error() 已经包含 [CODE] 前缀，直接输出
		fmt.Fprintf(os.Stderr, "❌ %v\n", engineErr)
	default:
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
	}
}
