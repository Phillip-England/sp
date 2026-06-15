package sp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrUsage = errors.New("usage: sp init | sp list | sp read [selection] | sp tui [selection] | sp all | sp clipboard | sp copy [-r|--recent|selection] | sp \"title\" \"idea text\" | sp \"title\" --cb")

const (
	specDir       = ".sp"
	legacySpecDir = ".spec"
)

func Run(args []string) error {
	if len(args) == 0 {
		return runTUI(nil)
	}

	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printHelp()
		return nil
	}

	if args[0] == "init" {
		return initSpecDir()
	}

	if args[0] == "clipboard" {
		return addIdeaFromClipboard(time.Now)
	}

	if args[0] == "copy" {
		return copySpecMarkdownToClipboard(args[1:], time.Now)
	}

	if args[0] == "list" {
		return listIdeas()
	}

	if args[0] == "read" {
		return readIdea(args[1:])
	}

	if args[0] == "tui" || args[0] == "browse" {
		return runTUI(args[1:])
	}

	if args[0] == "all" || args[0] == "global" {
		return runGlobalTUI()
	}

	return addIdea(args, time.Now)
}

func printHelp() {
	fmt.Print(formatHelp(stdoutStyle()))
}

func formatHelp(style terminalStyle) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s collects project specification ideas as markdown files.\n\n", style.boldCyan("sp"))
	fmt.Fprintf(&b, "%s\n", style.bold("Usage:"))
	for _, command := range []string{
		"sp",
		"sp init",
		"sp list",
		"sp read [selection]",
		"sp tui [selection]",
		"sp all",
		"sp clipboard",
		"sp copy [-r|--recent|selection]",
		`sp "some title" "some text for title and idea"`,
		`sp "another title" --cb`,
	} {
		fmt.Fprintf(&b, "  %s\n", style.green(command))
	}

	fmt.Fprintf(&b, "\n%s\n", style.bold("Notes:"))
	for _, note := range []string{
		"Ideas are written to ./.sp as timestamped markdown files.",
		"Run sp with no arguments to open the terminal UI.",
		"Use sp all to search and copy ideas from .sp directories under your home directory.",
		"Use sp list to show newest ideas first.",
		"Use sp read 1 or sp read 1-3 to print ideas from sp list.",
		"Use sp tui to manage ideas with read, copy, and find modes.",
		"Use sp copy -r to copy only ideas not previously copied.",
		"Use sp copy 1,2,4-6 to copy selected ideas from sp list.",
		"Use sp clipboard with clipboard content shaped as:",
	} {
		fmt.Fprintf(&b, "  %s %s\n", style.yellow("-"), note)
	}

	fmt.Fprintf(&b, "\n%s\n\n%s\n", style.boldCyan("# Some title"), style.dim("Some plain text paragraph."))
	return b.String()
}

func initSpecDir() error {
	if err := ensureSpecDir(); err != nil {
		return err
	}

	entries, err := os.ReadDir(specDir)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		readme := "# Sp\n\nProject-specific ideas collected by `sp` live here as markdown files.\n"
		if err := os.WriteFile(filepath.Join(specDir, "README.md"), []byte(readme), 0o644); err != nil {
			return err
		}
	}

	style := stdoutStyle()
	fmt.Println(style.green("initialized") + " " + style.dim("./"+specDir))
	return nil
}

func addIdeaFromClipboard(now func() time.Time) error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	title, body, err := parseIdeaMarkdown(clipboard)
	if err != nil {
		return err
	}

	return writeIdea(title, body, now)
}

func addIdea(args []string, now func() time.Time) error {
	if len(args) != 2 {
		return ErrUsage
	}

	title := strings.TrimSpace(args[0])
	if title == "" {
		return fmt.Errorf("title cannot be empty")
	}

	body := args[1]
	if args[1] == "--cb" {
		clipboard, err := readClipboard()
		if err != nil {
			return err
		}
		body = clipboard
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("idea text cannot be empty")
	}

	return writeIdea(title, body, now)
}

func writeIdea(title, body string, now func() time.Time) error {
	path, err := writeIdeaFile(title, body, now)
	if err != nil {
		return err
	}

	style := stdoutStyle()
	fmt.Println(style.green("added") + " " + style.dim(path))
	return nil
}

func writeIdeaFile(title, body string, now func() time.Time) (string, error) {
	if err := ensureSpecDir(); err != nil {
		return "", err
	}

	createdAt := now().UTC()
	name := ideaFilename(title, createdAt)
	path := filepath.Join(specDir, name)
	content := ideaMarkdown(title, body, createdAt)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}

	return path, nil
}

func copySpecMarkdownToClipboard(args []string, now func() time.Time) error {
	recentOnly := false
	if len(args) == 1 {
		recentOnly = args[0] == "-r" || args[0] == "--recent"
	}
	if recentOnly && len(args) > 1 {
		return ErrUsage
	}

	var files []specFile
	var err error
	if recentOnly || len(args) == 0 {
		files, err = specMarkdownFiles(recentOnly)
	} else {
		files, err = selectedSpecMarkdownFiles(args)
	}
	if err != nil {
		return err
	}

	content, err := collectSpecMarkdown(files)
	if err != nil {
		return err
	}

	if err := writeClipboard(content); err != nil {
		return err
	}

	if err := markSpecFilesCopied(files, now().UTC()); err != nil {
		return err
	}

	style := stdoutStyle()
	switch {
	case recentOnly:
		fmt.Println(style.green("copied") + " recent " + style.dim("./"+specDir+" markdown") + " to clipboard")
	case len(args) > 0:
		fmt.Printf("%s %s selected %s to clipboard\n", style.green("copied"), style.yellow(strconv.Itoa(len(files))), style.dim("./"+specDir+" markdown files"))
	default:
		fmt.Println(style.green("copied") + " " + style.dim("./"+specDir+" markdown") + " to clipboard")
	}
	return nil
}

func readIdea(args []string) error {
	files, err := specMarkdownFilesForSelection(args)
	if err != nil {
		return err
	}

	content, err := collectSpecMarkdown(files)
	if err != nil {
		return err
	}

	fmt.Println(formatReadIdea(content, stdoutStyle()))
	return nil
}
