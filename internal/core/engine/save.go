// internal/core/engine/save.go
//
// 存档管理：子版保存、加载、路径构建
//
// 本文件集中管理所有与 .meph 存档文件相关的操作：
//   - ChildData: 子版数据模型
//   - Save(): 保存当前会话为子版
//   - LoadChild(): 从文件加载子版
//   - LoadChildData(): 将加载的数据应用到 Runtime
//   - buildChildContent(): 构建 .meph 格式的子版内容
//   - BuildChildPath(): 子版文件路径计算
//
// 设计原则：
//   - 存档逻辑与引擎核心流程（Run）分离
//   - 所有文件 I/O 集中于此文件，便于测试和替换
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mephisto/internal/core/parser"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// ============================================================
// 子版数据模型
// ============================================================

// ChildData 表示从子版加载的数据。
type ChildData struct {
	State    map[string]any        // 运行时状态
	Memories []string              // 长期记忆
	History  []domain.HistoryEntry // 对话历史
	Found    bool                  // 文件是否存在
}

// ============================================================
// 引擎方法：保存
// ============================================================

// Save 保存当前会话状态到子版文件。
//
// 子版 = 母版所有静态区块 + 更新后的【状态】 + 【记忆】 + 【历史】
//
// 参数：
//   - filename: 母版文件路径
//   - branch: 分支名（空字符串表示默认子版）
//
// 命名规则：
//   - 默认：story.meph → story_child.meph
//   - 分支：--branch dark → story_dark.meph
//
// 返回值：
//   - error: 保存失败时的错误
func (e *Engine) Save(filename, branch string) error {
	content := e.buildChildContent()
	path := BuildChildPath(filename, branch)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// LoadChildData 从子版加载数据并应用到 Runtime。
//
// 参数：
//   - data: 从 LoadChild 获取的数据
//
// 返回值：
//   - error: 应用失败时的错误
func (e *Engine) LoadChildData(data *ChildData) error {
	if data == nil || !data.Found {
		return nil
	}
	e.runtime.ReplaceState(data.State)
	e.runtime.ReplaceMemories(data.Memories)
	e.runtime.ReplaceHistory(data.History)
	return nil
}

// ============================================================
// 包级函数：加载
// ============================================================

// LoadChild 加载子版存档。
//
// 参数：
//   - filename: 母版文件路径
//   - branch: 分支名
//
// 返回值：
//   - *ChildData: 加载的数据（Found=false 表示文件不存在）
//   - error: 加载失败时的错误
func LoadChild(filename, branch string) (*ChildData, error) {
	path := BuildChildPath(filename, branch)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ChildData{Found: false}, nil
	}

	contract, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("解析子版文件失败：%w", err)
	}

	return &ChildData{
		State:    shared.KeyValuesToMap(contract.State),
		Memories: contract.Memories,
		History:  contract.History,
		Found:    true,
	}, nil
}

// ============================================================
// 子版内容构建（私有）
// ============================================================

// buildChildContent 构建子版文件内容。
//
// 输出格式与 .meph 契约文件完全一致，包含 10 个标准区块：
//
//	角色名、锚点、世界观、角色背景、开局场景、状态、规则、校验、记忆、历史
//
// 变量替换策略：
//   - 替换：锚点、世界观、角色背景、开局场景、规则动作
//   - 不替换：状态（变量源）、记忆（已生成）、历史（已记录）
func (e *Engine) buildChildContent() string {
	var sb strings.Builder

	contract := e.contract
	state := e.runtime.State()
	memories := e.runtime.Memories()
	history := e.runtime.History()

	// 构建变量映射（用于占位符替换）
	vars := shared.BuildPlaceholderVars(contract.RoleName, state)

	// ---- 1. 角色名（不替换） ----
	fmt.Fprintf(&sb, "【角色名】\n%s\n\n", contract.RoleName)

	// ---- 2. 锚点（替换占位符） ----
	if len(contract.Anchor) > 0 {
		fmt.Fprint(&sb, "【锚点】\n")
		for _, kv := range contract.Anchor {
			value := shared.ReplacePlaceholders(kv.Value, vars)
			fmt.Fprintf(&sb, "- %s: %s\n", kv.Key, value)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 3. 世界观（替换占位符） ----
	if contract.Worldview != "" {
		content := shared.ReplacePlaceholders(contract.Worldview, vars)
		fmt.Fprintf(&sb, "【世界观】\n%s\n", content)
	}

	// ---- 4. 角色背景（替换占位符） ----
	if contract.Background != "" {
		content := shared.ReplacePlaceholders(contract.Background, vars)
		fmt.Fprintf(&sb, "【角色背景】\n%s\n", content)
	}

	// ---- 5. 开局场景（替换占位符） ----
	if contract.Opening != "" {
		content := shared.ReplacePlaceholders(contract.Opening, vars)
		fmt.Fprintf(&sb, "【开局场景】\n%s\n", content)
	}

	// ---- 6. 状态（不替换占位符，保持字面量） ----
	if len(state) > 0 {
		// 按契约中的顺序输出，运行时新增的键追加在末尾
		orderKeys := make([]string, 0, len(contract.State))
		for _, kv := range contract.State {
			orderKeys = append(orderKeys, kv.Key)
		}
		// 将运行时新增的键（不在 contract.State 中的）追加到 orderKeys
		for k := range state {
			found := false
			for _, ordered := range orderKeys {
				if k == ordered {
					found = true
					break
				}
			}
			if !found {
				orderKeys = append(orderKeys, k)
			}
		}
		stateKVs := shared.MapToKeyValues(state, orderKeys)

		fmt.Fprint(&sb, "【状态】\n")
		for _, kv := range stateKVs {
			fmt.Fprintf(&sb, "- %s: %v\n", kv.Key, kv.Value)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 7. 规则（动作替换占位符） ----
	if len(contract.Rules) > 0 {
		fmt.Fprint(&sb, "【规则】\n")
		for _, rule := range contract.Rules {
			action := shared.ReplacePlaceholders(rule.Action, vars)
			if rule.Group != "" {
				fmt.Fprintf(&sb, "[%s] if %s -> [group:%s] %s\n", rule.Name, rule.Cond, rule.Group, action)
			} else {
				fmt.Fprintf(&sb, "[%s] if %s -> %s\n", rule.Name, rule.Cond, action)
			}
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 8. 记忆（不替换，直接存储） ----
	if len(memories) > 0 {
		fmt.Fprint(&sb, "【记忆】\n")
		for _, mem := range memories {
			fmt.Fprintf(&sb, "- %s\n", mem)
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 9. 历史（不替换，直接存储，换行转义） ----
	if len(history) > 0 {
		fmt.Fprint(&sb, "【历史】\n")
		for _, entry := range history {
			content := strings.ReplaceAll(entry.Content, "\n", "\\n")
			fmt.Fprintf(&sb, "- %s: %s\n", entry.Role, content)
		}
		fmt.Fprint(&sb, "\n")
	}

	return sb.String()
}

// ============================================================
// 子版路径构建
// ============================================================

// BuildChildPath 构建子版文件路径。
//
// 命名规则：
//   - 默认：story.meph → story_child.meph
//   - 分支：--branch dark → story_dark.meph
//   - 如果母版本身已经是子版（含 _child 或 _分支名），直接覆盖
//
// 参数：
//   - filename: 母版文件路径
//   - branch:   分支名（空字符串表示默认子版）
//
// 返回值：
//   - string: 子版文件的完整路径
func BuildChildPath(filename string, branch string) string {
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// ---- 检查是否已经是子版文件（包含 _child 或任何 _xxx 后缀） ----
	if strings.HasSuffix(name, "_child") {
		return filename
	}

	// 检查是否包含任何下划线后缀（如 _dark, _light, _branch 等）
	lastUnderscore := strings.LastIndex(name, "_")
	if lastUnderscore != -1 && lastUnderscore < len(name)-1 {
		// 有下划线后缀，说明已经是子版文件，直接覆盖
		return filename
	}

	// ---- 构建子版路径 ----
	if branch != "" {
		return filepath.Join(dir, name+"_"+branch+ext)
	}

	return filepath.Join(dir, name+"_child"+ext)
}