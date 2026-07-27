package combine

import (
	"path/filepath"
	"strings"

	"lyrictools/src/infra/chars"
	"lyrictools/src/infra/patterns"
)

type Language string

const (
	LangUnknown  Language = ""
	LangJapanese Language = "jp"
	LangChinese  Language = "cn"
	LangRomanji  Language = "ro"
	LangEnglish  Language = "en"
	LangMixed    Language = "mix"
)

type TimestampedLine struct {
	Timestamp string
	Content   string
	IsMeta    bool
	IsExtra   bool
}

type SourceFile struct {
	Path          string
	Language      Language
	MetaTags      map[string]string
	Lines         []TimestampedLine
	ExtraLines    []TimestampedLine
	HasTimestamps bool
}

type ExtraLine struct {
	Timestamp string
	Content   string
}

type LineGroup struct {
	Timestamp  string
	Columns    []string
	ExtraLines []ExtraLine
}

type CombineResult struct {
	Groups      []LineGroup
	SourceFiles []SourceFile
	Warnings    []string
}

func ParseSourceFile(path string, lines []string) SourceFile {
	sf := SourceFile{
		Path:     path,
		MetaTags: make(map[string]string),
	}

	hasAnyTimestamp := false
	var contentLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		tl := TimestampedLine{}
		if patterns.IsMetaLine(trimmed) {
			tl.IsMeta = true
			tl.Content = trimmed
			key := extractMetaKey(trimmed)
			if key != "" {
				sf.MetaTags[key] = extractMetaValue(trimmed)
			}
		} else {
			ts := patterns.ExtractTimestamp(trimmed)
			if ts != "" {
				hasAnyTimestamp = true
				tl.Timestamp = ts
				tl.Content = patterns.StripTimestamp(trimmed, ts)
				if isExtraLine(tl.Content) {
					tl.IsExtra = true
				} else {
					contentLines = append(contentLines, tl.Content)
				}
			} else {
				tl.Content = trimmed
				contentLines = append(contentLines, tl.Content)
			}
		}

		if tl.IsExtra {
			sf.ExtraLines = append(sf.ExtraLines, tl)
		}

		sf.Lines = append(sf.Lines, tl)
	}

	sf.HasTimestamps = hasAnyTimestamp
	sf.Language = InferFileLanguage(contentLines)
	return sf
}

func extractMetaKey(line string) string {
	start := strings.Index(line, "[")
	colon := strings.Index(line, ":")
	if start >= 0 && colon > start {
		return strings.TrimSpace(line[start+1 : colon])
	}
	return ""
}

func extractMetaValue(line string) string {
	colon := strings.Index(line, ":")
	closing := strings.Index(line, "]")
	if colon >= 0 && closing > colon {
		return strings.TrimSpace(line[colon+1 : closing])
	}
	return ""
}

func isExtraLine(content string) bool {
	if content == "" {
		return false
	}
	runes := []rune(content)
	nonSymbol := 0
	for _, r := range runes {
		if r == '♪' || r == '♫' || r == '～' || r == '~' || r == '-' || r == '*' || r == '=' || r == ' ' {
			continue
		}
		if chars.IsEnglishChar(r) || chars.IsJapaneseChar(r) || chars.IsChineseChar(r) {
			nonSymbol++
		}
	}
	return nonSymbol == 0
}

func AlignAndCombine(files []SourceFile) ([]LineGroup, []string) {
	if len(files) == 0 {
		return nil, nil
	}

	var warnings []string

	allHaveTimestamps := true
	for _, f := range files {
		if !f.HasTimestamps {
			allHaveTimestamps = false
			break
		}
	}
	for _, f := range files {
		if !f.HasTimestamps && f.Path != files[0].Path {
			warnings = append(warnings, "文件 "+f.Path+" 缺少时间戳，将使用行号顺序匹配")
		}
	}

	var groups []LineGroup
	if allHaveTimestamps {
		groups = alignByTimestamp(files)
	} else {
		groups = alignByLineNumber(files)
	}

	allExtraLines := collectExtraLines(files)
	groups = mergeExtraLines(groups, allExtraLines)

	return groups, warnings
}

func alignByTimestamp(files []SourceFile) []LineGroup {
	baseLines := contentLines(files[0])
	otherIndexes := make([]map[string]string, len(files)-1)
	for i := 1; i < len(files); i++ {
		otherIndexes[i-1] = buildTimestampIndex(contentLines(files[i]))
	}

	var groups []LineGroup
	for _, bl := range baseLines {
		if bl.IsMeta {
			continue
		}
		lg := LineGroup{
			Timestamp: bl.Timestamp,
			Columns:   make([]string, len(files)),
		}
		lg.Columns[0] = bl.Content

		for j := 1; j < len(files); j++ {
			if content, ok := otherIndexes[j-1][bl.Timestamp]; ok {
				lg.Columns[j] = content
			}
		}

		groups = append(groups, lg)
	}

	return groups
}

func alignByLineNumber(files []SourceFile) []LineGroup {
	baseContent := contentLines(files[0])

	offsets := make([]int, len(files))
	offsets[0] = 0
	for i := 1; i < len(files); i++ {
		offsets[i] = detectOffset(contentLines(files[0]), contentLines(files[i]))
	}

	maxCols := len(files)
	includedFiles := true
	maxGroupCount := findMaxGroupCount(contentLines(files[0]), offsets[0], maxCols, includedFiles)

	var groups []LineGroup
	for idx := 0; idx < maxGroupCount; idx++ {
		lg := LineGroup{
			Columns: make([]string, len(files)),
		}
		if baseContent[idx].Timestamp != "" {
			lg.Timestamp = baseContent[idx].Timestamp
		}

		for fi := range files {
			adjustedIdx := idx + offsets[fi]
			otherLines := contentLines(files[fi])
			if adjustedIdx >= 0 && adjustedIdx < len(otherLines) {
				lg.Columns[fi] = otherLines[adjustedIdx].Content
				if fi > 0 && lg.Timestamp == "" && otherLines[adjustedIdx].Timestamp != "" {
					lg.Timestamp = otherLines[adjustedIdx].Timestamp
				}
			}
		}

		groups = append(groups, lg)
	}

	return groups
}

func contentLines(sf SourceFile) []TimestampedLine {
	var result []TimestampedLine
	for _, l := range sf.Lines {
		if !l.IsMeta {
			result = append(result, l)
		}
	}
	return result
}

func buildTimestampIndex(lines []TimestampedLine) map[string]string {
	index := make(map[string]string, len(lines))
	for _, l := range lines {
		if l.Timestamp != "" {
			index[l.Timestamp] = l.Content
		}
	}
	return index
}

func detectOffset(base, other []TimestampedLine) int {
	if len(base) == 0 || len(other) == 0 {
		return 0
	}

	bestOffset := 0
	bestScore := 0
	maxCheck := 5
	if maxCheck > len(base) {
		maxCheck = len(base)
	}
	if maxCheck > len(other) {
		maxCheck = len(other)
	}

	for offset := -min(len(other), 10); offset <= min(len(base), 10); offset++ {
		score := 0
		compared := 0
		for i := 0; i < maxCheck; i++ {
			baseIdx := i
			otherIdx := i + offset
			if baseIdx >= 0 && baseIdx < len(base) && otherIdx >= 0 && otherIdx < len(other) {
				score += charMatchScore(base[baseIdx].Content, other[otherIdx].Content)
				compared++
			}
		}
		if compared > 0 && score > bestScore {
			bestScore = score
			bestOffset = offset
		}
	}

	return bestOffset
}

func charMatchScore(a, b string) int {
	aRunes := []rune(a)
	bRunes := []rune(b)
	minLen := len(aRunes)
	if len(bRunes) < minLen {
		minLen = len(bRunes)
	}

	score := 0
	for i := 0; i < minLen; i++ {
		if aRunes[i] == bRunes[i] {
			score++
		}
	}
	return score
}

func findMaxGroupCount(lines []TimestampedLine, offset int, maxCols int, include bool) int {
	if !include {
		return len(lines)
	}
	return len(lines)
}

func collectExtraLines(files []SourceFile) []TimestampedLine {
	var all []TimestampedLine
	seen := make(map[string]bool)
	for _, f := range files {
		for _, l := range f.ExtraLines {
			if !seen[l.Timestamp+":"+l.Content] {
				all = append(all, l)
				seen[l.Timestamp+":"+l.Content] = true
			}
		}
	}
	return all
}

func mergeExtraLines(groups []LineGroup, extraLines []TimestampedLine) []LineGroup {
	if len(extraLines) == 0 {
		return groups
	}

	extraMap := make(map[string][]ExtraLine)
	for _, el := range extraLines {
		extraMap[el.Timestamp] = append(extraMap[el.Timestamp], ExtraLine{
			Timestamp: el.Timestamp,
			Content:   el.Content,
		})
	}

	result := make([]LineGroup, 0, len(groups))
	added := make(map[string]bool)

	for _, g := range groups {
		if extras, ok := extraMap[g.Timestamp]; ok {
			g.ExtraLines = append(g.ExtraLines, extras...)
			added[g.Timestamp] = true
		}
		result = append(result, g)
	}

	for _, el := range extraLines {
		if !added[el.Timestamp] {
			result = append(result, LineGroup{
				Timestamp: el.Timestamp,
				ExtraLines: []ExtraLine{{
					Timestamp: el.Timestamp,
					Content:   el.Content,
				}},
			})
			added[el.Timestamp] = true
		}
	}

	return result
}

func BuildOutput(result CombineResult, config OutputConfig) []string {
	var output []string

	metaLines, warnings := buildMetaOutput(result)
	output = append(output, metaLines...)
	for _, w := range warnings {
		output = append(output, w)
	}

	if len(output) > 0 {
		output = append(output, "")
	}

	for i := 0; i < len(result.Groups); i++ {
		g := result.Groups[i]

		for _, extra := range g.ExtraLines {
			if extra.Timestamp != "" {
				output = append(output, "["+extra.Timestamp+"]"+extra.Content)
			} else {
				output = append(output, extra.Content)
			}
		}

		for colIdx, content := range g.Columns {
			if !config.NoDedupe && colIdx > 0 {
				if content != "" && g.Columns[0] == content {
					continue
				}
			}

			if config.NoBlankColumns && content == "" {
				continue
			}

			if content == "" {
				if g.Timestamp != "" {
					output = append(output, "["+g.Timestamp+"]")
				}
				continue
			}

			if g.Timestamp != "" {
				output = append(output, "["+g.Timestamp+"]"+content)
			} else {
				output = append(output, content)
			}
		}

		if config.BlankLineBetweenGroups && i < len(result.Groups)-1 {
			output = append(output, "")
		}
	}

	return output
}

func buildMetaOutput(result CombineResult) (metaLines []string, warnings []string) {
	seen := make(map[string]string)

	for i, sf := range result.SourceFiles {
		for k, v := range sf.MetaTags {
			if i == 0 {
				seen[k] = v
			} else {
				if existing, ok := seen[k]; ok {
					if existing != v {
						warnings = append(warnings, "[# conflict: "+k+":"+v+"]")
					}
				} else {
					seen[k] = v
				}
			}
		}
	}

	for _, key := range metaTagOrder {
		if v, ok := seen[key]; ok {
			metaLines = append(metaLines, "["+key+":"+v+"]")
		}
	}
	for k, v := range seen {
		if !isStandardMetaTag(k) {
			metaLines = append(metaLines, "["+k+":"+v+"]")
		}
	}

	return metaLines, warnings
}

var metaTagOrder = []string{"ti", "ar", "al", "by", "offset"}

func isStandardMetaTag(k string) bool {
	for _, std := range metaTagOrder {
		if k == std {
			return true
		}
	}
	return false
}

func InferFileLanguage(lines []string) Language {
	const sampleSize = 20
	sample := lines
	if len(sample) > sampleSize {
		sample = sample[:sampleSize]
	}

	jpKana := 0
	cjkHan := 0
	alphaCount := 0
	totalChars := 0

	for _, line := range sample {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if patterns.IsMetaLine(line) {
			continue
		}
		ts := patterns.ExtractTimestamp(line)
		if ts != "" {
			line = patterns.StripTimestamp(line, ts)
		}

		for _, r := range line {
			if chars.IsJapaneseChar(r) {
				jpKana++
			} else if chars.IsChineseChar(r) {
				cjkHan++
			} else if chars.IsEnglishChar(r) {
				alphaCount++
			}
			totalChars++
		}
	}

	if totalChars == 0 {
		return LangUnknown
	}

	kanaRatio := float64(jpKana) / float64(totalChars)
	cjkRatio := float64(cjkHan) / float64(totalChars)
	alphaRatio := float64(alphaCount) / float64(totalChars)

	if kanaRatio > 0.02 && cjkRatio > 0.1 {
		if alphaRatio > 0.05 {
			return LangMixed
		}
		return LangJapanese
	}

	if cjkRatio > 0.3 && kanaRatio < 0.02 {
		return LangChinese
	}

	if alphaRatio > 0.5 {
		if kanaRatio > 0.02 || cjkRatio > 0.05 {
			return LangMixed
		}
		return LangEnglish
	}

	if kanaRatio > 0.02 {
		return LangJapanese
	}

	if alphaRatio > 0.3 {
		if cjkRatio > 0.05 {
			return LangMixed
		}
		return LangEnglish
	}

	if cjkRatio > 0.1 {
		return LangChinese
	}

	return LangUnknown
}

var romanjiSuffixes = []string{"_ro", "_romaji", "_romanji", "_roman", "_rm"}

func LangHintFromFilename(path string) (Language, bool) {
	base := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	for _, s := range romanjiSuffixes {
		if strings.HasSuffix(name, s) {
			return LangRomanji, true
		}
	}
	return LangUnknown, false
}

type OutputConfig struct {
	NoDedupe               bool
	NoBlankLine            bool
	BlankLineBetweenGroups bool
	NoBlankColumns         bool
}

func DefaultOutputConfig() OutputConfig {
	return OutputConfig{
		NoDedupe:               false,
		NoBlankLine:            false,
		BlankLineBetweenGroups: true,
		NoBlankColumns:         false,
	}
}

func (c OutputConfig) WithDedupe() OutputConfig {
	c.NoDedupe = false
	return c
}

func (c OutputConfig) WithoutDedupe() OutputConfig {
	c.NoDedupe = true
	return c
}

func (c OutputConfig) WithBlankLines() OutputConfig {
	c.BlankLineBetweenGroups = true
	return c
}

func (c OutputConfig) WithEmptyPlaceholders() OutputConfig {
	c.NoBlankColumns = false
	return c
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
