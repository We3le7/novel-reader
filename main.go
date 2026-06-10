package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type readerState int

const (
	stateReading readerState = iota
	stateBossPanic
)

const (
	snapshotVersion   = 1
	maxPanicLogLines  = 1000
	chapterAnchorBias = 0
	chapterAlignScan  = 8
)

type snapshotFile struct {
	SchemaVersion int                        `json:"schema_version"`
	UpdatedAt     string                     `json:"updated_at"`
	LastFile      string                     `json:"last_file"`
	Profiles      map[string]snapshotProfile `json:"profiles"`
	LastLibrary   *libraryScanCache          `json:"last_library_scan,omitempty"`
}

type libraryScanCache struct {
	Root      string                `json:"root"`
	UpdatedAt string                `json:"updated_at"`
	Items     []snapshotLibraryItem `json:"items"`
}

type snapshotLibraryItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

type snapshotProfile struct {
	ByteOffset int64          `json:"byte_offset"`
	LineOffset int            `json:"line_offset"`
	TotalLines int            `json:"total_lines"`
	Window     snapshotWindow `json:"window"`
	Theme      string         `json:"theme"`
	PanicKey   string         `json:"panic_key"`
}

type snapshotWindow struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type panicTickMsg struct{}

type saveStateMsg struct{}

type model struct {
	state readerState

	filePath string
	lines    []string
	lineByte []int64

	readerVP viewport.Model
	panicVP  viewport.Model
	progress progress.Model

	width        int
	height       int
	headerHeight int
	footerHeight int

	panicLogs   []string
	panicScenes [][]string
	panicScene  int
	rand        *rand.Rand

	panicKey string
	err      error

	chapterLines []int

	styles styles
}

var (
	chapterCNPattern = regexp.MustCompile(`^第\s*[0-9零一二三四五六七八九十百千两〇]+\s*[章回节卷]\s*.*`)
	chapterENPattern = regexp.MustCompile(`(?i)^chapter\s+[0-9ivxlcdm]+.*`)
)

type styles struct {
	appBorder lipgloss.Style
	header    lipgloss.Style
	footer    lipgloss.Style
	body      lipgloss.Style
	muted     lipgloss.Style
	warn      lipgloss.Style
	errorLine lipgloss.Style
}

func newStyles() styles {
	accent := lipgloss.Color("#6272A4")
	muted := lipgloss.Color("#7080B5")
	warn := lipgloss.Color("#E6DB74")
	border := lipgloss.Color("#3B4261")

	return styles{
		appBorder: lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(border),
		header:    lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		footer:    lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
		body:      lipgloss.NewStyle().Foreground(accent).Padding(0, 1),
		muted:     lipgloss.NewStyle().Foreground(muted),
		warn:      lipgloss.NewStyle().Foreground(warn),
		errorLine: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")),
	}
}

func initialModel(filePath string, lines []string, lineByte []int64, startLine int, err error) model {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	readerVP := viewport.New(0, 0)
	panicVP := viewport.New(0, 0)
	pg := progress.New(progress.WithSolidFill("#8BE9FD"), progress.WithWidth(28), progress.WithoutPercentage())

	st := newStyles()
	pg.Full = '='
	pg.Empty = '-'
	pg.EmptyColor = "#3B4261"

	m := model{
		state:        stateReading,
		filePath:     filePath,
		lines:        lines,
		lineByte:     lineByte,
		readerVP:     readerVP,
		panicVP:      panicVP,
		progress:     pg,
		headerHeight: 1,
		footerHeight: 2,
		panicScenes:  buildPanicScenes(),
		panicScene:   0,
		rand:         r,
		panicKey:     "esc",
		err:          err,
		chapterLines: buildChapterLineIndex(lines),
		styles:       st,
	}

	if len(lines) > 0 {
		m.readerVP.SetContent(disguiseLines(lines))
		m.setReaderTopLine(clamp(startLine, 0, len(lines)-1))
	}

	m.panicLogs = []string{
		"[11:42:10] INFO  workspace resolver initialized",
		"[11:42:10] INFO  restoring dependency cache",
	}
	m.panicVP.SetContent(strings.Join(m.panicLogs, "\n"))
	m.panicVP.GotoBottom()

	return m
}

func buildPanicScenes() [][]string {
	return [][]string{
		{
			"npm WARN deprecated inflight@1.0.6: this module is not supported",
			"npm info run esbuild@0.23.1 postinstall node install.js",
			"added 128 packages, audited 129 packages in 1.8s",
			"found 0 vulnerabilities",
		},
		{
			"=== RUN   TestIndexBuilder",
			"--- PASS: TestIndexBuilder (0.01s)",
			"=== RUN   TestParser_Fallback",
			"--- PASS: TestParser_Fallback (0.00s)",
			"ok      reader/internal/core   0.256s",
		},
		{
			"#13 [builder 4/8] RUN go mod download",
			"#13 DONE 1.7s",
			"#16 [builder 7/8] RUN CGO_ENABLED=0 go build -o app",
			"#16 DONE 3.2s",
			"#18 exporting to image",
		},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(saveStateCmd(m), tea.EnterAltScreen)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Batch(saveStateCmd(m), tea.Quit)
		}
		if msg.String() == "esc" || msg.String() == "q" {
			if m.state == stateReading {
				m.state = stateBossPanic
				return m, tea.Batch(nextPanicTickCmd(m.rand), saveStateCmd(m))
			}
			m.state = stateReading
			return m, saveStateCmd(m)
		}

		if m.state == stateBossPanic {
			switch msg.String() {
			case "r":
				m.panicScene = (m.panicScene + 1) % len(m.panicScenes)
			case "c":
				m.panicLogs = m.panicLogs[:0]
				m.panicVP.SetContent("")
			}
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			m.readerVP.LineDown(1)
			return m, saveStateCmd(m)
		case "k", "up":
			m.readerVP.LineUp(1)
			return m, saveStateCmd(m)
		case "n", "]":
			if m.jumpChapter(1) {
				return m, saveStateCmd(m)
			}
			return m, nil
		case "p", "[":
			if m.jumpChapter(-1) {
				return m, saveStateCmd(m)
			}
			return m, nil
		case " ":
			m.readerVP.ViewDown()
			return m, saveStateCmd(m)
		case "b":
			m.readerVP.ViewUp()
			return m, saveStateCmd(m)
		case "g":
			m.readerVP.GotoTop()
			return m, saveStateCmd(m)
		case "G":
			m.readerVP.GotoBottom()
			return m, saveStateCmd(m)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutViewport()
		return m, saveStateCmd(m)

	case panicTickMsg:
		if m.state != stateBossPanic {
			return m, nil
		}
		m.appendPanicLogLine()
		return m, nextPanicTickCmd(m.rand)

	case saveStateMsg:
		return m, nil
	}

	var cmd tea.Cmd
	if m.state == stateReading {
		m.readerVP, cmd = m.readerVP.Update(msg)
	} else {
		m.panicVP, cmd = m.panicVP.Update(msg)
	}
	return m, cmd
}

func (m *model) layoutViewport() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	bodyHeight := m.height - m.headerHeight - m.footerHeight - 2
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	bodyWidth := m.width - 4
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	m.readerVP.Width = bodyWidth
	m.readerVP.Height = bodyHeight
	m.panicVP.Width = bodyWidth
	m.panicVP.Height = bodyHeight
}

func (m *model) appendPanicLogLine() {
	scene := m.panicScenes[m.panicScene]
	if len(scene) == 0 {
		return
	}
	base := scene[m.rand.Intn(len(scene))]
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), base)
	m.panicLogs = append(m.panicLogs, line)
	if len(m.panicLogs) > maxPanicLogLines {
		m.panicLogs = m.panicLogs[len(m.panicLogs)-maxPanicLogLines:]
	}
	m.panicVP.SetContent(strings.Join(m.panicLogs, "\n"))
	m.panicVP.GotoBottom()
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "initializing terminal layout..."
	}
	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()

	ui := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return m.styles.appBorder.Width(m.width - 2).Render(ui)
}

func (m model) renderHeader() string {
	mode := "profile: reader-index / task: dependency scan"
	if m.state == stateBossPanic {
		mode = "profile: ci-pipeline / task: live verification"
	}
	if m.err != nil {
		mode += " | warning: fallback mode"
	}
	return m.styles.header.Width(max(1, m.width-4)).Render(mode)
}

func (m model) renderBody() string {
	if m.err != nil && len(m.lines) == 0 {
		return m.styles.errorLine.Width(max(1, m.width-4)).Render("error: " + m.err.Error())
	}
	if m.state == stateBossPanic {
		return m.styles.body.Width(max(1, m.width-4)).Render(m.panicVP.View())
	}
	return m.styles.body.Width(max(1, m.width-4)).Render(m.readerVP.View())
}

func (m model) renderFooter() string {
	percent := m.readingPercent()
	progressLabel := "Compiling assets"
	if m.state == stateBossPanic {
		progressLabel = "Running verification pipeline"
		percent = clampFloat(float64(len(m.panicLogs)%100)/100.0, 0, 1)
	}
	bar := m.progress.ViewAs(percent)
	status := fmt.Sprintf("%s %3.0f%%", progressLabel, percent*100)

	left := m.styles.footer.Render(status + " " + bar)
	right := m.styles.muted.Render("keys: j/k n/p [/] space/b esc q")
	line := lipgloss.JoinHorizontal(lipgloss.Left, left, "  ", right)

	meta := fmt.Sprintf(
		"offset=%d line=%d top=%d file=%s chapter=%s topText=%s",
		m.currentByteOffset(),
		m.currentLine(),
		m.readerVP.YOffset,
		filepath.Base(m.filePath),
		m.currentChapterLabel(),
		m.currentTopLinePreview(36),
	)
	metaLine := m.styles.muted.Render(meta)

	return lipgloss.JoinVertical(lipgloss.Left, line, metaLine)
}

func (m model) currentTopLinePreview(maxLen int) string {
	if len(m.lines) == 0 || maxLen <= 0 {
		return "N/A"
	}

	line := clamp(m.readerVP.YOffset, 0, len(m.lines)-1)
	text := strings.TrimSpace(m.lines[line])
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)
	if text == "" {
		return "N/A"
	}

	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func (m model) currentChapterLabel() string {
	if len(m.chapterLines) == 0 || len(m.lines) == 0 {
		return "N/A"
	}

	anchor := clamp(m.currentLine(), 0, len(m.lines)-1)
	idx := -1
	for i, line := range m.chapterLines {
		if line <= anchor {
			idx = i
			continue
		}
		break
	}

	if idx < 0 || idx >= len(m.chapterLines) {
		return "N/A"
	}

	heading := strings.TrimSpace(m.lines[m.chapterLines[idx]])
	heading = strings.TrimPrefix(heading, "//")
	heading = strings.TrimSpace(heading)
	if heading == "" {
		return "N/A"
	}

	return heading
}

func (m *model) jumpChapter(direction int) bool {
	if len(m.chapterLines) == 0 {
		return false
	}

	anchor := clamp(m.readerVP.YOffset+chapterAnchorBias, 0, len(m.lines)-1)
	currentChapterIdx := -1
	for i, line := range m.chapterLines {
		if line <= anchor {
			currentChapterIdx = i
			continue
		}
		break
	}

	if currentChapterIdx == -1 {
		// 如果在第一章之前，向下按去第一章，向上按不动
		if direction > 0 {
			m.setReaderTopLine(m.chapterLines[0])
			return true
		}
		return false
	}

	var targetIdx int
	if direction > 0 {
		// 下一章
		targetIdx = currentChapterIdx + 1
	} else {
		// 上一章逻辑优化：
		// 如果当前行已经非常接近（或就在）当前章的开头，则跳到上一章
		// 否则，先回到当前章的开头
		if anchor <= m.chapterLines[currentChapterIdx]+chapterAlignScan {
			targetIdx = currentChapterIdx - 1
		} else {
			targetIdx = currentChapterIdx
		}
	}

	// 边界检查
	if targetIdx < 0 || targetIdx >= len(m.chapterLines) {
		return false
	}
	targetLine := m.chapterLines[targetIdx]
	m.setReaderTopLine(targetLine)
	return true
}

func (m *model) findNearestChapterLineAround(center, radius int) (int, bool) {
	if len(m.lines) == 0 {
		return 0, false
	}
	center = clamp(center, 0, len(m.lines)-1)

	if isChapterHeadingLine(m.lines[center]) {
		return center, true
	}

	for d := 1; d <= radius; d++ {
		up := center - d
		if up >= 0 && isChapterHeadingLine(m.lines[up]) {
			return up, true
		}
		down := center + d
		if down < len(m.lines) && isChapterHeadingLine(m.lines[down]) {
			return down, true
		}
	}

	return 0, false
}

func (m *model) setReaderTopLine(line int) {
	if len(m.lines) == 0 {
		m.readerVP.YOffset = 0
		return
	}
	line = clamp(line, 0, len(m.lines)-1)
	// 直接设置 YOffset 性能最好且最精准
	m.readerVP.YOffset = line
}

func buildChapterLineIndex(lines []string) []int {
	idx := make([]int, 0, 128)
	for i, line := range lines {
		if isChapterHeadingLine(line) {
			idx = append(idx, i)
		}
	}
	return idx
}

func isChapterHeadingLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	// 去掉可能存在的代码伪装前缀，防止切章失效
	trimmed = strings.TrimPrefix(trimmed, "//")
	trimmed = strings.TrimSpace(trimmed)

	if trimmed == "" {
		return false
	}
	return chapterCNPattern.MatchString(trimmed) || chapterENPattern.MatchString(trimmed)
}

func (m model) readingPercent() float64 {
	total := len(m.lines)
	if total <= 1 {
		return 0
	}
	return clampFloat(float64(m.currentLine())/float64(total-1), 0, 1)
}

func (m model) currentLine() int {
	if m.state == stateBossPanic {
		return m.readerVP.YOffset
	}
	return m.readerVP.YOffset
}

func (m model) currentByteOffset() int64 {
	line := m.currentLine()
	if line < 0 || line >= len(m.lineByte) {
		return 0
	}
	return m.lineByte[line]
}

func nextPanicTickCmd(r *rand.Rand) tea.Cmd {
	d := time.Duration(80+r.Intn(180)) * time.Millisecond
	return tea.Tick(d, func(time.Time) tea.Msg {
		return panicTickMsg{}
	})
}

func saveStateCmd(m model) tea.Cmd {
	return func() tea.Msg {
		_ = writeSnapshot(m)
		return saveStateMsg{}
	}
}

func writeSnapshot(m model) error {
	if m.filePath == "" {
		return nil
	}
	snap := snapshotFile{
		SchemaVersion: snapshotVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		LastFile:      m.filePath,
		Profiles:      map[string]snapshotProfile{},
	}

	if existing, err := readSnapshotAuto(); err == nil {
		snap = existing
		if snap.Profiles == nil {
			snap.Profiles = map[string]snapshotProfile{}
		}
	}

	snap.SchemaVersion = snapshotVersion
	snap.LastFile = m.filePath
	snap.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	snap.Profiles[m.filePath] = snapshotProfile{
		ByteOffset: m.currentByteOffset(),
		LineOffset: m.currentLine(),
		TotalLines: len(m.lines),
		Window: snapshotWindow{
			Width:  m.width,
			Height: m.height,
		},
		Theme:    "dracula",
		PanicKey: m.panicKey,
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return writeSnapshotBytes(data)
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func snapshotPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", "dev-env-status.json"), nil
}

func projectSnapshotPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, ".novel-reader-state.json"), nil
}

func writeSnapshotBytes(data []byte) error {
	primaryPath, primaryErr := snapshotPath()
	if primaryErr == nil {
		if mkErr := os.MkdirAll(filepath.Dir(primaryPath), 0o700); mkErr == nil {
			if writeErr := atomicWrite(primaryPath, data); writeErr == nil {
				return nil
			}
		}
	}

	fallbackPath, fallbackErr := projectSnapshotPath()
	if fallbackErr != nil {
		if primaryErr != nil {
			return primaryErr
		}
		return fallbackErr
	}
	return atomicWrite(fallbackPath, data)
}

func readSnapshotAuto() (snapshotFile, error) {
	if p, err := snapshotPath(); err == nil {
		if snap, readErr := readSnapshot(p); readErr == nil {
			return snap, nil
		}
	}
	if p, err := projectSnapshotPath(); err == nil {
		if snap, readErr := readSnapshot(p); readErr == nil {
			return snap, nil
		}
	}
	return snapshotFile{}, fmt.Errorf("snapshot not found")
}

func readSnapshot(path string) (snapshotFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshotFile{}, err
	}
	var snap snapshotFile
	if err := json.Unmarshal(data, &snap); err != nil {
		return snapshotFile{}, err
	}
	if snap.Profiles == nil {
		snap.Profiles = map[string]snapshotProfile{}
	}
	return snap, nil
}

func saveLastLibraryScan(root string, items []novelItem) error {
	cachedItems := make([]snapshotLibraryItem, 0, len(items))
	for _, it := range items {
		cachedItems = append(cachedItems, snapshotLibraryItem{
			ID:     it.ID,
			Title:  it.Title,
			Path:   it.Path,
			Source: it.Source,
		})
	}

	cache := libraryScanCache{
		Root:      root,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     cachedItems,
	}

	// Prefer snapshot file, fallback to project cache file when ~/.config is not writable.
	if err := saveLibraryScanIntoSnapshot(cache); err == nil {
		return nil
	}
	return saveLibraryScanToProjectFile(cache)
}

func saveLibraryScanIntoSnapshot(cache libraryScanCache) error {
	snap := snapshotFile{
		SchemaVersion: snapshotVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Profiles:      map[string]snapshotProfile{},
	}
	if existing, readErr := readSnapshotAuto(); readErr == nil {
		snap = existing
		if snap.Profiles == nil {
			snap.Profiles = map[string]snapshotProfile{}
		}
	}

	snap.SchemaVersion = snapshotVersion
	snap.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	snap.LastLibrary = &cache

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return writeSnapshotBytes(data)
}

func loadLastLibraryScan() (*libraryScanCache, error) {
	if cache, err := loadLastLibraryScanFromSnapshot(); err == nil {
		return cache, nil
	}
	return loadLastLibraryScanFromProjectFile()
}

func loadLastLibraryScanFromSnapshot() (*libraryScanCache, error) {
	snap, err := readSnapshotAuto()
	if err != nil {
		return nil, err
	}
	if snap.LastLibrary == nil || len(snap.LastLibrary.Items) == 0 {
		return nil, fmt.Errorf("empty library scan cache")
	}
	return snap.LastLibrary, nil
}

func projectLibraryScanCachePath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, ".novel-reader-scan.json"), nil
}

func saveLibraryScanToProjectFile(cache libraryScanCache) error {
	p, err := projectLibraryScanCachePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(p, data)
}

func loadLastLibraryScanFromProjectFile() (*libraryScanCache, error) {
	p, err := projectLibraryScanCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var cache libraryScanCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if len(cache.Items) == 0 {
		return nil, fmt.Errorf("empty project library scan cache")
	}
	return &cache, nil
}

func loadLastFilePath() (string, error) {
	snap, err := readSnapshotAuto()
	if err != nil {
		return "", err
	}
	if snap.LastFile == "" {
		return "", fmt.Errorf("empty last file in snapshot")
	}
	return snap.LastFile, nil
}

func loadStartPosition(filePath string) int {
	snap, err := readSnapshotAuto()
	if err != nil {
		return 0
	}
	p, ok := snap.Profiles[filePath]
	if !ok {
		return 0
	}
	return p.LineOffset
}

func loadLines(path string) ([]string, []int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 4*1024*1024)

	var lines []string
	var bytes []int64
	var offset int64
	for s.Scan() {
		line := s.Text()
		lines = append(lines, line)
		bytes = append(bytes, offset)
		offset += int64(len(line)) + 1
	}
	if err := s.Err(); err != nil {
		return nil, nil, err
	}
	if len(lines) == 0 {
		return []string{"(empty file)"}, []int64{0}, nil
	}
	return lines, bytes, nil
}

func disguiseLines(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, "// "+l)
	}
	return strings.Join(out, "\n")
}

func runReader(filePath string) error {
	lines, lineByte, err := loadLines(filePath)
	if err != nil {
		return err
	}
	startLine := loadStartPosition(filePath)
	m := initialModel(filePath, lines, lineByte, startLine, nil)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, runErr := p.Run()
	return runErr
}

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
