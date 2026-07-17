// ============================================================
// interactive.go - 交互式对话界面（支持流式 LLM 和 Context 取消）
// 职责：
// 1. 提供多轮对话循环
// 2. 每次用户输入时更新上下文并执行规则引擎
// 3. 显示触发的规则结果
// 4. 支持退出命令（quit / exit / q）
// 5. 支持流式 LLM 生成响应（M4）
// 6. 输入提示符固定为「命运」，符合梅菲斯特命名理念
// 7. 规则触发时，只要有注入或 LLM 动作，就生成叙事文本
// 8. 每次请求使用独立的 context，支持单独取消
// 9. 对话历史管理，支持多轮连贯叙事
// 10. 后置校验，过滤不符合规则的输出
// 11. 记忆提取与压缩（M5）
// 12. 每轮结束后自动保存子版（M5）
// ============================================================

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chzyer/readline"

	"mephisto/engine"
	"mephisto/llm"
	"mephisto/parser"
)

// ============================================================
// 对话历史管理
// ============================================================

// ConversationHistory 管理对话轮次
type ConversationHistory struct {
	messages []llm.Message
	maxSize  int
}

// NewConversationHistory 创建对话历史管理器
func NewConversationHistory(maxSize int) *ConversationHistory {
	if maxSize <= 0 {
		maxSize = 10 // 默认保留 10 轮
	}
	return &ConversationHistory{
		messages: []llm.Message{},
		maxSize:  maxSize,
	}
}

// Add 添加一条消息到历史
// content 应该是"干净"的（不含展示层格式）
func (h *ConversationHistory) Add(role, content string) {
	h.messages = append(h.messages, llm.Message{Role: role, Content: content})
	if len(h.messages) > h.maxSize {
		h.messages = h.messages[1:]
	}
}

// GetMessages 获取所有历史消息（用于 LLM 请求）
func (h *ConversationHistory) GetMessages() []llm.Message {
	return h.messages
}

// GetSize 获取当前历史消息数量
func (h *ConversationHistory) GetSize() int {
	return len(h.messages)
}

// Clear 清空历史
func (h *ConversationHistory) Clear() {
	h.messages = []llm.Message{}
}

// ============================================================
// 主循环
// ============================================================

// StartInteractive 启动交互式对话循环
// 参数：
//   - eng: 规则引擎实例
//   - ctx: 规则引擎上下文（包含所有状态变量）
//   - llmClient: LLM 客户端（nil 表示无 LLM 模式）
//   - quiet: 安静模式，隐藏规则注入信息
//   - retain: 对话历史保留轮数
//   - filename: 母版文件名（用于生成子版存档）
//   - branch: 分支名（用于多分支故事线）
//   - initialHistory: 从子版恢复的初始对话历史（nil 表示新建）
func StartInteractive(eng *engine.RuleEngine, ctx engine.Context, llmClient *llm.Client,
	quiet bool, retain int, filename string, branch string, initialHistory *ConversationHistory) {

	// 输入提示符固定为「（命运）: 」，符合梅菲斯特命名理念
	prompt := "（命运）: "

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     getHistoryFilePath(),
		AutoComplete:    nil,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	fmt.Println("🤖 对话模式（输入 quit / exit / q 退出）")
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println()

	// ============================================================
	// 初始化对话历史
	// 优先使用从子版恢复的历史，否则创建新的
	// ============================================================
	if retain <= 0 {
		retain = 10
	}

	var history *ConversationHistory
	if initialHistory != nil && initialHistory.GetSize() > 0 {
		if initialHistory.maxSize != retain {
			newHistory := NewConversationHistory(retain)
			for _, msg := range initialHistory.GetMessages() {
				newHistory.Add(msg.Role, msg.Content)
			}
			history = newHistory
			fmt.Printf("📝 恢复对话历史并调整保留轮数为 %d\n", retain)
		} else {
			history = initialHistory
		}
		fmt.Printf("📝 已恢复 %d 条对话历史\n", history.GetSize())
	} else {
		history = NewConversationHistory(retain)
		fmt.Printf("📝 开始新对话（保留 %d 轮）\n", retain)
	}
	fmt.Printf("🔍 [StartInteractive] 初始历史条数: %d\n", history.GetSize())

	// 对话轮数计数器（用于记忆提取频率控制）
	round := 0

	for {
		// ---- 读取用户输入 ----
		userInput, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("\n👋 再见！")
				break
			}
			fmt.Println("\n👋 再见！")
			break
		}

		userInput = strings.TrimSpace(userInput)
		lowerInput := strings.ToLower(userInput)

		// 退出命令
		if lowerInput == "quit" || lowerInput == "exit" || lowerInput == "q" {
			fmt.Println("👋 再见！")
			break
		}

		if userInput == "" {
			continue
		}

		// ---- 更新上下文 ----
		round++
		eng.Context["输入"] = userInput

		// ---- 🔑 将用户输入添加到历史（必须在 handleResults 之前） ----
		history.Add("user", userInput)
		fmt.Printf("🔍 [添加user后] 历史条数: %d\n", history.GetSize()) // ← 新增

		// ---- 执行规则引擎 ----
		results, err := eng.Execute()
		if err != nil {
			fmt.Printf("❌ 规则引擎错误: %v\n", err)
			continue
		}

		// ---- 创建独立 context（支持请求取消） ----
		reqCtx, cancel := context.WithCancel(context.Background())

		// ---- 处理结果（包含 LLM 生成、记忆提取） ----
		handleResults(results, ctx, llmClient, userInput, reqCtx, cancel, history, quiet, round)

		// ============================================================
		// 每轮结束后自动保存子版（写入成本极低，实时持久化）
		// ============================================================
		if llmClient != nil {
			fmt.Printf("🔍 [保存前] 历史条数: %d\n", history.GetSize()) // ← 新增

			if err := SaveChild(filename, branch, ctx, history); err != nil {
				// 保存失败只记录日志，不中断对话
				fmt.Printf("\n⚠️ 自动保存失败: %v\n", err)
			}
		}
	}

	// ---- 退出时最终保存（确保最新状态） ----
	if llmClient != nil {
		if err := SaveChild(filename, branch, ctx, history); err != nil {
			fmt.Printf("\n⚠️ 最终保存失败: %v\n", err)
		} else {
			childName := buildChildFilename(filename, branch)
			fmt.Printf("\n💾 已保存子版: %s\n", childName)
		}
	}
}

// getHistoryFilePath 返回历史文件路径（跨平台）
func getHistoryFilePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "mephisto_history.txt")
}

// ============================================================
// 结果处理
// ============================================================

// handleResults 处理规则引擎的执行结果
// 流程：
//
//	阶段1：分类结果（注入、LLM指令、骰子、普通消息）
//	阶段2：显示注入内容（作为叙事前奏）
//	阶段3：生成叙事（LLM 调用）
//	阶段4：后置校验（检查输出是否违规）
//	阶段5：显示骰子结果
//	阶段6：显示普通规则消息（仅当无 LLM 生成时）
//	阶段7：自由对话（无任何触发时）
//	阶段8：记忆提取（每次 LLM 生成后，根据 round 控制频率）
//
// 参数：
//   - results: 规则引擎执行结果列表
//   - ctx: 规则引擎上下文
//   - llmClient: LLM 客户端
//   - userInput: 用户本轮输入
//   - reqCtx: 请求上下文（用于取消）
//   - cancel: 取消函数
//   - history: 对话历史管理器
//   - quiet: 安静模式
//   - round: 当前对话轮数（用于记忆提取频率控制）
func handleResults(results []engine.ActionResult, ctx engine.Context,
	llmClient *llm.Client, userInput string, reqCtx context.Context,
	cancel context.CancelFunc, history *ConversationHistory, quiet bool, round int) {

	defer cancel()

	// ============================================================
	// 阶段1：分类结果
	// ============================================================
	var llmResults []string
	var injectResults []string
	var plainResults []string
	var diceResults []string

	for _, r := range results {
		switch r.Type {
		case engine.ActionLLM:
			llmResults = append(llmResults, r.Data)
		case engine.ActionInject:
			injectResults = append(injectResults, r.Data)
		case engine.ActionDice:
			diceResults = append(diceResults, r.Message)
		default:
			plainResults = append(plainResults, r.Message)
		}
	}

	// ============================================================
	// 阶段2：显示注入内容（作为叙事的前奏）
	// 注入内容是角色当下的心理状态或背景信息，
	// 在叙事之前显示能让用户理解角色的行为动机。
	// ============================================================
	if !quiet && len(injectResults) > 0 {
		fmt.Println("\n📖 叙事注入:")
		for _, content := range injectResults {
			fmt.Printf("  %s\n", content)
		}
		fmt.Println() // 注入内容后空一行，与叙事分隔
	}

	// ============================================================
	// 阶段3：生成叙事（LLM 调用）
	// 如果有注入或 LLM 指令，调用 LLM 生成叙事。
	// 注入内容会被融入叙事指令中，LLM 将其自然融入故事。
	// ============================================================
	var generatedResponse string
	hasLLMTrigger := (len(injectResults) > 0 || len(llmResults) > 0) && llmClient != nil

	if hasLLMTrigger {
		var instruction string
		if len(llmResults) > 0 {
			// 使用显式的 LLM 指令
			instruction = strings.Join(llmResults, "\n")
		} else {
			// 从注入内容构建叙事指令
			instruction = buildNarrativeInstructionFromInjects(injectResults, userInput)
		}
		generatedResponse = generateStreamingWithLLM(reqCtx, ctx, instruction, llmClient, history)
	}

	// ============================================================
	// 阶段4：后置校验（检查输出是否违规）
	// 根据「校验」区块的配置检查输出是否包含禁止内容。
	// 校验失败只警告，不阻断流程（违规响应尚未加入历史）。
	// ============================================================
	if generatedResponse != "" {
		valid, failures := validateOutput(ctx, generatedResponse)
		if !valid {
			fmt.Println("\n⚠️ 校验失败:")
			for _, failure := range failures {
				fmt.Printf("  %s\n", failure)
			}
			// 注意：违规响应尚未添加到历史，无需回滚
		}
	}

	// ============================================================
	// 阶段5：显示骰子结果
	// ============================================================
	if len(diceResults) > 0 {
		fmt.Println("\n🎲 骰子结果:")
		for _, msg := range diceResults {
			fmt.Printf("  %s\n", msg)
		}
	}

	// ============================================================
	// 阶段6：显示普通规则消息（仅在没有 LLM 生成时显示）
	// 如果有 LLM 生成叙事，普通规则消息会被叙事覆盖，
	// 避免信息冗余。
	// ============================================================
	if len(plainResults) > 0 && !hasLLMTrigger {
		fmt.Println("\n⚡ 规则触发:")
		for _, msg := range plainResults {
			fmt.Printf("  %s\n", msg)
		}
	}

	// ============================================================
	// 阶段7：自由对话（无任何触发，但 LLM 可用）
	// 当没有任何规则触发时，进入自由对话模式。
	// LLM 根据 System Prompt 和对话历史自然回应。
	// ============================================================
	if len(results) == 0 && llmClient != nil {
		generatedResponse = generateStreamingWithLLM(reqCtx, ctx, "自由对话", llmClient, history)
	}

	// ============================================================
	// 阶段8：记忆提取（每次 LLM 生成后执行）
	// 根据 round 控制提取频率（默认每 5 轮提取一次）。
	// 使用独立 context（超时 30 秒），不阻塞主对话流程。
	// 提取的记忆追加到 ctx[parser.KeyMemory]，超过阈值自动压缩。
	// ============================================================
	if generatedResponse != "" && llmClient != nil && ShouldExtract(round) {
		// 使用独立 context，超时 30 秒，不影响主流程
		memCtx, memCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer memCancel()

		newMemories, err := ExtractMemories(memCtx, ctx, userInput, generatedResponse, llmClient)
		if err == nil && len(newMemories) > 0 {
			appendMemories(ctx, newMemories)
			if shouldCompress(ctx) {
				compressed, err := CompressMemories(memCtx, ctx, llmClient)
				if err == nil && len(compressed) > 0 {
					ctx[parser.KeyMemory] = compressed
				}
			}
		}
	}
}

// ============================================================
// 叙事指令构建
// ============================================================

// buildNarrativeInstructionFromInjects 根据注入内容和用户输入构建叙事指令
// 注入内容代表角色的心理状态或背景信息，LLM 会将其自然融入叙事。
func buildNarrativeInstructionFromInjects(injectContents []string, userInput string) string {
	var sb strings.Builder

	sb.WriteString("请以角色身份推进剧情，基于以下信息：\n\n")
	fmt.Fprintf(&sb, "用户指令：%s\n", userInput)

	if len(injectContents) > 0 {
		sb.WriteString("\n以下是角色当前的心理状态和背景信息（融入叙事，不要直接引用）：\n")
		for _, content := range injectContents {
			fmt.Fprintf(&sb, "- %s\n", content)
		}
	}

	sb.WriteString("\n请以流畅的叙事方式推进剧情，像小说一样描述角色的行动、对话和内心活动。")
	return sb.String()
}

// ============================================================
// 后置校验
// ============================================================

// extractForbiddenText 从校验行中提取被禁止的文本
// 支持双引号（"）和单引号（'），返回第一个匹配的禁止文本
func extractForbiddenText(line string) string {
	// 尝试双引号
	first := strings.Index(line, `"`)
	if first != -1 {
		second := strings.Index(line[first+1:], `"`)
		if second != -1 {
			return line[first+1 : first+1+second]
		}
	}
	// 尝试单引号
	first = strings.Index(line, `'`)
	if first != -1 {
		second := strings.Index(line[first+1:], `'`)
		if second != -1 {
			return line[first+1 : first+1+second]
		}
	}
	return ""
}

// validateOutput 检查 LLM 输出是否通过后置校验
// 从上下文中读取「校验」区块的配置，逐条检查
// 返回：是否通过，失败原因列表
func validateOutput(ctx engine.Context, output string) (bool, []string) {
	var failures []string

	// 获取校验配置
	validationConfig, ok := ctx[parser.KeyValidation]
	if !ok {
		return true, nil // 没有校验规则，直接通过
	}

	// 解析校验规则
	configStr := fmt.Sprintf("%v", validationConfig)
	lines := strings.Split(configStr, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "后置检查") || strings.Contains(line, "输出不含") {
			forbidden := extractForbiddenText(line)
			if forbidden != "" && strings.Contains(output, forbidden) {
				failures = append(failures, fmt.Sprintf("输出包含禁止内容: %q", forbidden))
			}
		}
	}

	return len(failures) == 0, failures
}

// ============================================================
// 流式生成
// ============================================================

// generateStreamingWithLLM 调用流式 API，实时打印响应
// 返回完整的响应文本（不含缩进，用于校验和记录历史）
//
// 设计要点：
//   - displayResponse：带缩进的格式化内容（用于打印，两个空格缩进）
//   - cleanResponse：不含缩进的内容（用于存储到历史，干净无额外空格）
//   - 这样「历史」中保存的是干净内容，「显示」中展示的是格式化的内容
func generateStreamingWithLLM(ctx context.Context, engineCtx engine.Context,
	instruction string, client *llm.Client, history *ConversationHistory) string {

	systemPrompt := BuildSystemPrompt(engineCtx)
	userInput, _ := engineCtx["输入"].(string)
	userMessage := BuildUserMessage(userInput, instruction)

	// ---- 构建消息列表 ----
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}
	if history != nil && history.GetSize() > 0 {
		messages = append(messages, history.GetMessages()...)
	}
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})

	fmt.Printf("\n📖 命运:\n")

	// ---- 双缓冲区：显示用（含缩进），存储用（不含缩进） ----
	var displayResponse strings.Builder
	var cleanResponse strings.Builder

	isNewParagraph := true

	onChunk := func(chunk string) {
		for _, ch := range chunk {
			// 显示版本（带缩进）
			if isNewParagraph {
				fmt.Print("  ")
				displayResponse.WriteString("  ")
				isNewParagraph = false
			}
			fmt.Print(string(ch))
			displayResponse.WriteRune(ch)

			// 存储版本（不含缩进）
			cleanResponse.WriteRune(ch)

			if ch == '\n' {
				isNewParagraph = true
			}
		}
	}

	// ---- 执行流式调用 ----
	err := client.ChatStream(ctx, messages, onChunk)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n（请求已取消）")
		} else {
			fmt.Printf("\n⚠️ LLM 流式调用失败: %v\n", err)
		}
		return ""
	}

	// 叙事结束后空两行，与后续内容分隔
	fmt.Println()
	fmt.Println()

	// ---- 获取干净内容并记录历史 ----
	cleanContent := cleanResponse.String()
	if history != nil && cleanContent != "" {
		// 注意：user 消息已在外部添加，这里只添加 assistant
		history.Add("assistant", cleanContent)
	}

	return cleanContent
}
