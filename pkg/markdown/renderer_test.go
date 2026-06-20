package markdown

import (
	"bytes"
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/render"
)

func TestRendererBold(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf)

	n, err := r.Write([]byte("**hello**"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 9 {
		t.Errorf("wrote %d bytes, expected 9", n)
	}

	err = r.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	output := buf.String()
	if !contains(output, "\033[1mhello\033[0m") {
		t.Errorf("expected bold hello, got %q", output)
	}
}

func TestRendererWithCustomTheme(t *testing.T) {
	var buf bytes.Buffer
	custom := render.DefaultTheme
	custom.Bold = render.Style{Prefix: "[[", Suffix: "]]"}

	r := NewRenderer(&buf, WithTheme(custom))
	r.Write([]byte("**hi**"))
	r.Close()

	output := buf.String()
	if !contains(output, "[[hi]]") {
		t.Errorf("expected [[hi]], got %q", output)
	}
}

func TestRendererStreaming(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf)

	chunks := []string{"**hel", "lo**", " world"}
	for _, c := range chunks {
		r.Write([]byte(c))
	}
	r.Close()

	output := buf.String()
	if !contains(output, "hello") {
		t.Errorf("expected hello, got %q", output)
	}
	if !contains(output, "world") {
		t.Errorf("expected world, got %q", output)
	}
}

func TestRendererFlush(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf)

	// Write a partial bold (no closing ** yet)
	r.Write([]byte("**hel"))
	r.Flush()

	// Should have flushed the BoldStart + "hel" so far but still in bold state
	output := buf.String()
	if !contains(output, "\033[1mhel") {
		t.Errorf("expected bold hel after flush, got %q", output)
	}

	// Continue writing the rest
	r.Write([]byte("lo**"))
	r.Close()

	output = buf.String()
	if !contains(output, "\033[1mhello\033[0m") {
		t.Errorf("expected complete bold hello, got %q", output)
	}
}

func TestRendererReset(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf)

	r.Write([]byte("**hel"))
	r.Reset()

	// After reset, write new content — should start fresh
	r.Write([]byte("plain **bold**"))
	r.Close()

	output := buf.String()
	if !contains(output, "plain") {
		t.Errorf("expected plain, got %q", output)
	}
	if !contains(output, "\033[1mbold\033[0m") {
		t.Errorf("expected bold, got %q", output)
	}
}

func TestRendererWriteAfterFlush(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(&buf)

	r.Write([]byte("# Ti"))
	r.Flush()
	r.Write([]byte("tle\n"))
	r.Close()

	output := buf.String()
	if !contains(output, "\033[1;48;5;63;38;5;228m") {
		t.Errorf("expected H1 style, got %q", output)
	}
	if !contains(output, "Title") {
		t.Errorf("expected Title, got %q", output)
	}
}
