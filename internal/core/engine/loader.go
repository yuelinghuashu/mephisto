// internal/core/engine/loader.go
//
// 本文件提供子版存档的加载功能。
// 子版 = 母版所有静态区块 + 更新后的【状态】 + 【记忆】 + 【历史】
//
// 加载流程：
//  1. 检查子版文件是否存在
//  2. 如果存在，使用 parser.ParseFile 解析
//  3. 从子版中提取【状态】、【记忆】、【历史】
//  4. 返回加载的数据，由调用方应用到引擎
//
// 文件命名规则（与 saver.go 保持一致）：
//   - 默认：story.meph → story_child.meph
//   - 分支：--branch dark → story_dark.meph
package engine

import (
	"fmt"
	"os"

	"mephisto/internal/core/parser"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// ChildData 表示从子版加载的数据。
type ChildData struct {
	State    map[string]any
	Memories []string
	History  []domain.HistoryEntry
	Found    bool
}

// LoadChild 加载子版存档。
func LoadChild(filename string, branch string) (*ChildData, error) {
	// ---- 1. 构建子版文件路径 ----
	childPath := BuildChildPath(filename, branch)

	// ---- 2. 检查文件是否存在 ----
	if _, err := os.Stat(childPath); os.IsNotExist(err) {
		return &ChildData{Found: false}, nil
	}

	// ---- 3. 解析子版文件 ----
	contract, err := parser.ParseFile(childPath) // ← 修复2：现在调用的是你的 parser
	if err != nil {
		return nil, fmt.Errorf("解析子版文件失败: %w", err)
	}

	// ---- 4. 提取状态 ----
	state := make(map[string]any)
	for _, kv := range contract.State {
		state[kv.Key] = shared.ParseValue(kv.Value)
	}

	// ---- 5. 提取记忆 ----
	memories := contract.Memories
	if memories == nil {
		memories = []string{}
	}

	// ---- 6. 提取历史 ----
	history := contract.History
	if history == nil {
		history = []domain.HistoryEntry{}
	}

	return &ChildData{
		State:    state,
		Memories: memories,
		History:  history,
		Found:    true,
	}, nil
}

// HasChild 检查子版文件是否存在。
func HasChild(filename string, branch string) bool {
	childPath := BuildChildPath(filename, branch)
	_, err := os.Stat(childPath)
	return err == nil
}
