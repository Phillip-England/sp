package sp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func ideaFilename(title string, createdAt time.Time) string {
	slug := slugify(title)
	if slug == "" {
		slug = "idea"
	}

	return fmt.Sprintf("%s-%s.md", createdAt.UTC().Format("20060102-150405"), slug)
}

type specFile struct {
	Name    string
	Path    string
	Dir     string
	Title   string
	Created time.Time
	Copied  bool
}

func specMarkdownFiles(recentOnly bool) ([]specFile, error) {
	if err := migrateLegacySpecDir("."); err != nil {
		return nil, err
	}
	return specMarkdownFilesInDir(specDir, "", recentOnly)
}

func specMarkdownFilesInDir(dir, source string, recentOnly bool) ([]specFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []specFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		title := markdownTitle(string(data), entry.Name())
		created := markdownCreatedAt(string(data), entry.Name(), info.ModTime())
		copied := hasCopiedAt(string(data))
		if recentOnly && copied {
			continue
		}

		files = append(files, specFile{
			Name:    entry.Name(),
			Path:    path,
			Dir:     source,
			Title:   title,
			Created: created,
			Copied:  copied,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	if len(files) == 0 {
		if recentOnly {
			return nil, fmt.Errorf("no recent markdown files found in %s", dir)
		}
		return nil, fmt.Errorf("no markdown files found in %s", dir)
	}

	return files, nil
}

func globalSpecMarkdownFiles() ([]specFile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return globalSpecMarkdownFilesUnder(home)
}

func globalSpecMarkdownFilesUnder(root string) ([]specFile, error) {
	return globalSpecMarkdownFilesUnderWithProgress(root, nil)
}

func globalSpecMarkdownFilesUnderWithProgress(root string, progress func(string)) ([]specFile, error) {
	var files []specFile
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if progress != nil {
			progress(path)
		}

		name := entry.Name()
		switch name {
		case ".git", "node_modules", "Library", ".cache":
			return filepath.SkipDir
		case specDir:
			source := filepath.Dir(path)
			dirFiles, err := specMarkdownFilesInDir(path, source, false)
			if err == nil {
				files = append(files, dirFiles...)
			}
			return filepath.SkipDir
		case legacySpecDir:
			source := filepath.Dir(path)
			dirFiles, err := specMarkdownFilesInDir(path, source, false)
			if err == nil {
				files = append(files, dirFiles...)
			}
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .sp markdown files found under %s", root)
	}

	return sortSpecFilesForList(files), nil
}

func collectSpecMarkdown(files []specFile) (string, error) {
	var parts []string
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return "", err
		}
		parts = append(parts, strings.TrimSpace(string(data)))
	}

	return strings.Join(parts, "\n\n"), nil
}

func listIdeas() error {
	files, err := specMarkdownFiles(false)
	if err != nil {
		return err
	}

	fmt.Print(formatIdeaListWithStyle(files, stdoutStyle()))
	return nil
}

func formatIdeaList(files []specFile) string {
	return formatIdeaListWithStyle(files, terminalStyle{})
}

func formatIdeaListWithStyle(files []specFile, style terminalStyle) string {
	files = sortSpecFilesForList(files)

	var b strings.Builder
	count := strconv.Itoa(len(files))
	fmt.Fprintf(
		&b,
		"%s %s %s\n\n",
		style.boldCyan(count+" ideas"),
		style.dim("in ./"+specDir),
		style.dim("(newest first)"),
	)
	for i, file := range files {
		number := style.yellow(strconv.Itoa(len(files)-i) + ".")
		name := style.green(ideaListName(file.Name))
		fmt.Fprintf(&b, "%s %s\n", number, name)
	}

	return b.String()
}

func formatReadIdea(content string, style terminalStyle) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if !style.enabled {
		return content
	}

	lines := strings.Split(content, "\n")
	inFrontMatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case i == 0 && trimmed == "---":
			inFrontMatter = true
			lines[i] = style.dim(line)
		case inFrontMatter && trimmed == "---":
			inFrontMatter = false
			lines[i] = style.dim(line)
		case inFrontMatter:
			lines[i] = style.dim(line)
		case strings.HasPrefix(trimmed, "# "):
			lines[i] = style.boldCyan(line)
		}
	}

	return strings.Join(lines, "\n")
}

func sortSpecFilesForList(files []specFile) []specFile {
	files = append([]specFile(nil), files...)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Created.Equal(files[j].Created) {
			return files[i].Name > files[j].Name
		}
		return files[i].Created.After(files[j].Created)
	})
	return files
}

func ideaListName(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if len(name) > len("20060102-150405-") {
		if _, err := time.Parse("20060102-150405", name[:len("20060102-150405")]); err == nil && name[len("20060102-150405")] == '-' {
			return name[len("20060102-150405-"):]
		}
	}

	return name
}

func selectedSpecMarkdownFiles(args []string) ([]specFile, error) {
	files, err := specMarkdownFiles(false)
	if err != nil {
		return nil, err
	}

	files = sortSpecFilesForList(files)
	indexes, err := parseSelection(args, len(files))
	if err != nil {
		return nil, err
	}

	selected := make([]specFile, 0, len(indexes))
	for _, index := range indexes {
		selected = append(selected, files[len(files)-index])
	}

	return selected, nil
}

func specMarkdownFilesForSelection(args []string) ([]specFile, error) {
	if len(args) == 0 {
		files, err := specMarkdownFiles(false)
		if err != nil {
			return nil, err
		}
		return sortSpecFilesForList(files), nil
	}

	return selectedSpecMarkdownFiles(args)
}

func parseSelection(args []string, max int) ([]int, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("selection cannot be empty")
	}

	var indexes []int
	seen := make(map[int]bool)
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, fmt.Errorf("selection contains an empty item")
			}

			start, end, err := parseSelectionPart(part)
			if err != nil {
				return nil, err
			}
			if start < 1 || end > max {
				return nil, fmt.Errorf("selection %s is out of range; choose 1-%d", part, max)
			}

			step := 1
			if start > end {
				step = -1
			}
			for index := start; ; index += step {
				if seen[index] {
					if index == end {
						break
					}
					continue
				}
				indexes = append(indexes, index)
				seen[index] = true
				if index == end {
					break
				}
			}
		}
	}

	return indexes, nil
}

func parseSelectionPart(part string) (int, int, error) {
	if strings.Contains(part, "-") {
		pieces := strings.Split(part, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("invalid selection %q", part)
		}

		start, err := strconv.Atoi(strings.TrimSpace(pieces[0]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid selection %q", part)
		}
		end, err := strconv.Atoi(strings.TrimSpace(pieces[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid selection %q", part)
		}
		return start, end, nil
	}

	index, err := strconv.Atoi(part)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid selection %q", part)
	}
	return index, index, nil
}

func markSpecFilesCopied(files []specFile, copiedAt time.Time) error {
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return err
		}

		content := setCopiedAt(string(data), copiedAt)
		if err := os.WriteFile(file.Path, []byte(content), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func markdownTitle(content, fallback string) string {
	if title, ok := frontMatterValue(content, "title"); ok && title != "" {
		return title
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	return strings.TrimSuffix(fallback, filepath.Ext(fallback))
}

func markdownCreatedAt(content, name string, fallback time.Time) time.Time {
	if value, ok := frontMatterValue(content, "created_at"); ok {
		createdAt, err := time.Parse(time.RFC3339, value)
		if err == nil {
			return createdAt
		}
	}

	prefixLength := len("20060102-150405")
	if len(name) >= prefixLength {
		createdAt, err := time.Parse("20060102-150405", name[:prefixLength])
		if err == nil {
			return createdAt
		}
	}

	return fallback
}

func hasCopiedAt(content string) bool {
	value, ok := frontMatterValue(content, "copied_at")
	return ok && value != ""
}

func frontMatterValue(content, key string) (string, bool) {
	body, ok := frontMatter(content)
	if !ok {
		return "", false
	}

	prefix := key + ":"
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		return strings.Trim(value, `"`), true
	}

	return "", false
}

func frontMatter(content string) (string, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}

	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}

	return rest[:end], true
}

func setCopiedAt(content string, copiedAt time.Time) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	copiedLine := fmt.Sprintf("copied_at: %q", copiedAt.Format(time.RFC3339))

	if !strings.HasPrefix(content, "---\n") {
		return fmt.Sprintf("---\n%s\n---\n\n%s", copiedLine, strings.TrimSpace(content)) + "\n"
	}

	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fmt.Sprintf("---\n%s\n---\n\n%s", copiedLine, strings.TrimSpace(content)) + "\n"
	}

	header := rest[:end]
	after := rest[end:]
	lines := strings.Split(header, "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "copied_at:") {
			lines[i] = copiedLine
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, copiedLine)
	}

	return "---\n" + strings.Join(lines, "\n") + after
}
