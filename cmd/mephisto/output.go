// cmd/mephisto/output.go
//
// 输出格式化 JSON 和 写入
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mephisto/internal/domain"
)

// serialize 将 Contract 序列化为 JSON 格式
func serialize(contract *domain.Contract) ([]byte, error) {
	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// writeOutput 将数据写入文件或标准输出
func writeOutput(data []byte, outputPath string, quiet bool) error {
	if outputPath == "" {
		fmt.Print(string(data))
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	if !quiet {
		fmt.Printf("✅ 已输出到 %s\n", outputPath)
	}
	return nil
}
