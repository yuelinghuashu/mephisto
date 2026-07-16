package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// init 注册一个 mock 解析函数，避免导入 engine 造成循环依赖
// 测试中不需要真正的 AST 求值，所以返回 nil 即可
func init() {
	ParseExprFunc = func(string) (Expr, error) {
		return nil, nil
	}
}

// getTestDataPath 返回测试数据文件的绝对路径
// 它基于当前测试文件的目录（parser/）向上查找 data/ 目录
func getTestDataPath(filename string) string {
	// 获取当前文件的绝对路径
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("无法获取当前文件路径")
	}
	// 当前文件在 parser/ 下，返回 ../data/filename
	dir := filepath.Dir(currentFile)
	return filepath.Join(dir, "..", "data", filename)
}

// TestParseSample 契约测试：确保 sample.meph 能完整解析
func TestParseSample(t *testing.T) {
	samplePath := getTestDataPath("sample.meph")
	got, err := ParseFile(samplePath)
	if err != nil {
		t.Fatalf("解析 sample.meph 失败: %v", err)
	}

	goldenPath := getTestDataPath("sample.golden")
	var want ParsedFile
	if err := loadGolden(goldenPath, &want); err != nil {
		t.Logf("Golden 文件不存在，正在生成: %s", goldenPath)
		if err := saveGolden(goldenPath, got); err != nil {
			t.Fatalf("生成 golden 文件失败: %v", err)
		}
		t.Logf("✅ Golden 文件已生成，请检查内容后重新运行测试")
		t.FailNow()
	}

	opts := []cmp.Option{
		cmpopts.IgnoreFields(BlockEntry{}, "RuleExpr", "RuleAction"),
		cmpopts.EquateEmpty(),
	}

	diff := cmp.Diff(want, got, opts...)
	if diff != "" {
		t.Errorf("解析结果与预期不符 (-want +got):\n%s", diff)
		t.Logf("💡 如果更改是预期的，请更新 golden 文件：")
		t.Logf("   go test -update")
	}
}

// TestParseBlockTypes 测试每个区块的类型是否正确
func TestParseBlockTypes(t *testing.T) {
	samplePath := getTestDataPath("sample.meph")
	pf, err := ParseFile(samplePath)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	expectedTypes := map[string]BlockType{
		"角色名":  SingleLineText,
		"锚点":   KeyValueList,
		"世界观":  MultiLineText,
		"角色背景": MultiLineText,
		"开局场景": MultiLineText,
		"状态":   KeyValueList,
		"规则":   RuleList,
		"校验":   KeyValueList,
		"记忆":   KeyValueList,
	}

	for _, block := range pf.Blocks {
		spec, ok := BlockRegistry[block.Name]
		if !ok {
			t.Errorf("未知区块: %s", block.Name)
			continue
		}
		expected, ok := expectedTypes[block.Name]
		if !ok {
			t.Errorf("区块 %s 未在预期列表中", block.Name)
			continue
		}
		if spec.Type != expected {
			t.Errorf("区块 %s 类型错误: got %v, want %v", block.Name, spec.Type, expected)
		}
	}
}

// loadGolden 从 JSON 文件加载预期结果
func loadGolden(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// saveGolden 将解析结果保存为 JSON（用于首次生成）
func saveGolden(path string, pf *ParsedFile) error {
	clean := cleanParsedFile(pf)
	data, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// cleanParsedFile 清理 AST 字段，生成可序列化的快照
func cleanParsedFile(pf *ParsedFile) *ParsedFile {
	cleaned := &ParsedFile{
		Blocks:     make([]*ParsedBlock, len(pf.Blocks)),
		References: pf.References,
	}
	for i, block := range pf.Blocks {
		cleaned.Blocks[i] = &ParsedBlock{
			Name:    block.Name,
			Line:    block.Line,
			Raw:     block.Raw,
			Entries: make([]*BlockEntry, len(block.Entries)),
		}
		for j, entry := range block.Entries {
			cleaned.Blocks[i].Entries[j] = &BlockEntry{
				Type:  entry.Type,
				Key:   entry.Key,
				Value: entry.Value,
				Line:  entry.Line,
				// RuleExpr 和 RuleAction 被跳过
			}
		}
	}
	return cleaned
}

// TestMain 支持 -update 标志更新 golden 文件
func TestMain(m *testing.M) {
	for _, arg := range os.Args {
		if arg == "-update" {
			samplePath := getTestDataPath("sample.meph")
			pf, err := ParseFile(samplePath)
			if err != nil {
				panic(err)
			}
			goldenPath := getTestDataPath("sample.golden")
			if err := saveGolden(goldenPath, pf); err != nil {
				panic(err)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestParseInvalidFile(t *testing.T) {
	// 测试缺少必填区块【角色名】
	content := `【状态】\n- 情绪: 暴怒`
	tmpFile, err := os.CreateTemp("", "invalid_*.meph")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(content)
	tmpFile.Close()

	_, err = ParseFile(tmpFile.Name())
	if err == nil {
		t.Error("预期解析失败（缺少必填区块），但实际成功了")
	}
	// 可选：检查错误信息是否包含 "【角色名】区块内容不能为空"
}
