// internal/core/engine/engine.go
//
// 本文件提供 Mephisto 运行时引擎的核心实现。
// 引擎负责加载契约、管理状态、执行规则匹配和生成响应。
//
// 使用流程：
//
//	contract, _ := parser.ParseFile("role.meph")
//	eng := engine.New(contract)
//	response, _ := eng.Run("用户输入")
//
// 设计原则：
//  1. 引擎只负责"编排"，具体条件评估和动作执行由接口实现
//  2. 状态和历史通过副本暴露，防止外部污染
//  3. 历史有容量限制，防止无限增长
//  4. 采用选项模式，便于扩展配置
//  5. 动作执行统一由 ActionExecutor 处理，引擎不区分动作类型
//  6. 记忆管理由 MemoryManager 独立处理
//  7. 调试模式输出规则匹配过程，帮助用户排查问题
package engine

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"mephisto/internal/core/llm"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// Engine 是运行时引擎的核心结构。
//
// 字段说明：
//   - contract        : 契约（不可变），包含角色名、锚点、规则等静态配置
//   - state           : 运行时状态（可变），存储动态变量如情绪、生命值等
//   - history         : 对话历史（可变），记录 fate/assistant 的交互
//   - memories        : 长期记忆（可变），由"注入"动作和记忆提取共同维护
//   - maxHistory      : 最大保留轮数（每轮包含 fate + assistant 两条记录）
//   - debug           : 是否启用调试模式（输出规则匹配详细信息）
//   - memoryMgr       : 记忆管理器（可选），负责长期记忆的提取和压缩
//   - condEvaluator   : 条件评估器（可插拔）
//   - actionExecutor  : 动作执行器（可插拔，负责所有动作类型的执行）
//   - llmClient       : LLM 客户端（可选，用于 LLM 动作和记忆提取）
type Engine struct {
	contract   *domain.Contract
	state      map[string]any
	history    []domain.HistoryEntry
	memories   []string
	maxHistory int
	debug      bool
	memoryMgr  *MemoryManager

	condEvaluator  ConditionEvaluator
	actionExecutor ActionExecutor
	llmClient      llm.Client
}

// New 创建新的引擎实例。
//
// 参数：
//   - contract: 已验证的契约（由 parser + validator 产生）
//   - opts    : 可选的配置函数
//
// 返回值：
//   - *Engine: 可用的引擎实例
//
// 初始化行为：
//   - 状态从契约复制（而非引用），避免污染原始数据
//   - 历史初始为空
//   - 记忆从契约继承
//   - 最大保留轮数默认 20 轮
//   - 调试模式默认关闭
//   - 使用默认的条件评估器和动作执行器
//   - 记忆管理器默认为 nil（通过 WithMemoryManager 注入）
func New(contract *domain.Contract, opts ...Option) *Engine {
	// contract.State 是 []KeyValue（保持用户书写顺序）
	// 引擎内部使用 map 便于查询和修改
	stateMap := make(map[string]any)
	for _, kv := range contract.State {
		stateMap[kv.Key] = shared.ParseValue(kv.Value)
	}

	e := &Engine{
		contract:       contract,
		state:          stateMap,
		history:        []domain.HistoryEntry{},
		memories:       append([]string{}, contract.Memories...),
		maxHistory:     20,
		debug:          false,
		memoryMgr:      nil,
		condEvaluator:  &DefaultConditionEvaluator{},
		actionExecutor: &DefaultActionExecutor{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ============================================================
// 选项函数
// ============================================================

// Option 配置函数，用于设置引擎的非默认参数。
type Option func(*Engine)

// WithMaxHistory 设置最大保留轮数。
// 每轮产生两条历史记录（fate + assistant），
// 因此实际存储上限为 maxHistory * 2 条。
func WithMaxHistory(num int) Option {
	return func(e *Engine) { e.maxHistory = num }
}

// WithConditionEvaluator 替换默认的条件评估器。
// 用于注入自定义条件语法或测试 mock。
func WithConditionEvaluator(ce ConditionEvaluator) Option {
	return func(e *Engine) { e.condEvaluator = ce }
}

// WithActionExecutor 替换默认的动作执行器。
// 用于注入自定义动作类型（如骰子等）或测试 mock。
func WithActionExecutor(ae ActionExecutor) Option {
	return func(e *Engine) { e.actionExecutor = ae }
}

// WithLLMClient 注入 LLM 客户端，用于支持 `LLM:` 动作和普通文本的 LLM 生成。
func WithLLMClient(client llm.Client) Option {
	return func(e *Engine) { e.llmClient = client }
}

// WithMemoryManager 注入记忆管理器，用于支持长期记忆的提取和压缩。
//
// 记忆管理器需要 LLM 客户端才能工作，因此通常在 WithLLMClient 之后调用。
// 如果未注入记忆管理器，记忆提取功能将不可用。
func WithMemoryManager(mgr *MemoryManager) Option {
	return func(e *Engine) { e.memoryMgr = mgr }
}

// WithDebug 启用或禁用调试模式。
//
// 调试模式下，引擎会在规则匹配时输出详细信息：
//   - 正在检查的规则名和条件
//   - 条件评估结果（true/false）
//   - 触发的规则名
//   - 无规则匹配时的提示
//
// 这些信息会输出到标准输出，帮助用户理解规则的执行过程。
func WithDebug(debug bool) Option {
	return func(e *Engine) { e.debug = debug }
}

// ============================================================
// 核心方法
// ============================================================

// Run 执行一次对话轮次，支持可选的流式回调。
//
// 流程：
//  1. 记录用户输入到历史（角色：fate）
//  2. 按规则列表顺序匹配第一条满足条件的规则
//  3. 若匹配成功：
//     a. 执行对应的动作
//     b. 若动作返回空字符串（如注入），则继续调用 LLM 生成叙事
//     c. 否则直接返回动作结果（如状态修改确认）
//  4. 若匹配失败且 LLM 客户端可用：调用 LLM 生成自然响应
//  5. 若匹配失败且无 LLM 客户端：返回默认响应
//  6. 记录引擎响应到历史（角色：assistant）
//  7. 返回响应文本
//
// 参数：
//   - input  : 用户输入（命运的指引）
//   - onChunk: 流式回调，逐块输出响应内容（可为 nil，表示非流式）
//
// 返回值：
//   - response: 引擎生成的响应文本
//   - error   : 评估或执行过程中的错误
//
// 设计说明：
//   - 引擎不区分动作类型（注入/状态/LLM），统一交给 ActionExecutor
//   - 流式回调由 ActionExecutor 内部处理，引擎只负责传递
//   - 注入动作返回空字符串，引擎会继续调用 LLM 生成叙事
func (e *Engine) Run(input string, onChunk func(string)) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("输入不能为空")
	}

	e.addHistory("fate", input)

	rule, err := e.matchRule(input)
	if err != nil {
		return "", err
	}

	var response string

	if rule != nil {
		// ---- 规则匹配成功：执行动作 ----
		ctx := ActionContext{
			Input:     input,
			State:     e.state,
			History:   e.history,
			Memories:  e.memories,
			Contract:  e.contract,
			LLMClient: e.llmClient,
		}

		actionResult, err := e.actionExecutor.Execute(rule.Action, &ctx, onChunk)
		if err != nil {
			return "", err
		}
		// 同步更新记忆（ActionExecutor 可能修改了 ctx.Memories）
		e.memories = ctx.Memories

		// ---- 判断动作是否产生了直接输出 ----
		if actionResult != "" {
			// 动作产生了直接输出（如状态修改确认），直接返回
			response = actionResult
			e.addHistory("assistant", response)
			return response, nil
		}

		// ---- 动作未产生直接输出（注入动作），继续调用 LLM ----
		if e.llmClient != nil {
			// 使用当前的记忆（已包含注入内容）调用 LLM
			llmCtx := ActionContext{
				Input:     input,
				State:     e.state,
				History:   e.history,
				Memories:  e.memories,
				Contract:  e.contract,
				LLMClient: e.llmClient,
			}
			// 使用 "继续推进剧情" 作为指令，让 LLM 自然叙事
			response, err = e.actionExecutor.Execute("继续推进剧情", &llmCtx, onChunk)
			if err != nil {
				return "", err
			}
			e.memories = llmCtx.Memories
			e.addHistory("assistant", response)
			return response, nil
		}

		// ---- 无 LLM 客户端：默认响应 ----
		response = e.defaultResponse()
		if onChunk != nil {
			for _, ch := range response {
				onChunk(string(ch))
			}
		}
		e.addHistory("assistant", response)
		return response, nil
	}

	// ---- 无规则匹配 ----
	if e.llmClient != nil {
		// ---- 有 LLM 客户端：自由叙事 ----
		ctx := ActionContext{
			Input:     input,
			State:     e.state,
			History:   e.history,
			Memories:  e.memories,
			Contract:  e.contract,
			LLMClient: e.llmClient,
		}
		response, err = e.actionExecutor.Execute("自由回应命运的指引", &ctx, onChunk)
		if err != nil {
			return "", err
		}
		e.memories = ctx.Memories
	} else {
		// ---- 无 LLM 客户端：默认响应 ----
		response = e.defaultResponse()
		if onChunk != nil {
			for _, ch := range response {
				onChunk(string(ch))
			}
		}
	}

	e.addHistory("assistant", response)
	return response, nil
}

// matchRule 按顺序匹配第一条满足条件的规则。
//
// 规则匹配顺序：
//
//	从规则列表头部开始遍历，返回第一个条件评估为 true 的规则。
//	这实现了"优先级由书写顺序决定"的设计。
//
// 互斥组说明：
//
//	[group:] 标记由顺序匹配隐式实现：同一组规则按书写顺序排列，
//	先匹配的规则优先执行。由于每轮只触发一条规则，同组内其他规则
//	不会在同一轮中被执行，等效于互斥效果。
//
// 调试输出：
//
//	当 e.debug 为 true 时，会输出每条规则的检查过程和匹配结果。
func (e *Engine) matchRule(input string) (*domain.Rule, error) {
	if e.debug {
		fmt.Println("\n🔍 [调试] 开始规则匹配")
		fmt.Println(strings.Repeat("-", 50))
	}

	for _, rule := range e.contract.Rules {
		if e.debug {
			fmt.Printf("📌 检查规则: [%s] 条件: %s\n", rule.Name, rule.Cond)
		}

		ctx := ConditionContext{
			Input:    input,
			State:    e.state,
			History:  e.history,
			Memories: e.memories,
		}
		matched, err := e.condEvaluator.Evaluate(rule.Cond, ctx)
		if err != nil {
			return nil, fmt.Errorf("规则 '%s' 条件评估失败: %w", rule.Name, err)
		}

		if e.debug {
			fmt.Printf("  结果: %v\n", matched)
		}

		if matched {
			if e.debug {
				fmt.Printf("✅ 触发规则: [%s]\n", rule.Name)
				fmt.Println(strings.Repeat("-", 50))
			}
			return rule, nil
		}
	}

	if e.debug {
		fmt.Println("❌ 无规则匹配")
		fmt.Println(strings.Repeat("-", 50))
	}
	return nil, nil
}

// addHistory 添加历史记录，并自动截断至容量上限。
//
// 容量策略：
//
//	每轮产生两条记录（fate + assistant），
//	当总记录数超过 maxHistory * 2 时，移除最早的记录。
//	这样保证了历史记录始终包含最近 maxHistory 轮的完整对话。
func (e *Engine) addHistory(role, content string) {
	e.history = append(e.history, domain.HistoryEntry{
		Role:    role,
		Content: content,
	})
	max := e.maxHistory * 2
	if len(e.history) > max {
		e.history = e.history[len(e.history)-max:]
	}
}

// defaultResponse 返回默认响应（无规则匹配且无 LLM 客户端时使用）。
// 响应内容包含角色名，增加沉浸感。
func (e *Engine) defaultResponse() string {
	if e.contract.RoleName != "" {
		return fmt.Sprintf("%s 沉默地注视着命运。", e.contract.RoleName)
	}
	return "角色沉默地注视着命运。"
}

// ============================================================
// 记忆管理方法
// ============================================================

// ProcessMemories 处理记忆提取和压缩。
//
// 在每轮对话结束后调用此方法。
//
// 流程：
//  1. 检查是否需要提取记忆（根据轮数和配置）
//  2. 如果需要，从对话历史中提取关键事件摘要
//  3. 去重后追加到现有记忆列表
//  4. 如果记忆超过阈值，自动压缩
//
// 参数：
//   - ctx:    上下文（用于超时控制）
//   - input:  用户本轮输入
//   - response: 引擎本轮响应
//
// 返回值：
//   - error: 提取或压缩过程中的错误（非致命错误会记录日志）
//
// 注意：此方法会调用 LLM，可能耗时较长。
// 建议在后台 goroutine 中调用，避免阻塞主对话流程。
func (e *Engine) ProcessMemories(ctx context.Context, input, response string) error {
	if e.memoryMgr == nil || e.llmClient == nil {
		return nil
	}

	// 计算当前轮数
	round := len(e.history) / 2

	// 检查是否需要提取
	if !e.memoryMgr.ShouldExtract(round) {
		return nil
	}

	// 提取并追加记忆
	newMemories, err := e.memoryMgr.ExtractAndAppend(ctx, e.history, e.memories)
	if err != nil {
		return err
	}

	// 更新记忆
	e.memories = newMemories

	return nil
}

// ============================================================
// 只读访问方法
// ============================================================

// State 返回当前状态的副本（只读）。
// 使用 maps.Clone 确保外部修改不会影响引擎内部状态。
func (e *Engine) State() map[string]any {
	return maps.Clone(e.state)
}

// History 返回历史记录的副本（只读）。
// 通过复制切片避免外部修改影响引擎内部历史。
func (e *Engine) History() []domain.HistoryEntry {
	return append([]domain.HistoryEntry{}, e.history...)
}

// Memories 返回记忆的副本（只读）。
// 通过复制切片避免外部修改影响引擎内部记忆。
func (e *Engine) Memories() []string {
	return append([]string{}, e.memories...)
}

// Contract 返回引擎持有的契约（只读）。
// 用于在外部获取角色名、锚点、开局场景等静态信息。
func (e *Engine) Contract() *domain.Contract {
	return e.contract
}

// ============================================================
// 子版存档方法
// ============================================================

// Save 保存当前会话状态到子版文件。
//
// 参数：
//   - filename: 母版文件路径
//   - branch:   分支名（空字符串表示默认子版）
//
// 返回值：
//   - error: 保存失败时的错误
func (e *Engine) Save(filename string, branch string) error {
	return SaveChild(
		e.contract,
		e.state,
		e.memories,
		e.history,
		filename,
		branch,
	)
}

// LoadChildData 从子版加载数据并应用到引擎。
//
// 参数：
//   - data: LoadChild 返回的数据
//
// 返回值：
//   - error: 应用数据失败时的错误
//
// 覆盖策略：
//   - 状态：直接替换
//   - 记忆：直接替换
//   - 历史：直接替换
func (e *Engine) LoadChildData(data *ChildData) error {
	if data == nil || !data.Found {
		return nil
	}

	// 覆盖状态
	maps.Copy(e.state, data.State)

	// 覆盖记忆
	e.memories = append([]string{}, data.Memories...)

	// 覆盖历史
	e.history = append([]domain.HistoryEntry{}, data.History...)

	return nil
}
