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

> 以下输出使用了 `--constraints` 自定义约束，LLM 遵循约束文件中的格式要求生成叙事。

```text
命运 > 你与梅菲斯特展开了一场关于真理的论道
　　（书斋四壁，羊皮卷轴堆积如山。烛火摇曳，将浮士德的影子撕成两半）

　　浮士德：
　　这扭曲的影子啊！左边是学问堆积的尸骨，
　　右边——（抓起一面铜镜）这镜中的陌生人是谁？
　　他对我微笑，用我千日的疲倦，
　　却眨着地狱才有的硫磺色瞳孔！

　　（猛地将镜子砸向石墙，碎片迸溅如魔星）
　　我吞下过多少墨汁？像夜鹭啄食腐水，
　　从托勒密的天球啃到阿奎那的经院——
　　每一卷都吐出灰烬的舌头：
　　“真理——在下一章！下一章！”
　　可翻到羊皮纸的背面，
　　只有我自己咬出的血洞！

　　梅菲斯特
　　（从书脊的阴影中渗出，衣袍边缘燃烧着绿色磷火）：
　　致伟大的浮士德——头颅像钟楼般空响！
　　你解剖过多少次彩虹？用解剖刀削去七种颜色，
　　剩下的是死灰色的骨架，叫做“光学”！
　　你囚禁过多少次闪电？在瓶子里收集上帝的叹息——
　　结果只得到焦黑的蝴蝶标本！

　　（突然压低嗓音，如地窖渗水）
　　真理？瞧那边——（指向窗外绞刑架上的乌鸦）
　　你说它为何不歌唱？不是没有歌曲，
　　而是夜莺唱得比它更动听！
　　存在即谎言！唯一真实的，是谎言本身！

　　浮士德
　　（疯狂翻动案头的手稿，纸张纷飞如雪）：
　　住口！你像蛆虫啃噬理解的根基！
　　倘若真理是谎言，那么——
　　（突然停住，声音颤抖）
　　那么我在此的求索……是什么？
　　（仰望穹顶的圣母像，烛泪滑落如血滴）
　　圣母啊，假若您存在…
　　为何我将颅骨当酒盏痛饮四十年，
　　杯中仍是空虚的热气？

　　（转向梅菲斯特，眼中燃起炭火）
　　我懂了！您这位地狱来的外交官，
　　带着硫磺味的蜜糖。
　　请回答——用您那欺骗者的诚实：
　　若定义真理为“欺骗的完美形态”，
　　那么这定义本身，是否已被真理毒害？

　　梅菲斯特
　　（发出岩石崩裂般的笑声）：
　　妙极！人类终于学会用自己的绳索打结！
　　当你用“完美”形容欺骗——
　　（指尖轻触浮士德的额头，留下一道灼痕）
　　这灼热的烙印，是真理吗？
　　（后退一步，身形在烛火中扭曲成问号）
　　我亲爱的求知者，您已踏入最后的迷宫：
　　出口与入口本是同一扇门。
　　（指向书斋门扉，门缝渗出沥青般的黑暗）
　　要打开它吗？用您那半枚灵魂作为门栓……

　　浮士德
　　（沉默良久，手指抚过胸前十字架）
　　不…我不愿做这最后的跳跃。
　　但既然契约已咬进我的骨髓——
　　（撕下半页羊皮纸，书写时指尖燃起蓝焰）
　　来吧！让我们用更危险的方式论道：
　　您说存在即谎言——那我便用您的“谎言”，
　　（将写满文字的纸页投入壁炉，火焰瞬间化为蓝色）
　　反哺这名为“存在”的饿鬼！
　　每一个否定，都将成为新理解的嫩芽。
　　——请看着，这便是我的回答。

　　（书斋震颤，书架上的书脊同时发出天鹅咽气般的悲鸣。梅菲斯特的衣袍化作无数蛆虫，又重组为人类形态。）
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

| 文档                                                                                                  | 说明                         |
| ----------------------------------------------------------------------------------------------------- | ---------------------------- |
| **[语法手册](./docs/SYNTAX.md)**                                                                      | 如何书写契约（`.meph` 语法） |
| **[规则引擎深度解析](./docs/RULES.md)**                                                               | 驱动叙事的齿轮与法则         |
| **[实战示例](./data/faust.meph)**                                                                     | 浮士德的完整契约             |
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
  -constraints <文件>           自定义输出约束文件（默认使用内置约束）
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
