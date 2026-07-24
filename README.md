<p align="center">
  <img src="assets/logo-dark.svg" alt="Mephisto Logo" width="200">
</p>

# 梅菲斯特（Mephisto）

> **长线叙事引擎 —— 用纯文本契约文件驱动规则与大模型**

<p align="center">
  <i>"梅菲斯特与你立约。"</i>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v1.0.2-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

---

## ✨ 这是什么？

梅菲斯特是一个**长线叙事引擎**。它读取纯文本的 `.meph` 契约文件，解析为结构化数据，驱动规则引擎执行条件逻辑，并调用大模型生成流式叙事。

**核心能力**：

- 📜 **契约驱动**：用 `【】` 定义区块，用 `-` 定义状态，用 `[规则]` 定义行为
- ⚡ **规则引擎**：条件判断、逻辑运算、骰子表达式（`roll(1d100)`）、自定义阈值（`roll(1d100) >= 80`）、互斥组、复合赋值（`状态.堕落指数 += 10`）、两阶段执行（被动规则批量执行 + 主动规则互斥匹配）
- 🧠 **LLM 集成**：骰子结果影响叙事生成、流式输出、对话历史管理
- 💾 **记忆编织**：智能提取关键事件、自动压缩摘要、长期记忆持久化
- 📂 **子版存档**：每轮自动保存，支持多分支故事线，直接运行子版自动覆盖
- 🎯 **命运视角**：你是"命运"（叙事的推动者），输入指令，驱动角色行动
- 🎲 **骰子透明**：每次判定结果实时展示给用户，规则名 + 点数 + ✅/❌ 一目了然
- 🔌 **VS Code 扩展**：语法高亮、实时诊断、代码补全、悬停提示、格式化、大纲视图，[一键安装](https://marketplace.visualstudio.com/items?itemName=yuelinghuashu.vscode-mephisto)

---

## ⚖️ 立约（快速开始）

### 1. 配置大模型 API

Mephisto 支持三种 LLM 后端，选择其中一种即可。

#### 选项 A：DeepSeek（推荐，开箱即用）

在项目根目录创建 `.env` 文件：

```bash
# 选择客户端类型和模型
MEPHISTO_CLIENT=openai
MEPHISTO_MODEL=deepseek-v4-flash

# 【必填】填入你的 DeepSeek API Key
OPENAI_API_KEY=sk-你的DeepSeek密钥

# 【可选】API 基础 URL（使用官方服务时无需修改）
# OPENAI_BASE_URL=https://api.deepseek.com/v1
```

#### 选项 B：OpenAI

```bash
# 选择客户端类型和模型
MEPHISTO_CLIENT=openai
MEPHISTO_MODEL=gpt-4o-mini

# 【必填】填入你的 OpenAI API Key
OPENAI_API_KEY=sk-你的OpenAI密钥

# 【可选】API 基础 URL（使用官方服务时无需修改）
# OPENAI_BASE_URL=https://api.openai.com/v1
```

#### 选项 C：Ollama（完全离线，免费）

```bash
# 选择客户端类型和本地模型
MEPHISTO_CLIENT=ollama
MEPHISTO_MODEL=llama3.2

# 无需 API Key
# 确保 Ollama 服务已启动：ollama serve
```

> **配置说明**：你也可以直接在命令行中传入这些参数，优先级为：**命令行参数 > 环境变量 > `.env` 文件**。

### 2. 构建并运行

```bash
# 构建
go build -o ./mephisto ./cmd/mephisto

# 运行
./mephisto run data/sample.meph
```

### 3. 交互示例

```text
命运 > 你想要获取超越人类认知的知识

　　书斋里的烛火摇曳，将满墙的羊皮卷和典籍映照出深浅不一的阴影。浮士德枯坐在堆满手稿的书桌前，指尖摩挲着一本破旧星象书的封面，目光却穿过窗棂，望向一片漆黑的夜空。他低声自语，声音沙哑得几乎被风吹散："追寻了一生，终究连一扇门也未曾推开。"

　　这时，身后的书架间传来一声极轻的响动，像老鼠啃咬木屑，又像一声压抑的嗤笑。浮士德并未回头，只是冷冷问道："又是你吗，瓦格纳？夜已经深了，不必再来送热汤。"

　　脚步声却很轻，轻得不像那个笨拙的弟子。一个低沉的、带着金属质感的声音从暗处响起："瓦格纳只配为你添柴烧水，我带来的，是另一种暖意。"说话者从阴影中缓步走出——他穿着华丽的猩红长袍，面容清瘦，嘴角挂着似笑非笑的弧度，手中把玩着一枚古铜色的戒指。他站定在书桌前，微微倾身，目光直刺浮士德的双瞳："你方才说，穷尽一生也推不开那扇门。可你有没有想过，门根本不是用来推的？"

　　浮士德缓缓抬起头，盯住这位不速之客。他的手指按住那本星象书，沉声道："你是何人？未经允准，擅入我的书斋。"

　　那人轻轻一笑，将戒指在烛光下转了转，戒指竟投出一片扭曲的影子，仿佛是某种无法言说的符文。"我是你所有问题的答案，也是你所有渴望的代价。"他伸出一只手，掌心向上，五指张开，掌纹中隐约流转着暗红色的光，"我可以让你看见星辰背后的纹路，可以让你听见创世之初的旋律，可以让你触碰法则本身。只要你允许我带走一件微不足道的东西。"

　　浮士德站起身，衣袖拂过桌面上散落的草稿纸，那些写满公式和推演的纸张飘落一地。他盯着那只递来的手，沉默了许久，才开口道："你要什么？"

　　那人弯起嘴角，声音轻柔得像羽毛划过刀锋："你的灵魂。不过请放心，那东西你平日里也用不上——它既不能帮你解开方程，也不能让你飞上苍穹。你留着它，不过是让日渐腐朽的肉体多一块赘肉罢了。"他收回手，转而从怀中取出一卷漆黑的羊皮纸，摊开在桌面上。纸面上没有字迹，只有一片深不见底的暗色，仿佛能吞噬周围的光线。"签下它，我便立即兑现一切。"

　　浮士德的呼吸变得急促，他低头看着那片漆黑，又抬头看向那人的眼睛——那双眼睛里倒映着无数星辰的陨落与诞生。他终于伸出手，指尖触到羊皮纸的瞬间，一股凉意顺着指骨爬上肩头。他没有再犹豫，接过那人递来的羽毛笔，笔尖刺破了自己的拇指，带着血珠落向纸面。

　　就在此时，书斋的门被推开一道缝，瓦格纳捧着一盏昏黄的油灯探进头来。他看到房中多了一个陌生人，又看到老师指尖渗血的姿态，面色顿时发白，颤抖着喊道："老师！您在做什么？此人是何时进来的？"

　　浮士德没有停下动作，血字已在羊皮纸上成型。他头也不回地说："瓦格纳，关上门，今夜你将见证一位学者的夙愿。"话音未落，羊皮纸上的暗色开始涌动，如同一片无星之夜在室内铺展开来，而那位红衣人的笑声，在书卷间回荡不绝。

💾 已保存子版: data/sample_child.meph
```

---

## 🎭 多分支故事线

梅菲斯特支持在同一故事世界中创建多个分支，探索不同的剧情走向。

```bash
# 默认子版（相当于"主线"）
./mephisto run data/sample.meph

# 指定分支
./mephisto run data/sample.meph -branch dark

# 忽略子版，从母版重新开始
./mephisto run data/sample.meph -reset

# 组合使用
./mephisto run data/sample.meph -reset -branch dark
```

---

## 📚 原典释读（完整文档）

> 契约已立，当逐条宣读。

| 文档                                    | 说明                         |
| --------------------------------------- | ---------------------------- |
| **[语法手册](./docs/SYNTAX.md)**        | 如何书写契约（`.meph` 语法） |
| **[规则引擎深度解析](./docs/RULES.md)** | 驱动叙事的齿轮与法则         |
| **[实战示例](./data/faust.meph)**       | 浮士德的完整契约             |
| **[VS Code 扩展](https://marketplace.visualstudio.com/items?itemName=yuelinghuashu.vscode-mephisto)** | 语法高亮、诊断、补全、格式化 |

---

## 🛠️ 命令行选项

```bash
./mephisto <子命令> [选项] <文件>

子命令:
  parse <文件> [选项]           解析 .meph 契约，输出 JSON
  run   <文件> [选项]           启动交互式对话模式
  version                       显示版本信息
  help                          显示此帮助信息

parse 选项:
  -o <路径>                     输出到文件（默认输出到 stdout）
  -q                            静默模式，只输出错误

run 选项:
  -branch <分支名>              分支名（用于多分支故事线）
  -reset                        忽略子版存档，从母版重新开始
  -debug                        启用规则调试模式
  -client <类型>                LLM 客户端: deepseek/openai/ollama
  -model <模型名>               模型名称
  -api-key <密钥>               API 密钥
  -base-url <URL>               API 基础 URL
  -max-tokens <N>               最大生成 Token 数（默认 4096）

环境变量:
  OPENAI_API_KEY                API 密钥（优先级低于命令行）
  OPENAI_BASE_URL               API 基础 URL
  MEPHISTO_MODEL                模型名称
  MEPHISTO_CLIENT               客户端类型（deepseek/openai/ollama）
  MEPHISTO_BRANCH               默认分支名
  MEPHISTO_DEBUG                启用调试模式
  MEPHISTO_RESET                忽略子版存档
```

---

## 📁 项目结构

```text
mephisto/
├── cmd/
│   └── mephisto/          # CLI 入口（config + commands + session + help）
├── internal/
│   ├── core/              # 核心层
│   │   ├── parser/        # 契约解析器（lexer + parser + parse_block）
│   │   ├── engine/        # 叙事引擎（engine + dice + condition + matcher + executor + runtime + memory + save）
│   │   ├── llm/           # LLM 客户端（openai + ollama + prompt）
│   │   └── integration/   # 集成测试
│   ├── domain/            # 领域模型（Contract、Rule、HistoryEntry）
│   └── shared/            # 共享工具（convert.go、errors.go）
├── data/                  # 示例契约文件
├── docs/                  # 完整文档
└── assets/                # Logo 资源
```

---

## 📦 环境要求

- Go 1.26+

---

## 📝 更新日志

详见 [CHANGELOG.md](./CHANGELOG.md)

---

## 🎭 关于命名

梅菲斯特（Mephisto）源自歌德《浮士德》中的魔鬼 Mephistopheles。

在《浮士德》中，浮士德不断追求，却从未真正满足。直到临终前，他听见铁锹声，以为自己在为人民建造新世界，才说出那句 **"你真美呀，请停留一下"**，然后倒地死去。

梅菲斯特 **不是恶魔，不是守护者**。  
他是那个 **让叙事无法停下的机制**。

> **他设定条件，但从不阻止你前行。**  
> **他让你一直走下去，直到你主动说出：**  
> **"到此为止，我心满意足。"**

---

## 📄 License

MIT