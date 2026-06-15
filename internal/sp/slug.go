package sp

import (
	"strings"
	"unicode"
)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))

	var b strings.Builder
	previousDash := false

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			previousDash = false
		case !previousDash:
			b.WriteByte('-')
			previousDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}
