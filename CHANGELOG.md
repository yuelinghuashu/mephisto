# 更新日志

本文件记录梅菲斯特（Mephisto）项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v0.1.0] — 2026-07-15

### 🎉 首次发布

梅菲斯特解析器首个版本发布，实现了完整的 `.meph` / `.mephisto` 文件解析能力。

### ✨ 新增

- **区块分割器** (`block.go`)
  - 按 `【区块名】` 分割文件
  - UTF-8 BOM 自动处理
  - 标题格式验证（多括号检测、尾部文字检测）
  - 区块白名单（8 个标准区块）
  - 注释行保留（维持行号对齐）
  - 重复区块检测
  - 必填区块检测（`【角色名】`）
  - 空区块检测（`【角色名】`、`【世界观】`、`【角色背景】`）

- **语义解析器** (`semantic.go`)
  - 列表项解析（`- 键: 值`，支持中英文冒号）
  - 规则解析（`[名称] if 条件 -> 动作`）
  - 多行自由文本解析（保留段落格式）
  - 文本区块保留空行，结构化区块过滤空行
  - 绝对行号（错误提示精准定位文件位置）

- **类型验证器** (`validate.go`)
  - 声明式区块校验（基于 `BlockRegistry`）
  - 4 种区块类型：`SingleLineText`、`MultiLineText`、`KeyValueList`、`RuleList`
  - 类型混入检测（文本区块不能有列表/规则）

- **入口程序** (`main.go`)
  - 文件扩展名验证（`.meph` / `.mephisto`）
  - 友好的格式化输出
  - 解析统计信息

- **Go 1.26 优化**
  - 使用 `strings.Lines` 按行迭代
  - 使用 `strings.SplitSeq` 分割行
  - 使用 `strings.FieldsSeq` 扫描外部引用

### 📝 文档

- `README.md` — 项目说明与快速开始
- `LICENSE` — MIT 许可证
- `CHANGELOG.md` — 更新日志
