package sp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

type tuiModel struct {
	files       []specFile
	filter      string
	selected    int
	offset      int
	view        bool
	viewOffset  int
	input       string
	newTitle    string
	message     string
	help        bool
	mode        tuiMode
	viewIndexes []int
	scanPaths   []string
}

type tuiMode string

const (
	tuiModeRead     tuiMode = "read"
	tuiModeCopy     tuiMode = "copy"
	tuiModeDelete   tuiMode = "delete"
	tuiModeFind     tuiMode = "find"
	tuiModeNewTitle tuiMode = "new title"
	tuiModeNewBody  tuiMode = "new body"
	tuiModeScanPath tuiMode = "scan path"
)

func runTUI(args []string) error {
	if len(args) == 0 {
		if err := ensureSpecDir(); err != nil {
			return err
		}
	}

	files, err := specMarkdownFilesForSelection(args)
	if err != nil {
		if len(args) > 0 || !isNoMarkdownFilesError(err) {
			return err
		}
		files = nil
	}

	return runTUIWithFiles(files)
}

func runGlobalTUI() error {
	settings, err := loadSystemSettings()
	if err != nil {
		return err
	}
	if len(settings.ScanPaths) == 0 {
		return runTUIWithModel(tuiModel{
			mode:      tuiModeRead,
			scanPaths: settings.ScanPaths,
			message:   "no scan paths configured; press s to add one",
		})
	}

	status := newGlobalScanStatus(os.Stderr)
	status.start(strings.Join(settings.ScanPaths, ", "))
	files, err := globalSpecMarkdownFilesUnderPaths(settings.ScanPaths, status.progress)
	if err != nil {
		return err
	}
	status.complete(len(files))
	return runTUIWithModel(tuiModel{
		files:     files,
		mode:      tuiModeRead,
		scanPaths: settings.ScanPaths,
	})
}

type globalScanStatus struct {
	writer io.Writer
}

func newGlobalScanStatus(writer io.Writer) globalScanStatus {
	return globalScanStatus{writer: writer}
}

func (status globalScanStatus) start(root string) {
	fmt.Fprint(status.writer, formatGlobalScanStart(root))
}

func (status globalScanStatus) progress(path string) {
	fmt.Fprint(status.writer, formatGlobalScanProgress(path))
}

func (status globalScanStatus) complete(count int) {
	fmt.Fprint(status.writer, formatGlobalScanComplete(count))
}

func formatGlobalScanStart(root string) string {
	return fmt.Sprintf("\r\033[2Ksp: scanning %s for .sp directories...", root)
}

func formatGlobalScanProgress(path string) string {
	return fmt.Sprintf("\r\033[2Ksp: scanning %s", path)
}

func formatGlobalScanComplete(count int) string {
	return fmt.Sprintf("\r\033[2Ksp: found %d markdown ideas; opening TUI\n", count)
}

func runTUIWithFiles(files []specFile) error {
	settings, err := loadSystemSettings()
	if err != nil {
		return err
	}
	return runTUIWithModel(tuiModel{files: files, mode: tuiModeRead, scanPaths: settings.ScanPaths})
}

func runTUIWithModel(model tuiModel) error {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("tui requires an interactive terminal: %w", err)
	}
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print("\x1b[?1049l\x1b[?25h")
	}()

	fmt.Print("\x1b[?1049h\x1b[?25l")
	if err := renderTUI(model); err != nil {
		return err
	}

	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return err
		}

		next, done := updateTUI(model, string(buf[:n]))
		model = next
		if done {
			return nil
		}
		if err := renderTUI(model); err != nil {
			return err
		}
	}
}

func updateTUI(model tuiModel, key string) (tuiModel, bool) {
	model.mode = model.effectiveMode()
	if model.isTextEntryMode() && key != "\r" && key != "\n" && key != "\x1b" && key != "\x7f" && key != "\b" {
		if key >= " " {
			model = appendTUIInput(model, key)
		}
		return model, false
	}

	switch key {
	case "q", "\x03":
		return model, true
	case "\x1b":
		model.mode = tuiModeRead
		model.input = ""
		model.newTitle = ""
		model.view = false
		model.message = "read mode"
	case "j", "\x1b[B":
		if model.view {
			model.viewOffset++
		} else {
			model.selected = clampTUISelection(model, model.selected+1)
		}
	case "k", "\x1b[A":
		if model.view {
			model.viewOffset = max(0, model.viewOffset-1)
		} else {
			model.selected = clampTUISelection(model, model.selected-1)
		}
	case "\x1b[C":
		if model.view {
			model = moveTUIViewSibling(model, 1)
		}
	case "\x1b[D":
		if model.view {
			model = moveTUIViewSibling(model, -1)
		}
	case "}":
		if model.view {
			model.viewOffset += tuiPageSize(model)
		} else {
			model.offset = clampTUISelection(model, model.offset+tuiPageSize(model))
			model.selected = model.offset
		}
	case "{":
		if model.view {
			model.viewOffset = max(0, model.viewOffset-tuiPageSize(model))
		} else {
			model.offset = clampTUISelection(model, model.offset-tuiPageSize(model))
			model.selected = model.offset
		}
	case "g":
		if model.view {
			model.viewOffset = 0
		} else {
			model.selected = 0
			model.offset = 0
		}
	case "G":
		visible := model.visibleIndexes()
		if len(visible) > 0 {
			model.selected = len(visible) - 1
			model.offset = model.selected
		}
	case "\r", "\n":
		model = applyTUIModeInput(model)
	case "v":
		model = viewTUIIndexes(model, []int{model.currentFileIndex()})
	case "r":
		model.mode = tuiModeRead
		model.input = ""
		model.view = false
		model.message = "read mode"
	case "c":
		model.mode = tuiModeCopy
		model.input = ""
		model.view = false
		model.message = "copy mode"
	case "d":
		model.mode = tuiModeDelete
		model.input = ""
		model.view = false
		model.message = "delete mode"
	case "f":
		model.mode = tuiModeFind
		model.input = model.filter
		model.view = false
		model.message = "find mode"
	case "n":
		model.mode = tuiModeNewTitle
		model.input = ""
		model.newTitle = ""
		model.view = false
		model.message = "new idea title"
	case "s":
		model.mode = tuiModeScanPath
		model.input = ""
		model.view = false
		model.message = "add scan path"
	case "?":
		model.help = !model.help
	case "C":
		model = copyTUIIndexes(model, model.visibleIndexes())
	case "\x7f", "\b":
		model = backspaceTUIInput(model)
	default:
		if key >= " " && key != "\x7f" {
			model = appendTUIInput(model, key)
		}
	}

	return model, false
}

func applyTUIModeInput(model tuiModel) tuiModel {
	model.mode = model.effectiveMode()
	switch model.mode {
	case tuiModeFind:
		model.mode = tuiModeRead
		model.input = ""
		if model.filter == "" {
			model.message = "showing all ideas"
		} else {
			model.message = "filtered ideas"
		}
	case tuiModeCopy:
		indexes, err := parseTUISelectionInput(model)
		if err != nil {
			model.message = err.Error()
			return model
		}
		model = copyTUIIndexes(model, indexes)
		model.input = ""
	case tuiModeDelete:
		indexes, err := parseTUISelectionInput(model)
		if err != nil {
			model.message = err.Error()
			return model
		}
		model = deleteTUIIndexes(model, indexes)
		model.input = ""
	case tuiModeRead:
		indexes, err := parseTUISelectionInput(model)
		if err != nil {
			model.message = err.Error()
			return model
		}
		model = viewTUIIndexes(model, indexes)
		model.input = ""
	case tuiModeNewTitle:
		title := strings.TrimSpace(model.input)
		if title == "" {
			model.message = "title cannot be empty"
			return model
		}
		model.newTitle = title
		model.input = ""
		model.mode = tuiModeNewBody
		model.message = "new idea text"
	case tuiModeNewBody:
		body := strings.TrimSpace(model.input)
		if body == "" {
			model.message = "idea text cannot be empty"
			return model
		}
		model = createTUIIdea(model, model.newTitle, body, time.Now)
	case tuiModeScanPath:
		settings, normalized, err := addSystemScanPath(model.input)
		if err != nil {
			model.message = err.Error()
			return model
		}
		model.scanPaths = settings.ScanPaths
		model.mode = tuiModeRead
		model.input = ""
		model.message = fmt.Sprintf("added scan path %s", normalized)
	}
	return model
}

func ensureSpecDir() error {
	if err := migrateLegacySpecDir("."); err != nil {
		return err
	}
	return os.MkdirAll(specDir, 0o755)
}

func migrateLegacySpecDir(root string) error {
	current := filepath.Join(root, specDir)
	legacy := filepath.Join(root, legacySpecDir)

	if _, err := os.Stat(current); err == nil {
		return nil
	}
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return os.Rename(legacy, current)
}

func parseTUISelectionInput(model tuiModel) ([]int, error) {
	input := strings.TrimSpace(model.input)
	if input == "" {
		index := model.currentFileIndex()
		if index < 0 {
			return nil, fmt.Errorf("nothing selected")
		}
		return []int{index}, nil
	}

	indexes, err := parseSelection([]string{input}, len(model.files))
	if err != nil {
		return nil, err
	}
	for i := range indexes {
		indexes[i] = model.fileIndexForIdeaNumber(indexes[i])
	}
	return indexes, nil
}

func appendTUIInput(model tuiModel, value string) tuiModel {
	model.mode = model.effectiveMode()
	switch model.mode {
	case tuiModeFind:
		model.input += value
		model.filter = model.input
		model.selected = 0
		model.offset = 0
		model.view = false
	case tuiModeRead, tuiModeCopy, tuiModeDelete:
		if isTUISelectionText(value) {
			model.input += value
		}
	case tuiModeNewTitle, tuiModeNewBody, tuiModeScanPath:
		model.input += value
	}
	return model
}

func backspaceTUIInput(model tuiModel) tuiModel {
	if model.input == "" {
		return model
	}
	_, size := utf8.DecodeLastRuneInString(model.input)
	model.input = model.input[:len(model.input)-size]
	if model.mode == tuiModeFind {
		model.filter = model.input
		model.selected = 0
		model.offset = 0
		model.view = false
	}
	return model
}

func createTUIIdea(model tuiModel, title, body string, now func() time.Time) tuiModel {
	path, err := writeIdeaFile(title, body, now)
	if err != nil {
		model.message = err.Error()
		return model
	}

	files, err := specMarkdownFilesForSelection(nil)
	if err != nil {
		model.message = err.Error()
		return model
	}

	model.files = files
	model.mode = tuiModeRead
	model.input = ""
	model.newTitle = ""
	model.filter = ""
	model.view = false
	model.viewOffset = 0
	model.offset = 0
	model.selected = indexOfSpecPath(files, path)
	if model.selected < 0 {
		model.selected = 0
	}
	model.message = fmt.Sprintf("added %s", ideaListName(filepath.Base(path)))
	return model
}

func isTUISelectionText(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && r != ',' && r != '-' && r != ' ' {
			return false
		}
	}
	return true
}

func viewTUIIndexes(model tuiModel, indexes []int) tuiModel {
	if len(indexes) == 0 || indexes[0] < 0 {
		model.message = "nothing to read"
		return model
	}
	model.viewIndexes = indexes
	model.view = true
	model.viewOffset = 0
	model.selected = clampTUISelection(model, model.selected)
	model.message = fmt.Sprintf("reading %d markdown files", len(indexes))
	return model
}

func moveTUIViewSibling(model tuiModel, delta int) tuiModel {
	current := model.currentViewFileIndex()
	if current < 0 {
		return model
	}
	next := current + delta
	if next < 0 || next >= len(model.files) {
		model.message = "no sibling idea"
		return model
	}
	return viewTUIIndexes(model, []int{next})
}

func copyTUIIndexes(model tuiModel, indexes []int) tuiModel {
	if len(indexes) == 0 || indexes[0] < 0 {
		model.message = "nothing to copy"
		return model
	}

	files := make([]specFile, 0, len(indexes))
	for _, index := range uniqueTUIIndexes(indexes) {
		if index >= 0 && index < len(model.files) {
			files = append(files, model.files[index])
		}
	}
	content, err := collectSpecMarkdown(files)
	if err != nil {
		model.message = err.Error()
		return model
	}
	if err := writeClipboard(content); err != nil {
		model.message = err.Error()
		return model
	}
	model.message = fmt.Sprintf("copied %d markdown files", len(files))
	return model
}

func deleteTUIIndexes(model tuiModel, indexes []int) tuiModel {
	if len(indexes) == 0 || indexes[0] < 0 {
		model.message = "nothing to delete"
		return model
	}

	deleted := 0
	for _, index := range uniqueTUIIndexes(indexes) {
		if index < 0 || index >= len(model.files) {
			continue
		}
		if err := os.Remove(model.files[index].Path); err != nil {
			model.message = err.Error()
			return model
		}
		deleted++
	}

	files, err := specMarkdownFilesForSelection(nil)
	if err != nil {
		if !isNoMarkdownFilesError(err) {
			model.message = err.Error()
			return model
		}
		files = nil
	}

	model.files = files
	model.mode = tuiModeRead
	model.input = ""
	model.view = false
	model.viewIndexes = nil
	model.viewOffset = 0
	model.offset = 0
	model.selected = clampTUISelection(model, min(model.selected, len(files)-1))
	model.message = fmt.Sprintf("deleted %d markdown files", deleted)
	return model
}

func renderTUI(model tuiModel) error {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 40 {
		width = 100
	}
	if height < 12 {
		height = 24
	}

	fmt.Print("\x1b[H\x1b[2J")
	fmt.Print(model.render(width, height, terminalStyle{enabled: true}))
	return nil
}

func (m tuiModel) render(width, height int, style terminalStyle) string {
	var b strings.Builder
	m.mode = m.effectiveMode()
	visible := m.visibleIndexes()
	if m.selected >= len(visible) {
		m.selected = max(0, len(visible)-1)
	}

	header := style.boldCyan("sp tui") + " " + style.dim(fmt.Sprintf("%d ideas", len(m.files)))
	modeLabel := strings.ToUpper(string(m.mode))
	header += " " + style.yellow("mode:") + " " + modeLabel
	if m.filter != "" {
		header += " " + style.yellow("find:") + " " + m.filter
	}
	if m.input != "" && m.mode != tuiModeFind && !m.isNewIdeaMode() {
		header += " " + style.yellow("input:") + " " + m.input
	}
	b.WriteString(fitLine(header, width))
	b.WriteString("\r\n")

	if m.help {
		help := "q quit  n new  r read  c copy  C copy all  d delete  f find  s scan paths  enter run  j/k scroll  { } page"
		b.WriteString(fitLine(style.dim(help), width))
		b.WriteString("\r\n")
	} else if m.message != "" {
		b.WriteString(fitLine(style.green(m.message), width))
		b.WriteString("\r\n")
	} else {
		status := "read: type 1 or 1-3 enter  copy: c then 1-3 enter  C copy all  find: f then type  s scan paths"
		if m.view {
			status = "reading: j/k scroll line  { } page  g top  r list  c copy  d delete  q quit"
		} else if m.mode == tuiModeNewTitle {
			status = "new idea: type title then enter  esc cancel"
		} else if m.mode == tuiModeNewBody {
			status = "new idea: type text then enter to save  esc cancel"
		} else if m.mode == tuiModeScanPath {
			status = "scan paths: type directory then enter  esc cancel"
		} else if m.mode == tuiModeDelete {
			status = "delete: type 1 or 1-3 then enter  esc cancel"
		}
		b.WriteString(fitLine(style.dim(status), width))
		b.WriteString("\r\n")
	}

	bodyHeight := height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	if m.mode == tuiModeScanPath {
		b.WriteString(m.renderScanPaths(width, bodyHeight, style))
	} else if m.isNewIdeaMode() {
		b.WriteString(m.renderNewIdea(width, bodyHeight, style))
	} else if m.view {
		b.WriteString(m.renderView(width, bodyHeight, visible, style))
	} else {
		b.WriteString(m.renderList(width, bodyHeight, visible, style))
	}

	return b.String()
}

func (m tuiModel) renderScanPaths(width, height int, style terminalStyle) string {
	var b strings.Builder
	lines := []string{style.boldCyan("Scan paths"), ""}
	if len(m.scanPaths) == 0 {
		lines = append(lines, style.dim("No scan paths configured."))
	} else {
		for i, path := range m.scanPaths {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, path))
		}
	}
	lines = append(lines, "", style.yellow("Add"), m.input)
	lines = tailLines(lines, height)

	for i := 0; i < min(len(lines), height); i++ {
		if i > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString(fitLine(lines[i], width))
	}
	return b.String()
}

func (m tuiModel) renderNewIdea(width, height int, style terminalStyle) string {
	var b strings.Builder
	lines := []string{
		style.boldCyan("New idea"),
		"",
		style.yellow("Title"),
	}
	lines = append(lines, wrapInputLines(m.newIdeaTitleValue(), width)...)
	lines = append(lines, "", style.yellow("Text"))
	lines = append(lines, wrapInputLines(m.newIdeaBodyValue(), width)...)
	lines = tailLines(lines, height)

	for i := 0; i < min(len(lines), height); i++ {
		if i > 0 {
			b.WriteString("\r\n")
		}
		b.WriteString(fitLine(lines[i], width))
	}
	return b.String()
}

func (m tuiModel) newIdeaTitleValue() string {
	if m.mode == tuiModeNewTitle {
		return m.input
	}
	return m.newTitle
}

func (m tuiModel) newIdeaBodyValue() string {
	if m.mode == tuiModeNewBody {
		return m.input
	}
	return ""
}

func (m tuiModel) renderList(width, height int, visible []int, style terminalStyle) string {
	var b strings.Builder
	if len(visible) == 0 {
		b.WriteString(fitLine(style.dim("No matching ideas."), width))
		return b.String()
	}

	start := max(0, min(m.offset, len(visible)-1))
	if m.selected < start {
		start = m.selected
	}
	if m.selected >= start+height {
		start = m.selected - height + 1
	}
	end := min(len(visible), start+height)
	for row, listIndex := range visible[start:end] {
		file := m.files[listIndex]
		prefix := "  "
		name := ideaListName(file.Name)
		if file.Dir != "" {
			name += "  " + style.dim(shortSourceDir(file.Dir))
		}
		if start+row == m.selected {
			prefix = style.yellow("> ")
			if file.Dir == "" {
				name = style.green(name)
			} else {
				name = style.green(ideaListName(file.Name)) + "  " + style.dim(shortSourceDir(file.Dir))
			}
		}
		line := fmt.Sprintf("%s%2d. %s", prefix, m.ideaNumberForFileIndex(listIndex), name)
		b.WriteString(fitLine(line, width))
		if row < end-start-1 {
			b.WriteString("\r\n")
		}
	}
	return b.String()
}

func (m tuiModel) renderView(width, height int, visible []int, style terminalStyle) string {
	index := m.currentFileIndex()
	if index < 0 || index >= len(m.files) {
		return style.dim("No idea selected.")
	}

	indexes := m.viewIndexes
	if len(indexes) == 0 {
		indexes = []int{index}
	}

	files := make([]specFile, 0, len(indexes))
	for _, index := range indexes {
		if index >= 0 && index < len(m.files) {
			files = append(files, m.files[index])
		}
	}
	content, err := collectSpecMarkdown(files)
	if err != nil {
		return err.Error()
	}

	lines := strings.Split(formatReadIdea(content, style), "\n")
	var b strings.Builder
	title := fmt.Sprintf("reading %d file(s)", len(files))
	if len(files) == 1 {
		title = fmt.Sprintf("%d. %s", m.ideaNumberForFileIndex(indexes[0]), ideaListName(files[0].Name))
	}
	b.WriteString(fitLine(style.green(title), width))
	start := min(max(0, m.viewOffset), max(0, len(lines)-1))
	end := min(len(lines), start+height-1)
	if len(lines) == 0 {
		end = 0
	}
	for i := start; i < end; i++ {
		b.WriteString("\r\n")
		b.WriteString(fitLine(lines[i], width))
	}
	return b.String()
}

func (m tuiModel) currentFileIndex() int {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return -1
	}
	selected := m.selected
	if selected < 0 {
		selected = 0
	}
	if selected >= len(visible) {
		selected = len(visible) - 1
	}
	return visible[selected]
}

func (m tuiModel) currentViewFileIndex() int {
	if len(m.viewIndexes) > 0 {
		index := m.viewIndexes[0]
		if index >= 0 && index < len(m.files) {
			return index
		}
	}
	return m.currentFileIndex()
}

func (m tuiModel) ideaNumberForFileIndex(index int) int {
	return len(m.files) - index
}

func (m tuiModel) fileIndexForIdeaNumber(number int) int {
	return len(m.files) - number
}

func uniqueTUIIndexes(indexes []int) []int {
	seen := make(map[int]bool, len(indexes))
	unique := make([]int, 0, len(indexes))
	for _, index := range indexes {
		if seen[index] {
			continue
		}
		seen[index] = true
		unique = append(unique, index)
	}
	return unique
}

func (m tuiModel) visibleIndexes() []int {
	filter := strings.ToLower(strings.TrimSpace(m.filter))
	indexes := make([]int, 0, len(m.files))
	for i, file := range m.files {
		if filter == "" {
			indexes = append(indexes, i)
			continue
		}
		haystack := strings.ToLower(file.Title + " " + ideaListName(file.Name) + " " + file.Dir)
		if strings.Contains(haystack, filter) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func indexOfSpecPath(files []specFile, path string) int {
	for i, file := range files {
		if file.Path == path {
			return i
		}
	}
	return -1
}

func isNoMarkdownFilesError(err error) bool {
	return os.IsNotExist(err) || strings.Contains(err.Error(), "no markdown files found")
}

func shortSourceDir(dir string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(dir, home) {
		return "~" + strings.TrimPrefix(dir, home)
	}
	return dir
}

func (m tuiModel) effectiveMode() tuiMode {
	if m.mode == "" {
		return tuiModeRead
	}
	return m.mode
}

func (m tuiModel) isNewIdeaMode() bool {
	return m.mode == tuiModeNewTitle || m.mode == tuiModeNewBody
}

func (m tuiModel) isTextEntryMode() bool {
	return m.mode == tuiModeFind || m.isNewIdeaMode() || m.mode == tuiModeScanPath
}

func clampTUISelection(model tuiModel, selected int) int {
	visible := model.visibleIndexes()
	if len(visible) == 0 {
		return 0
	}
	return max(0, min(selected, len(visible)-1))
}

func tuiPageSize(model tuiModel) int {
	size := termHeightOrDefault() - 4
	if size < 1 {
		return 1
	}
	return size
}

func termHeightOrDefault() int {
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || height < 12 {
		return 24
	}
	return height
}

func fitLine(value string, width int) string {
	value = strings.ReplaceAll(value, "\t", "  ")
	if printableLen(value) <= width {
		return value
	}
	if width <= 1 {
		return ""
	}

	var b strings.Builder
	used := 0
	hasEscape := false
	for i := 0; i < len(value); {
		if value[i] == '\x1b' {
			hasEscape = true
			end := i + 1
			for end < len(value) && value[end] != 'm' {
				end++
			}
			if end < len(value) {
				end++
			}
			b.WriteString(value[i:end])
			i = end
			continue
		}
		if used >= width-1 {
			break
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		b.WriteRune(r)
		used++
		i += size
	}
	b.WriteString("...")
	if hasEscape {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

func visibleInputTail(value string, width int) string {
	if width <= 0 || printableLen(value) <= width {
		return value
	}
	if width <= 3 {
		runes := []rune(value)
		if width > len(runes) {
			width = len(runes)
		}
		return string(runes[len(runes)-width:])
	}

	runes := []rune(value)
	keep := width - 3
	if keep > len(runes) {
		keep = len(runes)
	}
	return "..." + string(runes[len(runes)-keep:])
}

func wrapInputLines(value string, width int) []string {
	if value == "" {
		return []string{""}
	}
	if width <= 0 {
		return []string{value}
	}

	var lines []string
	for _, paragraph := range strings.Split(value, "\n") {
		runes := []rune(paragraph)
		if len(runes) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(runes) > width {
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		lines = append(lines, string(runes))
	}
	return lines
}

func tailLines(lines []string, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	return lines[len(lines)-height:]
}

func printableLen(value string) int {
	length := 0
	inEscape := false
	for i := 0; i < len(value); i++ {
		switch {
		case value[i] == '\x1b':
			inEscape = true
		case inEscape && value[i] == 'm':
			inEscape = false
		case !inEscape:
			length++
		}
	}
	return length
}
