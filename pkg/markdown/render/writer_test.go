package render

import (
	"bytes"
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/parser"
)

func TestWriterBold(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.BoldStartEvent})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "hello"})
	_ = w.Handle(parser.Event{Type: parser.BoldEndEvent})

	expected := "\033[1mhello\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterHeader(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.HeaderStartEvent, Value: "h1"})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "Hello"})
	_ = w.Handle(parser.Event{Type: parser.HeaderEndEvent})

	expected := "\033[1;48;5;63;38;5;228mHello\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterHeaderH2(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.HeaderStartEvent, Value: "h2"})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "Hello"})
	_ = w.Handle(parser.Event{Type: parser.HeaderEndEvent})

	expected := "\033[1;34m## Hello\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterCodeBlock(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.CodeBlockStartEvent})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "code"})
	_ = w.Handle(parser.Event{Type: parser.CodeBlockEndEvent})

	expected := "\033[38;5;245mcode\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterInlineCode(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.InlineCodeStartEvent})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "code"})
	_ = w.Handle(parser.Event{Type: parser.InlineCodeEndEvent})

	expected := "\033[38;5;215;48;5;236mcode\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterBulletItem(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.BulletItemEvent})

	expected := "• "
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterNewline(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.NewlineEvent})

	if buf.String() != "\n" {
		t.Errorf("expected newline, got %q", buf.String())
	}
}

func TestWriterItalic(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.ItalicStartEvent})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "slanted"})
	_ = w.Handle(parser.Event{Type: parser.ItalicEndEvent})

	expected := "\033[3mslanted\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterStrikethrough(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.StrikethroughStartEvent})
	_ = w.Handle(parser.Event{Type: parser.TextEvent, Value: "gone"})
	_ = w.Handle(parser.Event{Type: parser.StrikethroughEndEvent})

	expected := "\033[9mgone\033[0m"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriterHorizontalRule(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)

	_ = w.Handle(parser.Event{Type: parser.HorizontalRuleEvent})

	expected := "\033[2m────────────────\033[0m\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}
