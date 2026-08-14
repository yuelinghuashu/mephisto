// internal/core/engine/save.go
//
// 存档管理：子版保存、加载、路径构建
//
// 本文件集中管理所有与 .meph 存档文件相关的操作：
//   - ChildData: 子版数据模型
//   - Save(): 保存当前会话为子版
//   - LoadChild(): 从文件加载子版
//   - LoadChildData(): 将加载的数据应用到 Runtime
//   - buildChildContentWithRules(): 构建 .meph 格式的子版内容
//   - BuildChildPath(): 子版文件路径计算
//
// 设计原则：
//   - 存档逻辑与引擎核心流程（Run）分离
//   - 所有文件 I/O 集中于此文件，便于测试和替换
//
// 子版命名规则（与 Flutter 版对齐，点分隔）：
//   - 默认：story.meph → story.child.meph
//   - 分支：story.meph + branch=dark → story.dark.meph
//   - 已是子版（文件名含 .child 或 .分支）→ 直接覆盖同名文件
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"mephisto/internal/core/parser"
	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// childSuffix 默认子版后缀（对齐 Flutter 的 defaultChildSuffix=".child"）。
const childSuffix = ".child"

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
// 命名规则（与 Flutter 版对齐，点分隔）：
//   - 默认：story.meph → story.child.meph
//   - 分支：--branch dark → story.dark.meph
//   - 已是子版：直接覆盖
//
// 外部实时编辑支持（方案 A）：
//   保存前先读取磁盘上的子版文件（若存在），以磁盘上的【规则】区块为最新规则。
//   这样用户在编辑器中对规则区块的实时修改不会被本层自动保存覆盖。
//   静态区块（锚点/世界观/背景等）、状态、记忆、历史仍由引擎内存快照重建。
//
// 返回值：
//   - error: 保存失败时的错误
func (e *Engine) Save(filename, branch string) error {
	path := BuildChildPath(filename, branch)

	// 若磁盘上已有子版，采用其规则（用户在外部实时编辑的最新规则）
	// 否则使用引擎当前内存中的规则（与 Run 使用同一份，保证一致性）
	latestRules := e.runtime.Contract().Rules
	if _, err := os.Stat(path); err == nil {
		if disk, err := parser.ParseFile(path); err == nil && len(disk.Rules) > 0 {
			latestRules = disk.Rules
		}
	}

	content := e.buildChildContentWithRules(latestRules)

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
		State:    contract.StateMap(),
		Memories: contract.Memories,
		History:  contract.History,
		Found:    true,
	}, nil
}

// ============================================================
// 子版内容构建（私有）
// ============================================================

// buildChildContentWithRules 构建子版文件内容，可指定规则来源。
//
// 输出格式与 .meph 契约文件完全一致，包含 9 个标准区块 + 1 个系统保留区块：
//
//	@命运（可选）、角色名、锚点、世界观、角色背景、开局场景、状态、规则、记忆、历史
//
// 变量替换策略：
//   - 替换：锚点、世界观、角色背景、开局场景、规则动作
//   - 不替换：状态（变量源）、记忆（已生成）、历史（已记录）
//
// 参数 rules 允许外部传入不同来源的规则（如磁盘上用户实时编辑的最新规则），
// 使保存时能保留用户在编辑器中的修改。
func (e *Engine) buildChildContentWithRules(rules []*domain.Rule) string {
	var sb strings.Builder

	contract := e.runtime.Contract()
	state := e.runtime.State()
	memories := e.runtime.Memories()
	history := e.runtime.History()

	// 构建变量映射（用于占位符替换）
	vars := shared.BuildPlaceholderVars(contract.RoleName, state)

	// ---- 0. @命运 系统门面区块（若有，置于文件最顶部，与 Flutter serializer 对齐）----
	if contract.BranchTitle != "" {
		fmt.Fprintf(&sb, "@命运\n%s\n\n", contract.BranchTitle)
	}

	// ---- 1. 角色名（不替换） ----
	fmt.Fprintf(&sb, "【角色名】\n%s\n\n", contract.RoleName)

	// ---- 2. 锚点（替换占位符） ----
	if len(contract.Anchor) > 0 {
		fmt.Fprint(&sb, "【锚点】\n")
		for _, kv := range contract.Anchor {
			value := shared.ReplacePlaceholders(fmt.Sprintf("%v", kv.Value.Raw()), vars)
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
		// 使用 map 查找将 O(n²) 降为 O(n)
		contractKeys := make(map[string]bool, len(orderKeys))
		for _, k := range orderKeys {
			contractKeys[k] = true
		}
		for k := range state {
			if !contractKeys[k] {
				orderKeys = append(orderKeys, k)
			}
		}
		stateKVs := shared.MapToStateItems(state, orderKeys)

		fmt.Fprint(&sb, "【状态】\n")
		for _, kv := range stateKVs {
			fmt.Fprintf(&sb, "- %s: %s\n", kv.Key, formatStateValue(kv.Value))
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 7. 规则（动作替换占位符） ----
	if len(rules) > 0 {
		fmt.Fprint(&sb, "【规则】\n")
		for _, rule := range rules {
			action := shared.ReplacePlaceholders(rule.Action, vars)
			if rule.Group != "" {
				fmt.Fprintf(&sb, "[%s] if %s -> [group:%s] %s\n", rule.Name, rule.Cond, rule.Group, action)
			} else {
				fmt.Fprintf(&sb, "[%s] if %s -> %s\n", rule.Name, rule.Cond, action)
			}
		}
		fmt.Fprint(&sb, "\n")
	}

	// ---- 8. 记忆（不替换，直接存储；保留/补齐 [权重] 前缀，与 Flutter 对齐） ----
	if len(memories) > 0 {
		fmt.Fprint(&sb, "【记忆】\n")
		for _, mem := range memories {
			fmt.Fprintf(&sb, "- %s\n", withMemoryWeightPrefix(mem))
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
// 辅助函数
// ============================================================

// formatStateValue 将状态值格式化为 .meph 文本（与 Flutter meph_serializer.dart 的 _formatStateValue 对齐）：
//   - 字符串带引号（避免被解析为数字/布尔）
//   - 数字/布尔直接输出
func formatStateValue(v domain.StateValue) string {
	switch t := v.Raw().(type) {
	case string:
		// 字符串值加双引号（与 Flutter serializer 对齐）
		escaped := strings.ReplaceAll(t, `"`, `\"`)
		return `"` + escaped + `"`
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", v.Raw())
	}
}

// memoryWeightPattern 匹配记忆条目开头的 `[权重] ` 前缀（如 `[4] `）。
var memoryWeightPattern = regexp.MustCompile(`^\[\d\]\s`)

// withMemoryWeightPrefix 确保记忆条目带 `[权重] ` 前缀（与 Flutter serializer 对齐）。
//
// - 已有 `[权重] ` 前缀：原样返回（保留原始权重，方案 B 不改写）
// - 无前缀的旧格式记忆：补齐默认权重 `[3] `（与 Flutter 的 Memory.defaultImportance=3 对齐）
func withMemoryWeightPrefix(mem string) string {
	if memoryWeightPattern.MatchString(mem) {
		return mem
	}
	return "[3] " + mem
}

// ============================================================
// 子版路径构建
// ============================================================

// BuildChildPath 构建子版文件路径。
//
// 命名规则（与 Flutter 版对齐，点分隔）：
//   - 默认：story.meph → story.child.meph
//   - 分支：--branch dark → story.dark.meph
//   - 已是子版（文件名含 `.child` 或 `.分支名`）→ 直接覆盖
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

	// ---- 已是子版：直接覆盖 ----
	// 识别规则：
	//   - 默认子版：`story.child.meph`（文件名含 `.child`）
	//   - 分支子版：`story.dark.meph`（文件名含 `.分支名` 且分支名以字母开头）
	if isChildFileName(name) {
		return filename
	}

	// ---- 构建子版路径（点分隔，对齐 Flutter） ----
	if branch != "" {
		return filepath.Join(dir, name+"."+branch+ext)
	}

	return filepath.Join(dir, name+childSuffix+ext)
}

// isChildFileName 判断文件名（不含扩展名）是否为子版文件。
//
// 规则（对齐 Flutter child_save_store.dart 的精准匹配）：
//   - 默认子版：`xxx.child`
//   - 分支子版：`xxx.分支名`（分支名以字母开头，排除纯数字序号）
//
// 这样 `my_story_1.meph`（数字序号）不会被误判为子版。
func isChildFileName(name string) bool {
	// 默认子版：.child 后缀
	if strings.HasSuffix(name, childSuffix) {
		return true
	}

	// 分支子版：最后一个点和字母分支名
	lastDot := strings.LastIndex(name, ".")
	if lastDot == -1 || lastDot == len(name)-1 {
		return false
	}
	suffixPart := name[lastDot+1:]
	if suffixPart == "" {
		return false
	}
	// 仅字母开头的后缀视为分支名（排除空/数字/下划线）
	first := suffixPart[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}