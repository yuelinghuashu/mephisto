<p align="center">
  <img src="assets/logo-dark.svg" alt="Mephisto Logo" width="200">
</p>

# 梅菲斯特（Mephisto）

> **长线叙事引擎 —— 用纯文本契约文件驱动规则与大模型**

<p align="center">
  <i>"梅菲斯特与你立约。"</i>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v0.5.0-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

---

## ✨ 这是什么？

梅菲斯特是一个**长线叙事引擎**。它读取纯文本的 `.meph` 契约文件，解析为结构化数据，驱动规则引擎执行条件逻辑，并调用大模型生成流式叙事。

**核心能力**：

- 📜 **契约驱动**：用 `【】` 定义区块，用 `-` 定义状态，用 `[规则]` 定义行为
- ⚡ **规则引擎**：条件判断、逻辑运算、骰子表达式（`roll(1d100)`）、互斥组
- 🧠 **LLM 集成**：流式输出、对话历史管理、后置校验
- 💾 **记忆编织**：智能提取关键事件、自动压缩摘要、长期记忆持久化
- 📂 **子版存档**：每轮自动保存，支持多分支故事线，直接运行子版自动覆盖
- 🎯 **命运视角**：你是“命运”（叙事的推动者），输入指令，驱动角色行动
- 🧪 **契约测试**：Golden File 测试保障解析器稳定性

---

## ⚖️ 立约（快速开始）

### 1. 铭刻密钥（配置 API Key）

```bash
# 在项目根目录创建 .env 文件
echo 'OPENAI_API_KEY=sk-你的OpenAI API密钥' > .env
```

### 2. 构建并运行

```bash
# 构建
go build -o mephisto ./cmd/mephisto

# 运行
./mephisto run data/sample.meph
```

### 3. 交互示例

```text
命运 > 你来到了光之国

贝利亚悬浮在光之国上空，俯视着下方戒备的战士们，发出一声低沉的冷笑：
“这么多年了，这光还是这么刺眼。”

奥特之父上前一步，沉声道：“贝利亚，光之国不会再次容忍你的暴行。”

贝利亚转过头，猩红的眼睛盯着对方：“你们的容忍，对我而言一文不值。”

💾 已保存子版: data/sample_child.meph
```

---

## 🎭 多分支故事线

梅菲斯特支持在同一故事世界中创建多个分支，探索不同的剧情走向。

```bash
# 默认子版（相当于"主线"）
./mephisto run data/sample.meph

# 指定分支
./mephisto -branch dark run data/sample.meph

# 直接加载分支文件（自动覆盖）
./mephisto run data/sample_dark.meph

# 忽略子版，从母版重新开始
./mephisto -reset run data/sample.meph
```

---

## 📚 原典释读（完整文档）

> 契约已立，当逐条宣读。

| 文档                                    | 说明                         |
| --------------------------------------- | ---------------------------- |
| **[语法手册](./docs/SYNTAX.md)**        | 如何书写契约（`.meph` 语法） |
| **[规则引擎深度解析](./docs/RULES.md)** | 驱动叙事的齿轮与法则         |
| **[实战示例](./data/sample.meph)**      | 贝利亚奥特曼的完整契约       |

---

## 🛠️ 命令行选项

```bash
./mephisto [全局选项] <子命令> [参数]

子命令:
  parse <文件路径> [选项]   # 解析 .meph 契约，输出 JSON
  run <文件路径> [选项]     # 启动交互式对话模式
  version                   # 显示版本信息
  help                      # 显示帮助信息

全局选项（可放在子命令之前）:
  -h, -help                显示帮助信息
  -q, -quiet               静默模式，只输出错误
  -branch <分支名>         分支名（用于多分支故事线，默认为空）
  -reset                   忽略子版存档，从母版重新开始

run 子命令选项:
  -m, -model <模型名>      LLM 模型名称（默认从 MEPHISTO_MODEL 读取）
  -client <类型>           LLM 客户端类型: deepseek, openai, ollama
  -api-key <密钥>          API 密钥（默认从 OPENAI_API_KEY 读取）
  -base-url <URL>          API 基础 URL（默认从 OPENAI_BASE_URL 读取）

parse 子命令选项:
  -o, -output <路径>       输出到文件（默认输出到 stdout）
  -q, -quiet               静默模式，只输出错误

环境变量:
  OPENAI_API_KEY           API 密钥（优先级低于命令行）
  OPENAI_BASE_URL          API 基础 URL
  MEPHISTO_MODEL           模型名称
  MEPHISTO_CLIENT          客户端类型（openai/ollama）
```

---

## 📁 项目结构

```text
mephisto/
├── cmd/
│   └── mephisto/          # CLI 入口
│       ├── main.go
│       ├── help.go
│       ├── output.go
│       ├── flags.go
│       ├── session.go
│       └── utils.go
├── internal/
│   ├── core/              # 核心层
│   │   ├── parser/        # 契约解析器
│   │   ├── engine/        # 规则引擎
│   │   ├── llm/           # LLM 客户端
│   │   └── validator/     # 契约验证器
│   ├── domain/            # 领域模型
│   └── shared/            # 共享工具
├── data/                  # 示例契约文件
├── docs/                  # 完整文档
└── assets/                # Logo 资源
```

---

## 📦 环境要求

- Go 1.23+（推荐 1.24+）

---

## 📝 更新日志

详见 [CHANGELOG.md](./CHANGELOG.md)

---

## 🎭 关于命名

梅菲斯特（Mephisto）源自歌德《浮士德》中的魔鬼 Mephistopheles。

在《浮士德》中，浮士德不断追求，却从未真正满足。直到临终前，他听见铁锹声，以为自己在为人民建造新世界，才说出那句 **“你真美呀，请停留一下”**，然后倒地死去。

梅菲斯特 **不是恶魔，不是守护者**。  
他是那个 **让叙事无法停下的机制**。

> **他设定条件，但从不阻止你前行。**  
> **他让你一直走下去，直到你主动说出：**  
> **“到此为止，我心满意足。”**

---

## 📄 License

MIT
