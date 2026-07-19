// internal/core/parser/parser.go
//
// 本文件是解析器包的对外入口。
// 它组合 lexer（区块切分）和 parseBlocks（结构化解析）两个阶段，
// 对外提供统一的 ParseFile / ParseString 接口。
//
// 使用示例：
//
//	contract, err := parser.ParseFile("data/sample.meph")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("角色名: %s\n", contract.RoleName)
package parser

import (
	"mephisto/internal/domain"
	"os"
)

// ParseFile 解析 .meph 文件。
//
// 参数：
//   - path: .meph 文件的路径
//
// 返回值：
//   - *domain.Contract: 解析后的契约数据
//   - error: 文件读取错误或解析错误
//
// 这是最常用的入口函数，适用于从文件系统加载契约的场景。
func ParseFile(path string) (*domain.Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseString(string(data))
}

// ParseString 解析 .meph 文本内容。
//
// 参数：
//   - text: .meph 格式的完整文本内容（UTF-8 编码的字符串）
//
// 返回值：
//   - *domain.Contract: 解析后的契约数据
//   - error: 解析错误
//
// 适用场景：
//   - 从数据库或网络加载的契约文本
//   - 单元测试中直接构造文本进行测试
//   - 需要从 io.Reader 读取时，可先读取为字符串再调用此函数
//
// 处理流程：
//  1. Lex(text) 将文本切分为区块列表 []Block
//  2. parseBlocks(blocks) 将区块列表解析为 *domain.Contract
//  3. 返回解析结果
//
// 错误处理：
//
//	任何阶段的错误都会直接返回，包含精确的行号和错误描述。
//	调用方可以根据错误信息向用户报告问题位置
func ParseString(text string) (*domain.Contract, error) {
	blocks, err := Lex(text)
	if err != nil {
		return nil, err
	}
	return parseBlocks(blocks)
}
