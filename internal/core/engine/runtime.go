// internal/core/engine/runtime.go
//
// 运行时状态管理
// 职责：存储状态、历史、记忆，提供读写锁和深拷贝
//
// 线程安全说明：
//   Runtime 使用 sync.RWMutex 保护所有可变状态。
//   所有读方法（State/History/Memories）返回深拷贝，防止外部意外修改。
//   所有写方法（AddHistory/SetState/AppendMemory）自动加写锁。
//
// 深拷贝策略：
//   - state: 使用 maps.Clone 复制 map
//   - history: 使用 append 复制切片
//   - memories: 使用 append 复制切片
//
// 使用示例：
//
//	r := NewRuntime(contract, 20)
//	r.AddHistory("fate", "你来到光之国")
//	state := r.State()  // 返回副本
package engine

import (
	"maps"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
	"sync"
)

// Runtime 管理引擎的所有可变状态。
//
// 字段说明：
//   - mu         : 读写锁，保护所有可变字段
//   - contract   : 契约数据（只读，初始化后不变）
//   - state      : 运行时状态（键值对，值类型为 string/bool/int/float64）
//   - history    : 对话历史（fate 和 assistant 交替发言）
//   - memories   : 长期记忆（由"注入"动作和记忆提取累积）
//   - maxHistory : 最大保留轮数（每轮 = fate + assistant 两条记录）
type Runtime struct {
	mu         sync.RWMutex
	contract   *domain.Contract
	state      map[string]any
	history    []domain.HistoryEntry
	memories   []string
	maxHistory int
}

// NewRuntime 创建运行时实例。
//
// 参数：
//   - contract: 契约数据（用于初始化状态和记忆）
//   - maxHistory: 最大保留对话轮数
//
// 初始化流程：
//  1. 从契约的 State 区块初始化运行时状态
//  2. 从契约的 Memories 区块初始化记忆列表
//  3. 设置历史记录容量上限
//
// 返回值：
//   - *Runtime: 可用的运行时实例
func NewRuntime(contract *domain.Contract, maxHistory int) *Runtime {
	r := &Runtime{
		contract:   contract,
		state:      make(map[string]any),
		history:    []domain.HistoryEntry{},
		memories:   append([]string{}, contract.Memories...),
		maxHistory: maxHistory,
	}

	// 初始化状态：将契约中的键值对转换为运行时类型
	// 使用 shared.ParseValue 自动推断类型（bool/int/float64/string）
	for _, kv := range contract.State {
		r.state[kv.Key] = shared.ParseValue(kv.Value)
	}

	return r
}

// ============================================================
// 读方法（读锁 + 深拷贝）
// ============================================================

// State 返回当前状态的深拷贝（只读）。
//
// 返回值是 map 的副本，外部修改不会影响引擎内部状态。
// 状态值均为基本类型（string/bool/int/float64），拷贝安全。
func (r *Runtime) State() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return maps.Clone(r.state)
}

// History 返回对话历史的深拷贝（只读）。
func (r *Runtime) History() []domain.HistoryEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]domain.HistoryEntry{}, r.history...)
}

// Memories 返回长期记忆的深拷贝（只读）。
func (r *Runtime) Memories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string{}, r.memories...)
}

// Contract 返回契约（只读）。
func (r *Runtime) Contract() *domain.Contract {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.contract
}

// HistoryLength 返回历史记录条数（轻量查询，无需拷贝）。
func (r *Runtime) HistoryLength() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.history)
}

// ============================================================
// 写方法（写锁）
// ============================================================

// AddHistory 添加历史记录，自动截断至容量上限。
//
// 参数：
//   - role: "fate"（命运）或 "assistant"（角色）
//   - content: 对话内容
//
// 容量控制：
//   实际保留条数 = maxHistory * 2（每条记录对应一轮中的一方发言）
//   超出时删除最早记录。
func (r *Runtime) AddHistory(role, content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append(r.history, domain.HistoryEntry{Role: role, Content: content})
	max := r.maxHistory * 2
	if len(r.history) > max {
		r.history = r.history[len(r.history)-max:]
	}
}

// SetState 更新状态值。
//
// 参数：
//   - key: 状态键名
//   - value: 状态值（类型自动保留）
func (r *Runtime) SetState(key string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state[key] = value
}

// AppendMemory 追加一条记忆。
func (r *Runtime) AppendMemory(memory string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memories = append(r.memories, memory)
}

// ReplaceMemories 替换整个记忆列表（用于加载子版）。
func (r *Runtime) ReplaceMemories(memories []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memories = append([]string{}, memories...)
}

// ReplaceHistory 替换整个历史记录（用于加载子版）。
func (r *Runtime) ReplaceHistory(history []domain.HistoryEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append([]domain.HistoryEntry{}, history...)
}

// ReplaceState 替换整个状态（用于加载子版）。
func (r *Runtime) ReplaceState(state map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = maps.Clone(state)
}

// ClearHistory 清空历史记录。
func (r *Runtime) ClearHistory() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = []domain.HistoryEntry{}
}