package combine

import (
	"testing"
)

func TestParseSourceFile(t *testing.T) {
	lines := []string{
		"[ti:Test Song]",
		"[ar:Test Artist]",
		"[00:01.00]Line one JP",
		"[00:02.00]Line two JP",
		"",
		"[00:01.00]第一行 中文",
		"[00:02.00]第二行 中文",
	}

	sf := ParseSourceFile("test.lrc", lines)

	if sf.Path != "test.lrc" {
		t.Errorf("Path = %q, want %q", sf.Path, "test.lrc")
	}
	if sf.MetaTags["ti"] != "Test Song" {
		t.Errorf("MetaTags[ti] = %q, want %q", sf.MetaTags["ti"], "Test Song")
	}
	if sf.MetaTags["ar"] != "Test Artist" {
		t.Errorf("MetaTags[ar] = %q, want %q", sf.MetaTags["ar"], "Test Artist")
	}
	if !sf.HasTimestamps {
		t.Error("HasTimestamps should be true with timestamped lines")
	}
}

func TestParseSourceFileNoTimestamps(t *testing.T) {
	lines := []string{
		"[ti:No TS]",
		"Line one",
		"Line two",
	}

	sf := ParseSourceFile("no_ts.lrc", lines)

	if sf.HasTimestamps {
		t.Error("HasTimestamps should be false when no timestamps")
	}
	if len(sf.MetaTags) != 1 {
		t.Errorf("expected 1 meta tag, got %d", len(sf.MetaTags))
	}
}

func TestAlignByTimestamp(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.lrc", []string{
			"[ti:Song]",
			"[00:01.00]JP Line 1",
			"[00:02.00]JP Line 2",
			"[00:03.00]JP Line 3",
		}),
		ParseSourceFile("cn.lrc", []string{
			"[ar:Artist]",
			"[00:01.00]CN Line 1",
			"[00:03.00]CN Line 3",
			"[00:04.00]CN Line 4",
		}),
	}

	groups, warnings := AlignAndCombine(files)

	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups (base file timestamps), got %d", len(groups))
	}

	if groups[0].Timestamp != "00:01.00" {
		t.Errorf("group[0].Timestamp = %q, want %q", groups[0].Timestamp, "00:01.00")
	}
	if groups[0].Columns[0] != "JP Line 1" {
		t.Errorf("group[0].Columns[0] = %q, want %q", groups[0].Columns[0], "JP Line 1")
	}
	if groups[0].Columns[1] != "CN Line 1" {
		t.Errorf("group[0].Columns[1] = %q, want %q", groups[0].Columns[1], "CN Line 1")
	}
	if groups[1].Timestamp != "00:02.00" {
		t.Errorf("group[1].Timestamp = %q, want %q", groups[1].Timestamp, "00:02.00")
	}
	if groups[1].Columns[1] != "" {
		t.Errorf("group[1].Columns[1] = %q, want empty", groups[1].Columns[1])
	}
	if groups[2].Columns[1] != "CN Line 3" {
		t.Errorf("group[2].Columns[1] = %q, want %q", groups[2].Columns[1], "CN Line 3")
	}
}

func TestAlignByLineNumber(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.txt", []string{
			"JP Line 1",
			"JP Line 2",
			"JP Line 3",
		}),
		ParseSourceFile("cn.txt", []string{
			"Extra header",
			"CN Line 1",
			"CN Line 2",
			"CN Line 3",
		}),
	}

	groups, warnings := AlignAndCombine(files)

	if len(warnings) == 0 {
		t.Log("expected warnings for missing timestamps")
	}

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	if groups[0].Columns[0] != "JP Line 1" {
		t.Errorf("group[0].Columns[0] = %q, want %q", groups[0].Columns[0], "JP Line 1")
	}
	if groups[0].Columns[1] != "CN Line 1" {
		t.Errorf("group[0].Columns[1] = %q, want %q", groups[0].Columns[1], "CN Line 1")
	}
}

func TestBuildOutput(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.lrc", []string{
			"[ti:Song]",
			"[00:01.00]JP 1",
			"[00:02.00]JP 2",
		}),
		ParseSourceFile("cn.lrc", []string{
			"[00:01.00]CN 1",
			"[00:02.00]CN 2",
		}),
	}

	groups, _ := AlignAndCombine(files)
	result := CombineResult{
		Groups:      groups,
		SourceFiles: files,
	}

	config := DefaultOutputConfig()
	config = config.WithDedupe()
	output := BuildOutput(result, config)

	if len(output) != 7 {
		t.Fatalf("expected 7 output lines (meta + blank + 2 groups + blank between groups), got %d: %v", len(output), output)
	}

	if output[0] != "[ti:Song]" {
		t.Errorf("output[0] = %q, want %q", output[0], "[ti:Song]")
	}
	if output[2] != "[00:01.00]JP 1" {
		t.Errorf("output[2] = %q, want %q", output[2], "[00:01.00]JP 1")
	}
	if output[3] != "[00:01.00]CN 1" {
		t.Errorf("output[3] = %q, want %q", output[3], "[00:01.00]CN 1")
	}
}

func TestInferFileLanguage(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected Language
	}{
		{
			name: "Japanese",
			lines: []string{
				"交差点の真ん中 急ぐ人に紛れて",
				"朝焼けがきれいで 今日が始まる",
				"ああ また新しい一日",
			},
			expected: LangJapanese,
		},
		{
			name: "Chinese",
			lines: []string{
				"置身十字路口正中央 混入熙来攘往的人群",
				"朝霞绚丽 新的一天就要开启",
				"啊 崭新的一天又来啦",
			},
			expected: LangChinese,
		},
		{
			name: "English",
			lines: []string{
				"Hello world this is a test",
				"The quick brown fox jumps over",
				"Another day another dollar",
			},
			expected: LangEnglish,
		},
		{
			name: "Mixed JP+CN",
			lines: []string{
				"朝焼けがきれいで 朝霞绚丽",
				"交差点の真ん中 十字路口正中央",
			},
			expected: LangJapanese,
		},
		{
			name: "Mixed JP+EN (Romaji)",
			lines: []string{
				"夏の終わり Hello world",
				"君と町で Goodbye summer",
			},
			expected: LangMixed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferFileLanguage(tt.lines)
			if got != tt.expected {
				t.Errorf("InferFileLanguage() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDetectOffset(t *testing.T) {
	base := []TimestampedLine{
		{Content: "JP Line 1"},
		{Content: "JP Line 2"},
		{Content: "JP Line 3"},
	}

	other := []TimestampedLine{
		{Content: "Extra Header"},
		{Content: "CN Line 1"},
		{Content: "CN Line 2"},
		{Content: "CN Line 3"},
	}

	offset := detectOffset(base, other)
	if offset != 1 {
		t.Errorf("detectOffset = %d, want 1 (other file has 1 extra header line)", offset)
	}
}

func TestDeduplication(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.lrc", []string{
			"[00:01.00]Hello world",
			"[00:02.00]Hello world",
		}),
		ParseSourceFile("cn.lrc", []string{
			"[00:01.00]Hello world",
			"[00:02.00]Different text",
		}),
	}

	groups, _ := AlignAndCombine(files)
	result := CombineResult{
		Groups:      groups,
		SourceFiles: files,
	}

	dedupeConfig := DefaultOutputConfig().WithDedupe()
	output := BuildOutput(result, dedupeConfig)

	count := 0
	for _, line := range output {
		if line == "[00:01.00]Hello world" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("with dedupe, 'Hello world' should appear once, got %d", count)
	}

	noDedupConfig := DefaultOutputConfig().WithoutDedupe()
	output2 := BuildOutput(result, noDedupConfig)

	count2 := 0
	for _, line := range output2 {
		if line == "[00:01.00]Hello world" {
			count2++
		}
	}
	if count2 != 2 {
		t.Errorf("without dedupe, 'Hello world' should appear twice, got %d", count2)
	}
}

func TestEmptyPlaceholder(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.lrc", []string{
			"[00:01.00]JP 1",
			"[00:02.00]JP 2",
		}),
		ParseSourceFile("cn.lrc", []string{
			"[00:01.00]CN 1",
		}),
	}

	groups, _ := AlignAndCombine(files)
	result := CombineResult{
		Groups:      groups,
		SourceFiles: files,
	}

	config := DefaultOutputConfig().WithEmptyPlaceholders()
	output := BuildOutput(result, config)

	found := false
	for _, line := range output {
		if line == "[00:02.00]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected empty placeholder [00:02.00] for missing CN content")
	}
}

func TestMetaConflict(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.lrc", []string{
			"[ti:Song]",
			"[ar:Artist A]",
			"[00:01.00]JP 1",
		}),
		ParseSourceFile("cn.lrc", []string{
			"[ar:Artist B]",
			"[00:01.00]CN 1",
		}),
	}

	groups, _ := AlignAndCombine(files)
	result := CombineResult{
		Groups:      groups,
		SourceFiles: files,
	}

	output := BuildOutput(result, DefaultOutputConfig())

	foundConflict := false
	for _, line := range output {
		if len(line) >= 8 {
			for i := 0; i <= len(line)-8; i++ {
				if line[i:i+8] == "conflict" {
					foundConflict = true
					break
				}
			}
		}
	}
	if !foundConflict {
		t.Errorf("expected conflict marker in output, got: %v", output)
	}
}

func TestBlankLinesBetweenGroups(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("jp.lrc", []string{
			"[00:01.00]JP 1",
			"[00:02.00]JP 2",
		}),
		ParseSourceFile("cn.lrc", []string{
			"[00:01.00]CN 1",
			"[00:02.00]CN 2",
		}),
	}

	groups, _ := AlignAndCombine(files)
	result := CombineResult{
		Groups:      groups,
		SourceFiles: files,
	}

	withBlanks := DefaultOutputConfig().WithBlankLines()
	output := BuildOutput(result, withBlanks)

	blankCount := 0
	for _, line := range output {
		if line == "" {
			blankCount++
		}
	}
	if blankCount < 1 {
		t.Errorf("with blank lines, expected at least 1 blank line between groups, got %d", blankCount)
	}
}

func TestOutputConfigImmutability(t *testing.T) {
	cfg1 := DefaultOutputConfig()
	cfg2 := cfg1.WithDedupe()
	cfg4 := cfg1.WithoutDedupe()

	if cfg1.NoDedupe || cfg2.NoDedupe {
		t.Error("WithDedupe should set NoDedupe to false")
	}
	if !cfg4.NoDedupe {
		t.Error("WithoutDedupe should set NoDedupe to true")
	}
	if !cfg1.BlankLineBetweenGroups {
		t.Error("default config should have BlankLineBetweenGroups = true")
	}

	cfgNoBlanks := cfg1
	cfgNoBlanks.BlankLineBetweenGroups = false
	if cfg1.BlankLineBetweenGroups != true {
		t.Error("original config should not be affected by modifying copy")
	}
}

func TestContentLinesFiltering(t *testing.T) {
	sf := ParseSourceFile("test.lrc", []string{
		"[ti:Song]",
		"[ar:Artist]",
		"[00:01.00]Line 1",
		"[00:02.00]Line 2",
	})

	cl := contentLines(sf)

	if len(cl) != 2 {
		t.Fatalf("expected 2 content lines, got %d", len(cl))
	}
	if cl[0].Timestamp != "00:01.00" {
		t.Errorf("cl[0].Timestamp = %q, want %q", cl[0].Timestamp, "00:01.00")
	}
	if cl[1].Timestamp != "00:02.00" {
		t.Errorf("cl[1].Timestamp = %q, want %q", cl[1].Timestamp, "00:02.00")
	}
}

func TestEmptyCombine(t *testing.T) {
	groups, warnings := AlignAndCombine([]SourceFile{})

	if groups != nil {
		t.Error("expected nil groups for empty input")
	}
	if warnings != nil {
		t.Error("expected nil warnings for empty input")
	}
}

func TestSingleFileCombine(t *testing.T) {
	files := []SourceFile{
		ParseSourceFile("only.lrc", []string{
			"[ti:Only]",
			"[00:01.00]Line 1",
			"[00:02.00]Line 2",
		}),
	}

	groups, _ := AlignAndCombine(files)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups for single file, got %d", len(groups))
	}
	if len(groups[0].Columns) != 1 {
		t.Errorf("expected 1 column for single file, got %d", len(groups[0].Columns))
	}
}
