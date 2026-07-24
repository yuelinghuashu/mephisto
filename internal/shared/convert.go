// internal/shared/convert.go
//
// 共享工具：类型转换、键值对操作、占位符替换
package shared

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mephisto/internal/domain"
)

// ============================================================
// 类型解析
// ============================================================

// ParseValue 解析字符串值并推断类型。
//
// 类型推断规则：
//   - "true"/"false"（不区分大小写）→ bool
//   - 整数值（如 "42"）→ int
//   - 浮点数值（如 "3.14"）→ float64
//   - 其他 → string
//
// 引用处理：
//   - 如果值被双引号包裹（如 `"hello"`），自动去除引号
//   - 空字符串返回空字符串
func ParseValue(v string) any {
	// 去除首尾空白
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}

	// 尝试去除引号
	unquoted := Unquote(v)
	if unquoted != v {
		// 有引号，直接作为字符串返回（去除引号后的内容）
		return unquoted
	}

	// 检查布尔值
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}

	// 检查整数
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}

	// 检查浮点数
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}

	return v
}

// Unquote 去除字符串两端的引号（如果存在）。
func Unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// ============================================================
// 键值对操作
// ============================================================

// KeyValuesToMap 将 KeyValue 列表转为 map[string]string。
func KeyValuesToMap(kvs []domain.KeyValue) map[string]any {
	m := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = ParseValue(kv.Value)
	}
	return m
}

// MapToKeyValues 将 map 转为 KeyValue 列表，按指定键顺序输出。
func MapToKeyValues(m map[string]any, orderKeys []string) []domain.KeyValue {
	var kvs []domain.KeyValue
	seen := make(map[string]bool)

	// 按指定顺序输出
	for _, k := range orderKeys {
		if v, ok := m[k]; ok {
			kvs = append(kvs, domain.KeyValue{Key: k, Value: fmt.Sprintf("%v", v)})
			seen[k] = true
		}
	}

	// 输出剩余未输出的键
	for k, v := range m {
		if !seen[k] {
			kvs = append(kvs, domain.KeyValue{Key: k, Value: fmt.Sprintf("%v", v)})
		}
	}

	return kvs
}

// ============================================================
// 占位符替换
// ============================================================

// ReplacePlaceholders 替换字符串中的 {占位符}。
//
// 支持的占位符格式：
//   - {角色名} → 替换为角色名称
//   - {任意键} → 替换为 vars 中对应的值
//
// 参数：
//   - template: 包含占位符的模板字符串
//   - vars: 占位符映射表
//
// 返回值：
//   - string: 替换后的字符串（占位符不存在时保留原样）
func ReplacePlaceholders(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// ============================================================
// 变量映射构建
// ============================================================

// BuildPlaceholderVars 构建占位符替换用的变量映射。
//
// 总是包含以下变量：
//   - {角色名} → roleName
//
// 另外将 state 中的每个键值对作为占位符变量。
//
// 参数：
//   - roleName: 角色名
//   - state: 运行时状态（可以是 nil）
//
// 返回值：
//   - map[string]string: 占位符映射表
func BuildPlaceholderVars(roleName string, state map[string]any) map[string]string {
	vars := map[string]string{
		"角色名": roleName,
	}
	for k, v := range state {
		vars[k] = fmt.Sprintf("%v", v)
	}
	return vars
}

// ============================================================
// 语义去重（记忆）
// ============================================================

// DeduplicateMemories 对记忆列表进行语义去重。
//
// 去重策略：
//   - 完全相同的字符串 → 只保留一个
//   - 语义相似（共享至少 2 个相同关键词）→ 只保留较短的
//
// 参数：
//   - memories: 原始记忆列表
//
// 返回值：
//   - []string: 去重后的记忆列表
func DeduplicateMemories(memories []string) []string {
	if len(memories) <= 1 {
		return memories
	}

	// 第一步：精确去重
	unique := make([]string, 0, len(memories))
	seen := make(map[string]bool)
	for _, m := range memories {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}

	// 第二步：语义去重（关键词重合度过高则保留较短的）
	// 提取每个记忆的关键词
	type memoryEntry struct {
		text     string
		keywords []string
	}
	entries := make([]memoryEntry, 0, len(unique))
	for _, m := range unique {
		entries = append(entries, memoryEntry{
			text:     m,
			keywords: extractKeywords(m),
		})
	}

	result := make([]string, 0, len(entries))
	removed := make([]bool, len(entries))

	for i := 0; i < len(entries); i++ {
		if removed[i] {
			continue
		}
		for j := i + 1; j < len(entries); j++ {
			if removed[j] {
				continue
			}
			if similarityScore(entries[i].keywords, entries[j].keywords) >= 0.5 {
				// 保留较短的
				if len(entries[i].text) <= len(entries[j].text) {
					removed[j] = true
				} else {
					removed[i] = true
					break
				}
			}
		}
		if !removed[i] {
			result = append(result, entries[i].text)
		}
	}

	return result
}

// extractKeywords 从文本中提取关键词（中文分词简化版）。
// 提取长度 >= 2 的连续中文字符片段和非中文单词。
func extractKeywords(text string) []string {
	var keywords []string
	seen := make(map[string]bool)

	// 提取中文字符片段（中文按字划分，取 >= 2 个连续字）
	re := regexp.MustCompile(`[\p{Han}]{2,}`)
	matches := re.FindAllString(text, -1)
	for _, m := range matches {
		if !seen[m] {
			keywords = append(keywords, m)
			seen[m] = true
		}
	}

	return keywords
}

// similarityScore 计算两个关键词列表的相似度得分 [0, 1]。
func similarityScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	setB := make(map[string]bool, len(b))
	for _, kw := range b {
		setB[kw] = true
	}

	intersect := 0
	for _, kw := range a {
		if setB[kw] {
			intersect++
		}
	}

	// Jaccard 相似度
	union := len(a) + len(b) - intersect
	if union == 0 {
		return 0
	}
	return float64(intersect) / float64(union)
}