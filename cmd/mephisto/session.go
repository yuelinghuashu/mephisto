// cmd/mephisto/session.go
//
// 交互式对话会话管理。
// 负责维护一次完整的交互式对话生命周期：欢迎、输入循环、命令处理、退出。
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"

	"mephisto/internal/core/engine"
	"mephisto/internal/shared"
)

// Session 表示一次交互式对话会话。
//
// 字段说明：
//   - engine   : 引擎实例，持有契约、状态、历史、记忆等
//   - filename : 母版文件路径
//   - branch   : 分支名
//   - reset    : 是否忽略子版存档，从母版重新开始
//   - childPath: 子版文件路径（用于显示）
//   - hasChild : 是否已加载子版（缓存，避免重复加载）
type Session struct {
	engine    *engine.Engine
	filename  string
	branch    string
	reset     bool
	childPath string
	hasChild  bool
}

// NewSession 创建交互会话实例。
//
// 参数：
//   - eng    : 已初始化的引擎实例
//   - filename: 母版文件路径
//   - branch : 分支名
//   - reset  : 是否忽略子版存档
//
// 返回值：
//   - *Session: 可用的会话实例
func NewSession(eng *engine.Engine, filename string, branch string, reset bool) *Session {
	return &Session{
		engine:   eng,
		filename: filename,
		branch:   branch,
		reset:    reset,
	}
}

// Start 启动交互式会话。
//
// 流程：
//  1. 如果 reset 为 true，跳过子版加载
//  2. 否则尝试加载子版（如果存在）
//  3. 打印欢迎信息
//  4. 注册退出时的自动保存
//  5. 进入对话循环
//  6. 每轮对话后自动保存
//  7. 支持 /save 手动保存
//  8. 每轮对话后处理记忆提取
//
// 返回值：
//   - error: 会话执行过程中的错误
func (s *Session) Start() error {
	// ---- 1. 构建子版路径 ----
	s.childPath = engine.BuildChildPath(s.filename, s.branch)

	// ---- 2. 尝试加载子版（除非指定了 --reset） ----
	if !s.reset {
		data, err := engine.LoadChild(s.filename, s.branch)
		if err != nil {
			fmt.Printf("⚠️ 加载子版失败: %v\n", err)
			fmt.Println("   继续使用初始状态...")
		} else if data.Found {
			if err := s.engine.LoadChildData(data); err != nil {
				fmt.Printf("⚠️ 应用子版数据失败: %v\n", err)
				fmt.Println("   继续使用初始状态...")
			} else {
				s.hasChild = true
				fmt.Printf("📂 已加载子版: %s\n", s.childPath)
				if len(data.History) > 0 {
					fmt.Printf("  恢复 %d 轮对话历史\n", len(data.History)/2)
				}
				if len(data.Memories) > 0 {
					fmt.Printf("  恢复 %d 条记忆\n", len(data.Memories))
				}
				if len(data.State) > 0 {
					fmt.Printf("  恢复 %d 项状态\n", len(data.State))
				}
				fmt.Println()
			}
		}
	} else {
		// ---- 用户明确要求从母版重新开始 ----
		fmt.Println("🔄 已忽略子版存档，从母版重新开始")
		fmt.Println()
	}

	// ---- 3. 打印欢迎信息 ----
	s.printWelcome()

	// ---- 4. 退出时自动保存 ----
	defer func() {
		if err := s.engine.Save(s.filename, s.branch); err != nil {
			fmt.Printf("\n⚠️ 自动保存失败: %v\n", err)
		} else {
			fmt.Printf("\n💾 已保存子版: %s\n", s.childPath)
		}
	}()

	// ---- 5. 进入对话循环 ----
	for {
		var input string
		prompt := &survey.Input{
			Message: "命运 >",
		}
		err := survey.AskOne(prompt, &input)
		if err != nil {
			if strings.Contains(err.Error(), "interrupt") {
				fmt.Println("\n契约终结。梅菲斯特静候下一次召唤。")
				return nil
			}
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// ---- 6. 命令处理 ----
		switch {
		case s.isExitCommand(input):
			fmt.Println("契约终结。梅菲斯特静候下一次召唤。")
			return nil

		case s.isStateCommand(input):
			s.showState()

		case s.isHistoryCommand(input):
			s.showHistory()

		case s.isSaveCommand(input):
			if err := s.engine.Save(s.filename, s.branch); err != nil {
				fmt.Printf("❌ 保存失败: %v\n", err)
			} else {
				fmt.Printf("✅ 已保存子版: %s\n", s.childPath)
			}

		default:
			// ---- 7. 普通输入：交给引擎 ----
			s.handleInput(input)

			// ---- 8. 每轮对话后自动保存 ----
			if err := s.engine.Save(s.filename, s.branch); err != nil {
				fmt.Printf("\n⚠️ 自动保存失败: %v\n", err)
			}
		}
	}
}

// printWelcome 打印会话欢迎信息。
//
// 展示内容包括：
//   - 角色名
//   - 首次启动（无子版）：锚点、世界观、角色背景、开局场景
//   - 子版加载后：仅提示"已恢复进度"
//   - 当前状态（始终显示）
//   - 规则列表（始终显示）
//   - 操作提示
func (s *Session) printWelcome() {
	contract := s.engine.Contract()
	roleName := contract.RoleName

	// ---- 构建变量映射 ----
	vars := map[string]string{
		"角色名": roleName,
	}
	for _, kv := range contract.State {
		vars[kv.Key] = kv.Value
	}

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Mephisto 叙事引擎\n")
	fmt.Printf("  角色: %s\n", roleName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// ---- 根据是否加载子版决定显示内容 ----
	if !s.hasChild {
		// ---- 首次启动（无子版）：显示完整信息 ----
		if len(contract.Anchor) > 0 {
			fmt.Println("【锚点】")
			for _, kv := range contract.Anchor {
				value := shared.ReplacePlaceholders(kv.Value, vars)
				fmt.Printf("  %s: %s\n", kv.Key, value)
			}
			fmt.Println()
		}

		if contract.Worldview != "" {
			fmt.Println("【世界观】")
			fmt.Println(shared.ReplacePlaceholders(contract.Worldview, vars))
			fmt.Println()
		}

		if contract.Background != "" {
			fmt.Println("【角色背景】")
			fmt.Println(shared.ReplacePlaceholders(contract.Background, vars))
			fmt.Println()
		}

		if contract.Opening != "" {
			fmt.Println("【开局场景】")
			fmt.Println(shared.ReplacePlaceholders(contract.Opening, vars))
			fmt.Println()
		}
	} else {
		// ---- 加载子版后：只显示提示，略过静态信息 ----
		fmt.Println("💡 已恢复之前的进度，继续你的叙事旅程。")
		fmt.Println()
	}

	// ---- 当前状态（始终显示，使用引擎的实际状态） ----
	state := s.engine.State()
	if len(state) > 0 {
		fmt.Println("【当前状态】")
		// 按用户书写的顺序显示
		for _, kv := range contract.State {
			if val, ok := state[kv.Key]; ok {
				fmt.Printf("  %s: %v\n", kv.Key, val)
			} else {
				fmt.Printf("  %s: %s\n", kv.Key, kv.Value)
			}
		}
		fmt.Println()
	}

	// ---- 规则列表（始终显示） ----
	if len(contract.Rules) > 0 {
		fmt.Printf("【已加载的规则】%d 条\n", len(contract.Rules))
		for _, rule := range contract.Rules {
			fmt.Printf("  %s\n", rule.Name)
		}
		fmt.Println()
	}

	// ---- 操作提示 ----
	fmt.Printf("💡 输入 'exit' 或 'quit' 或 'q' 退出对话\n")
	fmt.Printf("💡 输入 '/state' 查看当前状态\n")
	fmt.Printf("💡 输入 '/history' 查看对话历史\n")
	fmt.Printf("💡 输入 '/save' 手动保存进度\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

// ============================================================
// 命令判断方法
// ============================================================

// isSaveCommand 检查输入是否为保存命令。
func (s *Session) isSaveCommand(input string) bool {
	return input == "/save"
}

// isExitCommand 检查输入是否为退出命令。
func (s *Session) isExitCommand(input string) bool {
	return input == "exit" || input == "quit" || input == "q"
}

// isStateCommand 检查输入是否为状态查看命令。
func (s *Session) isStateCommand(input string) bool {
	return input == "/state"
}

// isHistoryCommand 检查输入是否为历史查看命令。
func (s *Session) isHistoryCommand(input string) bool {
	return input == "/history"
}

// ============================================================
// 显示方法
// ============================================================

// showState 显示当前状态。
//
// 逐行输出引擎状态中的每个键值对，
// 格式为 "  键: 值"，便于阅读。
func (s *Session) showState() {
	state := s.engine.State()
	if len(state) == 0 {
		fmt.Println("当前状态为空")
		return
	}
	fmt.Println("当前状态：")
	for k, v := range state {
		fmt.Printf("  %s: %v\n", k, v)
	}
}

// showHistory 显示对话历史。
//
// 逐行输出历史记录中的每一条，
// 角色名会被转换为更友好的中文名称：
//   - "fate" → "命运"
//   - "assistant" → "角色"
func (s *Session) showHistory() {
	history := s.engine.History()
	if len(history) == 0 {
		fmt.Println("暂无对话历史")
		return
	}
	fmt.Println("对话历史：")
	for _, entry := range history {
		role := entry.Role
		switch role {
		case "fate":
			role = "命运"
		case "assistant":
			role = "角色"
		}
		fmt.Printf("  %s: %s\n", role, entry.Content)
	}
}

// ============================================================
// 输入处理方法
// ============================================================

// handleInput 处理用户的普通输入（非命令）。
//
// 流程：
//  1. 将用户输入传递给引擎，启用流式输出
//  2. 响应流式输出完成后换行
//  3. 触发记忆提取（使用独立 context，不阻塞主流程）
//
// 参数：
//   - input: 用户的普通输入
func (s *Session) handleInput(input string) {
	const indent = "　　"

	needIndent := true
	inParagraph := false

	// ---- 流式回调 ----
	onChunk := func(chunk string) {
		for _, ch := range chunk {
			if ch == '\n' {
				fmt.Println()
				needIndent = true
				inParagraph = false
			} else {
				if !inParagraph && needIndent {
					fmt.Print(indent)
					needIndent = false
				}
				fmt.Print(string(ch))
				inParagraph = true
			}
		}
	}

	// ---- 执行引擎 ----
	response, err := s.engine.Run(input, onChunk)
	if err != nil {
		fmt.Printf("\n❌ 错误: %v\n", err)
		return
	}

	if response != "" {
		fmt.Println()
	}

	// ---- 记忆提取（独立 context，30 秒超时） ----
	if s.engine != nil {
		memCtx, memCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer memCancel()

		if err := s.engine.ProcessMemories(memCtx, input, response); err != nil {
			// 记忆提取失败不中断对话，只记录提示
			fmt.Printf("\n⚠️ 记忆提取失败: %v\n", err)
		}
	}
}
