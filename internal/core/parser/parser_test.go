// internal/core/parser/parser_test.go
//
// 本文件包含解析器包的测试。
// 采用 Golden File 测试模式：
//   - 首次运行自动生成 testdata/sample.golden
//   - 后续运行对比解析结果与 golden 文件
//   - 运行 `go test -update` 更新 golden 文件
//
// 测试覆盖：
//  1. 完整 .meph 文件解析（集成测试，使用 Golden File）
//  2. 各类格式错误的报错信息（内联测试）
//  3. 关键解析函数的独立单元测试（内联测试）
package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"mephisto/internal/domain"
)

// ============================================================
// Golden File 加载/保存
// ============================================================

// loadGolden 从 JSON 文件加载预期结果。
func loadGolden(path string, target *domain.Contract) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// saveGolden 将解析结果保存为 JSON（用于首次生成或更新）。
func saveGolden(path string, contract *domain.Contract) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false) // 防止 & < > 被转义为 \u0026 \u003c \u003e
	return encoder.Encode(contract)
}

// ============================================================
// 集成测试：Golden File 模式
// ============================================================

// TestParseSample 契约测试：确保 sample.meph 能完整解析。
// 首次运行会自动生成 testdata/sample.golden。
// 后续运行对比解析结果与 golden 文件。
// 运行 `go test -update` 更新 golden 文件。
func TestParseSample(t *testing.T) {
	samplePath := filepath.Join("testdata", "sample.meph")
	got, err := ParseFile(samplePath)
	if err != nil {
		t.Fatalf("解析 sample.meph 失败: %v", err)
	}
	goldenPath := filepath.Join("testdata", "sample.golden")

	var want domain.Contract

	if err := loadGolden(goldenPath, &want); err != nil {
		// Golden 文件不存在，自动生成
		t.Logf("Golden 文件不存在，正在生成: %s", goldenPath)
		if err := saveGolden(goldenPath, got); err != nil {
			t.Fatalf("生成 golden 文件失败: %v", err)
		}
		t.Logf("✅ Golden 文件已生成，请检查内容后重新运行测试")
		t.FailNow()
	}

	opts := []cmp.Option{
		cmpopts.EquateEmpty(),
		cmp.Transformer("NormalizeNumbers", func(m map[string]any) map[string]any {
			result := make(map[string]any)
			for k, v := range m {
				switch val := v.(type) {
				case int:
					result[k] = float64(val)
				case float64:
					result[k] = val
				default:
					result[k] = val
				}
			}
			return result
		}),
	}

	diff := cmp.Diff(want, *got, opts...)
	if diff != "" {
		t.Errorf("解析结果与预期不符 (-want +got):\n%s", diff)
		t.Logf("💡 如果更改是预期的，请运行: go test -update")
	}
}

// ============================================================
// TestMain：支持 -update 标志
// ============================================================

func TestMain(m *testing.M) {
	for _, arg := range os.Args {
		if arg == "-update" {
			// 重新生成 golden 文件（使用 parser 包自己的 testdata）
			samplePath := filepath.Join("testdata", "sample.meph")
			contract, err := ParseFile(samplePath)
			if err != nil {
				panic("解析 sample.meph 失败: " + err.Error())
			}
			goldenPath := filepath.Join("testdata", "sample.golden")
			if err := saveGolden(goldenPath, contract); err != nil {
				panic("保存 golden 文件失败: " + err.Error())
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

// ============================================================
// 错误场景测试（内联）
// ============================================================

// TestParseErrors 测试各类格式错误能否正确报错。
func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string // 错误信息应包含的子串
	}{
		{
			name:    "区块外有内容",
			input:   "这是区块外的内容\n【角色名】\n浮士德",
			wantErr: "内容出现在任何区块之外",
		},
		{
			name:    "列表项缺少 - 前缀",
			input:   "【锚点】\n核心信念：\"力量就是一切\"",
			wantErr: "列表项必须以 '-' 开头",
		},
		{
			name:    "列表项缺少冒号",
			input:   "【锚点】\n- 核心信念 \"力量就是一切\"",
			wantErr: "缺少 ':' 或 '：'",
		},
		{
			name:    "键不能为空",
			input:   "【锚点】\n- ：力量就是一切",
			wantErr: "键不能为空",
		},
		{
			name:    "规则缺少 [",
			input:   "【规则】\n攻击 if 包含 \"攻击\" -> 攻击",
			wantErr: "必须以 '[' 开头",
		},
		{
			name:    "规则名不能为空",
			input:   "【规则】\n[] if 包含 \"攻击\" -> 攻击",
			wantErr: "规则名不能为空",
		},
		{
			name:    "规则缺少 ->",
			input:   "【规则】\n[攻击] if 包含 \"攻击\" 攻击",
			wantErr: "缺少 '->'",
		},
		{
			name:    "规则缺少 if",
			input:   "【规则】\n[攻击] 包含 \"攻击\" -> 攻击",
			wantErr: "规则格式错误",
		},
		{
			name:    "历史角色无效",
			input:   "【历史】\n- system: 系统提示",
			wantErr: "历史条目必须以 'fate:' 或 'assistant:' 开头",
		},
		{
			name:    "历史缺少冒号",
			input:   "【历史】\n- fate 你走入森林",
			wantErr: "历史条目必须以 'fate:' 或 'assistant:' 开头",
		},
		{
			name:    "空角色名区块",
			input:   "【角色名】\n\n【锚点】\n- 核心信念：力量",
			wantErr: "角色名不能为空",
		},
		{
			name:    "规则条件为空",
			input:   "【规则】\n[攻击] if  -> 攻击",
			wantErr: "条件或动作不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(tt.input)
			if err == nil {
				t.Errorf("期望错误但未返回")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("错误信息不包含 '%s'\n实际: %v", tt.wantErr, err)
			}
		})
	}
}

// ============================================================
// 单元测试：公共解析函数
// ============================================================

// TestParseKeyValue 测试 parseKeyValue 函数。
func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		name      string
		lines     []Line
		blockName string
		want      []domain.KeyValue
		wantErr   bool
	}{
		{
			name: "正常解析",
			lines: []Line{
				{Text: "- 键1: 值1", Number: 1},
				{Text: "- 键2：值2", Number: 2},
			},
			blockName: "锚点",
			want: []domain.KeyValue{
				{Key: "键1", Value: "值1"},
				{Key: "键2", Value: "值2"},
			},
			wantErr: false,
		},
		{
			name: "跳过空行和注释",
			lines: []Line{
				{Text: "  ", Number: 1},
				{Text: "# 这是注释", Number: 2},
				{Text: "- 键: 值", Number: 3},
			},
			blockName: "锚点",
			want: []domain.KeyValue{
				{Key: "键", Value: "值"},
			},
			wantErr: false,
		},
		{
			name: "缺少 - 前缀",
			lines: []Line{
				{Text: "键: 值", Number: 1},
			},
			blockName: "锚点",
			want:      nil,
			wantErr:   true,
		},
		{
			name: "键为空",
			lines: []Line{
				{Text: "- : 值", Number: 1},
			},
			blockName: "锚点",
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKeyValue(tt.lines, tt.blockName)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseKeyValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("结果不匹配 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParsePlainList 测试 parsePlainList 函数。
func TestParsePlainList(t *testing.T) {
	tests := []struct {
		name      string
		lines     []Line
		blockName string
		want      []string
		wantErr   bool
	}{
		{
			name: "正常解析",
			lines: []Line{
				{Text: "- 条目1", Number: 1},
				{Text: "- 条目2", Number: 2},
			},
			blockName: "记忆",
			want:      []string{"条目1", "条目2"},
			wantErr:   false,
		},
		{
			name: "跳过空行和注释",
			lines: []Line{
				{Text: "  ", Number: 1},
				{Text: "# 注释", Number: 2},
				{Text: "- 条目", Number: 3},
			},
			blockName: "记忆",
			want:      []string{"条目"},
			wantErr:   false,
		},
		{
			name: "缺少 - 前缀",
			lines: []Line{
				{Text: "条目", Number: 1},
			},
			blockName: "记忆",
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePlainList(tt.lines, tt.blockName)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePlainList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("结果不匹配 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParseRuleLine 测试 parseRuleLine 函数。
func TestParseRuleLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		lineNumber int
		blockName  string
		want       *domain.Rule
		wantErr    bool
	}{
		{
			name:       "正常规则",
			line:       `[攻击] if 包含 "攻击" -> 全力攻击`,
			lineNumber: 1,
			blockName:  "规则",
			want: &domain.Rule{
				Name:   "攻击",
				Cond:   `包含 "攻击"`,
				Action: "全力攻击",
				Group:  "",
				Line:   1,
			},
			wantErr: false,
		},
		{
			name:       "带互斥组的规则",
			line:       `[攻击] if 包含 "攻击" -> [group:combat] 全力攻击`,
			lineNumber: 1,
			blockName:  "规则",
			want: &domain.Rule{
				Name:   "攻击",
				Cond:   `包含 "攻击"`,
				Action: "全力攻击",
				Group:  "combat",
				Line:   1,
			},
			wantErr: false,
		},
		{
			name:       "规则缺少 if",
			line:       `[攻击] 包含 "攻击" -> 攻击`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "缺少 ]",
			line:       `[攻击 if 条件 -> 动作`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "规则名为空",
			line:       `[] if 条件 -> 动作`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "缺少 if",
			line:       `[攻击] 条件 -> 动作`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "缺少 ->",
			line:       `[攻击] if 条件 动作`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "条件为空",
			line:       `[攻击] if -> 动作`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "动作为空",
			line:       `[攻击] if 条件 ->`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
		{
			name:       "规则名被条件中的 ] 误闭合",
			line:       `[攻击 if 包含 "武器]" -> 攻击`,
			lineNumber: 1,
			blockName:  "规则",
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRuleLine(tt.line, tt.lineNumber, tt.blockName)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRuleLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("结果不匹配 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParseHistory 测试 parseHistory 函数。
func TestParseHistory(t *testing.T) {
	tests := []struct {
		name      string
		lines     []Line
		blockName string
		want      []domain.HistoryEntry
		wantErr   bool
	}{
		{
			name: "正常解析",
			lines: []Line{
				{Text: "- fate: 你走入森林", Number: 1},
				{Text: "- assistant: 我谨慎前行", Number: 2},
			},
			blockName: "历史",
			want: []domain.HistoryEntry{
				{Role: "fate", Content: "你走入森林"},
				{Role: "assistant", Content: "我谨慎前行"},
			},
			wantErr: false,
		},
		{
			name: "内容包含转义换行",
			lines: []Line{
				{Text: "- fate: 第一行\\n第二行", Number: 1},
			},
			blockName: "历史",
			want: []domain.HistoryEntry{
				{Role: "fate", Content: "第一行\n第二行"},
			},
			wantErr: false,
		},
		{
			name: "跳过空行和注释",
			lines: []Line{
				{Text: "  ", Number: 1},
				{Text: "# 注释", Number: 2},
				{Text: "- fate: 内容", Number: 3},
			},
			blockName: "历史",
			want: []domain.HistoryEntry{
				{Role: "fate", Content: "内容"},
			},
			wantErr: false,
		},
		{
			name: "角色无效",
			lines: []Line{
				{Text: "- system: 系统提示", Number: 1},
			},
			blockName: "历史",
			want:      nil,
			wantErr:   true,
		},
		{
			name: "缺少 - 前缀",
			lines: []Line{
				{Text: "fate: 内容", Number: 1},
			},
			blockName: "历史",
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHistory(tt.lines, tt.blockName)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHistory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("结果不匹配 (-want +got):\n%s", diff)
			}
		})
	}
}

// TestParseTextBlock 测试 parseTextBlock 函数。
func TestParseTextBlock(t *testing.T) {
	lines := []Line{
		{Text: "第一行", Number: 1},
		{Text: "第二行", Number: 2},
		{Text: "第三行", Number: 3},
	}
	want := "第一行\n第二行\n第三行"
	got := parseTextBlock(lines)
	if got != want {
		t.Errorf("parseTextBlock() 期望 '%s'，实际 '%s'", want, got)
	}
}
