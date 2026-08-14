// internal/core/parser/validator.go
//
// 规则语法校验（对齐 Flutter meph_parser.dart 的校验逻辑）。
//
// 负责在规则解析时尽早报出会导致「静默失效」的语法错误：
//   - 运算符空格：`状态.键 + = 值`、`状态.键 > = 值`
//   - 关键词空格：`不 包含`、`包 含`、`注 入`、`状 态`
//   - roll 表达式：空格位置、括号闭合、骰子面数限制
//   - 条件括号匹配
//   - 复合动作 `&&` 分隔符
//   - 互斥组缺失 `]`
package parser

import (
	"regexp"
	"strings"

	"mephisto/internal/shared"
)

// ============================================================
// 共享正则（对齐 Flutter meph_dsl.dart）
// ============================================================

// quotedStringPattern 匹配括号内双引号内容（用于屏蔽引号内容，避免误报）。
var quotedStringPattern = regexp.MustCompile(`"([^"]*)"`)

// dslKeywordFixPatterns DSL 关键词 → 匹配「关键词被空格拆开」的正则模式。
//
// key: 标准关键字（如 `不包含`）
// value: 匹配「关键字被空格拆开」的正则模式（如 `不\s+包含`）
var dslKeywordFixPatterns = []struct {
	Key   string
	Regex *regexp.Regexp
}{
	{"不包含", regexp.MustCompile(`不\s+包含`)},
	{"包含", regexp.MustCompile(`包\s+含`)},
	{"注入", regexp.MustCompile(`注\s+入`)},
	{"状态", regexp.MustCompile(`状\s+态`)},
	// 校验 pattern：要求「状态」与「.」之间至少一个空格才报错（合法 `状态.键` 不误报）
	{"状态.", regexp.MustCompile(`状态\s+\.`)},
}

// spacedComparisonPattern 匹配「两个比较字符间含空白」的正则（如 `> =`、`= =`、`! =`）。
var spacedComparisonPattern = regexp.MustCompile(`[<>=!]\s+[<>=!]`)

// spacedCompoundOperatorBasePattern 匹配「复合赋值符号与等号间含空白」的正则基础（如 `+ =`、`- =`）。
// RE2 不支持 lookahead，因此「排除 ==」的逻辑由 hasSpacedCompoundOperator 手动完成。
var spacedCompoundOperatorBasePattern = regexp.MustCompile(`[+\-*/]\s+=`)

// hasSpacedCompoundOperator 检测「复合赋值符号与等号间含空白」的模式（如 `状态.x + = 5`）。
//
// 对齐 Flutter meph_dsl.dart 的 spacedCompoundOperatorPattern `[+\-*/]\s+=(?!=)`：
// Flutter 用 lookahead 排除等号后紧跟另一个等号的场景（如 `==`），
// Go RE2 不支持 lookahead，此处手动检查匹配末尾的下一个字符。
func hasSpacedCompoundOperator(s string) bool {
	for len(s) > 0 {
		loc := spacedCompoundOperatorBasePattern.FindStringIndex(s)
		if loc == nil {
			return false
		}
		end := loc[1]
		// 等号后紧跟另一个等号（==）时不视为复合运算符空格
		if end >= len(s) || s[end] != '=' {
			return true
		}
		s = s[end:]
	}
	return false
}

// rollSpacePattern 匹配 roll 与左括号间的空白（如 `roll (1d100)`）。
var rollSpacePattern = regexp.MustCompile(`roll\s+\(`)

// rollUnclosedPattern 匹配 roll 左括号未闭合（如 `roll(1d100 > 50`）。
var rollUnclosedPattern = regexp.MustCompile(`roll\([^)]*$`)

// rollExprPattern 匹配完整的 roll(...) 表达式并捕获内部内容。
var rollExprPattern = regexp.MustCompile(`roll\(([^)]*)\)`)

// rollValidPattern 校验合法的骰子表达式（仅 1d2 与 1d100，与 Flutter 一致）。
var rollValidPattern = regexp.MustCompile(`^(1d2|1d100)$`)

// compoundSepNoLeftSpacePattern 匹配 && 左无空格（如 `动作&& 状态+=1`）。
var compoundSepNoLeftSpacePattern = regexp.MustCompile(`\S&&`)

// compoundSepNoRightSpacePattern 匹配 && 右无空格（如 `注入 "x" &&状态+=1`）。
var compoundSepNoRightSpacePattern = regexp.MustCompile(`&&\S`)

// ============================================================
// 校验入口
// ============================================================

// validateRuleSyntax 对规则的条件和动作执行全部语法校验。
//
// 参数：
//   - cond: 条件字符串（如 `包含 "攻击"`）
//   - action: 动作字符串（如 `注入 "描述"`）
//   - lineNumber: 行号（报错定位）
//   - blockName: 区块名（报错定位）
func validateRuleSyntax(cond, action string, lineNumber int, blockName string) error {
	// 校验动作中的运算符空格：`状态.键 + = 值`
	if err := validateOperatorSpacing(action, lineNumber, blockName); err != nil {
		return err
	}

	// 校验条件中的比较运算符空格：`状态.键 > = 值`
	if err := validateComparisonOperatorSpacing(cond, lineNumber, blockName); err != nil {
		return err
	}

	// 校验关键词空格：`不 包含 "x"`、`包 含 "x"`
	if err := validateKeywordSpacing(cond, action, lineNumber, blockName); err != nil {
		return err
	}

	// 校验条件括号匹配：`( ... || ...` 缺右括号
	if err := validateParenBalance(cond, lineNumber, blockName); err != nil {
		return err
	}

	// 校验复合动作分隔符：`&&` 前后缺空格
	if err := validateCompoundSeparator(action, lineNumber, blockName); err != nil {
		return err
	}

	return nil
}

// ============================================================
// 校验函数
// ============================================================

// validateOperatorSpacing 检测复合动作中的运算符空格。
//
// 覆盖两类：
//   - 复合赋值 `+ =`、`- =`、`* =`、`/ =`（符号与等号间含空白，且非 `==`）
//   - 动作中的比较 `= =`、`! =`、`> =` 等（两个比较字符间含空白）
//
// 合法的 `+=` / `-=` / `*=` / `/=`、`=`、`==`、`!=` 均不受影响。
func validateOperatorSpacing(action string, lineNumber int, blockName string) error {
	parts := strings.Split(action, " && ")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		// 仅校验状态赋值动作
		if !strings.HasPrefix(trimmed, "状态.") {
			continue
		}
		if hasSpacedCompoundOperator(trimmed) {
			return &shared.ParseError{
				Line:      lineNumber,
				BlockName: blockName,
				Message:   "复合运算符（如 '+='、'-='）中间不能有空格",
			}
		}
		if spacedComparisonPattern.MatchString(trimmed) {
			return &shared.ParseError{
				Line:      lineNumber,
				BlockName: blockName,
				Message:   "比较运算符（如 '>='、'=='）中间不能有空格",
			}
		}
	}
	return nil
}

// validateComparisonOperatorSpacing 检测条件中的比较运算符空格。
//
// 合法的 `>=` / `<=` / `==` / `!=` / `>` / `<` 均为单个或紧连字符，不会被误报。
// 引号字符串值中的 `>` / `<` 后跟普通字符也不受影响（正则要求两边都是比较字符）。
func validateComparisonOperatorSpacing(condition string, lineNumber int, blockName string) error {
	if spacedComparisonPattern.MatchString(condition) {
		return &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "比较运算符（如 '>='、'=='）中间不能有空格",
		}
	}
	return nil
}

// maskQuotedStrings 屏蔽字符串中的双引号内容为 `""`（避免引号内文字被校验正则误报）。
func maskQuotedStrings(s string) string {
	return quotedStringPattern.ReplaceAllString(s, `""`)
}

// validateKeywordSpacing 检测 DSL 关键词被空格拆开。
//
// 引擎用 `startsWith('包含 ')` / `startsWith('不包含 ')` 精确匹配，
// 关键词间出现空格会导致条件静默失效（规则永不触发）。
// 检测前先屏蔽 `"..."` 引号内容，避免引号内文字被误报。
func validateKeywordSpacing(cond, action string, lineNumber int, blockName string) error {
	// 屏蔽引号内容后的字符串（引号内的"不 包含"等文字不误报）
	masked := maskQuotedStrings(cond + " " + action)

	for _, entry := range dslKeywordFixPatterns {
		if entry.Regex.MatchString(masked) {
			return &shared.ParseError{
				Line:      lineNumber,
				BlockName: blockName,
				Message:   "关键词「" + entry.Key + "」中间不能有空格",
			}
		}
	}

	// roll 后紧跟空白（如 `roll (1d100)`）导致 `roll(` 无法识别
	if rollSpacePattern.MatchString(masked) {
		return &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "roll 与 '(' 之间不能有空格",
		}
	}

	// roll 后缺少左括号（如 `roll 1d100`、`roll1d100`）导致 roll 表达式无法识别
	// 仅检查条件部分；lookahead 限定 roll 后跟骰子特征（数字或 d），避免「状态.roll值」误报。
	maskedCond := maskQuotedStrings(cond)
	if isRollMissingParen(maskedCond) {
		return &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "roll 表达式缺少 '('，应写作 roll(1dN)（如 roll(1d100)）",
		}
	}

	// roll 左括号未闭合（如 `roll(1d100`、`roll(1d100 > 50`）
	if rollUnclosedPattern.MatchString(maskedCond) {
		return &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "roll 表达式缺少 ')'，应写作 roll(1dN)（如 roll(1d100)）",
		}
	}

	// roll 表达式必须为 `roll(1d2)` 或 `roll(1d100)`：
	//   - `roll( 1d100)` / `roll(1 d100)` / `roll(1d 100)`：括号内/d 两侧空格
	//   - `roll(2d100)`：多骰个数（不支持）
	//   - `roll(1d6)` / `roll(1d20)`：非受支持的面数
	//   - `roll(d100)` / `roll(1dx)` / `roll(1d)`：非法格式
	for _, m := range rollExprPattern.FindAllStringSubmatch(masked, -1) {
		inner := m[1]
		if !rollValidPattern.MatchString(inner) {
			return &shared.ParseError{
				Line:      lineNumber,
				BlockName: blockName,
				Message:   "骰子表达式格式无效，仅支持 roll(1d2)（二元判定）与 roll(1d100)（高精度判定）",
			}
		}
	}

	return nil
}

// validateParenBalance 检测条件中的括号匹配。
//
// 缺右括号或多余右括号都会导致条件静默失效。
func validateParenBalance(condition string, lineNumber int, blockName string) error {
	depth := 0
	for _, ch := range condition {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return &shared.ParseError{
					Line:      lineNumber,
					BlockName: blockName,
					Message:   "条件的括号不匹配（出现多余的 \")\"）",
				}
			}
		}
	}
	if depth != 0 {
		return &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "条件的括号不匹配（可能有未闭合的 \"(\"）",
		}
	}
	return nil
}

// validateCompoundSeparator 检测复合动作分隔符。
//
// `注入 "x" &&状态+=1`（左无空格）或 `注入 "x"&& 状态+=1` 时，
// 执行层 `split(' && ')` 失败，整段被当作 LLM 指令文本处理。
// 引号内文字不受影响。
func validateCompoundSeparator(action string, lineNumber int, blockName string) error {
	masked := maskQuotedStrings(action)
	// `&&` 前后至少一边紧贴非空白字符（即缺少标准 ` && ` 分隔）
	if compoundSepNoRightSpacePattern.MatchString(masked) || compoundSepNoLeftSpacePattern.MatchString(masked) {
		return &shared.ParseError{
			Line:      lineNumber,
			BlockName: blockName,
			Message:   "复合动作应用 ' && ' 分隔（&& 前后各一个空格）",
		}
	}
	return nil
}

// isRollMissingParen 检查 roll 后是否缺少左括号。
//
// 匹配如 `roll 1d100`、`roll1d100`（roll 后跟数字或 d，而非 (）。
// 合法 `roll(1d100)` 不受影响。
func isRollMissingParen(cond string) bool {
	idx := 0
	for {
		pos := strings.Index(cond[idx:], "roll")
		if pos == -1 {
			return false
		}
		abs := idx + pos
		rest := cond[abs+4:]
		// 跳过空白
		trimmed := strings.TrimLeft(rest, " \t")
		// 如果后面紧跟 ( 则是合法
		if strings.HasPrefix(trimmed, "(") {
			idx = abs + 4
			continue
		}
		// 如果后面跟数字或 d，则是缺少左括号
		if len(trimmed) > 0 && (trimmed[0] == 'd' || (trimmed[0] >= '0' && trimmed[0] <= '9')) {
			return true
		}
		idx = abs + 4
	}
}