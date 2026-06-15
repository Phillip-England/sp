package sp

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func readClipboard() (string, error) {
	var candidates [][]string

	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbpaste"}}
	case "windows":
		candidates = [][]string{{"powershell", "-NoProfile", "-Command", "Get-Clipboard"}}
	default:
		candidates = [][]string{{"wl-paste", "--no-newline"}, {"xclip", "-selection", "clipboard", "-out"}, {"xsel", "--clipboard", "--output"}}
	}

	var errs []string
	for _, candidate := range candidates {
		out, err := exec.Command(candidate[0], candidate[1:]...).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\r\n"), nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", candidate[0], err))
	}

	return "", fmt.Errorf("could not read clipboard (%s)", strings.Join(errs, "; "))
}

func writeClipboard(value string) error {
	var candidates [][]string

	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	case "windows":
		candidates = [][]string{{"powershell", "-NoProfile", "-Command", "Set-Clipboard"}}
	default:
		candidates = [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	}

	var errs []string
	for _, candidate := range candidates {
		cmd := exec.Command(candidate[0], candidate[1:]...)
		cmd.Stdin = strings.NewReader(value)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", candidate[0], err))
		}
	}

	return fmt.Errorf("could not write clipboard (%s)", strings.Join(errs, "; "))
}
