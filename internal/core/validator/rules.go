// internal/core/validator/rules.go
//
// 具体的验证规则实现。
// 每个验证函数接收 *domain.Contract，返回 []ValidationError。
// 返回空切片表示该检查通过。
package validator

import (
	"fmt"
	"strings"

	"mephisto/internal/domain"
	"mephisto/internal/shared"
)

// validateRoleName 验证角色名。
//
// 规则：角色名为必填字段，不能为空字符串或仅包含空白字符。
// 原因：角色名是 Mephisto 的核心标识，没有角色名则无法构建叙事主体。
func validateRoleName(contract *domain.Contract) []ValidationError {
	var errs []ValidationError
	if strings.TrimSpace(contract.RoleName) == "" {
		errs = append(errs, ValidationError{
			Field:   "RoleName",
			Message: "角色名不能为空",
		})
	}
	return errs
}

// validateStateTypes 验证状态值的类型安全性。
//
// contract.State 是 []KeyValue（保持用户书写顺序），
// 每个值的类型由 shared.ParseValue 在运行时动态转换。
//
// 检查策略：
//  1. 如果值看起来像布尔值（true/false），ParseValue 会返回 bool
//  2. 如果值看起来像数字（整数或浮点数），ParseValue 会返回 int/float64
//  3. 否则返回字符串（原样保留）
//
// 由于 ParseValue 总是能成功解析（字符串总会返回 string），
// 不存在"解析失败"的情况，所以这个验证函数实际上是安全的。
// 但如果值包含不可见的控制字符或异常格式，这里可以提前警告。
//
// 当前实现：简单检查值是否为空字符串（允许任意非空字符串）。
// 未来如果引入更严格的值类型约束（如必须为数字），可在此扩展。
func validateStateTypes(contract *domain.Contract) []ValidationError {
	var errs []ValidationError

	if len(contract.State) == 0 {
		return errs
	}

	for _, kv := range contract.State {
		value := kv.Value

		// 空字符串是合法的（表示"未设置"）
		if value == "" {
			continue
		}

		// 通过 shared.ParseValue 检查是否能正确解析
		// 所有值都能被解析为 string/bool/int/float64，所以这里不报错
		// 但如果有特殊需求（如要求状态值必须为数字），可以在此扩展
		_ = shared.ParseValue(value)
	}

	return errs
}

// validateRules 验证规则完整性。
//
// 检查项：
//  1. 规则名不能为空
//  2. 条件不能为空
//  3. 动作不能为空
//
// 设计决策：不检查互斥组冲突。
//
//	互斥组的设计目的是容纳多条互斥规则（如 [攻击] 和 [防御] 在同一组），
//	运行时只会触发第一个匹配的规则。多条规则在同一组是正常设计，
//	不应作为验证错误或警告报告。
func validateRules(contract *domain.Contract) []ValidationError {
	var errs []ValidationError

	if len(contract.Rules) == 0 {
		return errs
	}

	for _, rule := range contract.Rules {
		// 规则名检查
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			errs = append(errs, ValidationError{
				Field:   "Name",
				Message: fmt.Sprintf("第 %d 行：规则名不能为空", rule.Line),
			})
			// 规则名为空时跳过后续检查，避免显示 '规则 "" 的条件不能为空'
			continue
		}

		// 条件检查
		if strings.TrimSpace(rule.Cond) == "" {
			errs = append(errs, ValidationError{
				Field:   "Cond",
				Message: fmt.Sprintf("第 %d 行：条件不能为空", rule.Line),
			})
		}

		// 动作检查
		if strings.TrimSpace(rule.Action) == "" {
			errs = append(errs, ValidationError{
				Field:   "Action",
				Message: fmt.Sprintf("第 %d 行：动作不能为空", rule.Line),
			})
		}
	}
	return errs
}
