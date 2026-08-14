// cmd/mephisto/session.go
//
// 交互式对话会话管理。
// 负责维护一次完整的交互式对话生命周期：欢迎、输入循环、命令处理、退出。
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fsnotify/fsnotify"

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
//   - stopWatch: 用于停止热重载监听器的通道
type Session struct {
	engine    *engine.Engine
	filename  string
	branch    string
	reset     bool
	childPath string
	hasChild  bool
	stopWatch chan struct{}
}

// NewSession 创建交互会话实例。
func NewSession(eng *engine.Engine, filename string, branch string, reset bool) *Session {
	return &Session{
		engine:    eng,
		filename:  filename,
		branch:    branch,
		reset:     reset,
		stopWatch: make(chan struct{}),
	}
}

// Start 启动交互式会话。
//
// 流程：
//  1. 如果 reset 为 true，跳过子版加载
//  2. 否则尝试加载子版（如果存在）
//  3. 打印欢迎信息
//  4. 启动文件监听（热重载规则）
//  5. 注册退出时的自动保存
//  6. 进入对话循环
//  7. 每轮对话后自动保存
//  8. 支持 /save 手动保存
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
		fmt.Println("🔄 已忽略子版存档，从母版重新开始")
		fmt.Println()
	}

	// ---- 3. 打印欢迎信息 ----
	s.printWelcome()

	// ---- 4. 启动文件监听（热重载规则） ----
	go s.watchFileChanges()

	// ---- 5. 退出时自动保存并停止监听 ----
	defer func() {
		close(s.stopWatch)
		if err := s.engine.Save(s.filename, s.branch); err != nil {
			fmt.Printf("\n⚠️ 自动保存失败: %v\n", err)
		} else {
			fmt.Printf("\n💾 已保存子版: %s\n", s.childPath)
		}
	}()

	// ---- 6. 进入对话循环 ----
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

		// ---- 7. 命令处理 ----
		switch {
		case s.isExitCommand(input):
			fmt.Println("契约终结。梅菲斯特静候下一次召唤。")
			return nil

		case s.isStateCommand(input):
			s.showState()

		case s.isHistoryCommand(input):
			s.showHistory()

		case s.isRulesCommand(input):
			s.showRules()

		case s.isSaveCommand(input):
			if err := s.engine.Save(s.filename, s.branch); err != nil {
				fmt.Printf("❌ 保存失败: %v\n", err)
			} else {
				fmt.Printf("✅ 已保存子版: %s\n", s.childPath)
			}

		default:
			// ---- 8. 普通输入：交给引擎（引擎内部自动处理记忆提取） ----
			s.handleInput(input)

			// ---- 9. 每轮对话后自动保存 ----
			if err := s.engine.Save(s.filename, s.branch); err != nil {
				fmt.Printf("\n⚠️ 自动保存失败: %v\n", err)
			}
		}
	}
}

// watchFileChanges 监听子版文件的变更，实现热重载。
//
// 用户可以在编辑器中修改规则文件并保存，引擎自动检测变更并应用新规则。
// 监听只在子版已加载（hasChild = true）时启动。
func (s *Session) watchFileChanges() {
	if !s.hasChild {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return // 监听失败，静默跳过（不影响主流程）
	}
	defer watcher.Close()

	if err := watcher.Add(s.childPath); err != nil {
		return
	}

	// 防抖：短时间内多次写入只触发一次重载
	var lastEvent time.Time

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			// 防抖：500ms 内的多次写入只处理一次
			if time.Since(lastEvent) < 500*time.Millisecond {
				continue
			}
			lastEvent = time.Now()

			// 等待文件写入完成（最多 100ms）：
			// 修复：之前用 time.Sleep 直接阻塞 select，若期间收到 stopWatch
			// 信号会延迟最多 50ms 才退出。现在用定时器 + select 等待，
			// 能及时响应停止信号。
			select {
			case <-time.After(50 * time.Millisecond):
				// 文件写入等待完成，继续
			case <-s.stopWatch:
				return
			}

			if err := s.engine.ReloadContract(s.childPath); err != nil {
				fmt.Printf("\n⚠️ 规则热重载失败: %v\n", err)
				fmt.Print("命运 >")
			} else {
				fmt.Printf("\n📜 规则已热更新（%d 条）\n", len(s.engine.Rules()))
				fmt.Print("命运 >")
			}

		case <-s.stopWatch:
			return
		}
	}
}

// printWelcome 打印会话欢迎信息。
func (s *Session) printWelcome() {
	contract := s.engine.Contract()
	roleName := contract.RoleName

	// ---- 构建变量映射 ----
	vars := shared.BuildPlaceholderVars(roleName, map[string]any{})
	for _, kv := range contract.State {
		vars[kv.Key] = fmt.Sprintf("%v", kv.Value.Raw())
	}

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Mephisto 叙事引擎\n")
	fmt.Printf("  角色: %s\n", roleName)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// ---- 根据是否加载子版决定显示内容 ----
	if !s.hasChild {
		if len(contract.Anchor) > 0 {
			fmt.Println("【锚点】")
			for _, kv := range contract.Anchor {
				value := shared.ReplacePlaceholders(fmt.Sprintf("%v", kv.Value.Raw()), vars)
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
		fmt.Println("💡 已恢复之前的进度，继续你的叙事旅程。")
		fmt.Println()
	}

	// ---- 当前状态 ----
	state := s.engine.State()
	if len(state) > 0 {
		fmt.Println("【当前状态】")
		for _, kv := range contract.State {
			if val, ok := state[kv.Key]; ok {
				fmt.Printf("  %s: %v\n", kv.Key, val)
			} else {
				fmt.Printf("  %s: %s\n", kv.Key, kv.Value.Raw())
			}
		}
		fmt.Println()
	}

	// ---- 规则列表 ----
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
	fmt.Printf("💡 输入 '/rules' 查看当前规则\n")
	fmt.Printf("💡 输入 '/save' 手动保存进度\n")
	fmt.Printf("💡 编辑规则文件后保存，引擎自动热重载\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

// ============================================================
// 命令判断方法
// ============================================================

func (s *Session) isSaveCommand(input string) bool    { return input == "/save" }
func (s *Session) isRulesCommand(input string) bool   { return input == "/rules" }
func (s *Session) isStateCommand(input string) bool   { return input == "/state" }
func (s *Session) isHistoryCommand(input string) bool { return input == "/history" }
func (s *Session) isExitCommand(input string) bool {
	return input == "exit" || input == "quit" || input == "q"
}

// ============================================================
// 显示方法
// ============================================================

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

func (s *Session) showRules() {
	rules := s.engine.Rules()
	if len(rules) == 0 {
		fmt.Println("当前没有规则")
		return
	}
	fmt.Printf("📜 当前规则（共 %d 条）：\n", len(rules))
	for i, rule := range rules {
		groupInfo := ""
		if rule.Group != "" {
			groupInfo = fmt.Sprintf(" [group:%s]", rule.Group)
		}
		fmt.Printf("  %d. [%s] if %s -> %s%s\n", i+1, rule.Name, rule.Cond, rule.Action, groupInfo)
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
//  3. 引擎内部已自动处理记忆提取，无需外部调用
func (s *Session) handleInput(input string) {
	const indent = "　　"

	needIndent := true
	inParagraph := false

	// 流式输出优化：先构建到 builder，再一次性输出整块 chunk。
	// 修复：之前 `for _, ch := range chunk` 对每个 rune 单独 fmt.Print，
	// 大段中文输出时产生大量系统调用。现在逻辑不变但结果累积到
	// strings.Builder，每块只做一次系统调用。
	onChunk := func(chunk string) {
		var sb strings.Builder
		sb.Grow(len(chunk) + len(indent))
		for _, ch := range chunk {
			if ch == '\n' {
				sb.WriteByte('\n')
				needIndent = true
				inParagraph = false
			} else {
				if !inParagraph && needIndent {
					sb.WriteString(indent)
					needIndent = false
				}
				sb.WriteRune(ch)
				inParagraph = true
			}
		}
		fmt.Print(sb.String())
	}

	// ---- 执行引擎（内部自动处理记忆提取） ----
	response, err := s.engine.Run(input, onChunk)
	if err != nil {
		fmt.Printf("\n❌ 错误: %v\n", err)
		return
	}

	if response != "" {
		fmt.Println()
	}
}