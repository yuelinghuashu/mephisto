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
// ============================================================

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
func (h *ConversationHistory) Add(role, content string) {
	h.messages = append(h.messages, llm.Message{Role: role, Content: content})
	// 如果超过最大容量，移除最早的消息
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
func StartInteractive(eng *engine.RuleEngine, ctx engine.Context, llmClient *llm.Client, quiet bool) {
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

	// 初始化对话历史（直接从结构化 map 取值）
	maxHistory := 10
	if memCfg, ok := ctx[parser.KeyMemory]; ok {
		if cfgMap, ok := memCfg.(map[string]any); ok {
			if val, ok := cfgMap["保留轮数"]; ok {
				if i, ok := val.(int); ok && i > 0 {
					maxHistory = i
				}
			}
		}
	}
	history := NewConversationHistory(maxHistory)

	for {
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

		if lowerInput == "quit" || lowerInput == "exit" || lowerInput == "q" {
			fmt.Println("👋 再见！")
			break
		}

		if userInput == "" {
			continue
		}

		// 更新上下文
		eng.Context["输入"] = userInput

		// 执行规则引擎
		results, err := eng.Execute()
		if err != nil {
			fmt.Printf("❌ 规则引擎错误: %v\n", err)
			continue
		}

		// ---- 每次请求创建独立的 context，支持单独取消 ----
		reqCtx, cancel := context.WithCancel(context.Background())

		// 处理结果，传入历史管理器
		handleResults(results, ctx, llmClient, userInput, reqCtx, cancel, history, quiet)
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
func handleResults(results []engine.ActionResult, ctx engine.Context,
	llmClient *llm.Client, userInput string, reqCtx context.Context,
	cancel context.CancelFunc, history *ConversationHistory, quiet bool) {

	// 确保函数退出时释放资源
	defer cancel()

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

	// 用于存储生成的响应（供校验和记录历史）
	var generatedResponse string

	// ---- 1. 如果有注入或 LLM 动作，且 LLM 客户端可用，生成叙事 ----
	if (len(injectResults) > 0 || len(llmResults) > 0) && llmClient != nil {
		var instruction string
		if len(llmResults) > 0 {
			instruction = strings.Join(llmResults, "\n")
		} else {
			instruction = buildNarrativeInstructionFromInjects(injectResults, userInput)
		}
		generatedResponse = generateStreamingWithLLM(reqCtx, ctx, instruction, llmClient, history)
	}

	// ---- 2. 后置校验（如果生成了响应） ----
	if generatedResponse != "" {
		valid, failures := validateOutput(ctx, generatedResponse)
		if !valid {
			fmt.Println("\n⚠️ 校验失败:")
			for _, failure := range failures {
				fmt.Printf("  %s\n", failure)
			}
			// 从历史中移除违规的响应
			// 注意：由于我们还没添加到历史，所以不需要移除
		}
	}

	// ---- 3. 显示注入内容 ----
	if !quiet && len(injectResults) > 0 {
		fmt.Println("\n📖 叙事注入:")
		for _, content := range injectResults {
			fmt.Printf("  %s\n", content)
		}
	}

	// ---- 4. 骰子结果 ----
	if len(diceResults) > 0 {
		fmt.Println("\n🎲 骰子结果:")
		for _, msg := range diceResults {
			fmt.Printf("  %s\n", msg)
		}
	}

	// ---- 5. 普通规则消息（仅在没有 LLM 生成时显示） ----
	hasLLMGenerated := (len(injectResults) > 0 || len(llmResults) > 0) && llmClient != nil
	if len(plainResults) > 0 && !hasLLMGenerated {
		fmt.Println("\n⚡ 规则触发:")
		for _, msg := range plainResults {
			fmt.Printf("  %s\n", msg)
		}
	}

	// ---- 6. 无任何触发，但 LLM 可用：自由对话 ----
	if len(results) == 0 && llmClient != nil {
		generateStreamingWithLLM(reqCtx, ctx, "自由对话", llmClient, history)
		return
	}
}

// ============================================================
// 叙事指令构建
// ============================================================

// buildNarrativeInstructionFromInjects 根据注入内容和用户输入构建叙事指令
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
// 返回完整的响应文本（用于校验和记录历史）
// 支持段落首行缩进（两个空格）
func generateStreamingWithLLM(ctx context.Context, engineCtx engine.Context,
	instruction string, client *llm.Client, history *ConversationHistory) string {

	systemPrompt := BuildSystemPrompt(engineCtx)
	userInput, _ := engineCtx["输入"].(string)
	userMessage := BuildUserMessage(userInput, instruction)

	// 构建消息列表：System Prompt + 历史消息 + 当前用户消息
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	// 添加历史消息
	if history != nil && history.GetSize() > 0 {
		messages = append(messages, history.GetMessages()...)
	}

	// 添加当前用户消息
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})

	// 输出前缀固定为「📖 命运: 」并换行，内容从下一行开始
	fmt.Printf("\n📖 命运:\n")

	defer fmt.Println()

	// 收集完整响应（用于校验和记录历史）
	var fullResponse strings.Builder

	// ---- 段落缩进状态 ----
	// isNewParagraph: true 表示下一个字符位于新段落开头，需要插入缩进
	isNewParagraph := true

	onChunk := func(chunk string) {
		for _, ch := range chunk {
			// 如果是新段落开头，先输出两个空格缩进
			if isNewParagraph {
				fmt.Print("  ")
				fullResponse.WriteString("  ")
				isNewParagraph = false
			}

			// 输出当前字符
			fmt.Print(string(ch))
			fullResponse.WriteRune(ch)

			// 如果遇到换行符，下一字符就是新段落开头
			if ch == '\n' {
				isNewParagraph = true
			}
		}
	}

	err := client.ChatStream(ctx, messages, onChunk)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("\n（请求已取消）")
		} else {
			fmt.Printf("\n⚠️ LLM 流式调用失败: %v\n", err)
		}
		return ""
	}

	// 故事输出结束后，留一个空行，使下一行的提示符与故事内容保持间距
	fmt.Println() // 结束故事内容
	fmt.Println() // 额外空行

	// 将对话记录到历史
	response := fullResponse.String()
	if history != nil && response != "" {
		// 只记录非空响应
		history.Add("user", userInput)
		history.Add("assistant", response)
	}

	return response
}
