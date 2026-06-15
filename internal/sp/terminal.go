package sp

import (
	"os"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

type terminalStyle struct {
	enabled bool
}

func stdoutStyle() terminalStyle {
	if os.Getenv("NO_COLOR") != "" {
		return terminalStyle{}
	}

	info, err := os.Stdout.Stat()
	if err != nil {
		return terminalStyle{}
	}

	return terminalStyle{enabled: info.Mode()&os.ModeCharDevice != 0}
}

func (s terminalStyle) wrap(code, value string) string {
	if !s.enabled {
		return value
	}
	return code + value + ansiReset
}

func (s terminalStyle) wrapAll(codes string, value string) string {
	if !s.enabled {
		return value
	}
	return codes + value + ansiReset
}

func (s terminalStyle) bold(value string) string {
	return s.wrap(ansiBold, value)
}

func (s terminalStyle) dim(value string) string {
	return s.wrap(ansiDim, value)
}

func (s terminalStyle) cyan(value string) string {
	return s.wrap(ansiCyan, value)
}

func (s terminalStyle) green(value string) string {
	return s.wrap(ansiGreen, value)
}

func (s terminalStyle) yellow(value string) string {
	return s.wrap(ansiYellow, value)
}

func (s terminalStyle) boldCyan(value string) string {
	return s.wrapAll(ansiBold+ansiCyan, value)
}
