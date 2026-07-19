// internal/core/validator/validator.go
//
// 本文件提供契约验证功能。
//
// 验证器的职责边界：
//  1. 结构完整性检查   —— 必填字段是否存在（如角色名）
//  2. 类型安全检查     —— 值是否为合法类型（string/int/float/bool）
//  3. 规则完整性检查   —— 规则名、条件、动作是否为空
//
// 验证器不负责：
//   - 业务值范围检查   —— 生命值是否为正数、堕落指数是否在 0-100 等
//   - 业务逻辑验证     —— 规则互斥组是否冲突、规则条件是否合理等
//
// 为什么只做这些？
//
//	业务规则是项目特有的，会随需求变化。Mephisto 作为解析器，
//	只保证数据“格式正确”，不干涉“业务正确”。
//	具体的业务约束（如生命值必须 > 0）应由上层应用（Engine）自行校验。
package validator

import (
	"fmt"
	"mephisto/internal/domain"
)

// ValidationError 表示一个验证错误。
type ValidationError struct {
	Field   string // 错误字段的路径（如 "State.生命值"），便于定位
	Message string // 人类可读的错误描述
	Value   any    // 触发错误的实际值（可选），便于调试
}

// Error 实现 error 接口。
// 当 Value 不为空时，错误信息会包含实际值
func (e ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("%s: %s (当前值: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Result 表示验证结果。
type Result struct {
	Errors []ValidationError
}

// IsValid 返回验证是否通过（无错误）。
func (r Result) IsValid() bool {
	return len(r.Errors) == 0
}

// List 返回所有验证错误
func (r Result) List() []ValidationError {
	return r.Errors
}

// Validate 验证 Contract 的数据完整性。
//
// 执行顺序：
//  1. 角色名验证（必填）
//  2. 状态类型验证（值类型合法性）
//  3. 规则完整性验证（名称/条件/动作非空）
//
// 返回值：Result，包含所有验证错误（如有）。
func Validate(contract *domain.Contract) Result {
	var errors []ValidationError

	errors = append(errors, validateRoleName(contract)...)
	errors = append(errors, validateStateTypes(contract)...)
	errors = append(errors, validateRules(contract)...)

	return Result{Errors: errors}
}
