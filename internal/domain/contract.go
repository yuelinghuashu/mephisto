// internal/domain/contract.go
package domain

// Contract 表示一个完整的 .meph 契约文件
// 它包含所有用户书写的区块 + 系统运行时生成的区块
type Contract struct {
	// ============================================================
	// 7 个用户区块（由创作者书写）
	// ============================================================

	// 【角色名】单行文本，必选
	RoleName string `json:"role_name"`

	// 【锚点】键值对列表，推荐
	// 核心人格设定，永不压缩
	// 存储为原始行列表，保留格式
	Anchor []KeyValue `json:"anchor,omitempty"`

	// 【世界观】多行文本，可选
	Worldview string `json:"worldview,omitempty"`

	// 【角色背景】多行文本，可选
	Background string `json:"background,omitempty"`

	// 【开局场景】多行文本，可选
	Opening string `json:"opening,omitempty"`

	// 【状态】键值对列表，可选
	// 动态变量（情绪、生命值、位置等）
	State []KeyValue `json:"state,omitempty"`

	// 【规则】规则列表，可选
	Rules []*Rule `json:"rules,omitempty"`

	// ============================================================
	// 2 个系统区块（由程序自动生成，不出现在用户书写的 .meph 中）
	// ============================================================

	// 【记忆】纯文本列表
	// 程序自动从对话中提取的长期记忆
	Memories []string `json:"memories,omitempty"`

	// 【历史】键值对列表
	// 程序自动记录的对话历史
	History []HistoryEntry `json:"history,omitempty"`
}

// KeyValue 表示一个键值对（用于锚点、状态等区块）
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Rule 表示一条规则定义
type Rule struct {
	Name   string `json:"name"`   // 规则名
	Cond   string `json:"cond"`   // 条件原始字符串
	Action string `json:"action"` // 动作原始字符串
	Group  string `json:"group"`  // 互斥组名
	Line   int    `json:"line"`   // 来源行号（报错定位）
}

// HistoryEntry 表示一条历史记录
type HistoryEntry struct {
	Role    string `json:"role"`    // "fate"（命运）或 "assistant"（角色）
	Content string `json:"content"` // 对话内容
}
