package scrape

import (
	"strings"
	"unicode"
)

// 名称相似度阈值：多结果时得分最高的候选超过该值才自动采用，
// 否则仍按原逻辑返回「多条记录」交给用户手工指定 tmdb id。
const (
	// AutoPickMinScore 自动采用的最低得分
	AutoPickMinScore = 0.60
	// YearMatchBonus 年份一致时的加分
	YearMatchBonus = 0.25
)

// normalizeName 归一化名称：转小写、去掉标点与空白，便于比较。
// 中文字符全部保留（中日韩表意文字）。
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			b.WriteRune(r)
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			// 标点、空格、符号一律丢弃
		}
	}
	return b.String()
}

// levenshtein 计算两个字符串的编辑距离（按 rune 计）
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	la, lb := len(ar), len(br)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	// 只保留相邻两行，节省内存
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// NameSimilarity 计算两个名称的相似度，返回 0~1。
// 归一化后按编辑距离折算：完全相同为 1，差异越大越接近 0。
func NameSimilarity(a, b string) float64 {
	na, nb := normalizeName(a), normalizeName(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 1
	}
	d := levenshtein(na, nb)
	maxLen := len([]rune(na))
	if l := len([]rune(nb)); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 0
	}
	s := 1.0 - float64(d)/float64(maxLen)
	if s < 0 {
		return 0
	}
	return s
}
