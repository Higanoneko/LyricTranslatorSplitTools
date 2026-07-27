package processor

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lyrictools/src/infra/combine"
	"lyrictools/src/infra/patterns"
	"lyrictools/src/infra/split"
)

func ProcessLine(line string) []string {
	original := strings.TrimRight(line, "\n\r")
	stripped := strings.TrimSpace(line)

	if stripped == "" {
		return []string{original}
	}

	stripped = patterns.RemoveCopyrightNotices(stripped)

	if patterns.IsMetaLine(stripped) {
		return []string{stripped}
	}

	ls := split.SplitLyricLine(stripped)

	if ls.Timestamp == "" {
		return []string{stripped}
	}

	if ls.Original != "" && ls.Translation != "" {
		return []string{
			split.FormatLine(ls.Timestamp, ls.Original),
			split.FormatLine(ls.Timestamp, ls.Translation),
		}
	}
	if ls.Original != "" {
		return []string{split.FormatLine(ls.Timestamp, ls.Original)}
	}
	if ls.Translation != "" {
		return []string{split.FormatLine(ls.Timestamp, ls.Translation)}
	}

	return []string{stripped}
}

func ProcessLines(lines []string) []string {
	var result []string
	for _, line := range lines {
		result = append(result, ProcessLine(line)...)
	}
	return result
}

func ReadLines(filepath string) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("找不到输入文件: %s", filepath)
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("无法解码文件 %s，请确保文件使用UTF-8编码", filepath)
	}
	return lines, nil
}

func WriteLines(filepathArg string, lines []string) error {
	dir := filepath.Dir(filepathArg)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepathArg)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, ln := range lines {
		_, err := writer.WriteString(ln + "\n")
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}

func GetOutputPath(inputPath string, suffix string) string {
	if suffix == "" {
		suffix = "_split"
	}
	return inputPath[:len(inputPath)-len(filepath.Ext(inputPath))] + suffix + filepath.Ext(inputPath)
}

func IsLyricFile(filename string) bool {
	return strings.HasSuffix(filename, ".txt") || strings.HasSuffix(filename, ".lrc")
}

func ProcessFile(inputFile string, outputFile string) (string, error) {
	if outputFile == "" {
		outputFile = GetOutputPath(inputFile, "_split")
	}
	lines, err := ReadLines(inputFile)
	if err != nil {
		return "", err
	}
	processed := ProcessLines(lines)
	if err := WriteLines(outputFile, processed); err != nil {
		return "", err
	}
	return outputFile, nil
}

type BatchResult struct {
	Filename   string
	Status     string
	OutputPath string
}

func BatchProcess(inputDir string, outputDir string) ([]BatchResult, error) {
	if outputDir == "" {
		outputDir = inputDir
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}

	var results []BatchResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !IsLyricFile(name) || strings.Contains(name, "_split") {
			continue
		}

		inputPath := filepath.Join(inputDir, name)
		outputPath := filepath.Join(outputDir, GetOutputPath(name, "_split"))

		_, procErr := ProcessFile(inputPath, outputPath)
		if procErr != nil {
			if errors.Is(procErr, os.ErrNotExist) {
				results = append(results, BatchResult{name, "失败: " + procErr.Error(), ""})
			} else {
				results = append(results, BatchResult{name, "失败: " + procErr.Error(), ""})
			}
		} else {
			results = append(results, BatchResult{name, "成功", outputPath})
		}
	}

	return results, nil
}

type CombineConfig struct {
	OutputPath    string
	NoDedupe      bool
	NoBlankLine   bool
	BlankLineSep  bool
	LangOverride  []combine.Language
	OutputDir     string
}

type CombineResultSummary struct {
	OutputPath string
	Groups     []combine.LineGroup
	Files      []combine.SourceFile
	Warnings   []string
	Error      error
}

func ProcessCombine(filePaths []string, config CombineConfig) CombineResultSummary {
	if len(filePaths) < 2 {
		return CombineResultSummary{
			Error: fmt.Errorf("合并至少需要 2 个文件"),
		}
	}

	var sourceFiles []combine.SourceFile
	for _, fp := range filePaths {
		lines, err := ReadLines(fp)
		if err != nil {
			return CombineResultSummary{
				Error: fmt.Errorf("读取 %s 失败: %w", fp, err),
			}
		}

		cleaned := removeCopyrightFromLines(lines)

		sf := combine.ParseSourceFile(fp, cleaned)
		if len(config.LangOverride) > len(sourceFiles) {
			sf.Language = config.LangOverride[len(sourceFiles)]
		}
		sourceFiles = append(sourceFiles, sf)
	}

	groups, warnings := combine.AlignAndCombine(sourceFiles)

	result := combine.CombineResult{
		Groups:      groups,
		SourceFiles: sourceFiles,
		Warnings:    warnings,
	}

	oc := combine.DefaultOutputConfig()
	if config.NoDedupe {
		oc = oc.WithoutDedupe()
	}
	if !config.NoBlankLine {
		oc = oc.WithBlankLines()
	}

	output := combine.BuildOutput(result, oc)

	outputPath := config.OutputPath
	if outputPath == "" {
		dir := config.OutputDir
		if dir == "" {
			dir = filepath.Dir(filePaths[0])
		}
		base := filepath.Base(filePaths[0])
		ext := filepath.Ext(base)
		prefix := base[:len(base)-len(ext)]
		outputPath = filepath.Join(dir, prefix+"_Combined"+ext)
	}

	if err := WriteLines(outputPath, output); err != nil {
		return CombineResultSummary{
			Error: fmt.Errorf("写入输出文件失败: %w", err),
		}
	}

	return CombineResultSummary{
		OutputPath: outputPath,
		Groups:     groups,
		Files:      sourceFiles,
		Warnings:   warnings,
	}
}

func removeCopyrightFromLines(lines []string) []string {
	var result []string
	for _, line := range lines {
		cleaned := patterns.RemoveCopyrightNotices(line)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

type BatchCombineResult struct {
	Groups       []CombineGroupResult
	OutputDir    string
	TotalGroups  int
	SuccessCount int
	FailCount    int
}

type CombineGroupResult struct {
	Prefix string
	Files  []string
	Result CombineResultSummary
}

var langSuffixes = []string{"_jp", "_cn", "_en", "_ro", "_romaji", "_romanji", "_roman", "_rm", "_ch", "_zh", "_ko"}

func GroupCombineFiles(filePaths []string) map[string][]string {
	groups := make(map[string][]string)

	for _, fp := range filePaths {
		base := filepath.Base(fp)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]

		prefix := name
		for _, s := range langSuffixes {
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(s)) {
				prefix = name[:len(name)-len(s)]
				break
			}
		}

		groups[prefix] = append(groups[prefix], fp)
	}

	return groups
}

func BatchCombine(inputDir string, outputDir string, config CombineConfig) BatchCombineResult {
	result := BatchCombineResult{
		OutputDir: outputDir,
	}

	if outputDir == "" {
		result.OutputDir = inputDir
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		result.TotalGroups = 1
		result.FailCount = 1
		result.Groups = append(result.Groups, CombineGroupResult{
			Prefix: inputDir,
			Result: CombineResultSummary{Error: fmt.Errorf("读取目录失败: %w", err)},
		})
		return result
	}

	var filePaths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if IsLyricFile(e.Name()) {
			filePaths = append(filePaths, filepath.Join(inputDir, e.Name()))
		}
	}

	groups := GroupCombineFiles(filePaths)
	result.TotalGroups = len(groups)

	for prefix, files := range groups {
		if len(files) < 2 {
			continue
		}

		cfg := config
		cfg.OutputDir = result.OutputDir
		cfg.OutputPath = filepath.Join(result.OutputDir, prefix+"_Combined.lrc")

		cr := ProcessCombine(files, cfg)

		gr := CombineGroupResult{
			Prefix: prefix,
			Files:  files,
			Result: cr,
		}

		if cr.Error != nil {
			result.FailCount++
		} else {
			result.SuccessCount++
		}

		result.Groups = append(result.Groups, gr)
	}

	return result
}
