package internal

import (
	"fmt"
	"strings"
)

type Diagnostics []Diagnostic

func (d Diagnostics) String() string {
	var sb strings.Builder
	for i, d := range d {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(d.Display())
	}
	return sb.String()
}

func (d Diagnostics) Error() string {
	return d.String()
}

type Diagnostic struct {
	Message  string
	Span     Span
	err      error
	location string
}

func NewDiagnostic(span Span, msg string, msgArgs ...any) *Diagnostic {
	return NewDiagnosticErr(span, nil, msg, msgArgs...)
}

func NewDiagnosticErr(span Span, err error, msg string, msgArgs ...any) *Diagnostic {
	message := fmt.Sprintf(msg, msgArgs...)
	location := Location(4)
	return &Diagnostic{message, span, err, location}
}

func (d Diagnostic) Error() string {
	return d.String()
}

func (d Diagnostic) String() string {
	var sb strings.Builder
	sb.WriteString(d.Message)
	sb.WriteString(" at\n")
	sb.WriteString(d.Span.StringWithSource(1))
	sb.WriteString("\n(")
	sb.WriteString(d.location)
	sb.WriteString(")")
	if d.err != nil {
		sb.WriteString(": ")
		sb.WriteString(d.err.Error())
	}
	return sb.String()
}

func (d Diagnostic) Display() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s\n", d.Span.String(), d.Message)
	sb.WriteString("    " + strings.ReplaceAll(d.Span.StringWithSource(1), "\n", "\n    "))
	return sb.String()
}
