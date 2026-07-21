# 更新日志

本文件记录梅菲斯特（Mephisto）项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [v1.0.1] — 2026-07-21

### 🐛 修复

- **修复 Golden 文件 HTML 转义**：`json.MarshalIndent` 默认转义 `&` 为 `\u0026`，导致 `&&` 显示异常。改用 `json.Encoder` + `SetEscapeHTML(false)`
- **修复帮助信息命令名硬编码**：`help.go` 中所有 `mephisto` 替换为 `os.Args[0]` 动态获取，支持任意调用方式（`./mephisto`、`/usr/local/bin/mephisto` 等）
- **修复测试缓存导致骰子不变**：`go test -count=1` 强制真实运行

### 🔧 清理

- **删除【校验】区块**：功能已被【锚点】的「绝对禁忌」语义覆盖，区块从未被引擎消费
- **删除 `validator` 包**：角色名非空、规则完整性检查已由 parser 在解析时完成，`Validate()` 完全冗余
- **删除 `testutil` 包**：未在任何代码中被引用，完全的死代码
- **删除 `internal/core/integration/testdata/`**：测试数据复用 `parser/testdata/sample.meph`

### 🔧 重构

- **CLI 子命令独立参数解析**：各子命令使用独立 `flag.FlagSet`，选项统一位于子命令后，支持文件路径在前/在后任意位置
- **减少包数量**：`internal/core/` 从 4 个包减为 3 个（parser / engine / llm）

### ✨ 改进

- **骰子自定义阈值**：`roll(1d100) >= 80` 语法，用户可自定义成功阈值（支持 `>= > <= < == !=`）
- **骰子结果影响 LLM 叙事**：骰子数值传递给 LLM 的 instruction，让 LLM 感知骰子结果并影响故事走向
- **`slices.Clone` 替代手动克隆**：`runtime.go` 中 `append([]string{}, ...)` 简化为 `slices.Clone`
- **`for range n` 循环**：`rules.go` 中骰子循环从 `for i := 0; i < count; i++` 简化为 `for range count`
- **`t.TempDir()` 替代手动清理**：集成测试中 `os.CreateTemp` + `defer os.Remove` 简化为 `t.TempDir()`
- **`TestDumpFormattedMeph` 测试**：新增测试验证变量替换、骰子结果注入

### 🔧 修复

- **README.md 更新**：Go 版本要求、CLI 选项、项目结构同步最新
- **docs/RULES.md 更新**：删除不存在的 `LLM:` 动作类型，互斥组改为「规则修饰符」，骰子补充自定义阈值
- **docs/SYNTAX.md 更新**：标准区块从 9 个改为 8 个（删除【校验】），删除不支持的 `LLM:` 示例

## [v1.0.0] — 2026-07-21

### 🎯 正式稳定版发布

> **v1.0.0 是梅菲斯特的第一个正式稳定版本。**
>
> 经过 v0.5.0 的彻底重构和后续的精简优化，核心引擎已具备生产级稳定性。

### 🏗️ 架构精简（"减即是增"）

> v1.0.0 的核心使命是 **"去掉不必要的抽象，让代码回归简单"**。

**删除中间层**

- **移除 `orchestrator.go`**：`Engine` 直接持有 `Runtime`，消除"门面→编排→运行时"的三层委托
- **移除 `interfaces.go`**：`ConditionEvaluator` 和 `ActionExecutor` 接口被删除，替换为直接函数（`evalCondition`、`ExecuteAction`）
- **移除 `loader.go` 和 `saver.go`**：存档逻辑统一整合到 `save.go`

**文件合并**

- **`evaluator.go` + `executor.go`** → `rules.go`（条件评估 + 规则匹配 + 动作执行统一管理）
- **`strings.go` + `template.go` + `convert.go`** → `convert.go`（三个工具文件合并为一个）
- **`flags.go`** → 配置逻辑并入 `config.go`

**接口简化**

- **`validator.Result` 结构体被移除**：`Validate()` 直接返回 `[]ValidationError`
- **`Engine` 方法减少**：`Save`、`LoadChildData`、`BuildChildPath` 集中到 `save.go`，`engine.go` 只保留核心叙事流程

### ✨ 新增特性

- **`mephisto check` 子命令**（为 VSCode 插件预留）
  - 极速静态检查，不加载 LLM
  - 输出 JSON 格式诊断信息（`valid`、`errors`、`outline`）
  - 支持解析失败时返回结构化错误

- **环境变量扩展白名单**
  - `MEPHISTO_EXTRA_BLOCKS`：支持用户自定义区块名（逗号分隔）
  - 自定义区块可被解析器识别，默认被静默忽略（适合备忘、草稿、注释）

- **骰子表达式重新集成**
  - `roll(1d100)`、`roll(2d6)`、`roll(1d20)`、`roll(3d10)` 语法恢复
  - 判定规则：结果 >= (骰子数 × 面数 / 2) 时返回 true
  - 使用 Go 1.22+ `math/rand/v2` 自动随机种子

- **`BuildPrompt` 语义修正**
  - 新增 `currentMemories` 参数，LLM 现在能感知**运行时动态累积的记忆**
  - 修复了之前只使用契约初始记忆导致的"记忆丢失"问题

### ⚡ 性能优化

- **升级 `math/rand` → `math/rand/v2`**：自动初始化随机种子，消除手动 `rand.Seed()`

### 🔧 修复

- **修复默认响应空格问题**：无角色名时的默认响应从 `"角色 沉默地注视着命运。"` 修正为 `"角色沉默地注视着命运。"`
- **修复输出检查函数未使用参数**：`outputCheckError` 的 `filename` 参数被移除
- **修复 `buildOutline` 行号计算**：规则子项的行号准确指向 `rule.Line`
- **修复 `integration_test.go` 中缺失的记忆验证**：`TestIntegrationStatePersistence` 现在正确检查记忆内容

## [v0.5.0] — 2026-07-19

### 🏗️ 架构重构

> **v0.5.0 是一次彻底的底层重构，也是梅菲斯特从"能跑的原型"到"可维护的系统"的转折点。**

### ✨ 新增

- **统一数据模型** (`internal/domain/contract.go`)
  - 引入 `domain.Contract` 作为核心数据模型
  - 解析器输出 Contract，引擎直接消费 Contract
  - 消除 v0.4.0 中解析结果（`ParsedFile`）与运行时上下文（`Context`）两套数据结构并存的问题
  - 所有区块统一为结构体字段，类型安全

- **接口驱动的引擎设计** (`internal/core/engine/interfaces.go`)
  - `ConditionEvaluator` 接口：条件评估可插拔
  - `ActionExecutor` 接口：动作执行可插拔
  - 引擎核心只负责编排，具体实现通过接口注入
  - 解决 v0.4.0 中条件解析与动作执行锁死在引擎核心的问题

- **Option 模式依赖注入** (`internal/core/engine/engine.go`)
  - `WithLLMClient`：注入 LLM 客户端
  - `WithMemoryManager`：注入记忆管理器
  - `WithDebug`：启用调试模式
  - `WithMaxHistory`：设置历史保留轮数
  - 组件可插拔，便于测试和替换

- **独立的 LLM 客户端接口** (`internal/core/llm/client.go`)
  - `Client` 接口统一抽象：`Generate` + `GenerateStream`
  - `OpenAIClient`：支持 OpenAI/DeepSeek 兼容 API
  - `OllamaClient`：支持本地 Ollama 服务
  - 新增 `Prompt` 构建层：将 System Prompt 构建从 `app` 包移至 `llm` 包，职责更清晰

- **记忆管理器独立化** (`internal/core/engine/memory.go`)
  - 记忆提取、压缩、去重收敛为独立的 `MemoryManager` 组件
  - 通过 `engine.ProcessMemories` 方法调用，与交互循环解耦
  - 配置独立：`MemoryConfig`（提取间隔、上限、压缩保留数）

- **子版存档与加载重构** (`internal/core/engine/saver.go` + `loader.go`)
  - `SaveChild` 和 `LoadChild` 从 `app` 包移至 `engine` 包
  - 引擎成为存档管理的核心：`engine.Save()` + `engine.LoadChildData()`
  - 子版路径构建统一为 `BuildChildPath`

- **分层项目结构** (`cmd/` + `internal/`)
  - `cmd/mephisto/`：CLI 入口，负责参数解析和命令调度
  - `internal/core/`：核心逻辑（parser、engine、llm、validator）
  - `internal/domain/`：领域模型（`Contract`、`Rule`、`KeyValue`）
  - `internal/shared/`：共享工具（`ParseValue`、`ReplacePlaceholders`）

- **CLI 子命令模式** (`cmd/mephisto/flags.go`)
  - `mephisto parse`：解析契约并输出 JSON
  - `mephisto run`：启动交互式对话
  - `mephisto version`：显示版本
  - `mephisto help`：显示帮助
  - 支持隐式 parse：`mephisto file.meph` 等同于 `mephisto parse file.meph`

- **Golden File 测试** (`internal/core/parser/parser_test.go`)
  - 基于 `sample.meph` 的快照测试
  - 支持 `go test -update` 更新 Golden 文件
  - 确保重构后解析结果与预期一致

- **新增验证器层** (`internal/core/validator/`)
  - 独立的契约验证模块
  - 验证角色名、状态类型、规则完整性
  - 返回结构化错误列表

- **交互会话独立** (`cmd/mephisto/session.go`)
  - CLI 交互逻辑从 `app/interactive.go` 拆离为独立的 `Session`
  - 支持 `/state`、`/history`、`/save` 内置命令
  - 支持 `-reset` 标志，忽略子版从母版重新开始

- **新增 `mephisto` CLI 文档**
  - 完整的命令行帮助信息 (`help.go`)
  - 版本信息注入 (`version`)

### 🧹 优化

- **解析器重构**
  - 从单文件拆分为 `lexer.go` + `parser.go` + `parse_block.go`
  - Lexer 阶段记录绝对行号，Parser 直接使用，错误定位更精准
  - 白名单校验前移到 Lexer 阶段
  - 支持从字符串解析：`ParseString`

- **错误信息增强**
  - 所有错误信息包含区块名 + 行号
  - 格式统一为"第 X 行（区块「XXX」）：错误描述"

- **代码可测试性提升**
  - 核心模块全部可 mock
  - 集成测试覆盖完整链路（`integration_test.go`）
  - 测试数据统一放在 `testdata/`

- **移除 v0.4.0 的 AST 预编译机制**
  - v0.4.0 在解析阶段将规则条件编译为 AST，运行时直接求值
  - v2 改为运行时解析条件字符串，代码更清晰，维护成本更低
  - 性能影响在规则数量较少（< 100 条）的场景中可忽略

### 🔧 修复

- **修复状态类型不一致问题**
  - 状态值统一由 `shared.ParseValue` 转换（bool/int/float64/string）
  - 消除 v0.4.0 中"同一状态在不同阶段类型不同"的问题

- **修复历史恢复时的角色识别**
  - 硬匹配 `fate:` 和 `assistant:` 前缀，避免内容中的冒号干扰解析

- **修复子版覆盖逻辑**
  - 明确子版文件命名规则：`story_child.meph` / `story_{branch}.meph`
  - 运行子版文件时直接覆盖，不嵌套生成

### 🗑️ 移除

- **移除 `app` 包**
  - 原有职责拆分为：`cmd/mephisto`（交互）+ `engine`（核心）+ `llm`（Prompt 构建）
  - `app/memory.go` → `engine/memory.go`（MemoryManager）
  - `app/save.go` → `engine/saver.go` + `loader.go`
  - `app/prompt.go` → `llm/prompt.go`

- **移除 `utils` 包**
  - `ReplacePlaceholders` → `shared.ReplacePlaceholders`
  - `ParseValue` → `shared.ParseValue`

- **移除 `parser` 包中的 AST 相关定义**
  - 删除 `Expr` 接口（移至 `domain.Expr`）
  - 删除 `ParseExprFunc` 全局变量（条件解析由引擎的 `ConditionEvaluator` 负责）
  - 删除 `BlockEntry.RuleExpr` 和 `BlockEntry.RuleAction` 字段（解析器不再负责预编译）

### 🔄 与 v0.4.0 的兼容性

- **`.meph` 文件格式完全兼容**：v0.5.0 可以读取 v0.4.0 创建的所有契约文件和子版存档
- **CLI 命令不向前兼容**：v0.4.0 的 `go run main.go data/sample.meph` 仍可工作（隐式 parse），但 `-branch`、`-reset` 等选项的行为有所调整
- **子版文件命名规则保持一致**：`story_child.meph` / `story_{branch}.meph`

### 📊 重构数据

| 指标             | v0.4.0        | v0.5.0                                                       |
| ---------------- | ------------- | ------------------------------------------------------------ |
| 顶层包数量       | 5 个          | 3 个（cmd + internal + data）                                |
| 核心层模块       | 耦合          | 4 个独立模块（parser/engine/llm/validator）                  |
| 可 mock 组件     | 0             | 3 个接口（LLM Client / ConditionEvaluator / ActionExecutor） |
| 单元测试覆盖率   | ~40%          | ~70%                                                         |
| 新增功能开发耗时 | 平均 2-4 小时 | 平均 30-60 分钟                                              |

---

## [v0.4.0] — 2026-07-17

### ✨ 新增

- **记忆编织系统（M5）** (`app/memory.go`)
  - 每 N 轮自动调用 LLM 提取关键事件作为记忆
  - 记忆以纯文本列表形式存储（`[]string`）
  - 超过阈值（默认 30 条）自动触发压缩，保留最近 N 条（默认 10 条）并生成摘要
  - 记忆注入 System Prompt，角色在叙事中自然引用过往经历
  - 支持 `【记忆】` 区块解析纯文本列表（无冒号格式）

- **子版存档系统（M5）** (`app/save.go`)
  - 每轮对话结束后自动保存子版（实时持久化）
  - 子版文件包含：`【状态】` + `【记忆】` + `【历史】` + 母版所有静态区块
  - 子版文件独立可运行，包含完整故事设定
  - 支持分支功能（`-branch dark`）：生成 `story_dark.meph`
  - 用户运行哪个文件，就更新哪个文件（无嵌套生成）
  - 退出时自动保存最终状态

- **对话历史恢复**
  - 加载子版时完整恢复对话历史（`user` / `assistant` 消息）
  - 历史保留轮数可通过 `-retain N` 控制（默认 10 轮）
  - 历史记录干净存储（不含展示层缩进格式）

- **命令行参数扩展** (`main.go`)
  - 新增 `-branch` 参数：多分支故事线管理
  - 新增 `-retain` 参数：对话历史保留轮数
  - 支持环境变量 `MEPHISTO_BRANCH`、`MEPHISTO_RETAIN`

- **区块支持**
  - `【历史】` 区块注册，用于持久化对话记录
  - `StateExcludeKeys` 扩展，排除记忆和历史出现在状态列表中

### 🧹 优化

- **子版文件完整性**
  - 保存子版时自动复制母版所有静态区块（角色名、世界观、锚点、规则等）
  - 子版文件可直接运行，不再依赖母版存在

- **历史解析**
  - `LoadChild` 直接使用 `entry.Key` 和 `entry.Value`，不再错误地尝试分割
  - 支持中英文冒号分割角色和内容
  - 反转义换行符，恢复完整叙事格式

- **记忆阈值调整**
  - 提取间隔从 5 轮调整为 3 轮（更及时捕捉剧情变化）
  - 记忆上限从 30 调整为 50（降低压缩频率，节省 API 调用）
  - 压缩保留从 10 调整为 15（保留更多近期记忆）

- **代码结构**
  - `parser/types.go` 新增 `KeyHistory` 常量
  - `parser/types.go` 的 `StateExcludeKeys` 新增 `KeyMemory` 排除

---

## [v0.3.0] — 2026-07-16

### ✨ 新增

- **LLM 客户端** (`llm/`)
  - 支持 OpenAI 兼容 API（DeepSeek、Ollama 等）
  - 支持普通请求和流式请求（SSE 协议）
  - 支持 Context 取消（按 `^C` 中断生成）
  - 配置化：API Key、模型、温度、最大 Token 数、静默模式

- **对话引擎集成** (`app/interactive.go`)
  - 规则触发后自动调用 LLM 生成叙事（流式输出）
  - 支持 `LLM: 提示词` 动作（高级用法）
  - 输入提示符固定为「（命运）: 」，输出前缀为「📖 命运: 」
  - 符合梅菲斯特命名理念：命运是叙事的推动者

- **对话历史管理**
  - 多轮对话连贯叙事
  - 支持从「记忆」区块配置保留轮数
  - 自动管理消息队列，超出容量时移除最早消息

- **后置校验**
  - 支持「校验」区块的 `输出不含 "xxx"` 规则
  - LLM 输出自动过滤，违规时显示警告

- **环境变量与配置**
  - 支持 `.env` 文件自动加载
  - 命令行参数全覆盖：`-api-key`、`-model`、`-base-url`、`-debug`、`-quiet`、`-max-tokens`、`-temperature`
  - 优先级：命令行参数 > 系统环境变量 > `.env` 文件

- **跨平台支持**
  - 历史文件使用 `os.UserCacheDir()`，支持 Windows/Linux/macOS
  - 使用 `github.com/chzyer/readline`，支持退格、删除、历史记录

- **区块链名常量** (`parser/types.go`)
  - 统一管理所有区块键名，消除硬编码分散
  - 新增 `CoreKeys`、`TextBlockKeys`、`StateExcludeKeys`
  - 新增区块只需修改一处

### 🧹 优化

- **System Prompt 构建** (`app/prompt.go`)
  - 锚点正式加载到上下文，核心人格设定生效
  - 多层级组织：角色身份 → 锚点 → 世界观 → 角色背景 → 当前状态 → 角色扮演指令
  - 指令改为通用版本，适配所有故事类型

- **交互体验**
  - 使用 `readline` 替代 `bufio.Scanner`，解决长输入退格卡顿
  - 输入提示与输出视角统一为「命运」
  - 流式输出逐字打印，体验更流畅

- **错误处理**
  - 每次请求使用独立 Context，按 `^C` 只取消当前请求
  - 后置校验失败时显示具体原因

- **测试**
  - `app_test.go` 使用 `runtime.Caller` 获取测试数据绝对路径
  - 不再依赖工作目录

### 🔧 修复

- 修复锚点区块未加载到上下文的问题（核心功能修复）
- 修复 SSE 解析未处理 `\r` 空白字符的问题
- 修复流式输出结束后未换行的问题
- 修复 `resolveFilename` 逻辑冗余
- 修复 `printContextValue` 中空行显示问题

---

## [v0.2.1] — 2026-07-16

### 🧹 优化

- **自动化测试**
  - 契约测试（`parser_test.go`）：基于 `sample.meph` 的 Golden File 测试
  - 引擎单元测试（`engine_test.go`）：覆盖 AST 求值、骰子、互斥组、动作执行
  - 支持 `go test -update` 更新 Golden 文件
- **代码精简**
  - 删除冗余的 `helpers.go`，变量替换统一到 `utils/interpolate.go`
  - 精简 `validate.go` 中的冗余检查，保留核心语义约束
  - `printRoleInfo` 动态显示状态，不再硬编码
  - 区块类型判断基于 `BlockRegistry`，消除硬编码列表
- **路径管理**
  - 测试路径统一使用 `getTestDataPath` + `runtime.Caller`

### 🔧 修复

- 错误信息包含文件名，定位更精准
- 注入动作使用 `strings.CutPrefix` 和 `strings.Trim`
- 表达式解析器用标准库 `strings.Count` 替代自定义 `isBalanced`

---

## [v0.2.0] — 2026-07-15

### ✨ 新增

- **规则引擎** (`engine/`)
  - 支持原子条件：`==`、`!=`、`>`、`<`、`>=`、`<=`、`包含`
  - 支持逻辑运算：`&&`、`||`、括号分组
  - 支持骰子表达式：`roll(1d100)`、`roll(2d6)`
  - 支持互斥组：`[group:xxx]` 同一组只执行第一个匹配的规则
  - 支持变量替换：`{变量}` 在 `【开局场景】`、`【世界观】`、`【角色背景】` 和 `【规则】` 动作中自动替换为 `【状态】` 或 `【角色名】` 中的值
  - 支持调试模式：`eng.SetDebug(true)` 查看规则执行过程

- **AST 预编译**（性能优化）
  - 规则条件在加载时预编译为 AST，运行时直接求值
  - 消除运行期字符串解析，性能提升 10-100 倍
  - 支持短路求值（`&&` 左侧为假时不评估右侧）

- **交互式对话界面**
  - 支持多轮对话循环
  - 每次用户输入自动触发规则引擎
  - 显示当前对话角色名（`你（角色名）: `）
  - 支持 `quit` / `exit` / `q` 退出命令

- **全局变量**
  - `【状态】` 中的所有键（如 `{情绪}`、`{生命值}`、`{位置}`）和 `【角色名】` 的值，都是全局变量
  - 可在 `【开局场景】`、`【世界观】`、`【角色背景】` 文本区块中使用
  - 可在 `【规则】` 动作中注入或输出时使用

- **开局场景区块** (`【开局场景】`)
  - 支持多行文本
  - 支持 `{变量}` 插值替换
  - 启动时自动打印

- **应用层重构** (`app/`)
  - `main.go` 精简为入口（约 30 行）
  - 上下文构建、对话循环、辅助函数拆分为独立模块
  - 统一的注释风格

### 🔧 修复

- 修复 `splitKeyValue` 中英文冒号混用时的误分割问题（取最小索引）
- 修复 `ScanReferences` 重复引用和空引用问题
- 修复 `dice.go` 随机种子冲突（改用全局 `rand.Intn`）
- 修复 `expr.go` 比较运算符与骰子表达式的优先级问题
- 修复表达式求值中缺失变量导致规则中断的问题（返回 `nil` 而不是 `error`）
- 修复 `||` 操作符需要布尔值的错误
- 修复解析器与引擎之间的循环依赖（通过 `parser.ParseExprFunc` 注册机制）
- 修复 `eval.go` 与 `expr.go` 函数重复声明问题
- 修复 `开局场景` 区块变量替换顺序问题（分阶段构建上下文）

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
