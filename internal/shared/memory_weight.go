// internal/shared/memory_weight.go
//
// 记忆权重工具（对齐 Flutter lib/services/memory/memory_manager.dart）。
//
// CLI 内部以纯文本存储记忆（方案 B），权重前缀（`[N] 内容`）保留在
// .meph 文件中；本文件在「使用点」动态解析权重，驱动注入排序与压缩保护——
// 零侵入数据模型（保持 []string 存储），完全对齐 Flutter 语义。
package shared

import (
	"regexp"
	"sort"
	"strconv"
)

// 权重常量（对齐 Flutter lib/domain/entities.dart + memory_manager.dart）：
//
//	- defaultImportance = 3        （无前缀/无法识别前缀的记忆默认权重）
//	- maxImportance = 5            （权重上限）
//	- highImportanceThreshold = 4  （≥4 视为「核心记忆」，压缩时永不丢弃）
//	- highImportanceCap = 15       （核心记忆数量上限，超过时最低权重降级参与压缩）
const (
	DefaultImportance       = 3
	MaxImportance           = 5
	HighImportanceThreshold = 4
	HighImportanceCap       = 15
)

// memoryImportancePattern 匹配记忆条目开头的 `[权重] ` 前缀（如 `[4] `）。
var memoryImportancePattern = regexp.MustCompile(`^\[(\d)\]\s+`)

// MemoryImportance 解析记忆条目的重要性权重（对齐 Flutter Memory.importance）。
//
//   - 带 `[N] ` 前缀：返回 N（clamp 1-5）
//   - 无前缀/无法识别：返回默认权重 [DefaultImportance]（3，与 Flutter 一致）
func MemoryImportance(mem string) int {
	if m := memoryImportancePattern.FindStringSubmatch(mem); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			if n < 1 {
				return 1
			}
			if n > MaxImportance {
				return MaxImportance
			}
			return n
		}
	}
	return DefaultImportance
}

// SortMemoriesByImportance 按权重降序稳定排序记忆（高权重在前，同权重保持原顺序）。
//
// 对齐 Flutter MemoryManager.sortByImportance：供注入提示词时使用，
// 保证人设核心/重大事件优先被模型看到。稳定排序：同权重不改变原顺序。
func SortMemoriesByImportance(memories []string) []string {
	if len(memories) < 2 {
		return memories
	}
	sorted := append([]string{}, memories...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return MemoryImportance(sorted[i]) > MemoryImportance(sorted[j])
	})
	return sorted
}

// ClipMemories 按 [maxMemories] 裁剪记忆：高权重必带 + 其余按权重降序补足。
//
// 对齐 Flutter MemoryManager.clipMemories：
//   - 高权重记忆（≥ [HighImportanceThreshold]）全部保留（人设核心）
//   - 其余按权重降序补足剩余名额
//   - 返回结果已按权重排序（高权重在前、低权重降序在后）
//
// **语义说明（对齐 Flutter 的有意设计）**：当高权重记忆数量超过 [maxMemories]
// 时，返回列表会**超过上限**——「高权重优先超限」是有意取舍：高权重记忆是
// 人设核心，宁可本轮回合多带几条也不丢弃核心设定。真正的容量保护由压缩
// （[CompressMemories] / [HighImportanceCap]）承担，此处只负责注入裁剪。
func ClipMemories(memories []string, maxMemories int) []string {
	if len(memories) <= maxMemories {
		return memories
	}
	var high, rest []string
	for _, m := range memories {
		if MemoryImportance(m) >= HighImportanceThreshold {
			high = append(high, m)
		} else {
			rest = append(rest, m)
		}
	}
	high = SortMemoriesByImportance(high)
	rest = SortMemoriesByImportance(rest)
	remainingSlots := maxMemories - len(high)
	if remainingSlots < 0 {
		remainingSlots = 0
	}
	if len(rest) > remainingSlots {
		rest = rest[:remainingSlots]
	}
	return append(high, rest...)
}
