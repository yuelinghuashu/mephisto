// internal/domain/statevalue.go
//
// 类型化状态值（对齐 Flutter 的 StateValue sealed class）。
// 解析器在解析 .meph 时即推断类型并构造对应子类型，
// 避免引擎在运行时二次推断类型。
package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ============================================================
// StateValue 接口
// ============================================================

// StateValue 表示一个类型安全的状态值。
//
// 类型体系（对齐 Flutter 的 sealed class StateValue）：
//   - IntValue    ：整数（如 100）
//   - DoubleValue ：浮点数（如 85.5）
//   - BoolValue   ：布尔（如 true）
//   - StringValue ：字符串（如 "书斋"）
type StateValue interface {
	// Raw 返回底层 Go 值（int / float64 / bool / string）
	Raw() any
	// Kind 返回类型名（int / double / bool / string）
	Kind() string
	// String 返回可读文本
	String() string
}

// ============================================================
// 具体类型实现
// ============================================================

// IntValue 整数状态值
type IntValue struct{ Value int }

func (v *IntValue) Raw() any     { return v.Value }
func (v *IntValue) Kind() string { return "int" }
func (v *IntValue) String() string { return strconv.Itoa(v.Value) }

// MarshalJSON 整数直接序列化
func (v *IntValue) MarshalJSON() ([]byte, error) { return json.Marshal(v.Value) }

// DoubleValue 浮点状态值
type DoubleValue struct{ Value float64 }

func (v *DoubleValue) Raw() any      { return v.Value }
func (v *DoubleValue) Kind() string  { return "double" }
func (v *DoubleValue) String() string { return strconv.FormatFloat(v.Value, 'f', -1, 64) }

// MarshalJSON 浮点直接序列化
func (v *DoubleValue) MarshalJSON() ([]byte, error) { return json.Marshal(v.Value) }

// BoolValue 布尔状态值
type BoolValue struct{ Value bool }

func (v *BoolValue) Raw() any      { return v.Value }
func (v *BoolValue) Kind() string  { return "bool" }
func (v *BoolValue) String() string { return strconv.FormatBool(v.Value) }

// MarshalJSON 布尔直接序列化
func (v *BoolValue) MarshalJSON() ([]byte, error) { return json.Marshal(v.Value) }

// StringValue 字符串状态值
type StringValue struct{ Value string }

func (v *StringValue) Raw() any      { return v.Value }
func (v *StringValue) Kind() string  { return "string" }
func (v *StringValue) String() string { return v.Value }

// MarshalJSON 字符串直接序列化
func (v *StringValue) MarshalJSON() ([]byte, error) { return json.Marshal(v.Value) }

// ============================================================
// StateItem：键 + 类型化值
// ============================================================

// StateItem 表示一个类型化键值对（用于锚点、状态等区块）。
type StateItem struct {
	Key   string
	Value StateValue
}

// MarshalJSON 序列化为与旧版兼容格式：`{"key":"生命值","value":100}`
func (s StateItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}{Key: s.Key, Value: s.Value.Raw()})
}

// UnmarshalJSON 反序列化并推断类型（数字→int/double，布尔→bool，其他→string）
func (s *StateItem) UnmarshalJSON(data []byte) error {
	var raw struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Key = raw.Key

	var v any
	if err := json.Unmarshal(raw.Value, &v); err != nil {
		return err
	}

	switch t := v.(type) {
	case float64:
		if t == float64(int(t)) {
			s.Value = &IntValue{Value: int(t)}
		} else {
			s.Value = &DoubleValue{Value: t}
		}
	case bool:
		s.Value = &BoolValue{Value: t}
	case string:
		s.Value = &StringValue{Value: t}
	default:
		s.Value = &StringValue{Value: fmt.Sprintf("%v", v)}
	}
	return nil
}

// ============================================================
// 字符串 → StateValue 类型推断
// ============================================================

// ParseStateValue 解析字符串值并推断类型（对齐 Flutter 的 parseStateValue）。
//
// 类型推断规则：
//   - "true"/"false"（不区分大小写）→ BoolValue
//   - 整数值（如 "42"）→ IntValue
//   - 浮点数值（如 "3.14"）→ DoubleValue
//   - 其他 → StringValue
//
// 引用处理：
//   - 值被双引号/单引号包裹 → 去除引号作为字符串
//   - 空字符串 → 空字符串
func ParseStateValue(v string) StateValue {
	v = strings.TrimSpace(v)
	if v == "" {
		return &StringValue{Value: ""}
	}

	// 尝试去除引号
	unquoted := Unquote(v)
	if unquoted != v {
		return &StringValue{Value: unquoted}
	}

	// 检查布尔
	switch v {
	case "true", "TRUE", "True":
		return &BoolValue{Value: true}
	case "false", "FALSE", "False":
		return &BoolValue{Value: false}
	}

	// 检查整数
	if i, err := strconv.Atoi(v); err == nil {
		return &IntValue{Value: i}
	}

	// 检查浮点数
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return &DoubleValue{Value: f}
	}

	return &StringValue{Value: v}
}

// Unquote 去除字符串两端的匹配引号（如果存在）。
func Unquote(s string) string {
	if len(s) >= 2 &&
		((s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// FromRaw 从底层 Go 值恢复到 StateValue（用于引擎运行时状态转回类型化值）。
//
// 支持的类型：
//   - int / int64 → IntValue
//   - float64 → DoubleValue
//   - bool → BoolValue
//   - string → StringValue
//   - 其他 → StringValue（fmt 格式化）
func FromRaw(v any) StateValue {
	switch t := v.(type) {
	case int:
		return &IntValue{Value: t}
	case int64:
		return &IntValue{Value: int(t)}
	case float64:
		return &DoubleValue{Value: t}
	case bool:
		return &BoolValue{Value: t}
	case string:
		return &StringValue{Value: t}
	default:
		return &StringValue{Value: fmt.Sprintf("%v", v)}
	}
}
