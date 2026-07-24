// internal/core/engine/engine.go
//
// # Mephisto 叙事引擎 - 主入口
//
// 职责：
//  1. 组合 Runtime（状态）+ rules（规则/动作）+ memory（记忆）
//  2. 提供 Run() 方法编排完整叙事流程
//
// 设计原则：
//  1. 单一入口：所有叙事逻辑通过 Run() 方法调用
//  2. 直接组合：Engine 直接持有 Runtime，无中间层
//  3. 无接口：条件评估和动作执行通过函数调用
//  4. 职责分离：状态管理在 Runtime，规则在 rules.go，记忆在 memory.go
//
// 使用示例：
//
//	contract, _ := parser.ParseFile("story.meph")
//	eng := engine.New(contract,
//	    engine.WithLLM(llmClient),
//	    engine.WithDebug(true),
//	)
//
//	response, _ := eng.Run("你来到光之国", func(chunk string) {
//	    fmt.Print(chunk)
//	})
//
//	eng.Save("story.meph", "dark")  // 保存到 story_dark.meph
package engine

import (
	"context"
	"fmt"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// ============================================================
// Engine 主结构
// ============================================================

// Engine 是叙事引擎的统一入口。
//
// 它组合 Runtime（状态管理）+ rules（规则/动作）+ memory（记忆），
// 通过 Run() 方法提供完整的叙事流程。
//
// 字段说明：
//   - runtime   : 运行时状态管理（含读写锁）
//   - contract  : 契约数据（只读）
//   - llmClient : LLM 客户端（可选）
//   - debug     : 是否启用调试模式
//   - memoryCfg : 记忆管理配置
type Engine struct {
	runtime   *Runtime
	contract  *domain.Contract
	llmClient llm.Client
	debug     bool
	memoryCfg MemoryConfig
}

// Option 配置选项。
type Option func(*Engine)

// WithLLM 注入 LLM 客户端。
// 如果不注入，引擎将使用静态文本响应。
func WithLLM(client llm.Client) Option {
	return func(e *Engine) { e.llmClient = client }
}

// WithDebug 启用调试模式。
// 调试模式下会打印规则匹配过程的详细信息。
func WithDebug(debug bool) Option {
	return func(e *Engine) { e.debug = debug }
}

// WithMemoryConfig 设置记忆管理配置。
// 如果不设置，使用 DefaultMemoryConfig。
func WithMemoryConfig(cfg MemoryConfig) Option {
	return func(e *Engine) { e.memoryCfg = cfg }
}

// WithMaxHistory 设置最大保留对话轮数。
// 每轮包含 fate 和 assistant 各一条，实际保留条数为 n * 2。
// 默认值为 20 轮（40 条记录）。
func WithMaxHistory(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.runtime.maxHistory = n
		}
	}
}

// New 创建引擎实例。
//
// 参数：
//   - contract: 已解析的契约数据
//   - opts: 可选的配置选项
//
// 返回值：
//   - *Engine: 可用的引擎实例
//
// 初始化流程：
//  1. 创建 Runtime（初始化状态和记忆）
//  2. 应用配置选项
//
// 示例：
//
//	eng := engine.New(contract,
//	    engine.WithLLM(llmClient),
//	    engine.WithDebug(true),
//	)
func New(contract *domain.Contract, opts ...Option) *Engine {
	e := &Engine{
		contract:  contract,
		runtime:   NewRuntime(contract, 20),
		debug:     false,
		memoryCfg: DefaultMemoryConfig,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// ============================================================
// 核心方法：Run
// ============================================================

// Run 执行一轮对话。
//
// 流程：
//  1. 验证输入非空
//  2. 记录用户输入到历史（角色：fate）
//  3. 规则匹配（支持互斥组）
//  4. 执行动作（注入/状态修改/LLM/静态）
//  5. 如果动作无输出（注入），调用 LLM 继续叙事
//  6. 记录响应到历史（角色：assistant）
//  7. 触发记忆提取（每 N 轮）
//  8. 返回响应文本
//
// 参数：
//   - input: 命运的指引（用户输入）
//   - onChunk: 流式输出回调（逐块返回响应内容，可为 nil）
//
// 返回值：
//   - string: 完整的响应文本
//   - error: 执行过程中的错误
//
// 流式输出说明：
//   - 如果 onChunk 不为 nil，引擎会逐字符/逐块调用回调
//   - 适用于 CLI 实时显示和 Web SSE 场景
//   - 即使使用流式，返回值仍然是完整的响应文本
//
// 示例：
//
//	response, err := eng.Run("你来到光之国", func(chunk string) {
//	    fmt.Print(chunk)  // 实时打印
//	})
func (e *Engine) Run(input string, onChunk func(string)) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", &shared.EngineError{
			Code:    "EMPTY_INPUT",
			Message: "输入不能为空",
		}
	}

	runtime := e.runtime // 局部变量，减少重复访问

	// 1. 记录命运指引
	runtime.AddHistory("fate", input)

	// 2. 两阶段规则匹配
	// 阶段一：批量执行所有被动规则（状态修改 + 注入记忆）
	// 被动规则产生副作用但不直接输出，所有匹配的都执行
	passiveMatched, passiveRollInfo := matchPassiveRules(e.contract.Rules, input, runtime, e.debug)

	// 阶段二：互斥匹配主动规则（LLM 指令/静态文本）
	// 主动规则产生直接输出，只执行第一个匹配的
	state := runtime.State()
	rule, activeMatched, activeRollInfo := matchActiveRule(e.contract.Rules, input, state, e.debug)

	// 合并 roll 信息（每行一个）
	var rollInfo string
	if passiveRollInfo != "" && activeRollInfo != "" {
		rollInfo = passiveRollInfo + "\n" + activeRollInfo
	} else if passiveRollInfo != "" {
		rollInfo = passiveRollInfo
	} else if activeRollInfo != "" {
		rollInfo = activeRollInfo
	}

	// 3. 输出骰子结果给用户（如果有）
	if rollInfo != "" && onChunk != nil {
		onChunk("\n" + rollInfo + "\n\n")
	}

	var response string

	if activeMatched {
		// 3a. 执行主动规则动作
		response = ExecuteAction(rule.Action, input, runtime, e.llmClient, onChunk, rollInfo)

		// 注入动作无输出，继续 LLM
		if response == "" {
			instruction := "继续推进剧情"
			if rollInfo != "" {
				instruction = fmt.Sprintf("继续推进剧情（%s）", rollInfo)
			}
			response = e.callLLM(input, instruction, onChunk)
		}
	} else if !passiveMatched {
		// 3b. 没有匹配任何规则（被动 + 主动都没有），自由叙事
		response = e.callLLM(input, "自由回应命运的指引", onChunk)
	} else {
		// 3c. 仅被动规则匹配，无主动规则匹配
		// 被动规则只产生副作用不输出，需要 LLM 继续叙事
		instruction := "继续推进剧情"
		if rollInfo != "" {
			instruction = fmt.Sprintf("继续推进剧情（%s）", rollInfo)
		}
		response = e.callLLM(input, instruction, onChunk)
	}

	// 4. 记录角色响应
	runtime.AddHistory("assistant", response)

	// 5. 记忆提取（每 N 轮）
	e.processMemories()

	return response, nil
}

// callLLM 调用 LLM 生成响应。
//
// 参数：
//   - input: 当前用户输入
//   - instruction: 额外指令（如 "继续推进剧情"）
//   - onChunk: 流式输出回调
//
// 返回值：
//   - string: LLM 生成的响应文本（无 LLM 时返回默认静态响应）
//
// 设计说明：
//   - 合并用户输入与指令，构建完整的 Prompt
//   - 支持流式和非流式两种调用方式
//   - 如果 LLM 客户端未注入，自动降级为静态响应
func (e *Engine) callLLM(input, instruction string, onChunk func(string)) string {
	if e.llmClient == nil {
		return e.defaultResponse(onChunk)
	}

	// 合并用户输入与指令
	combinedInput := input
	if instruction != "" && instruction != input {
		combinedInput = fmt.Sprintf("%s\n（指令：%s）", input, instruction)
	}

	// 构建 Prompt（使用运行时的记忆，而非契约初始值）
	prompt := llm.BuildPrompt(
		e.runtime.Contract(),
		e.runtime.State(),
		e.runtime.History(),
		e.runtime.Memories(),
		combinedInput,
		llm.NarrativeConstraints,
	)

	ctx := context.Background()

	// 调用 LLM（流式或非流式）
	if onChunk != nil {
		resp, err := e.llmClient.GenerateStream(ctx, prompt, onChunk)
		if err != nil {
			return e.defaultResponse(onChunk)
		}
		return resp
	}

	resp, err := e.llmClient.Generate(ctx, prompt)
	if err != nil {
		return e.defaultResponse(onChunk)
	}
	return resp
}

// defaultResponse 默认静态响应（无 LLM 或无规则匹配时使用）。
func (e *Engine) defaultResponse(onChunk func(string)) string {
	return defaultStaticResponse(e.contract.RoleName, onChunk)
}

// processMemories 处理记忆提取和压缩。
func (e *Engine) processMemories() {
	if e.llmClient == nil {
		return
	}

	runtime := e.runtime
	round := len(runtime.History()) / 2
	if !ShouldExtract(round, e.memoryCfg) {
		return
	}

	ctx := context.Background()

	// 提取新记忆
	newMemories, err := ExtractMemories(ctx, runtime.History(), runtime.Memories(), e.llmClient, e.memoryCfg)
	if err != nil || len(newMemories) == 0 {
		return
	}

	// 去重追加（使用语义去重）
	memories := runtime.Memories()
	memories = append(memories, newMemories...)
	memories = shared.DeduplicateMemories(memories)
	runtime.ReplaceMemories(memories)

	// 压缩（如果超过上限）
	compressed, err := CompressMemories(ctx, runtime.Memories(), e.llmClient, e.memoryCfg)
	if err == nil && len(compressed) > 0 {
		runtime.ReplaceMemories(compressed)
	}
}

// ============================================================
// 只读访问（委托给 Runtime）
// ============================================================

// State 返回当前状态的深拷贝（只读）。
func (e *Engine) State() map[string]any {
	return e.runtime.State()
}

// History 返回对话历史的深拷贝（只读）。
func (e *Engine) History() []domain.HistoryEntry {
	return e.runtime.History()
}

// Memories 返回长期记忆的深拷贝（只读）。
func (e *Engine) Memories() []string {
	return e.runtime.Memories()
}

// Contract 返回契约（只读）。
func (e *Engine) Contract() *domain.Contract {
	return e.runtime.Contract()
}