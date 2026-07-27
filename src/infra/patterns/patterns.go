package patterns

import (
	"regexp"
	"strings"
)

var (
	timeRe    = regexp.MustCompile(`^\[(\d{2}:\d{2}\.\d{2})\]`)
	metaRe    = regexp.MustCompile(`^\[(ti|ar|al|by|offset):`)
	bracketRe = regexp.MustCompile(`\([^)]*\)`)

	copyrightPatterns = []*regexp.Regexp{
		regexp.MustCompile(`QQ音乐享有本翻译作品的著作权`),
		regexp.MustCompile(`享有本翻译作品的著作权`),
		regexp.MustCompile(`翻译作品著作权`),
		regexp.MustCompile(`版权所有`),
		regexp.MustCompile(`Copyright.*`),
		regexp.MustCompile(`©.*`),
	}
)

func IsMetaLine(line string) bool {
	return metaRe.MatchString(strings.TrimSpace(line))
}

func ExtractTimestamp(line string) string {
	m := timeRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return ""
	}
	return m[1]
}

func StripTimestamp(line string, timestamp string) string {
	cleaned := strings.Replace(strings.TrimSpace(line), "["+timestamp+"]", "", 1)
	return strings.TrimSpace(cleaned)
}

var trailingDashRe = regexp.MustCompile(`\s*-\s*$`)
var whitespaceRe = regexp.MustCompile(`\s+`)

func RemoveCopyrightNotices(line string) string {
	for _, pat := range copyrightPatterns {
		line = pat.ReplaceAllString(line, "")
	}
	line = trailingDashRe.ReplaceAllString(line, "")
	line = whitespaceRe.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}

func FindBracketPairs(content string) [][2]int {
	var pairs [][2]int
	stack := make([]int, 0)
	for i, ch := range content {
		if ch == '(' {
			stack = append(stack, i)
		} else if ch == ')' && len(stack) > 0 {
			pairs = append(pairs, [2]int{stack[len(stack)-1], i})
			stack = stack[:len(stack)-1]
		}
	}
	return pairs
}

func FindBracketContents(content string) []string {
	var results []string
	for _, match := range bracketRe.FindAllString(content, -1) {
		results = append(results, match)
	}
	return results
}
