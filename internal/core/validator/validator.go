// internal/core/validator/validator.go
//
// 契约验证：结构完整性和必填项检查。
//
// 职责范围（只做两件事）：
//  1. 角色名是否为空（必填）
//  2. 规则名/条件/动作是否为空（完整性）
//
// 不负责业务值验证（如堕落指数 0-100），那是引擎层的职责。
package validator

import (
	"fmt"
	"strings"

	"mephisto/internal/domain"
)

// ValidationError 表示一个验证错误。
type ValidationError struct {
	Field   string // 错误字段路径
	Message string // 错误描述
}

// Error 实现 error 接口。
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate 验证 Contract 的数据完整性。
//
// 返回值：[]ValidationError，空切片表示验证通过。
func Validate(contract *domain.Contract) []ValidationError {
	var errs []ValidationError

	// 1. 角色名必填
	if strings.TrimSpace(contract.RoleName) == "" {
		errs = append(errs, ValidationError{Field: "RoleName", Message: "角色名不能为空"})
	}

	// 2. 规则完整性检查
	for _, rule := range contract.Rules {
		if strings.TrimSpace(rule.Name) == "" {
			errs = append(errs, ValidationError{
				Field:   "Name",
				Message: fmt.Sprintf("第 %d 行：规则名不能为空", rule.Line),
			})
			continue
		}
		if strings.TrimSpace(rule.Cond) == "" {
			errs = append(errs, ValidationError{
				Field:   "Cond",
				Message: fmt.Sprintf("第 %d 行：条件不能为空", rule.Line),
			})
		}
		if strings.TrimSpace(rule.Action) == "" {
			errs = append(errs, ValidationError{
				Field:   "Action",
				Message: fmt.Sprintf("第 %d 行：动作不能为空", rule.Line),
			})
		}
	}

	return errs
}