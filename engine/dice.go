// ============================================================
// dice.go - 骰子表达式解析器
// 职责：解析并执行 roll(1d100)、roll(2d6) 等骰子表达式
// 优化：
//   1. 使用全局 rand（线程安全，性能优秀）
//   2. 正则表达式支持括号内空格 roll( 1d100 )
//   3. 避免每次掷骰都创建新的 Rand 实例
// ============================================================

package engine

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// 优化：正则表达式支持 roll( 1d100 ) 带空格
// (?i) 表示不区分大小写，\s* 匹配任意数量的空白字符
var diceRegex = regexp.MustCompile(`(?i)roll\(\s*(\d+)\s*d\s*(\d+)\s*\)`)

// DiceRoll 表示骰子表达式结构
type DiceRoll struct {
	Num   int // 骰子数量
	Sides int // 每个骰子的面数
}

// ParseDice 解析骰子表达式
// 返回 DiceRoll 结构和是否匹配成功
func ParseDice(expr string) (*DiceRoll, bool) {
	matches := diceRegex.FindStringSubmatch(expr)
	if len(matches) != 3 {
		return nil, false
	}
	num, _ := strconv.Atoi(matches[1])
	sides, _ := strconv.Atoi(matches[2])
	return &DiceRoll{Num: num, Sides: sides}, true
}

// Roll 执行掷骰操作
// 使用全局 rand.Intn，Go 1.20+ 已自动随机种子，且线程安全
// 相比每次新建 rand.New(rand.NewSource(time.Now().UnixNano()))，性能提升巨大
func (dr *DiceRoll) Roll() int {
	total := 0
	for i := 0; i < dr.Num; i++ {
		total += rand.Intn(dr.Sides) + 1
	}
	return total
}

// IsDiceExpression 检查字符串是否为骰子表达式
func IsDiceExpression(s string) bool {
	s = strings.TrimSpace(s)
	return diceRegex.MatchString(s)
}

// RollDice 便捷函数：解析并执行骰子表达式
// 如果解析失败，返回 0 和 nil（而不是错误）
// 这样在规则引擎中可以直接使用，不会因骰子表达式解析失败而中断
func RollDice(expr string) (int, error) {
	dr, ok := ParseDice(expr)
	if !ok {
		return 0, nil // 不是骰子表达式，返回 0
	}
	return dr.Roll(), nil
}
