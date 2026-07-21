// cmd/mephisto/help.go
//
// 帮助信息和版本信息
package main

import (
	"fmt"
	"os"
)

// 版本信息（构建时注入）
var (
	Version   = "v1.0.0"
	BuildTime = "2026-07-21"
)

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("Mephisto %s (built %s)\n", Version, BuildTime)
}

// printHelp 打印帮助信息
func printHelp() {
	fmt.Println(`Mephisto - 长线叙事引擎

用法:
  mephisto [全局选项] <子命令> [参数]

子命令:
  parse <文件路径> [选项]   # 解析 .meph 契约，输出 JSON
  run <文件路径> [选项]     # 启动交互式对话模式
  check <文件路径>          # 快速检查契约（输出 JSON，供 VSCode 调用）
  version                  # 显示版本信息
  help                     # 显示此帮助信息

全局选项（可放在子命令之前）:
  -h, -help                显示帮助信息
  -q, -quiet               静默模式，只输出错误
  -branch <分支名>         分支名（用于多分支故事线，默认为空）
  -reset                   忽略子版存档，从母版重新开始

run 子命令选项:
  -m, -model <模型名>      LLM 模型名称（默认从 MEPHISTO_MODEL 读取）
  -client <类型>           LLM 客户端类型: deepseek, openai, ollama（默认从 MEPHISTO_CLIENT 读取）
  -api-key <密钥>          API 密钥（默认从 OPENAI_API_KEY 读取）
  -base-url <URL>          API 基础 URL（默认从 OPENAI_BASE_URL 读取）
  -debug                   启用规则调试模式（显示规则匹配过程）

parse 子命令选项:
  -o, -output <路径>       输出到文件（默认输出到 stdout）
  -q, -quiet               静默模式，只输出错误

交互模式 (run) 内置命令:
  /state                   显示当前状态
  /history                 显示对话历史
  /save                    手动保存进度
  exit / quit / q          退出对话

示例:
  mephisto data/sample.meph                          # 解析并输出 JSON（默认）
  mephisto parse data/sample.meph -o out.json       # 解析并保存到文件
  mephisto -reset run data/sample.meph              # 从母版重新开始对话
  mephisto -branch dark run data/sample.meph        # 使用 dark 分支
  mephisto run data/sample.meph -client ollama      # 使用 Ollama 运行
  mephisto run data/sample.meph -debug              # 启用调试模式运行


退出码:
  0  成功
  1  解析失败或参数错误`)
}

// printError 打印错误信息（统一格式）
func printError(err error) {
	fmt.Fprintf(os.Stderr, "❌ %v\n", err)
}
