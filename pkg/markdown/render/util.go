package render

import (
	"bytes"

	"github.com/cjccjj/mdflow/pkg/markdown/parser"
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func VisibleLen(s string) int {
	n := 0
	i := 0
	runes := []rune(s)
	for i < len(runes) {
		if runes[i] == '\033' {
			for i < len(runes) && runes[i] != 'm' {
				i++
			}
			if i < len(runes) {
				i++
			}
			continue
		}
		n++
		i++
	}
	return n
}

func RenderInline(text string, theme Theme) string {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, theme)
	p := parser.New()

	tokens := tokenizer.Tokenize([]byte(text))
	events := p.Parse(tokens)
	events = append(events, p.CloseStates()...)
	for _, e := range events {
		if err := w.Handle(e); err != nil {
			return buf.String()
		}
	}
	return buf.String()
}

func padRight(s string, width int) string {
	visible := VisibleLen(s)
	if visible >= width {
		return s
	}
	return s + spaces(width-visible)
}

func padLeft(s string, width int) string {
	visible := VisibleLen(s)
	if visible >= width {
		return s
	}
	return spaces(width-visible) + s
}

func spaces(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = ' '
	}
	return string(buf)
}
