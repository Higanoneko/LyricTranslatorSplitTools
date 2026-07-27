package split

import (
	"regexp"
	"strings"

	"lyrictools/src/infra/chars"
	"lyrictools/src/infra/patterns"
)

type Split struct {
	Original    string
	Translation string
}

type LineSplit struct {
	Timestamp   string
	Original    string
	Translation string
}

// ── Strategy 1: split by language groups (primary) ──

func splitByLanguageGroups(content string) Split {
	content = preprocessContent(content)
	segments := strings.Split(content, " ")
	if len(segments) < 2 {
		return Split{}
	}

	splitIdx := findOptimalSplitPoint(segments)
	if splitIdx > 0 {
		original := strings.TrimSpace(strings.Join(segments[:splitIdx], " "))
		translation := strings.TrimSpace(strings.Join(segments[splitIdx:], " "))
		if chars.ContainsOriginalLanguage(original) && chars.IsPureChinese(translation) {
			return Split{Original: original, Translation: translation}
		}
	}

	return trySpecialSymbolSplit(content)
}

func findOptimalSplitPoint(segments []string) int {
	for i := 1; i < len(segments); i++ {
		left := strings.Join(segments[:i], " ")
		right := strings.Join(segments[i:], " ")
		if chars.ContainsOriginalLanguage(left) && chars.IsPureChinese(right) {
			return i
		}
	}

	bestScore := 0.0
	bestIdx := 0
	for i := 1; i < len(segments); i++ {
		left := strings.Join(segments[:i], " ")
		right := strings.Join(segments[i:], " ")
		score := chars.CalcOriginalLanguageScore(left) + chars.CalcChinesePurityScore(right)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestScore > 0.5 {
		return bestIdx
	}
	return 0
}

func preprocessContent(content string) string {
	matches := patterns.FindBracketContents(content)
	if len(matches) == 0 {
		return content
	}

	for _, match := range matches {
		inner := match
		if strings.HasPrefix(match, "(") && strings.HasSuffix(match, ")") {
			inner = match[1 : len(match)-1]
		}
		if chars.ContainsOriginalLanguage(inner) {
			pos := strings.Index(content, match)
			if pos > 0 {
				before := strings.TrimSpace(content[:pos])
				after := strings.TrimSpace(content[pos+len(match):])
				if before != "" && after != "" {
					return before + " " + match
				}
			}
		} else if chars.IsPureChinese(inner) {
			pos := strings.Index(content, match)
			if pos > 0 {
				before := strings.TrimSpace(content[:pos])
				after := strings.TrimSpace(content[pos+len(match):])
				if before != "" && after != "" {
					return before
				}
			}
		}
	}

	return content
}

func trySpecialSymbolSplit(content string) Split {
	if !strings.Contains(content, "(") {
		return Split{}
	}

	pairs := patterns.FindBracketPairs(content)
	if len(pairs) == 0 {
		return Split{}
	}

	for _, pair := range pairs {
		start, end := pair[0], pair[1]
		inner := content[start+1 : end]
		before := strings.TrimSpace(content[:start])
		after := strings.TrimSpace(content[end+1:])

		if chars.ContainsOriginalLanguage(inner) {
			if after != "" && chars.IsPureChinese(after) {
				original := inner
				original = "(" + original + ")"
				if before != "" {
					original = before + " " + original
				}
				return Split{Original: strings.TrimSpace(original), Translation: after}
			}
		} else if chars.IsPureChinese(inner) {
			if before != "" && chars.ContainsOriginalLanguage(before) {
				translation := "(" + inner + ")"
				if after != "" {
					translation += " " + after
				}
				return Split{Original: before, Translation: strings.TrimSpace(translation)}
			}
		}
	}

	return bracketRegexFallback(content)
}

func bracketRegexFallback(content string) Split {
	simplePatterns := []struct {
		pattern string
		mode    string
	}{
		{`([^(]*)\s*\(([^)]*\x{4e2d}\x{6587}[^)]*)\s*([^)]*)`, "jp_cn_other"},
		{`([^(]*)\s*\(([^)]*)\s*([^)]*\x{4e2d}\x{6587}[^)]*)`, "jp_jp_cn"},
	}

	for _, sp := range simplePatterns {
		re := regexp.MustCompile(sp.pattern)
		m := re.FindStringSubmatch(content)
		if m == nil {
			continue
		}
		switch sp.mode {
		case "jp_cn_other":
			p1 := strings.TrimSpace(m[1])
			p2 := strings.TrimSpace(m[2])
			p3 := strings.TrimSpace(m[3])
			if p1 != "" && p2 != "" {
				original := p1 + " (" + p2 + ")"
				if chars.ContainsOriginalLanguage(original) && chars.IsPureChinese(p3) {
					return Split{Original: strings.TrimSpace(original), Translation: p3}
				}
			}
		case "jp_jp_cn":
			p1 := strings.TrimSpace(m[1])
			p2 := strings.TrimSpace(m[2])
			p3 := strings.TrimSpace(m[3])
			if p1 != "" && p3 != "" {
				original := p1 + " (" + p2 + ")"
				if chars.ContainsOriginalLanguage(original) && chars.IsPureChinese(p3) {
					return Split{Original: strings.TrimSpace(original), Translation: p3}
				}
			}
		}
	}
	return Split{}
}

// ── Strategy 2-5: fallback splits ──

func splitBySpace(content string) Split {
	parts := strings.SplitN(content, " ", 2)
	if len(parts) == 2 && chars.ContainsChinese(parts[1]) {
		if chars.IsMainlyJapanese(parts[0]) {
			return Split{Original: strings.TrimSpace(parts[0]), Translation: strings.TrimSpace(parts[1])}
		}
	}
	return Split{}
}

func splitByChineseDetection(content string) Split {
	runes := []rune(content)
	for i, ch := range runes {
		if chars.IsChineseChar(ch) && !chars.IsJapaneseChar(ch) {
			if i > 0 {
				prefix := string(runes[:i])
				if chars.IsMainlyJapanese(prefix) {
					return Split{
						Original:    strings.TrimSpace(prefix),
						Translation: strings.TrimSpace(string(runes[i:])),
					}
				}
			}
		}
	}
	return Split{}
}

func splitByPatternMatching(content string) Split {
	patternDefs := []struct {
		pat *regexp.Regexp
	}{
		{regexp.MustCompile(`([^\x{4e00}-\x{9fff}]+) ([\x{4e00}-\x{9fff}].+)`)},
		{regexp.MustCompile(`([\x{3040}-\x{30FF}]+) ([\x{4e00}-\x{9fff}].+)`)},
	}

	for _, pd := range patternDefs {
		m := pd.pat.FindStringSubmatch(content)
		if m != nil {
			left, right := m[1], m[2]
			if chars.IsMainlyJapanese(left) && chars.ContainsChinese(right) {
				return Split{
					Original:    strings.TrimSpace(left),
					Translation: strings.TrimSpace(right),
				}
			}
		}
	}
	return Split{}
}

func splitByCharacterAnalysis(content string) Split {
	runes := []rune(content)
	for i := len(runes) - 1; i > 0; i-- {
		if runes[i] == ' ' && i < len(runes)-1 {
			left := string(runes[:i])
			right := string(runes[i+1:])
			if chars.ContainsChinese(right) && chars.IsMainlyJapanese(left) {
				return Split{
					Original:    strings.TrimSpace(left),
					Translation: strings.TrimSpace(right),
				}
			}
		}
	}
	return Split{}
}

// ── Strategy list ──

var strategies = []func(string) Split{
	splitByLanguageGroups,
	splitBySpace,
	splitByChineseDetection,
	splitByPatternMatching,
	splitByCharacterAnalysis,
}

// ── Main dispatcher ──

func SplitLyricLine(line string) LineSplit {
	line = strings.TrimSpace(line)
	timestamp := patterns.ExtractTimestamp(line)

	if timestamp == "" {
		return LineSplit{Timestamp: "", Original: line, Translation: ""}
	}

	content := patterns.StripTimestamp(line, timestamp)

	if content == "" {
		return LineSplit{Timestamp: timestamp, Original: "", Translation: ""}
	}

	for _, strategy := range strategies {
		result := strategy(content)
		if result.Original != "" && result.Translation != "" {
			return LineSplit{
				Timestamp:   timestamp,
				Original:    result.Original,
				Translation: result.Translation,
			}
		}
	}

	return LineSplit{Timestamp: timestamp, Original: content, Translation: ""}
}

func FormatLine(timestamp string, text string) string {
	return "[" + timestamp + "]" + text
}
