// ============================================================
// interactive.go - 交互式对话界面
// 职责：
// 1. 提供多轮对话循环
// 2. 每次用户输入时更新上下文并执行规则引擎
// 3. 显示触发的规则结果
// 4. 支持退出命令（quit / exit / q）
// ============================================================

package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"mephisto/engine"
)

// StartInteractive 启动交互式对话循环
// 参数：
//   - eng: 已加载规则的规则引擎
//   - ctx: 包含角色状态和用户输入的上下文
func StartInteractive(eng *engine.RuleEngine, ctx engine.Context) {
	// 准备角色显示名（用于输入提示）
	roleDisplay := ""
	if name, ok := ctx["角色名"]; ok && name != "" {
		roleDisplay = fmt.Sprintf("（%v）", name)
	}

	fmt.Println("🤖 对话模式（输入 quit / exit / q 退出）")
	fmt.Println(strings.Repeat("━", 50))

	scanner := bufio.NewScanner(os.Stdin)

	for {
		// ---- 读取用户输入 ----
		fmt.Printf("\n你%s: ", roleDisplay)
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())

		// ---- 检查退出命令 ----
		if userInput == "quit" || userInput == "exit" || userInput == "q" {
			fmt.Println("👋 再见！")
			break
		}

		// ---- 跳过空输入 ----
		if userInput == "" {
			continue
		}

		// ---- 更新上下文中的用户输入 ----
		// 规则中可以使用 "输入" 变量，如：if 输入包含 "光之国"
		eng.Context["输入"] = userInput

		// ---- 执行规则引擎 ----
		results, err := eng.Execute()
		if err != nil {
			fmt.Printf("❌ 规则引擎错误: %v\n", err)
			continue
		}

		// ---- 打印触发的规则 ----
		if len(results) > 0 {
			fmt.Println("\n⚡ 规则触发:")
			for _, r := range results {
				fmt.Printf("  %s\n", r.Message)
			}
		}
		// 没有规则触发时不打印任何内容（保持对话干净）
	}

	// ---- 检查扫描器错误（如 Ctrl+D 或 IO 错误） ----
	if err := scanner.Err(); err != nil {
		fmt.Printf("\n⚠️ 读取输入时出错: %v\n", err)
	}
}
