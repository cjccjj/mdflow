package markdown

import (
	"bytes"
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/render"
)

func TestPipelineHeaderBold(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(&buf, render.DefaultTheme)

	p.Write([]byte("# Hello\n\n**world**\n"))
	p.Close()

	output := buf.String()
	// H1: blue bg + yellow fg + bold
	if !contains(output, "\033[1;48;5;63;38;5;228m") {
		t.Error("missing H1 prefix")
	}
	if !contains(output, "\033[1m") {
		t.Error("missing bold prefix")
	}
	if !contains(output, "\033[0m") {
		t.Error("missing reset")
	}
}

func TestPipelineStreaming(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(&buf, render.DefaultTheme)

	chunks := []string{
		"# Hel",
		"lo\n",
		"**wor",
		"ld**",
	}
	for _, c := range chunks {
		p.Write([]byte(c))
	}
	p.Close()

	output := buf.String()
	if !contains(output, "Hello") {
		t.Errorf("expected Hello in output, got %q", output)
	}
	if !contains(output, "world") {
		t.Errorf("expected world in output, got %q", output)
	}
	if !contains(output, "\033[1;48;5;63;38;5;228m") {
		t.Error("missing H1 style")
	}
	if !contains(output, "\033[1m") {
		t.Error("missing bold style")
	}
}

func TestPipelineBullet(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(&buf, render.DefaultTheme)

	p.Write([]byte("- item\n* another\n"))
	p.Close()

	output := buf.String()
	if !contains(output, "item") {
		t.Errorf("expected item in output, got %q", output)
	}
	if !contains(output, "another") {
		t.Errorf("expected another in output, got %q", output)
	}
}

func TestPipelineInlineCode(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(&buf, render.DefaultTheme)

	p.Write([]byte("use `code` here\n"))
	p.Close()

	output := buf.String()
	if !contains(output, "code") {
		t.Errorf("expected code in output, got %q", output)
	}
	if !contains(output, "\033[38;5;215;48;5;236m") {
		t.Error("missing inline code background")
	}
}

func TestPipelineCodeBlock(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(&buf, render.DefaultTheme)

	p.Write([]byte("```\ncode\n```\n"))
	p.Close()

	output := buf.String()
	if !contains(output, "code") {
		t.Errorf("expected code in output, got %q", output)
	}
	if !contains(output, "\033[38;5;245m") {
		t.Error("missing code block color")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
