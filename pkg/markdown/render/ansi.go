package render

import (
	"io"
)

type AnsiWriter struct {
	w io.Writer
}

func NewAnsiWriter(w io.Writer) *AnsiWriter {
	return &AnsiWriter{w: w}
}

func (a *AnsiWriter) Write(p []byte) (int, error) {
	return a.w.Write(p)
}

func (a *AnsiWriter) WriteString(s string) (int, error) {
	return io.WriteString(a.w, s)
}

func (a *AnsiWriter) WriteStyled(text string, style Style) (int, error) {
	n := 0
	if style.Prefix != "" {
		m, err := a.WriteString(style.Prefix)
		n += m
		if err != nil {
			return n, err
		}
	}
	m, err := a.WriteString(text)
	n += m
	if err != nil {
		return n, err
	}
	if style.Suffix != "" {
		m, err = a.WriteString(style.Suffix)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
