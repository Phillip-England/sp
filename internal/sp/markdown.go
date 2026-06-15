package sp

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

func ideaMarkdown(title, body string, createdAt time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", title))
	b.WriteString(fmt.Sprintf("created_at: %q\n", createdAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("slug: %q\n", slugify(title)))
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n")
	return b.String()
}

func parseIdeaMarkdown(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", fmt.Errorf("clipboard is empty")
	}

	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("clipboard markdown must be a # title followed by idea text")
	}

	titleLine := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(titleLine, "# ") || strings.HasPrefix(titleLine, "## ") {
		return "", "", fmt.Errorf("clipboard markdown must start with a single # heading")
	}

	title := strings.TrimSpace(strings.TrimPrefix(titleLine, "# "))
	if title == "" {
		return "", "", fmt.Errorf("clipboard markdown title cannot be empty")
	}

	bodyLines := lines[1:]
	for len(bodyLines) > 0 && strings.TrimSpace(bodyLines[0]) == "" {
		bodyLines = bodyLines[1:]
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if body == "" {
		return "", "", fmt.Errorf("clipboard markdown body cannot be empty")
	}

	if err := validatePlainTextBody(bodyLines); err != nil {
		return "", "", err
	}

	return title, body, nil
}

func validatePlainTextBody(lines []string) error {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			return fmt.Errorf("clipboard markdown body must be plain text paragraphs")
		case strings.HasPrefix(trimmed, "#"):
			return fmt.Errorf("clipboard markdown body cannot contain additional headings")
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ "):
			return fmt.Errorf("clipboard markdown body cannot contain lists")
		case startsOrderedList(trimmed):
			return fmt.Errorf("clipboard markdown body cannot contain lists")
		case strings.HasPrefix(trimmed, ">"):
			return fmt.Errorf("clipboard markdown body cannot contain blockquotes")
		case strings.HasPrefix(trimmed, "|"):
			return fmt.Errorf("clipboard markdown body cannot contain tables")
		case trimmed == "---" || trimmed == "***" || trimmed == "___":
			return fmt.Errorf("clipboard markdown body cannot contain markdown rules")
		}
	}

	return nil
}

func startsOrderedList(value string) bool {
	dot := strings.IndexByte(value, '.')
	if dot < 1 || dot == len(value)-1 || value[dot+1] != ' ' {
		return false
	}

	for _, r := range value[:dot] {
		if !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}
