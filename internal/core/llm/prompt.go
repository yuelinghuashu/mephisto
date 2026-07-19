// internal/core/llm/prompt.go
//
// 本文件提供 Prompt 模板的构建和渲染功能。
// Prompt 是 Mephisto 与 LLM 交互的桥梁，它将角色定义、叙事上下文、
// 当前状态和命运指引组合成完整的提示词。
//
// 设计原则：
//  1. Prompt 模板是固定的，但数据是动态注入的
//  2. 状态必须使用运行时当前值（而非契约初始值）
//  3. 输出约束确保 LLM 以角色身份回应，而非解释或描述
//
// 分层结构（由高到低）：
//  1. 角色定义   ：世界观 + 角色身份 + 背景（静态）
//  2. 叙事上下文 ：当前状态 + 历史 + 记忆（动态）
//  3. 命运指引   ：当前用户输入 + 任务指令
//  4. 输出约束   ：控制 LLM 的输出格式和行为
package llm

import (
	"fmt"
	"strings"

	"mephisto/internal/domain"
)

// PromptTemplate 是 Prompt 模板的结构定义。
// 它包含渲染 Prompt 所需的所有静态和动态数据。
//
// 字段说明：
//   - RoleName   : 角色名，来自契约的【角色名】区块
//   - Anchor     : 锚点列表，来自契约的【锚点】区块（核心人格设定）
//   - Worldview  : 世界观，来自契约的【世界观】区块
//   - Background : 角色背景，来自契约的【角色背景】区块
//   - State      : 当前状态（运行时动态值，非契约初始值）
//   - History    : 对话历史（命运与角色的交互记录）
//   - Memories   : 长期记忆（由"注入"动作累积）
//   - UserInput  : 当前命运的指引（用户输入）
//   - Constraints: 输出约束（控制 LLM 的输出格式）
type PromptTemplate struct {
	RoleName    string
	Anchor      []domain.KeyValue
	Worldview   string
	Background  string
	State       map[string]any
	History     []domain.HistoryEntry
	Memories    []string
	UserInput   string
	Constraints string
}

// NarrativeConstraints 是默认的输出约束。
const NarrativeConstraints = `【绝对格式要求】你必须以小说叙事风格输出。严禁使用括号、方括号、【】、冒号加引号等任何剧本标记。对话必须使用引号，并明确说话者（如“某某说”、“某某喊道”）。动作描写必须自然融入段落。

【互动要求】每段回复必须包含至少两名其他角色（非玩家）的对话和动作反应。如果场景中没有其他角色，请引入或创造至少一个互动对象。禁止只有玩家独角戏。

正确示例：
贝利亚悬浮在光之国上空，俯视着下方戒备的战士们，发出一声低沉的冷笑：“这么多年了，这光还是这么刺眼。”奥特之父上前一步，沉声道：“贝利亚，光之国不会再次容忍你的暴行。”贝利亚转过头，猩红的眼睛盯着对方：“你们的容忍，对我而言一文不值。”

错误示例（绝对禁止）：
（悬浮在光之国上空）【贝利亚】：（冷笑）“这么多年了...”`

// BuildPrompt 构建完整的 Prompt 文本。
//
// 参数：
//   - contract    : 契约数据（静态部分：角色名、世界观、背景、锚点）
//   - currentState: 当前运行时状态（动态部分：情绪、生命值等）
//   - history     : 对话历史（命运与角色的交互记录）
//   - input       : 当前命运的指引（用户输入）
//   - constraints : 自定义约束（空字符串时使用默认）
//
// 返回值：
//   - string: 完整的 Prompt 文本
//
// 关键设计：currentState 是独立参数，而非从 contract 中读取。
// 原因：contract.State 是契约初始值，引擎运行过程中状态会变化，
// LLM 需要感知的是当前状态（如堕落指数从 50 变为 85），而非初始值。
func BuildPrompt(contract *domain.Contract, currentState map[string]any, history []domain.HistoryEntry, input string, constraints string) string {
	if constraints == "" {
		constraints = NarrativeConstraints
	}

	// 合并记忆：契约初始记忆 + 引擎运行时新增记忆
	memories := contract.Memories
	if len(memories) == 0 {
		memories = []string{}
	}

	return RenderPrompt(PromptTemplate{
		RoleName:    contract.RoleName,
		Anchor:      contract.Anchor,
		Worldview:   contract.Worldview,
		Background:  contract.Background,
		State:       currentState, // 关键：使用当前状态，而非契约初始状态
		History:     history,
		Memories:    memories,
		UserInput:   input,
		Constraints: constraints,
	})
}

// RenderPrompt 根据模板数据渲染 Prompt。
//
// 模板结构分为四层：
//  1. 角色定义：世界观 + 角色身份 + 背景（让 LLM 理解"我是谁"）
//  2. 叙事上下文：当前状态 + 历史 + 记忆（让 LLM 理解"我在哪"）
//  3. 命运指引：当前输入（让 LLM 理解"要我做什么"）
//  4. 输出约束：控制 LLM 的输出格式（让 LLM 知道"怎么回应"）
//
// 每层之间用空行分隔，确保 LLM 能够清晰区分不同部分。
func RenderPrompt(tmpl PromptTemplate) string {
	var sb strings.Builder

	// ============================================================
	// 第一层：格式硬性要求（放在最前，强化记忆）
	// ============================================================
	sb.WriteString("【格式硬性要求】\n")
	sb.WriteString(tmpl.Constraints)
	sb.WriteString("\n\n")

	// ============================================================
	// 第二层：角色定义
	// ============================================================
	sb.WriteString("【世界观】\n")
	sb.WriteString(tmpl.Worldview)
	sb.WriteString("\n\n")

	sb.WriteString("【角色】\n")
	fmt.Fprintf(&sb, "你是 %s", tmpl.RoleName)
	if len(tmpl.Anchor) > 0 {
		style := extractStyle(tmpl.Anchor)
		if style != "" {
			fmt.Fprintf(&sb, "，一个 %s 的存在", style)
		}
	}
	sb.WriteString("。\n")
	fmt.Fprintf(&sb, "你的背景：%s\n", tmpl.Background)
	sb.WriteString("\n")

	// ============================================================
	// 第三层：叙事上下文
	// ============================================================
	sb.WriteString("【当前状态】\n")
	sb.WriteString(renderStateList(tmpl.State))
	sb.WriteString("\n")

	sb.WriteString("【命运的推动】\n")
	sb.WriteString(renderHistorySummary(tmpl.History))
	sb.WriteString("\n")

	if len(tmpl.Memories) > 0 {
		sb.WriteString("【你记得的过往】\n")
		sb.WriteString(renderMemoriesList(tmpl.Memories))
		sb.WriteString("\n")
	}

	// ============================================================
	// 第四层：命运指引
	// ============================================================
	sb.WriteString("【此刻】\n")
	fmt.Fprintf(&sb, "%s\n", tmpl.UserInput)
	sb.WriteString("作为角色，你在这个场景中如何行动和回应？描述你的动作、对手的反应、环境的变化。\n")
	sb.WriteString("\n")

	// ============================================================
	// 第五层：输出约束（末尾再强调一次）
	// ============================================================
	sb.WriteString("【要求】\n")
	sb.WriteString(tmpl.Constraints)
	sb.WriteString("\n")

	return sb.String()
}

// ============================================================
// 辅助渲染函数
// ============================================================

// extractStyle 从锚点中提取风格描述。
// 依次查找 "风格"、"说话风格"、"人格标签"、"核心信念" 键，
// 返回第一个非空的值。
func extractStyle(anchor []domain.KeyValue) string {
	styleKeys := []string{"风格", "说话风格", "人格标签", "核心信念"}
	for _, kv := range anchor {
		for _, key := range styleKeys {
			if kv.Key == key && kv.Value != "" {
				return kv.Value
			}
		}
	}
	return ""
}

// renderStateList 将状态渲染为缩进列表。
// 格式：
//   - 情绪: 暴怒
//   - 堕落指数: 85
func renderStateList(state map[string]any) string {
	if len(state) == 0 {
		return "  （无特殊状态）"
	}
	var sb strings.Builder
	for k, v := range state {
		fmt.Fprintf(&sb, "- %s: %v\n", k, v)
	}
	return sb.String()
}

// renderHistorySummary 将历史渲染为叙事摘要。
// 每轮显示 "命运: ..." 和 "角色: ..." 的交替。
func renderHistorySummary(history []domain.HistoryEntry) string {
	if len(history) == 0 {
		return "  （命运尚未推动剧情）"
	}
	var sb strings.Builder
	for _, entry := range history {
		prefix := entry.Role
		switch entry.Role {
		case "fate":
			prefix = "命运"
		case "assistant":
			prefix = "角色"
		}
		fmt.Fprintf(&sb, "- %s: %s\n", prefix, entry.Content)
	}
	return sb.String()
}

// renderMemoriesList 将记忆渲染为缩进列表。
func renderMemoriesList(memories []string) string {
	if len(memories) == 0 {
		return "  （暂无记忆）"
	}
	var sb strings.Builder
	for _, m := range memories {
		fmt.Fprintf(&sb, "- %s\n", m)
	}
	return sb.String()
}
