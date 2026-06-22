package render

import (
	"bytes"

	"github.com/mattn/go-runewidth"

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
		n += runewidth.RuneWidth(runes[i])
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

func WrapContent(rendered string, displayWidth int) []string {
	if displayWidth <= 0 || VisibleLen(rendered) <= displayWidth {
		return []string{rendered}
	}

	runes := []rune(rendered)
	var lines []string
	var active []string
	var curLine []rune
	visiblePos := 0

	i := 0
	for i < len(runes) {
		r := runes[i]

		if r == '\033' && i+1 < len(runes) && runes[i+1] == '[' {
			j := i + 2
			for j < len(runes) && runes[j] != 'm' {
				j++
			}
			if j < len(runes) {
				j++
			}
			curLine = append(curLine, runes[i:j]...)
			codeStr := string(runes[i+2 : j-1])
			if codeStr == "0" {
				if len(active) > 0 {
					active = active[:len(active)-1]
				}
			} else {
				active = append(active, "\033["+codeStr+"m")
			}
			i = j
			continue
		}

		rw := runewidth.RuneWidth(r)
		if rw == 0 {
			curLine = append(curLine, r)
			i++
			continue
		}

		if visiblePos > 0 && visiblePos+rw > displayWidth {
			curLine = append(curLine, []rune("\033[0m")...)
			lines = append(lines, string(curLine))
			curLine = nil
			visiblePos = 0
			for _, c := range active {
				curLine = append(curLine, []rune(c)...)
			}
		}

		curLine = append(curLine, r)
		visiblePos += rw
		i++
	}

	if len(curLine) > 0 {
		lines = append(lines, string(curLine))
	}

	if len(lines) == 0 {
		return []string{rendered}
	}
	return lines
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
