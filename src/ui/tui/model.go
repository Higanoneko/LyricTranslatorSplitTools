package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lyrictools/src/infra/combine"
	"lyrictools/src/infra/processor"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Padding(0, 1)

	highlightStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5D62")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#53B3CB"))

	currentDirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Italic(true).
			Padding(0, 1).PaddingBottom(1)

	previewBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
)

type state int

const (
	stateModeSelect state = iota
	stateBrowse
	stateResults
	stateConfirm
	stateCombineBrowse
	stateCombineResults
)

type fileEntry struct {
	name     string
	isDir    bool
	language combine.Language
}

type cursorPos struct {
	cur    int
	offset int
}

type model struct {
	currentDir    string
	files         []fileEntry
	state         state
	width         int
	height        int
	results       []processor.BatchResult
	err           error
	mode          string
	confirmIdx    int
	modeSelectIdx int

	splitCursor   cursorPos
	combineCursor cursorPos

	combinedSelected   []int
	combinedOrder      []int
	combineResult      processor.CombineResultSummary
	combineErr         error
	combineBatchMode   bool
	combineBatchResult processor.BatchCombineResult
	combineBatchCursor int
	combineGroups      map[string][]string
}

func Run() error {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel() model {
	dir, err := os.Getwd()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	m := model{
		currentDir: dir,
		state:      stateModeSelect,
	}
	m.loadFiles()
	return m
}

func (m *model) loadFiles() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.err = err
		return
	}

	m.files = nil

	var dirs, lyrics []fileEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, fileEntry{name: e.Name(), isDir: true})
		} else if processor.IsLyricFile(e.Name()) {
			lyrics = append(lyrics, fileEntry{name: e.Name(), isDir: false})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(lyrics, func(i, j int) bool { return lyrics[i].name < lyrics[j].name })

	m.files = append(dirs, lyrics...)
}

func (m *model) loadCombineLanguages() {
	for i := range m.files {
		f := &m.files[i]
		if f.isDir || f.language != combine.LangUnknown {
			continue
		}
		fp := filepath.Join(m.currentDir, f.name)
		if lang, ok := combine.LangHintFromFilename(fp); ok {
			f.language = lang
			continue
		}
		lines, err := processor.ReadLines(fp)
		if err != nil {
			continue
		}
		f.language = combine.InferFileLanguage(lines)
	}
}

func (m *model) enterDirectory() bool {
	if len(m.files) == 0 {
		return false
	}
	cp := m.activeCursor()
	if cp.cur < 0 || cp.cur >= len(m.files) {
		return false
	}
	if !m.files[cp.cur].isDir {
		return false
	}
	m.currentDir = filepath.Join(m.currentDir, m.files[cp.cur].name)
	m.loadFiles()
	m.resetCursor()
	return true
}

func (m *model) goUpOrBack() bool {
	parent := filepath.Dir(m.currentDir)
	if parent != m.currentDir {
		m.currentDir = parent
		m.loadFiles()
		m.resetCursor()
		return true
	}
	return false
}

func (m *model) goHome() {
	m.currentDir, _ = os.UserHomeDir()
	m.loadFiles()
	m.resetCursor()
}

func (m *model) resetCursor() {
	cp := m.activeCursor()
	cp.cur = 0
	cp.offset = 0
}

func (m *model) activeCursor() *cursorPos {
	switch m.state {
	case stateCombineBrowse:
		return &m.combineCursor
	default:
		return &m.splitCursor
	}
}

func (m *model) activeCursorVal() cursorPos {
	return *m.activeCursor()
}

func (m model) cursorUp(c cursorPos) cursorPos {
	if c.cur > 0 {
		c.cur--
		if c.cur < c.offset {
			c.offset = c.cur
		}
	}
	return c
}

func (m model) cursorDown(c cursorPos) cursorPos {
	if c.cur < len(m.files)-1 {
		c.cur++
		visible := m.visibleLines()
		if c.cur >= c.offset+visible {
			c.offset = c.cur - visible + 1
		}
	}
	return c
}

func (m model) cursorHome() cursorPos {
	return cursorPos{cur: 0, offset: 0}
}

func (m model) cursorEnd() cursorPos {
	return cursorPos{cur: len(m.files) - 1, offset: 0}
}

func (m model) visibleLines() int {
	v := m.height - 10
	if v < 1 {
		v = 1
	}
	return v
}

func (m model) visibleRange(offset int) (int, int) {
	visible := m.visibleLines()
	end := offset + visible
	if end > len(m.files) {
		end = len(m.files)
	}
	return offset, end
}

func (m model) title() string {
	return titleStyle.Render("🎵 LyricTools")
}

func (m model) currentDirLine() string {
	return currentDirStyle.Render("📂 " + m.currentDir)
}

func (m model) renderHeader(b *strings.Builder) {
	b.WriteString(m.title())
	b.WriteString("\n")
	b.WriteString(m.currentDirLine())
	b.WriteString("\n")
}

func (m model) renderError(b *strings.Builder) {
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("错误: %v", m.err)))
		b.WriteString("\n")
	}
}

type rowRenderer func(i int, f fileEntry) (string, lipgloss.Style)

func (m model) renderFileGrid(b *strings.Builder, cursor cursorPos, fn rowRenderer) {
	start, end := m.visibleRange(cursor.offset)
	for i := start; i < end; i++ {
		text, style := fn(i, m.files[i])
		b.WriteString(style.Render(text))
		b.WriteString("\n")
	}
	if len(m.files) == 0 {
		b.WriteString(normalStyle.Render("  (空目录)"))
		b.WriteString("\n")
	}
}

func (m model) combineOrderLabel() string {
	parts := make([]string, len(m.combinedOrder))
	for i, idx := range m.combinedOrder {
		parts[i] = fmt.Sprintf("%d:%s", i+1, m.files[idx].name)
	}
	return strings.Join(parts, "  ")
}

func (m model) isSelected(fileIdx int) (bool, int) {
	for oi, si := range m.combinedOrder {
		if si == fileIdx {
			return true, oi + 1
		}
	}
	return false, 0
}

func (m model) toggleSelection(fileIdx int) {
	removed := false
	newSelected := make([]int, 0, len(m.combinedSelected))
	for _, s := range m.combinedSelected {
		if s == fileIdx {
			removed = true
		} else {
			newSelected = append(newSelected, s)
		}
	}
	m.combinedSelected = newSelected
	newOrder := make([]int, 0, len(m.combinedOrder))
	for _, o := range m.combinedOrder {
		if o != fileIdx {
			newOrder = append(newOrder, o)
		}
	}
	m.combinedOrder = newOrder
	if !removed {
		m.combinedSelected = append(m.combinedSelected, fileIdx)
		m.combinedOrder = append(m.combinedOrder, fileIdx)
	}
}

func (m model) shiftOrderLeft(curIdx int) {
	if len(m.combinedOrder) < 2 {
		return
	}
	for i, fi := range m.combinedOrder {
		if fi == curIdx && i > 0 {
			m.combinedOrder[i], m.combinedOrder[i-1] = m.combinedOrder[i-1], m.combinedOrder[i]
			return
		}
	}
}

func (m model) shiftOrderRight(curIdx int) {
	if len(m.combinedOrder) < 2 {
		return
	}
	for i, fi := range m.combinedOrder {
		if fi == curIdx && i < len(m.combinedOrder)-1 {
			m.combinedOrder[i], m.combinedOrder[i+1] = m.combinedOrder[i+1], m.combinedOrder[i]
			return
		}
	}
}

func (m model) lyricFilePaths() []string {
	var paths []string
	for _, f := range m.files {
		if !f.isDir {
			paths = append(paths, filepath.Join(m.currentDir, f.name))
		}
	}
	return paths
}

func (m model) getLyricFiles() []string {
	var result []string
	for _, f := range m.files {
		if !f.isDir {
			result = append(result, f.name)
		}
	}
	return result
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		return m.handleQuit()
	}

	switch m.state {
	case stateModeSelect:
		return m.updateModeSelect(msg)
	case stateBrowse:
		return m.updateBrowse(msg)
	case stateResults:
		return m.updateResults(msg)
	case stateConfirm:
		return m.updateConfirm(msg)
	case stateCombineBrowse:
		return m.updateCombineBrowse(msg)
	case stateCombineResults:
		return m.updateCombineResults(msg)
	}

	return m, nil
}

func (m model) handleQuit() (tea.Model, tea.Cmd) {
	if m.state == stateModeSelect {
		return m, tea.Quit
	}
	m.state = stateModeSelect
	return m, nil
}

func (m model) updateModeSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "backspace":
		return m, tea.Quit

	case "up", "k":
		if m.modeSelectIdx > 0 {
			m.modeSelectIdx--
		}

	case "down", "j":
		if m.modeSelectIdx < 1 {
			m.modeSelectIdx++
		}

	case "enter":
		switch m.modeSelectIdx {
		case 0:
			m.state = stateBrowse
			m.loadFiles()
			m.splitCursor = cursorPos{}
		case 1:
			m.combinedSelected = nil
			m.combinedOrder = nil
			m.combineCursor = cursorPos{}
			m.state = stateCombineBrowse
			m.loadCombineLanguages()
		}
	}

	return m, nil
}

func (m model) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cp := m.splitCursor

	switch msg.String() {
	case "up", "k":
		m.splitCursor = m.cursorUp(cp)
	case "down", "j":
		m.splitCursor = m.cursorDown(cp)
	case "home":
		m.splitCursor = m.cursorHome()
	case "end":
		m.splitCursor = m.cursorEnd()
	case "r":
		m.loadFiles()
	case "h":
		m.goHome()

	case "backspace":
		if !m.goUpOrBack() {
			m.state = stateModeSelect
		}

	case "enter":
		if m.enterDirectory() {
			m.splitCursor = cursorPos{}
			return m, nil
		}
		if cp.cur < 0 || cp.cur >= len(m.files) || m.files[cp.cur].isDir {
			return m, nil
		}
		m.mode = "single"
		m.processSingle(filepath.Join(m.currentDir, m.files[cp.cur].name))
		return m, nil

	case "tab":
		lyricFiles := m.getLyricFiles()
		if len(lyricFiles) > 0 {
			m.mode = "batch"
			m.confirmIdx = 0
			m.state = stateConfirm
		}

	case " ":
		lyricFiles := m.getLyricFiles()
		if len(lyricFiles) > 0 {
			m.mode = "batch"
			m.confirmIdx = 0
			m.state = stateConfirm
		}
	}

	return m, nil
}

func (m *model) processSingle(path string) {
	out, err := processor.ProcessFile(path, "")
	if err != nil {
		m.results = []processor.BatchResult{
			{Filename: filepath.Base(path), Status: "失败: " + err.Error(), OutputPath: ""},
		}
	} else {
		m.results = []processor.BatchResult{
			{Filename: filepath.Base(path), Status: "成功", OutputPath: out},
		}
	}
	m.state = stateResults
}

func (m *model) processBatch(dir string) {
	results, err := processor.BatchProcess(dir, "")
	if err != nil {
		m.results = []processor.BatchResult{
			{Filename: dir, Status: "失败: " + err.Error(), OutputPath: ""},
		}
	} else {
		m.results = results
	}
	m.state = stateResults
}

func (m model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "b", "esc":
		m.state = stateBrowse
		m.results = nil
		m.loadFiles()
	case "backspace":
		m.state = stateModeSelect
		m.results = nil
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.processBatch(m.currentDir)
	case "n", "esc":
		m.state = stateBrowse
	case "backspace":
		m.state = stateModeSelect
	}
	return m, nil
}

func (m model) updateCombineBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.combineBatchMode {
		return m.updateCombineBatchMode(msg)
	}
	return m.updateCombineSingleMode(msg)
}

func (m model) updateCombineSingleMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cp := m.combineCursor

	switch msg.String() {
	case "up", "k":
		m.combineCursor = m.cursorUp(cp)
	case "down", "j":
		m.combineCursor = m.cursorDown(cp)
	case "home":
		m.combineCursor = m.cursorHome()
	case "end":
		m.combineCursor = m.cursorEnd()
	case "r":
		m.loadFiles()
		m.loadCombineLanguages()
	case "h":
		m.goHome()
		m.loadCombineLanguages()

	case "backspace":
		if !m.goUpOrBack() {
			m.state = stateModeSelect
		}
		m.loadCombineLanguages()

	case "enter":
		if m.enterDirectory() {
			m.loadCombineLanguages()
			m.combineCursor = cursorPos{}
			return m, nil
		}
		if cp.cur < 0 || cp.cur >= len(m.files) || m.files[cp.cur].isDir {
			return m, nil
		}
		if len(m.combinedOrder) < 2 {
			return m, nil
		}
		return m.executeCombine()

	case " ":
		if cp.cur >= 0 && cp.cur < len(m.files) && !m.files[cp.cur].isDir {
			m.toggleSelection(cp.cur)
		}

	case "left":
		m.shiftOrderLeft(cp.cur)
	case "right":
		m.shiftOrderRight(cp.cur)

	case "tab":
		m.combineBatchMode = true
		m.combineBatchCursor = 0
		m.combineGroups = processor.GroupCombineFiles(m.lyricFilePaths())
	}

	return m, nil
}

func (m model) updateCombineBatchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := sortedKeys(m.combineGroups)

	switch msg.String() {
	case "backspace":
		m.state = stateModeSelect
		return m, nil

	case "tab":
		m.combineBatchMode = false
		return m, nil

	case "up", "k":
		if m.combineBatchCursor > 0 {
			m.combineBatchCursor--
		}

	case "down", "j":
		if m.combineBatchCursor < len(keys)-1 {
			m.combineBatchCursor++
		}

	case "enter":
		if len(keys) == 0 {
			return m, nil
		}
		m.combineBatchResult = processor.BatchCombine(m.currentDir, "", processor.CombineConfig{})
		m.state = stateCombineResults

	case "r":
		m.combineGroups = processor.GroupCombineFiles(m.lyricFilePaths())
	}

	return m, nil
}

func (m model) executeCombine() (tea.Model, tea.Cmd) {
	var paths []string
	for _, idx := range m.combinedOrder {
		paths = append(paths, filepath.Join(m.currentDir, m.files[idx].name))
	}
	result := processor.ProcessCombine(paths, processor.CombineConfig{})
	if result.Error != nil {
		m.combineErr = result.Error
	} else {
		m.combineResult = result
		m.combineErr = nil
	}
	m.state = stateCombineResults
	return m, nil
}

func (m model) updateCombineResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "backspace", "esc":
		m.state = stateModeSelect
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateModeSelect:
		return m.viewModeSelect()
	case stateBrowse:
		return m.viewBrowse()
	case stateResults:
		return m.viewResults()
	case stateConfirm:
		return m.viewConfirm()
	case stateCombineBrowse:
		return m.viewCombineBrowse()
	case stateCombineResults:
		return m.viewCombineResults()
	}
	return ""
}

func (m model) viewBrowse() string {
	var b strings.Builder
	m.renderHeader(&b)
	m.renderError(&b)

	m.renderFileGrid(&b, m.splitCursor, func(i int, f fileEntry) (string, lipgloss.Style) {
		prefix, icon := "  ", "📄"
		if f.isDir {
			prefix, icon = "📁", ""
		}
		style := normalStyle
		if i == m.splitCursor.cur {
			style = highlightStyle
		}
		return fmt.Sprintf("%s %s%s", prefix, f.name, icon), style
	})

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑ ↓ / J K 导航  ENTER 处理文件  TAB 批量模式  BACKSPACE 返回  R 刷新  H 主目录  Q 返回"))
	b.WriteString("\n")
	return b.String()
}

func (m model) viewResults() string {
	var b strings.Builder

	b.WriteString(m.title() + " - 处理结果")
	b.WriteString("\n\n")

	if m.mode == "batch" {
		successCount := 0
		failCount := 0
		for _, r := range m.results {
			if r.Status == "成功" {
				successCount++
			} else {
				failCount++
			}
		}

		b.WriteString(fmt.Sprintf("%s  |  %s\n\n",
			successStyle.Render(fmt.Sprintf("成功 %d", successCount)),
			errorStyle.Render(fmt.Sprintf("失败 %d", failCount)),
		))

		const previewLines = 6
		total := len(m.results)
		show := total
		if show > previewLines {
			show = previewLines
		}
		for _, r := range m.results[:show] {
			if r.Status == "成功" {
				b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ %s", r.Filename)))
			} else {
				b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ %s: %s", r.Filename, r.Status)))
			}
			b.WriteString("\n")
		}
		if total > previewLines {
			b.WriteString(fmt.Sprintf("  ... 还有 %d 个结果\n", total-previewLines))
		}
	} else {
		for _, r := range m.results {
			if r.Status == "成功" {
				b.WriteString(successStyle.Render("✓ 处理成功！"))
				b.WriteString("\n\n")
				b.WriteString("输入: " + r.Filename + "\n")
				b.WriteString("输出: " + r.OutputPath + "\n")

				lines, err := processor.ReadLines(r.OutputPath)
				if err == nil && len(lines) > 0 {
					maxShow := 12
					if maxShow > len(lines) {
						maxShow = len(lines)
					}
					b.WriteString("\n" + previewBoxStyle.Render(strings.Join(lines[:maxShow], "\n")))
					if len(lines) > maxShow {
						b.WriteString(fmt.Sprintf("\n  ... 还有 %d 行", len(lines)-maxShow))
					}
				}
			} else {
				b.WriteString(errorStyle.Render("✗ " + r.Filename + ": " + r.Status))
			}
		}
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("ENTER / ESC / B 返回  Q 返回"))

	return b.String()
}

func (m model) viewConfirm() string {
	lyricFiles := m.getLyricFiles()

	var b strings.Builder

	b.WriteString(m.title() + " - 批量处理确认")
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("目录: %s\n", m.currentDir))
	b.WriteString(fmt.Sprintf("将处理 %d 个歌词文件:\n\n", len(lyricFiles)))

	const maxShow = 10
	show := len(lyricFiles)
	if show > maxShow {
		show = maxShow
	}
	for _, name := range lyricFiles[:show] {
		b.WriteString(fmt.Sprintf("  📄 %s\n", name))
	}
	if len(lyricFiles) > maxShow {
		b.WriteString(fmt.Sprintf("  ... 还有 %d 个文件\n", len(lyricFiles)-maxShow))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Y 确认处理  N / ESC 取消"))

	return b.String()
}

func (m model) viewModeSelect() string {
	var b strings.Builder

	b.WriteString(m.title())
	b.WriteString("\n\n")
	b.WriteString(normalStyle.Render("请选择操作模式:"))
	b.WriteString("\n\n")

	options := []struct {
		label       string
		description string
	}{
		{"📝 分割模式", "将混合语言歌词拆分为独立原文行和翻译行"},
		{"🔗 合并模式", "将多个独立歌词文件合并为一个多列文件"},
	}

	for i, opt := range options {
		style := normalStyle
		if i == m.modeSelectIdx {
			style = highlightStyle
		}
		b.WriteString(style.Render(opt.label))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("   " + opt.description))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("↑ ↓ / J K 选择  ENTER 确认  Q / BACKSPACE 退出"))

	return b.String()
}

func (m model) viewCombineBrowse() string {
	if m.combineBatchMode {
		return m.viewCombineBatchMode()
	}
	return m.viewCombineSingleMode()
}

func (m model) viewCombineSingleMode() string {
	var b strings.Builder

	m.renderHeader(&b)
	b.WriteString(infoStyle.Render("[单文件模式] 手动选择文件"))
	b.WriteString("\n\n")
	m.renderError(&b)

	if len(m.combinedOrder) > 0 {
		b.WriteString(infoStyle.Render(fmt.Sprintf("已选 %d 个文件 (列序: %s)",
			len(m.combinedOrder),
			m.combineOrderLabel(),
		)))
		b.WriteString("\n\n")
	}

	m.renderFileGrid(&b, m.combineCursor, func(i int, f fileEntry) (string, lipgloss.Style) {
		if f.isDir {
			style := normalStyle
			if i == m.combineCursor.cur {
				style = highlightStyle
			}
			return fmt.Sprintf("📁 %s", f.name), style
		}

		isSel, orderNum := m.isSelected(i)
		prefix := "[ ]"
		if isSel {
			prefix = fmt.Sprintf("[%d]", orderNum)
		}

		langLabel := ""
		if f.language != combine.LangUnknown {
			langLabel = fmt.Sprintf(" (%s)", f.language)
		}

		style := normalStyle
		if i == m.combineCursor.cur {
			style = highlightStyle
		} else if isSel {
			style = selectedStyle
		}

		return fmt.Sprintf("%s %s%s", prefix, f.name, langLabel), style
	})

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("SPACE 选择  ↑ ↓ / J K 导航  ← → 调整列序  ENTER 执行合并  TAB 批量模式  BACKSPACE 返回  R 刷新  H 主目录  Q 返回"))
	b.WriteString("\n")
	return b.String()
}

func (m model) viewCombineBatchMode() string {
	var b strings.Builder

	m.renderHeader(&b)
	b.WriteString(infoStyle.Render("[批量模式] 自动按文件名前缀分组"))
	b.WriteString("\n\n")
	m.renderError(&b)

	keys := sortedKeys(m.combineGroups)

	if len(keys) == 0 {
		b.WriteString(normalStyle.Render("  目录中没有可分组文件"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("TAB 切换单文件模式  BACKSPACE 返回  Q 返回"))
		return b.String()
	}

	b.WriteString(fmt.Sprintf("识别到 %d 个分组:\n\n", len(keys)))

	for i, key := range keys {
		files := m.combineGroups[key]
		var display string
		if len(files) >= 2 {
			display = fmt.Sprintf("  ✓ %s → %d 文件 → %s_Combined.lrc", key, len(files), key)
		} else {
			display = fmt.Sprintf("  - %s → 仅 %d 文件 (跳过)", key, len(files))
		}

		style := normalStyle
		if i == m.combineBatchCursor {
			style = highlightStyle
		}
		b.WriteString(style.Render(display))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑ ↓ 导航  TAB 切换单文件模式  ENTER 执行批量合并  BACKSPACE 返回  R 刷新  Q 返回"))
	b.WriteString("\n")
	return b.String()
}

func (m model) viewCombineResults() string {
	var b strings.Builder

	b.WriteString(m.title() + " - 合并结果")
	b.WriteString("\n\n")

	if m.combineBatchMode {
		br := m.combineBatchResult
		b.WriteString(fmt.Sprintf("批量合并完成 (成功 %d, 失败 %d):\n\n",
			br.SuccessCount, br.FailCount))

		const maxShow = 10
		show := len(br.Groups)
		if show > maxShow {
			show = maxShow
		}
		for _, gr := range br.Groups[:show] {
			if gr.Result.Error != nil {
				b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ %s: %v\n", gr.Prefix, gr.Result.Error)))
			} else {
				b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ %s → %s (%d 行组)\n",
					gr.Prefix, filepath.Base(gr.Result.OutputPath), len(gr.Result.Groups))))
			}
		}
		if len(br.Groups) > maxShow {
			b.WriteString(fmt.Sprintf("  ... 还有 %d 个结果\n", len(br.Groups)-maxShow))
		}
	} else {
		if m.combineErr != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("✗ 合并失败: %v", m.combineErr)))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("BACKSPACE / ESC 返回"))
			return b.String()
		}

		b.WriteString(successStyle.Render("✓ 合并成功！"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("输出文件: %s\n", m.combineResult.OutputPath))
		b.WriteString(fmt.Sprintf("行组数量: %d\n", len(m.combineResult.Groups)))

		if len(m.combineResult.Warnings) > 0 {
			b.WriteString("\n")
			b.WriteString(errorStyle.Render(fmt.Sprintf("⚠ 警告 (%d):", len(m.combineResult.Warnings))))
			b.WriteString("\n")
			const maxWarnings = 5
			showW := len(m.combineResult.Warnings)
			if showW > maxWarnings {
				showW = maxWarnings
			}
			for _, w := range m.combineResult.Warnings[:showW] {
				b.WriteString(fmt.Sprintf("  %s\n", w))
			}
			if len(m.combineResult.Warnings) > maxWarnings {
				b.WriteString(fmt.Sprintf("  ... 还有 %d 个警告\n", len(m.combineResult.Warnings)-maxWarnings))
			}
		}

		b.WriteString("\n")
		b.WriteString("源文件语种:\n")
		for _, f := range m.combineResult.Files {
			b.WriteString(fmt.Sprintf("  %s → %s\n", filepath.Base(f.Path), f.Language))
		}

		lines, err := processor.ReadLines(m.combineResult.OutputPath)
		if err == nil && len(lines) > 0 {
			maxShow := 10
			if maxShow > len(lines) {
				maxShow = len(lines)
			}
			b.WriteString("\n" + previewBoxStyle.Render(strings.Join(lines[:maxShow], "\n")))
			if len(lines) > maxShow {
				b.WriteString(fmt.Sprintf("\n  ... 还有 %d 行", len(lines)-maxShow))
			}
		}
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("BACKSPACE / ESC / Q 返回模式选择"))

	return b.String()
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
