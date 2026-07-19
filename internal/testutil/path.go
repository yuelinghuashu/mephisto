// internal/testutil/path.go
package testutil

import (
	"path/filepath"
	"runtime"
)

// GetTestDataPath 返回测试数据文件的绝对路径。
func GetTestDataPath(filename string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("无法获取当前文件路径")
	}
	dir := filepath.Dir(currentFile)
	return filepath.Join(dir, "testdata", filename)
}
