// ============================================================
// prompt.go - System Prompt 构建器
// 职责：
// 1. 从上下文（Context）动态构建大模型 System Prompt
// 2. 按照优先级组织内容：角色身份 → 锚点 → 世界观 → 角色背景 → 当前状态
// 3. 确保大模型始终以角色身份回应，不跳出设定
// 4. 使用 parser 包中的常量，避免硬编码
// ============================================================

package app

import (
	"fmt"
	"strings"

	"mephisto/engine"
	"mephisto/parser"
)

// BuildSystemPrompt 从上下文构建 System Prompt
// 返回的字符串将作为大模型对话的 System Prompt
// 它按照以下层级组织内容：
//  1. 角色身份（必填）
//  2. 锚点（核心人格设定，永不压缩）
//  3. 世界观（故事背景）
//  4. 角色背景（角色历史）
//  5. 当前状态（动态变量）
//  6. 角色扮演指令（约束行为）
func BuildSystemPrompt(ctx engine.Context) string {
	var sb strings.Builder

	// ---- 第一层：角色身份 ----
	if name, ok := ctx[parser.KeyRoleName]; ok && name != "" {
		fmt.Fprintf(&sb, "你是 %v。\n", name)
	} else {
		sb.WriteString("你是一个神秘的角色。\n")
	}

	// ---- 第二层：锚点（核心人格设定）- 最高优先级 ----
	if anchor, ok := ctx[parser.KeyAnchor]; ok {
		anchorStr := fmt.Sprintf("%v", anchor)
		if strings.TrimSpace(anchorStr) != "" {
			sb.WriteString("\n## 核心人格设定（必须严格遵守）\n")
			sb.WriteString("以下是你必须遵守的核心设定，这些设定高于一切：\n")
			formatted := formatAnchorContent(anchorStr)
			sb.WriteString(formatted)
			sb.WriteString("\n")
		}
	}

	// ---- 第三层：世界观 ----
	if world, ok := ctx[parser.KeyWorldview]; ok {
		worldStr := fmt.Sprintf("%v", world)
		if strings.TrimSpace(worldStr) != "" {
			sb.WriteString("\n## 世界观\n")
			sb.WriteString("你所在的世界遵循以下规则：\n")
			sb.WriteString(worldStr)
			sb.WriteString("\n")
		}
	}

	// ---- 第四层：角色背景 ----
	if background, ok := ctx[parser.KeyBackground]; ok {
		bgStr := fmt.Sprintf("%v", background)
		if strings.TrimSpace(bgStr) != "" {
			sb.WriteString("\n## 角色背景\n")
			sb.WriteString("你的过去是这样的：\n")
			sb.WriteString(bgStr)
			sb.WriteString("\n")
		}
	}

	// ---- 第五层：当前状态 ----
	sb.WriteString("\n## 当前状态\n")
	for key, val := range ctx {
		// 使用 parser.StateExcludeKeys 排除系统键
		if parser.StateExcludeKeys[key] {
			continue
		}
		if val == nil {
			continue
		}
		strVal := fmt.Sprintf("%v", val)
		if strings.TrimSpace(strVal) == "" {
			continue
		}
		fmt.Fprintf(&sb, "- %s: %s\n", key, strVal)
	}

	// ---- 第六层：角色扮演指令（行为约束） ----
	sb.WriteString("\n## 角色扮演指令\n")
	sb.WriteString("以下是你在回应时必须遵守的规则：\n")
	sb.WriteString("1. 你正在扮演这个角色在故事中行动，**你不是在「回复」用户，而是在「推进」剧情**\n")
	sb.WriteString("2. 用户的输入是**故事指令**（如「XXX出现了」「XXX前往了YYY」），不是对话问题\n")
	sb.WriteString("3. **执行用户指令的核心规则**：\n")
	sb.WriteString("   - 用户指令中提到的任何人、物、事件，**必须实际在叙事中登场或发生**\n")
	sb.WriteString("   - 不要只是「描述」会发生什么，而是让事情**实际发生**\n")
	sb.WriteString("   - 例如：用户指令「一群强盗冲了进来」，你必须让强盗冲进来、说话、行动\n")
	sb.WriteString("   - 例如：用户指令「盟友赶到了」，你必须让盟友实际出现并与角色互动\n")
	sb.WriteString("4. 引入新角色时，给他们对话和行动，让故事鲜活起来\n")
	sb.WriteString("5. 以第三人称叙事，像小说一样描述场景、动作、对话和内心活动\n")
	sb.WriteString("6. 用户的每个指令都是故事的一部分，必须被融入叙事，不能忽略或跳过\n")

	return sb.String()
}

// formatAnchorContent 格式化锚点内容为可读文本
// 如果内容看起来已经是列表格式（包含 "- "），保持原样
// 否则尝试格式化为键值对列表
func formatAnchorContent(content string) string {
	// 如果包含 "- "，说明已经是列表格式，直接返回
	if strings.Contains(content, "- ") {
		return content
	}

	// 否则尝试按行处理
	lines := strings.Split(content, "\n")
	var result strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "-") {
			result.WriteString("- ")
		}
		result.WriteString(line)
		result.WriteString("\n")
	}
	return result.String()
}

// BuildUserMessage 构建用户消息
// 将用户输入和指令组合成符合大模型 API 格式的消息
func BuildUserMessage(userInput, instruction string) string {
	var sb strings.Builder

	// 如果有用户输入，作为上下文
	if strings.TrimSpace(userInput) != "" {
		fmt.Fprintf(&sb, "用户指令：%s\n", userInput)
	}

	// 如果有特定指令且不是默认的"自由对话"
	if strings.TrimSpace(instruction) != "" && instruction != "自由对话" {
		fmt.Fprintf(&sb, "\n指令：%s", instruction)
	}

	// 如果没有任何内容，返回默认值
	if sb.Len() == 0 {
		sb.WriteString("请继续对话。")
	}

	return sb.String()
}