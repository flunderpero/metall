package base

import (
	"fmt"
	"strings"
)

type Debug interface {
	Print(level int, msg string, args ...any) Debug
	// Indent returns a function that will decrease the indentation level.
	// That function is idempotent.
	Indent() func()
	SetLevels(levels ...int)
}

type StdoutDebug struct {
	prefix string
	indent int
	levels map[int]bool
}

func NewStdoutDebug(prefix string) *StdoutDebug {
	if len(prefix) > 0 {
		prefix += ": "
	}
	return &StdoutDebug{prefix: prefix, indent: 0, levels: map[int]bool{}}
}

func (d *StdoutDebug) Print(level int, msg string, args ...any) Debug {
	if len(d.levels) > 0 && !d.levels[level] {
		return d
	}
	color := debugColor(level)
	indent := strings.Repeat("  ", d.indent)
	padding := strings.Repeat(" ", len(d.prefix))
	formatted := fmt.Sprintf(msg, args...)
	lines := strings.Split(formatted, "\n")
	for i, line := range lines {
		lineIndent := indent
		prefix := padding
		if i == 0 {
			prefix = d.prefix
		}
		fmt.Printf("%s%s%s%s%s\n", prefix, lineIndent, color, line, debugColorReset)
	}
	return d
}

func (d *StdoutDebug) Indent() func() {
	indent := d.indent
	d.indent++
	return func() {
		d.indent = indent
	}
}

func (d *StdoutDebug) SetLevels(levels ...int) {
	if len(levels) == 0 {
		d.levels = nil
		return
	}
	d.levels = map[int]bool{}
	for _, level := range levels {
		d.levels[level] = true
	}
}

type NilDebug struct{}

func (d NilDebug) Print(_ int, _ string, _ ...any) Debug {
	return d
}

func (d NilDebug) Indent() func() {
	return func() {}
}

func (d NilDebug) SetLevels(_ ...int) {}

const (
	debugColorYellow = "\033[33m"
	debugColorWhite  = "\033[37m"
	debugColorGray   = "\033[90m"
	debugColorReset  = "\033[0m"
)

func debugColor(level int) string {
	switch level {
	case 0:
		return debugColorYellow
	case 1:
		return debugColorWhite
	default:
		return debugColorGray
	}
}
