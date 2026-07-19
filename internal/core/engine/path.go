// internal/core/engine/path.go
//
// 本文件提供子版文件路径构建的统一函数。
// saver.go 和 loader.go 共享此函数，避免重复代码。
package engine

import (
	"path/filepath"
	"strings"
)

// BuildChildPath 构建子版文件路径。
//
// 命名规则：
//   - 默认：story.meph → story_child.meph
//   - 分支：--branch dark → story_dark.meph
//   - 如果母版本身已经是子版（含 _child 或 _分支名），直接覆盖
//
// 参数：
//   - filename: 母版文件路径
//   - branch:   分支名（空字符串表示默认子版）
//
// 返回值：
//   - string: 子版文件的完整路径
// internal/core/engine/path.go

func BuildChildPath(filename string, branch string) string {
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// ---- 检查是否已经是子版文件（包含 _child 或任何 _xxx 后缀） ----
	if strings.HasSuffix(name, "_child") {
		return filename
	}

	// 检查是否包含任何下划线后缀（如 _dark, _light, _branch 等）
	lastUnderscore := strings.LastIndex(name, "_")
	if lastUnderscore != -1 && lastUnderscore < len(name)-1 {
		// 有下划线后缀，说明已经是子版文件，直接覆盖
		return filename
	}

	// ---- 构建子版路径 ----
	if branch != "" {
		return filepath.Join(dir, name+"_"+branch+ext)
	}

	return filepath.Join(dir, name+"_child"+ext)
}
