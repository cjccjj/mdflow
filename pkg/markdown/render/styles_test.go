package render

import (
	"testing"
)

func TestDefaultThemeBold(t *testing.T) {
	bold := DefaultTheme.Bold
	if bold.Prefix != "\033[1m" {
		t.Errorf("expected bold prefix \\033[1m, got %q", bold.Prefix)
	}
	if bold.Suffix != "\033[0m" {
		t.Errorf("expected bold suffix \\033[0m, got %q", bold.Suffix)
	}
}

func TestDefaultThemeH1(t *testing.T) {
	h1 := DefaultTheme.H1
	if h1.Prefix != "\033[1;48;5;63;38;5;228m" {
		t.Errorf("expected H1 prefix \\033[1;48;5;63;38;5;228m, got %q", h1.Prefix)
	}
}

func TestDefaultThemeCodeBlock(t *testing.T) {
	cb := DefaultTheme.CodeBlock
	if cb.Prefix != "\033[38;5;245m" {
		t.Errorf("expected codeblock prefix \\033[38;5;245m, got %q", cb.Prefix)
	}
}

func TestDefaultThemeItalic(t *testing.T) {
	it := DefaultTheme.Italic
	if it.Prefix != "\033[3m" {
		t.Errorf("expected italic prefix \\033[3m, got %q", it.Prefix)
	}
}

func TestDefaultThemeStrikethrough(t *testing.T) {
	st := DefaultTheme.Strikethrough
	if st.Prefix != "\033[9m" {
		t.Errorf("expected strikethrough prefix \\033[9m, got %q", st.Prefix)
	}
}
