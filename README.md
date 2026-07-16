<p align="center">
  <img src="assets/logo-dark.svg" alt="Mephisto Logo" width="200">
</p>

# 梅菲斯特（Mephisto）

> **长线叙事引擎 —— 用纯文本契约文件驱动规则与大模型**

<p align="center">
  <i>"梅菲斯特与你立约。"</i>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v0.3.0-blue" alt="Version">
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
- 🎯 **命运视角**：你是“命运”（叙事的推动者），输入指令，驱动角色行动
- 🧪 **契约测试**：Golden File 测试保障解析器稳定性

---

## ⚖️ 立约（快速开始）

### 1. 铭刻密钥（配置 API Key）

```bash
# 在项目根目录创建 .env 文件
echo 'MEPHISTO_API_KEY=sk-你的DeepSeek密钥' > .env
```

### 2. 运行

```bash
go run main.go data/sample.meph
```

### 3. 交互示例

```text
（命运）: 贝利亚驾驶飞船前往光之国边境

📖 命运:
  黑色飞船撕裂宇宙空间，贝利亚站在驾驶舱中，
  赤红的双眼死死盯着前方逐渐放大的光之国度...

📖 叙事注入:
  贝利亚奥特曼的故乡是光之国，也是他最大的仇恨来源
  雷布朗多在你体内低语：力量才是唯一真理，贝利亚奥特曼
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
go run main.go [选项] <文件.meph>

选项:
  -api-key string      LLM API Key（或 .env）
  -model string        模型名称（默认 deepseek-chat）
  -base-url string     API Base URL（默认 https://api.deepseek.com/v1）
  -max-tokens int      最大生成 Token 数（默认 4096）
  -temperature float   温度值 0.0~2.0
  -debug               启用规则调试模式
  -quiet               安静模式（隐藏注入信息）
```

---

## 📁 项目结构

```text
mephisto/
├── main.go              # 程序入口
├── app/                 # 应用逻辑层
├── parser/              # 解析器
├── engine/              # 规则引擎
├── llm/                 # LLM 客户端
├── utils/               # 工具函数
├── data/                # 示例文件
├── docs/                # 完整文档
│   ├── SYNTAX.md        # 语法手册
│   ├── RULES.md         # 规则深度解析
│   └── EXAMPLES.md      # 实战示例
└── assets/              # Logo 资源
```

---

## 📦 环境要求

- Go 1.23+（推荐 1.26）

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
