package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lyrictools/src/infra/combine"
	"lyrictools/src/infra/processor"
	"lyrictools/src/ui/tui"
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "split":
		runSplit(os.Args[2:])
	case "combine":
		runCombine(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
	default:
		die("未知命令: %s\n\n", os.Args[1])
		printUsage()
	}
}

func runSplit(args []string) {
	fs := flag.NewFlagSet("split", flag.ExitOnError)
	output := fs.String("o", "", "输出文件路径或目录（可选）")
	batch := fs.Bool("b", false, "批量处理目录中的所有歌词文件")
	tuiFlag := fs.Bool("tui", false, "启动终端用户界面")
	fs.Usage = func() {
		fmt.Println("用法: lyrictools split [选项] <输入文件或目录>")
		fmt.Println()
		fs.PrintDefaults()
		fmt.Println()
		fmt.Println("不带参数运行 split 将启动 TUI 界面")
	}
	fs.Parse(args)

	input := fs.Arg(0)

	if *tuiFlag || (input == "" && !*batch) {
		runTUI()
		return
	}

	if input == "" {
		fs.Usage()
		os.Exit(1)
	}

	if *batch || isDir(input) {
		fmt.Printf("正在批量处理目录: %s\n", input)
		results, err := processor.BatchProcess(input, *output)
		if err != nil {
			die("错误: %v", err)
		}
		fmt.Println("\n处理结果:")
		for _, r := range results {
			fmt.Printf("  %s: %s\n", r.Filename, r.Status)
			if r.OutputPath != "" {
				fmt.Printf("    输出: %s\n", r.OutputPath)
			}
		}
		return
	}

	fmt.Printf("正在处理文件: %s\n", input)
	outFile, err := processor.ProcessFile(input, *output)
	if err != nil {
		die("错误: %v", err)
	}
	fmt.Printf("处理完成，输出文件: %s\n", outFile)

	if absOut, err := filepath.Abs(outFile); err == nil {
		fmt.Printf("完整路径: %s\n", absOut)
	}
}

func runCombine(args []string) {
	fs := flag.NewFlagSet("combine", flag.ExitOnError)
	output := fs.String("o", "", "输出文件路径（可选）")
	outputDir := fs.String("d", "", "输出目录（可选）")
	noDedupe := fs.Bool("no-dedupe", false, "关闭去重")
	noBlankLine := fs.Bool("no-blank-line", false, "关闭行组间空行")
	batch := fs.Bool("b", false, "批量模式：扫描目录按前缀自动分组合并")
	langOverride := fs.String("lang", "", "语言覆盖，逗号分隔（如 jp,cn,ro）")
	fs.Usage = func() {
		fmt.Println("用法: lyrictools combine [选项] <files...>")
		fmt.Println()
		fmt.Println("   或: lyrictools combine -b <目录>")
		fmt.Println()
		fs.PrintDefaults()
	}

	fs.Parse(args)

	if *batch {
		inputDir := fs.Arg(0)
		if inputDir == "" {
			fs.Usage()
			os.Exit(1)
		}

		var langOverrides []combine.Language
		if *langOverride != "" {
			for _, s := range strings.Split(*langOverride, ",") {
				s = strings.TrimSpace(s)
				langOverrides = append(langOverrides, combine.Language(s))
			}
		}

		config := processor.CombineConfig{
			OutputDir:    *outputDir,
			NoDedupe:     *noDedupe,
			NoBlankLine:  *noBlankLine,
			LangOverride: langOverrides,
		}

		fmt.Printf("正在扫描目录: %s\n", inputDir)
		result := processor.BatchCombine(inputDir, *outputDir, config)

		fmt.Printf("\n批量合并完成:\n")
		fmt.Printf("  总组数: %d\n", result.TotalGroups)
		fmt.Printf("  成功: %d, 失败: %d\n", result.SuccessCount, result.FailCount)

		for _, gr := range result.Groups {
			if gr.Result.Error != nil {
				fmt.Printf("  ✗ %s: %v\n", gr.Prefix, gr.Result.Error)
			} else {
				fmt.Printf("  ✓ %s → %s (%d 行组)\n", gr.Prefix, gr.Result.OutputPath, len(gr.Result.Groups))
			}
		}
		return
	}

	var filePaths []string
	for _, arg := range fs.Args() {
		if arg != "" {
			filePaths = append(filePaths, arg)
		}
	}

	if len(filePaths) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("正在合并 %d 个文件...\n", len(filePaths))
	for i, fp := range filePaths {
		fmt.Printf("  %d. %s\n", i+1, fp)
	}

	var langOverrides []combine.Language
	if *langOverride != "" {
		for _, s := range strings.Split(*langOverride, ",") {
			s = strings.TrimSpace(s)
			langOverrides = append(langOverrides, combine.Language(s))
		}
	}

	config := processor.CombineConfig{
		OutputPath:  *output,
		OutputDir:   *outputDir,
		NoDedupe:    *noDedupe,
		NoBlankLine: *noBlankLine,
		LangOverride: langOverrides,
	}

	result := processor.ProcessCombine(filePaths, config)

	if result.Error != nil {
		die("合并失败: %v", result.Error)
	}

	fmt.Printf("\n合并完成，输出文件: %s\n", result.OutputPath)
	if absOut, err := filepath.Abs(result.OutputPath); err == nil {
		fmt.Printf("完整路径: %s\n", absOut)
	}
	fmt.Printf("共 %d 个行组\n", len(result.Groups))

	if len(result.Warnings) > 0 {
		fmt.Println("\n警告:")
		for _, w := range result.Warnings {
			fmt.Printf("  %s\n", w)
		}
	}

	for _, f := range result.Files {
		fmt.Printf("  %s → 语种: %s\n", filepath.Base(f.Path), f.Language)
	}
}

func printUsage() {
	fmt.Println("用法: lyrictools <command> [选项]")
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  split   <file> [-o output] [-b] [-tui]  拆分混合语言歌词文件为独立原文行和翻译行")
	fmt.Println("  combine <files...>                       合并原文与翻译行为独立 LRC 文件")
	fmt.Println()
	fmt.Println("不带参数运行将启动 TUI 界面")
}

func runTUI() {
	if err := tui.Run(); err != nil {
		die("错误: %v", err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
