// internal/shared/errors.go
//
// 本文件定义 Mephisto 各层的自定义错误类型。
//
// 设计目标：
//  1. Parser 错误携带行号和区块名，便于精确定位
//  2. Engine 错误携带错误码，便于程序化处理
//  3. CLI 层通过 errors.As 判断类型，决定输出格式
//  4. VSCode 插件可以精确解析错误位置，无需字符串匹配
//
// 使用示例（Parser 层）：
//
//	return &ParseError{
//	    Line:      lineNumber,
//	    BlockName: "角色名",
//	    Message:   "角色名不能为空",
//	}
//
// 使用示例（Engine 层）：
//
//	return &EngineError{
//	    Code:    "EMPTY_INPUT",
//	    Message: "输入不能为空",
//	}
//
// 使用示例（CLI 层）：
//
//	var parseErr *ParseError
//	if errors.As(err, &parseErr) {
//	    // 输出包含行号的信息
//	}
package shared

import "fmt"

// ============================================================
// ParseError 解析错误
// ============================================================

// ParseError 表示解析 .meph 文件时发生的错误。
//
// 字段说明：
//   - Line     : 错误发生的绝对行号（从 1 开始），0 表示未知行号
//   - BlockName: 错误所在的区块名（可能为空字符串）
//   - Message  : 人类可读的错误描述
//   - Err      : 可选的下层包装错误（用于 errors.Unwrap）
//
// 与 fmt.Errorf 的区别：
//   - fmt.Errorf 将错误信息编码在字符串中，调用方必须用正则提取行号
//   - ParseError 将行号和区块名作为结构化字段，调用方可以直接读取
//
// 序列化说明：
//   - Error() 返回的字符串格式与原有 fmt.Errorf 保持一致：
//     "第 12 行（区块「角色名」）：角色名不能为空"
//   - 不携带行号的格式：
//     "没有有效区块"
type ParseError struct {
	Line      int    // 错误发生的绝对行号（1-based，0=未知）
	BlockName string // 区块名（可能为空）
	Message   string // 错误描述
	Err       error  // 包装的下层错误
}

func (e *ParseError) Error() string {
	if e.Line > 0 && e.BlockName != "" {
		return fmt.Sprintf("第 %d 行（区块「%s」）：%s", e.Line, e.BlockName, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("第 %d 行：%s", e.Line, e.Message)
	}
	return e.Message
}

func (e *ParseError) Unwrap() error { return e.Err }

// ============================================================
// EngineError 引擎运行时错误
// ============================================================

// EngineError 表示引擎运行时发生的错误。
//
// 字段说明：
//   - Code   : 错误码，如 "EMPTY_INPUT"、"LLM_FAILED"、"SAVE_FAILED"
//   - Message: 人类可读的错误描述
//   - Err    : 可选的下层包装错误（用于 errors.Unwrap）
//
// 错误码列表（约定）：
//   - EMPTY_INPUT  : 空输入
//   - LLM_GENERATE : LLM 生成失败
//   - LLM_EXTRACT  : 记忆提取/压缩失败
//   - SAVE_FAILED  : 保存子版失败
//   - LOAD_FAILED  : 加载子版失败
//
// 序列化说明：
//   - 无下层错误时：   "[EMPTY_INPUT] 输入不能为空"
//   - 有下层错误时：   "[SAVE_FAILED] 保存子版失败: 文件写入错误"
type EngineError struct {
	Code    string // 错误码
	Message string // 错误描述
	Err     error  // 包装的下层错误
}

func (e *EngineError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *EngineError) Unwrap() error { return e.Err }

