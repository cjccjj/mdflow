package render

import (
	"bytes"
	"testing"
)

func TestAnsiWriteStyled(t *testing.T) {
	var buf bytes.Buffer
	w := NewAnsiWriter(&buf)

	style := Style{Prefix: "\033[1m", Suffix: "\033[0m"}
	n, err := w.WriteStyled("hello", style)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len("\033[1mhello\033[0m") {
		t.Errorf("wrote %d bytes, expected %d", n, len("\033[1mhello\033[0m"))
	}

	expected := "\033[1mhello\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestAnsiWriteStyledNoPrefix(t *testing.T) {
	var buf bytes.Buffer
	w := NewAnsiWriter(&buf)

	style := Style{}
	n, err := w.WriteStyled("hello", style)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len("hello") {
		t.Errorf("wrote %d bytes, expected %d", n, len("hello"))
	}
	if buf.String() != "hello" {
		t.Errorf("expected %q, got %q", "hello", buf.String())
	}
}
