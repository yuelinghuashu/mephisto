// internal/core/validator/validator_test.go
//
// 本文件包含验证器包的测试。
// 测试覆盖：
//  1. 有效契约（所有检查通过）
//  2. 角色名为空
//  3. 状态值非空（合法）
//  4. 规则条件/动作为空
//  5. 空状态/空规则（合法）
package validator

import (
	"testing"

	"mephisto/internal/domain"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		contract *domain.Contract
		wantErr  bool
	}{
		{
			name: "有效契约",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State: []domain.KeyValue{
					{Key: "位置", Value: "宇宙空间站"},
					{Key: "生命值", Value: "100"},
					{Key: "堕落指数", Value: "85"},
				},
				Rules: []*domain.Rule{
					{Name: "攻击", Cond: "true", Action: "攻击"},
				},
			},
			wantErr: false,
		},
		{
			name: "角色名为空",
			contract: &domain.Contract{
				RoleName: "",
				State:    []domain.KeyValue{},
			},
			wantErr: true,
		},
		{
			name: "状态包含空值（合法）",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State: []domain.KeyValue{
					{Key: "空状态", Value: ""},
				},
			},
			wantErr: false,
		},
		{
			name: "状态包含特殊字符（合法）",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State: []domain.KeyValue{
					{Key: "消息", Value: "包含 空格 和 标点！"},
				},
			},
			wantErr: false,
		},
		{
			name: "规则条件为空",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State:    []domain.KeyValue{},
				Rules: []*domain.Rule{
					{Name: "攻击", Cond: "", Action: "攻击"},
				},
			},
			wantErr: true,
		},
		{
			name: "规则动作为空",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State:    []domain.KeyValue{},
				Rules: []*domain.Rule{
					{Name: "攻击", Cond: "true", Action: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "无规则（合法）",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State:    []domain.KeyValue{},
				Rules:    nil,
			},
			wantErr: false,
		},
		{
			name: "无状态（合法）",
			contract: &domain.Contract{
				RoleName: "贝利亚奥特曼",
				State:    nil,
				Rules:    []*domain.Rule{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Validate(tt.contract)
			if result.IsValid() == tt.wantErr {
				t.Errorf("Validate() isValid = %v, wantErr %v\n%v", result.IsValid(), tt.wantErr, result.List())
			}
		})
	}
}
