// ============================================================
// types.go - 数据结构定义 & 区块注册表
// 职责：
// 1. 定义核心数据结构（ParsedFile, ParsedBlock, BlockEntry）
// 2. 定义区块类型枚举和区块规格注册表（用于白名单和校验）
// 3. 定义 Expr 接口（供 engine 包实现，避免循环依赖）
// 4. 定义全局解析函数变量，由 engine 包注册
// 5. 为规则引擎提供预编译字段（RuleExpr, RuleAction）
// 这是整个解析器的基础，所有其他文件依赖它。
// ============================================================

package parser

// ============================================================
// Expr 接口（定义在 parser 包，避免循环依赖）
// engine 包中的 AST 节点实现此接口
// ============================================================

// Expr 是表达式接口，所有 AST 节点都必须实现
// 它只有一个 Eval 方法，在给定上下文中求值
type Expr interface {
	// Eval 在给定的上下文中求值，返回结果和可能的错误
	Eval(env map[string]any) (any, error)
}

// ============================================================
// 全局解析函数变量（由 engine 包在 init 中注册）
// 这样 parser 包不需要导入 engine，避免循环依赖
// ============================================================

// ParseExprFunc 是解析表达式字符串为 Expr 的函数类型
// 由 engine 包实现并注册
var ParseExprFunc func(string) (Expr, error)

// ========== 区块类型枚举 ==========

// BlockType 定义区块内容的结构类型
type BlockType int

const (
	SingleLineText BlockType = iota // 必填单行文本（例如：角色名）
	MultiLineText                   // 多行自由文本（例如：世界观、角色背景、开局场景）
	KeyValueList                    // 键值对列表（例如：锚点、状态、校验、记忆）
	RuleList                        // 规则列表（例如：规则）
)

// BlockSpec 描述一个区块的规格：内容类型 + 是否必填
type BlockSpec struct {
	Type     BlockType
	Required bool
}

// BlockRegistry 是区块名称到规格的映射表（白名单 + 类型定义）
// 所有允许的区块必须在这里注册，新增区块只需在此添加一行
var BlockRegistry = map[string]BlockSpec{
	"角色名":  {Type: SingleLineText, Required: true},
	"世界观":  {Type: MultiLineText},
	"角色背景": {Type: MultiLineText},
	"开局场景": {Type: MultiLineText},
	"锚点":   {Type: KeyValueList},
	"状态":   {Type: KeyValueList},
	"校验":   {Type: KeyValueList},
	"记忆":   {Type: KeyValueList},
	"规则":   {Type: RuleList},
}

// ========== 解析结果数据结构 ==========

// ParsedFile 是整个 .meph 文件的解析结果
type ParsedFile struct {
	Blocks     []*ParsedBlock // 所有区块的列表
	References []string       // 从内容中扫描到的 @别名 引用（留待扩展）
}

// ParsedBlock 表示一个区块（例如：【角色名】及其内容）
type ParsedBlock struct {
	Name    string        // 区块名（如 "角色名"）
	Line    int           // 区块标题所在行号（用于报错定位）
	Raw     string        // 原始内容（整个区块的文本，保留换行和注释）
	Entries []*BlockEntry // 解析后的结构化条目列表（由 semantic.go 填充）
}

// BlockEntry 表示区块内的一行结构化内容
// 它是"行"的抽象表示，统一承载三种类型：列表项、规则、普通文本
type BlockEntry struct {
	Type  string // "list" | "rule" | "text"
	Key   string // 键名（列表项）或规则名（规则）或为空（文本）
	Value string // 对应的值（列表项的值、规则的完整条件->动作、或文本内容）
	Line  int    // 该行在文件中的绝对行号（用于报错）

	// 预编译字段（仅 rule 类型使用）
	// 在语义解析阶段预编译 AST，避免运行时重新解析，极大提升性能
	RuleExpr   Expr   // 预编译的 AST 表达式树（使用 parser.Expr 接口）
	RuleAction string // 预编译的动作字符串（已分离出条件）
}
