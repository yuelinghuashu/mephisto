<p align="center">
  <img src="assets/logo-dark.svg" alt="Mephisto Logo" width="120">
</p>

# 梅菲斯特（Mephisto）

> **纯文本契约解析器 —— 将 `.meph` / `.mephisto` 文件解析为结构化数据**
> 
> ⚠️ 项目处于早期开发阶段（v0.1），文件格式尚未最终稳定，后续版本可能有调整。

---

## 这是什么？

定义角色的纯文本文件（用 `【】` 分割区块），解析成程序可用的结构化数据。

## 能干什么？

- 读取 `.meph` / `.mephisto` 文件
- 按 `【区块名】` 分割内容
- 解析列表项（`- 键: 值`）和规则（`[名] if 条件 -> 动作`）
- 验证格式是否正确
- 精准报错（带文件行号）

## 关于命名

> *"梅菲斯特与你立约。"*

梅菲斯特（Mephisto）源自歌德《浮士德》中的魔鬼 Mephistopheles，象征**契约与交易**。

## 跑起来

```bash
go run main.go sample.meph
```

## 项目结构

```text
mephisto/
├── main.go
├── go.mod
├── README.md
├── LICENSE
├── CHANGELOG.md
├── assets/
│   └── logo.svg
├── data/
│   └── sample.meph
└── parser/
    ├── types.go
    ├── block.go
    ├── semantic.go
    └── validate.go
```

## 下一步计划

规则引擎 → 对话引擎 → 记忆编织系统

## License

MIT