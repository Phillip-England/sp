package sp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	got := slugify(" Some Title: with IDEA! ")
	want := "some-title-with-idea"
	if got != want {
		t.Fatalf("slugify() = %q, want %q", got, want)
	}
}

func TestIdeaFilenameStartsWithTimestamp(t *testing.T) {
	createdAt := time.Date(2026, 6, 15, 12, 34, 56, 0, time.UTC)
	got := ideaFilename("Some Title", createdAt)
	want := "20260615-123456-some-title.md"
	if got != want {
		t.Fatalf("ideaFilename() = %q, want %q", got, want)
	}
}

func TestAddIdeaWritesMarkdownToSpecDir(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	now := func() time.Time {
		return time.Date(2026, 6, 15, 12, 34, 56, 0, time.UTC)
	}

	if err := addIdea([]string{"Some Title", "Some text for title and idea"}, now); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmp, ".sp", "20260615-123456-some-title.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, expected := range []string{
		`title: "Some Title"`,
		`created_at: "2026-06-15T12:34:56Z"`,
		`slug: "some-title"`,
		"# Some Title",
		"Some text for title and idea",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("markdown missing %q:\n%s", expected, content)
		}
	}
}

func TestParseIdeaMarkdownFromClipboard(t *testing.T) {
	for _, input := range []string{
		`# Clipboard Title

First plain paragraph.

Second plain paragraph.`,
		`# Clipboard Title
First plain paragraph.

Second plain paragraph.`,
	} {
		title, body, err := parseIdeaMarkdown(input)
		if err != nil {
			t.Fatal(err)
		}

		if title != "Clipboard Title" {
			t.Fatalf("title = %q, want %q", title, "Clipboard Title")
		}

		wantBody := "First plain paragraph.\n\nSecond plain paragraph."
		if body != wantBody {
			t.Fatalf("body = %q, want %q", body, wantBody)
		}
	}
}

func TestParseIdeaMarkdownRejectsAdditionalMarkdownElements(t *testing.T) {
	_, _, err := parseIdeaMarkdown(`# Clipboard Title

First plain paragraph.

## Nested heading`)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectSpecMarkdownSortsMarkdownFilesByName(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"20260615-123457-second.md": "# Second\n\nSecond body.\n",
		"notes.txt":                 "ignored",
		"20260615-123456-first.md":  "# First\n\nFirst body.\n",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(".sp", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	filesToCopy, err := specMarkdownFiles(false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := collectSpecMarkdown(filesToCopy)
	if err != nil {
		t.Fatal(err)
	}

	want := "# First\n\nFirst body.\n\n# Second\n\nSecond body."
	if got != want {
		t.Fatalf("collectSpecMarkdown() = %q, want %q", got, want)
	}
}

func TestSpecMarkdownFilesRecentOnlySkipsCopiedFiles(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"20260615-123457-copied.md": `---
title: "Copied"
created_at: "2026-06-15T12:34:57Z"
slug: "copied"
copied_at: "2026-06-15T12:35:00Z"
---

# Copied

Copied body.
`,
		"20260615-123456-new.md": `---
title: "New"
created_at: "2026-06-15T12:34:56Z"
slug: "new"
---

# New

New body.
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(".sp", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := specMarkdownFiles(true)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Name != "20260615-123456-new.md" {
		t.Fatalf("recent files = %#v, want only new file", got)
	}
}

func TestFormatIdeaListIsClearAndNewestFirst(t *testing.T) {
	files := []specFile{
		{
			Name:    "20260615-123456-old.md",
			Title:   "Old idea",
			Created: time.Date(2026, 6, 15, 12, 34, 56, 0, time.UTC),
			Copied:  true,
		},
		{
			Name:    "20260615-123500-new.md",
			Title:   "New idea",
			Created: time.Date(2026, 6, 15, 12, 35, 0, 0, time.UTC),
			Copied:  false,
		},
	}

	got := formatIdeaList(files)
	want := `2 ideas in ./.sp (newest first)

2. new
1. old
`

	if got != want {
		t.Fatalf("formatIdeaList() = %q, want %q", got, want)
	}
}

func TestFormatIdeaListWithStyleAddsTerminalColor(t *testing.T) {
	files := []specFile{
		{
			Name:    "20260615-123500-new.md",
			Title:   "New idea",
			Created: time.Date(2026, 6, 15, 12, 35, 0, 0, time.UTC),
		},
	}

	got := formatIdeaListWithStyle(files, terminalStyle{enabled: true})
	for _, expected := range []string{
		"\x1b[1m\x1b[36m1 ideas\x1b[0m",
		"\x1b[2min ./.sp\x1b[0m",
		"\x1b[33m1.\x1b[0m",
		"\x1b[32mnew\x1b[0m",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("styled list missing %q:\n%s", expected, got)
		}
	}
}

func TestFormatHelpWithStyleColorsSectionsAndCommands(t *testing.T) {
	got := formatHelp(terminalStyle{enabled: true})
	for _, expected := range []string{
		"\x1b[1m\x1b[36msp\x1b[0m collects",
		"\x1b[1mUsage:\x1b[0m",
		"\x1b[32msp\x1b[0m",
		"\x1b[32msp list\x1b[0m",
		"\x1b[1mNotes:\x1b[0m",
		"\x1b[33m-\x1b[0m Use sp read 1",
		"\x1b[1m\x1b[36m# Some title\x1b[0m",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("styled help missing %q:\n%s", expected, got)
		}
	}
}

func TestFormatReadIdeaWithStyleColorsMarkdownForTerminal(t *testing.T) {
	input := `---
title: "Some Title"
---

# Some Title

Some body.`

	got := formatReadIdea(input, terminalStyle{enabled: true})
	for _, expected := range []string{
		"\x1b[2m---\x1b[0m",
		"\x1b[2mtitle: \"Some Title\"\x1b[0m",
		"\x1b[1m\x1b[36m# Some Title\x1b[0m",
		"Some body.",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("styled read output missing %q:\n%s", expected, got)
		}
	}
}

func TestFormatReadIdeaWithoutStylePreservesMarkdown(t *testing.T) {
	input := "  # Some Title\n\nSome body.\n"
	got := formatReadIdea(input, terminalStyle{})
	want := "# Some Title\n\nSome body."
	if got != want {
		t.Fatalf("formatReadIdea() = %q, want %q", got, want)
	}
}

func TestIdeaListNameRemovesTimestampAndExtension(t *testing.T) {
	got := ideaListName("20260615-123456-some-idea.md")
	want := "some-idea"
	if got != want {
		t.Fatalf("ideaListName() = %q, want %q", got, want)
	}
}

func TestParseSelectionSupportsNumbersListsAndRanges(t *testing.T) {
	got, err := parseSelection([]string{"1,3-5", "2"}, 6)
	if err != nil {
		t.Fatal(err)
	}

	want := []int{1, 3, 4, 5, 2}
	if len(got) != len(want) {
		t.Fatalf("parseSelection() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseSelection() = %#v, want %#v", got, want)
		}
	}
}

func TestParseSelectionSupportsReverseRanges(t *testing.T) {
	got, err := parseSelection([]string{"5-3"}, 6)
	if err != nil {
		t.Fatal(err)
	}

	want := []int{5, 4, 3}
	if len(got) != len(want) {
		t.Fatalf("parseSelection() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseSelection() = %#v, want %#v", got, want)
		}
	}
}

func TestParseSelectionRejectsOutOfRange(t *testing.T) {
	_, err := parseSelection([]string{"3"}, 2)
	if err == nil {
		t.Fatal("expected out of range error")
	}
}

func TestSelectedSpecMarkdownFilesUsesListOrder(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"20260615-123456-old.md": `---
title: "Old"
created_at: "2026-06-15T12:34:56Z"
---

# Old

Old body.
`,
		"20260615-123500-new.md": `---
title: "New"
created_at: "2026-06-15T12:35:00Z"
---

# New

New body.
`,
		"20260615-123458-middle.md": `---
title: "Middle"
created_at: "2026-06-15T12:34:58Z"
---

# Middle

Middle body.
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(".sp", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := selectedSpecMarkdownFiles([]string{"1,3"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"20260615-123456-old.md", "20260615-123500-new.md"}
	if len(got) != len(want) {
		t.Fatalf("selected files = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("selected files = %#v, want %#v", got, want)
		}
	}
}

func TestSpecMarkdownFilesForSelectionDefaultsToAllInListOrder(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"20260615-123456-old.md": `---
title: "Old"
created_at: "2026-06-15T12:34:56Z"
---

# Old
`,
		"20260615-123500-new.md": `---
title: "New"
created_at: "2026-06-15T12:35:00Z"
---

# New
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(".sp", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := specMarkdownFilesForSelection(nil)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"20260615-123500-new.md", "20260615-123456-old.md"}
	if len(got) != len(want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("files = %#v, want %#v", got, want)
		}
	}
}

func TestTUIRenderShowsNumberedIdeasAndHelp(t *testing.T) {
	model := tuiModel{
		files: []specFile{
			{Name: "20260615-123500-new.md", Title: "New"},
			{Name: "20260615-123456-old.md", Title: "Old"},
		},
		help: true,
	}

	got := model.render(100, 12, terminalStyle{})
	for _, expected := range []string{
		"sp tui 2 ideas mode: READ",
		"n new  r read  c copy  d delete  f find",
		">  2. new",
		"   1. old",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tui render missing %q:\n%s", expected, got)
		}
	}
}

func TestTUICreateIdeaWritesMarkdownAndRefreshesList(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	now := func() time.Time {
		return time.Date(2026, 6, 15, 12, 34, 56, 0, time.UTC)
	}

	model := tuiModel{}
	model = createTUIIdea(model, "Some Title", "Some body.", now)
	if model.message != "added some-title" {
		t.Fatalf("message = %q", model.message)
	}
	if len(model.files) != 1 || model.files[0].Name != "20260615-123456-some-title.md" {
		t.Fatalf("files = %#v", model.files)
	}

	data, err := os.ReadFile(filepath.Join(".sp", "20260615-123456-some-title.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, expected := range []string{
		`title: "Some Title"`,
		`created_at: "2026-06-15T12:34:56Z"`,
		"# Some Title",
		"Some body.",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("markdown missing %q:\n%s", expected, content)
		}
	}
}

func TestEnsureSpecDirCreatesSpecDirectory(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := ensureSpecDir(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(".sp")
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal(".sp is not a directory")
	}
}

func TestEnsureSpecDirMigratesLegacySpecDirectory(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".spec", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".spec", "idea.md"), []byte("# Idea\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureSpecDir(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(".sp", "idea.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(".spec"); !os.IsNotExist(err) {
		t.Fatalf("legacy .spec still exists or unexpected stat error: %v", err)
	}
}

func TestGlobalSpecMarkdownFilesUnderFindsMultipleSPDirs(t *testing.T) {
	root := t.TempDir()
	projectA := filepath.Join(root, "a", ".sp")
	projectB := filepath.Join(root, "b", ".sp")
	if err := os.MkdirAll(projectA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectB, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		filepath.Join(projectA, "20260615-123456-a.md"): `---
title: "A"
created_at: "2026-06-15T12:34:56Z"
---

# A
`,
		filepath.Join(projectB, "20260615-123500-b.md"): `---
title: "B"
created_at: "2026-06-15T12:35:00Z"
---

# B
`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := globalSpecMarkdownFilesUnder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("global files = %#v, want 2", got)
	}
	if got[0].Name != "20260615-123500-b.md" || got[0].Dir != filepath.Join(root, "b") {
		t.Fatalf("first global file = %#v", got[0])
	}
	if got[1].Name != "20260615-123456-a.md" || got[1].Dir != filepath.Join(root, "a") {
		t.Fatalf("second global file = %#v", got[1])
	}
}

func TestGlobalSpecMarkdownFilesUnderReportsProgress(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project", ".sp")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "20260615-123456-a.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var seen []string
	_, err := globalSpecMarkdownFilesUnderWithProgress(root, func(path string) {
		seen = append(seen, path)
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range seen {
		if path == filepath.Join(root, "project") {
			return
		}
	}
	t.Fatalf("progress paths = %#v, want project dir", seen)
}

func TestGlobalScanStatusMessages(t *testing.T) {
	start := formatGlobalScanStart("/tmp/home")
	if start != "\r\033[2Ksp: scanning /tmp/home for .sp directories..." {
		t.Fatalf("start message = %q", start)
	}

	progress := formatGlobalScanProgress("/tmp/home/project")
	if progress != "\r\033[2Ksp: scanning /tmp/home/project" {
		t.Fatalf("progress message = %q", progress)
	}

	done := formatGlobalScanComplete(12)
	if done != "\r\033[2Ksp: found 12 markdown ideas; opening TUI\n" {
		t.Fatalf("complete message = %q", done)
	}
}

func TestGlobalScanStatusReusesOneLineForProgress(t *testing.T) {
	var out bytes.Buffer
	status := newGlobalScanStatus(&out)

	status.start("/tmp/home")
	status.progress("/tmp/home/project-a")
	status.progress("/tmp/home/project-b")
	status.complete(2)

	got := out.String()
	want := "\r\033[2Ksp: scanning /tmp/home for .sp directories..." +
		"\r\033[2Ksp: scanning /tmp/home/project-a" +
		"\r\033[2Ksp: scanning /tmp/home/project-b" +
		"\r\033[2Ksp: found 2 markdown ideas; opening TUI\n"
	if got != want {
		t.Fatalf("scan status output = %q, want %q", got, want)
	}

	if progress := strings.TrimSuffix(got, "\n"); strings.Contains(progress, "\n") {
		t.Fatalf("scan progress should not write new lines: %q", got)
	}
}

func TestTUINewIdeaModePromptsTitleThenBody(t *testing.T) {
	model := tuiModel{}
	model, _ = updateTUI(model, "n")
	if model.mode != tuiModeNewTitle {
		t.Fatalf("mode = %q, want new title", model.mode)
	}

	for _, key := range []string{"T", "i", "t", "l", "e"} {
		model, _ = updateTUI(model, key)
	}
	model, _ = updateTUI(model, "\r")
	if model.mode != tuiModeNewBody || model.newTitle != "Title" || model.input != "" {
		t.Fatalf("model after title = %#v", model)
	}
}

func TestTUINewIdeaModeRendersDedicatedScreen(t *testing.T) {
	model := tuiModel{
		files: []specFile{
			{Name: "20260615-123500-existing.md", Title: "Existing"},
		},
		mode:     tuiModeNewTitle,
		input:    "Fresh Idea",
		selected: 0,
	}

	got := model.render(100, 12, terminalStyle{})
	for _, expected := range []string{
		"New idea",
		"Title",
		"Fresh Idea",
		"Text",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("new idea screen missing %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "1. existing") {
		t.Fatalf("new idea screen should not render main list:\n%s", got)
	}
}

func TestTUINewIdeaModeWrapsLongInputAndKeepsTailVisible(t *testing.T) {
	model := tuiModel{
		mode:  tuiModeNewBody,
		input: "this is a very long body that should keep the end visible",
	}

	got := model.render(24, 12, terminalStyle{})
	for _, expected := range []string{
		"this is a very long body",
		" that should keep the en",
		"d visible",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("new idea body missing wrapped segment %q:\n%s", expected, got)
		}
	}
}

func TestTUIDeleteModeDeletesSelectedMarkdownAndRefreshesList(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(".sp", "20260615-123500-first.md")
	second := filepath.Join(".sp", "20260615-123459-second.md")
	if err := os.WriteFile(first, []byte("# First\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("# Second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := specMarkdownFilesForSelection(nil)
	if err != nil {
		t.Fatal(err)
	}
	model := tuiModel{files: files, mode: tuiModeDelete, input: "2"}
	model = applyTUIModeInput(model)

	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("first file still exists or unexpected stat error: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second file should remain: %v", err)
	}
	if len(model.files) != 1 || model.files[0].Name != "20260615-123459-second.md" {
		t.Fatalf("files after delete = %#v", model.files)
	}
	if model.message != "deleted 1 markdown files" {
		t.Fatalf("message = %q", model.message)
	}
}

func TestTUICommandFiltersAndViewsByNumber(t *testing.T) {
	model := tuiModel{
		files: []specFile{
			{Name: "20260615-123500-new.md", Title: "New"},
			{Name: "20260615-123456-old.md", Title: "Old"},
		},
	}

	var done bool
	model, done = updateTUI(model, "f")
	if done {
		t.Fatal("unexpected done")
	}
	for _, key := range []string{"o", "l", "d"} {
		model, done = updateTUI(model, key)
		if done {
			t.Fatal("unexpected done")
		}
	}
	if model.filter != "old" || model.currentFileIndex() != 1 {
		t.Fatalf("filtered model = %#v, current = %d", model, model.currentFileIndex())
	}

	model, _ = updateTUI(model, "\r")
	model, _ = updateTUI(model, "r")
	model, _ = updateTUI(model, "2")
	model, _ = updateTUI(model, "\r")
	if !model.view || len(model.viewIndexes) != 1 || model.viewIndexes[0] != 0 {
		t.Fatalf("view model = %#v", model)
	}
}

func TestTUIRenderUsesPageOffsetAndKeepsOriginalNumbers(t *testing.T) {
	model := tuiModel{
		files: []specFile{
			{Name: "20260615-123500-one.md", Title: "One"},
			{Name: "20260615-123459-two.md", Title: "Two"},
			{Name: "20260615-123458-three.md", Title: "Three"},
			{Name: "20260615-123457-four.md", Title: "Four"},
		},
		selected: 2,
		offset:   2,
	}

	got := model.render(100, 6, terminalStyle{})
	if strings.Contains(got, "1. one") || strings.Contains(got, "2. two") {
		t.Fatalf("paged render included previous page:\n%s", got)
	}
	for _, expected := range []string{
		">  2. three",
		"   1. four",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("paged render missing %q:\n%s", expected, got)
		}
	}
}

func TestTUIViewScrollsMarkdownContent(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(".sp", "20260615-123500-long.md")
	content := "# Long\n\nline one\n\nline two\n\nline three\n\nline four\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := specMarkdownFilesForSelection(nil)
	if err != nil {
		t.Fatal(err)
	}
	model := tuiModel{
		files:       files,
		view:        true,
		viewIndexes: []int{0},
	}

	top := model.render(100, 9, terminalStyle{})
	if !strings.Contains(top, "# Long") || !strings.Contains(top, "line one") {
		t.Fatalf("top render missing initial content:\n%s", top)
	}
	if strings.Contains(top, "line four") {
		t.Fatalf("top render should not include later content:\n%s", top)
	}

	model, _ = updateTUI(model, "}")
	scrolled := model.render(100, 9, terminalStyle{})
	if strings.Contains(scrolled, "# Long") {
		t.Fatalf("scrolled render still shows top heading:\n%s", scrolled)
	}
	if !strings.Contains(scrolled, "line four") {
		t.Fatalf("scrolled render missing later content:\n%s", scrolled)
	}
}

func TestTUIViewScrollResetsWhenOpeningSelection(t *testing.T) {
	model := tuiModel{
		files: []specFile{
			{Name: "20260615-123500-new.md", Title: "New"},
		},
		viewOffset: 12,
	}

	model = viewTUIIndexes(model, []int{0})
	if model.viewOffset != 0 {
		t.Fatalf("viewOffset = %d, want reset to 0", model.viewOffset)
	}
}

func TestTUIViewLeftRightNavigateSiblingIdeas(t *testing.T) {
	model := tuiModel{
		files: []specFile{
			{Name: "20260615-123500-new.md", Title: "New"},
			{Name: "20260615-123459-middle.md", Title: "Middle"},
			{Name: "20260615-123458-old.md", Title: "Old"},
		},
		view:        true,
		viewIndexes: []int{1},
		viewOffset:  4,
	}

	model, _ = updateTUI(model, "\x1b[C")
	if len(model.viewIndexes) != 1 || model.viewIndexes[0] != 2 || model.viewOffset != 0 {
		t.Fatalf("right sibling model = %#v", model)
	}

	model, _ = updateTUI(model, "\x1b[D")
	if len(model.viewIndexes) != 1 || model.viewIndexes[0] != 1 || model.viewOffset != 0 {
		t.Fatalf("left sibling model = %#v", model)
	}
}

func TestMarkSpecFilesCopiedAddsCopiedAt(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	})

	if err := os.MkdirAll(".sp", 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(".sp", "20260615-123456-new.md")
	if err := os.WriteFile(path, []byte("# New\n\nNew body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := specMarkdownFiles(false)
	if err != nil {
		t.Fatal(err)
	}

	copiedAt := time.Date(2026, 6, 15, 12, 35, 0, 0, time.UTC)
	if err := markSpecFilesCopied(files, copiedAt); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	for _, expected := range []string{
		"---\n",
		`copied_at: "2026-06-15T12:35:00Z"`,
		"# New",
		"New body.",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("marked markdown missing %q:\n%s", expected, content)
		}
	}
}
