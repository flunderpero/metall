package base

import (
	"fmt"
	"strings"
)

type Source struct {
	Name    string
	Content []rune
}

func NewSource(name string, content []rune) *Source {
	return &Source{Name: name, Content: content}
}

type Span struct {
	Source *Source
	Start  int
	End    int
}

func NewSpan(source *Source, start, end int) Span {
	if source == nil {
		panic("source cannot be nil")
	}
	return Span{Source: source, Start: start, End: end}
}

func (s Span) Combine(other Span) Span {
	if s.Source == nil {
		panic(Errorf("cannot combine spans where s.Source is nil"))
	}
	if other.Source == nil {
		panic(Errorf("cannot combine spans where other.Source is nil"))
	}
	if s.Source != other.Source {
		panic(Errorf("cannot combine spans from different sources: %s vs %s", s.Source.Name, other.Source.Name))
	}
	start := min(s.Start, other.Start)
	end := max(s.End, other.End)
	return Span{Source: s.Source, Start: start, End: end}
}

func (s Span) String() string {
	if s.Source == nil {
		return "<unknown>"
	}
	row, col := s.StartPos()
	return fmt.Sprintf("%s:%d:%d", s.Source.Name, row, col)
}

// Print the surrounding lines of the span and underline the span.
func (s Span) StringWithSource(surroundingLines int) string {
	if s.Source == nil || s.Source.Content == nil {
		return s.String()
	}
	srow, scol := s.StartPos()
	erow, ecol := s.EndPos()
	lines := strings.Split(string(s.Source.Content), "\n")
	rowFrom := max(srow-1-surroundingLines, 0)
	rowTo := max(min(erow+surroundingLines, len(lines)), 0)
	var sb strings.Builder
	for i := rowFrom; i < rowTo; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
		if i == srow-1 {
			for range scol - 1 {
				sb.WriteString(" ")
			}
			if srow == erow {
				for j := scol - 1; j < ecol; j++ {
					sb.WriteString("^")
				}
			} else {
				sb.WriteString("^\n")
			}
		}
		if i == erow-1 && erow != srow {
			for range ecol - 1 {
				sb.WriteString(" ")
			}
			sb.WriteString("^\n")
		}
	}
	return sb.String()
}

func (s Span) StartPos() (row, col int) {
	row = 1
	col = 1
	src := s.Source.Content
	for i := range s.Start {
		if src[i] == '\n' {
			row++
			col = 1
		} else {
			col++
		}
	}
	return row, col
}

func (s Span) EndPos() (row, col int) {
	row = 1
	col = 1
	src := s.Source.Content
	for i := range s.End {
		if src[i] == '\n' {
			row++
			col = 1
		} else {
			col++
		}
	}
	return row, col
}
