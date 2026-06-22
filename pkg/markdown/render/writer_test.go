package render

import (
	"bytes"
	"strings"
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

	_ = w.Handle(parser.Event{Type: parser.HeaderStartEvent, Level: 1})
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

	_ = w.Handle(parser.Event{Type: parser.HeaderStartEvent, Level: 2})
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

func TestVisibleLen_PlainText(t *testing.T) {
	if n := VisibleLen("hello"); n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
}

func TestVisibleLen_WithANSI(t *testing.T) {
	if n := VisibleLen("\033[1mhi\033[0m"); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestVisibleLen_WideEmoji(t *testing.T) {
	if n := VisibleLen("🟢"); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestVisibleLen_MixedEmoji(t *testing.T) {
	if n := VisibleLen("a🟢b"); n != 4 {
		t.Errorf("expected 4, got %d", n)
	}
}

func TestVisibleLen_CJK(t *testing.T) {
	if n := VisibleLen("中文"); n != 4 {
		t.Errorf("expected 4, got %d", n)
	}
}

func TestVisibleLen_ANSIWithWide(t *testing.T) {
	if n := VisibleLen("\033[1m中文\033[0m"); n != 4 {
		t.Errorf("expected 4, got %d", n)
	}
}

func TestVisibleLen_Empty(t *testing.T) {
	if n := VisibleLen(""); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func slicesEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWrapContent_NoWrap(t *testing.T) {
	result := WrapContent("hello", 10)
	if !slicesEq(result, []string{"hello"}) {
		t.Errorf("expected [\"hello\"], got %q", result)
	}
}

func TestWrapContent_ExactFit(t *testing.T) {
	result := WrapContent("hello", 5)
	if !slicesEq(result, []string{"hello"}) {
		t.Errorf("expected [\"hello\"], got %q", result)
	}
}

func TestWrapContent_SimpleWrap(t *testing.T) {
	result := WrapContent("abcdef", 3)
	expected := []string{"abc\033[0m", "def"}
	if !slicesEq(result, expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestWrapContent_BoldWrap(t *testing.T) {
	rendered := "\033[1mabcdef\033[0m"
	result := WrapContent(rendered, 3)
	expected := []string{"\033[1mabc\033[0m", "\033[1mdef\033[0m"}
	if !slicesEq(result, expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestWrapContent_MidStyle(t *testing.T) {
	rendered := "\033[1mab\033[0mcd"
	result := WrapContent(rendered, 2)
	expected := []string{"\033[1mab\033[0m\033[0m", "cd"}
	if !slicesEq(result, expected) {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestWrapContent_WideEmojiNoSplit(t *testing.T) {
	result := WrapContent("a🟢c", 3)
	if len(result) < 2 {
		t.Errorf("expected at least 2 lines, got %d: %q", len(result), result)
	}
}

func TestWrapContent_EmptyString(t *testing.T) {
	result := WrapContent("", 5)
	if !slicesEq(result, []string{""}) {
		t.Errorf("expected [\"\"], got %q", result)
	}
}

func TestWrapContent_ZeroWidth(t *testing.T) {
	result := WrapContent("hello", 0)
	if !slicesEq(result, []string{"hello"}) {
		t.Errorf("expected [\"hello\"], got %q", result)
	}
}

func TestWrapContent_WidthOne(t *testing.T) {
	result := WrapContent("abc", 1)
	if len(result) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(result), result)
	}
	for _, line := range result {
		if VisibleLen(line) > 1 {
			t.Errorf("line %q visible len > 1", line)
		}
	}
}

func widthsEq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLimitWidths_NoLimit(t *testing.T) {
	w := &Writer{termWidth: 0}
	result := w.limitWidths([]int{5, 5})
	if !widthsEq(result, []int{5, 5}) {
		t.Errorf("expected unchanged, got %v", result)
	}
}

func TestLimitWidths_Fits(t *testing.T) {
	w := &Writer{termWidth: 80}
	result := w.limitWidths([]int{5, 5})
	if !widthsEq(result, []int{5, 5}) {
		t.Errorf("expected unchanged, got %v", result)
	}
}

func TestLimitWidths_ScaleDown(t *testing.T) {
	w := &Writer{termWidth: 40}
	result := w.limitWidths([]int{30, 30})
	total := 0
	for _, cw := range result {
		total += cw
	}
	expectedMax := 40 - 3 // nCols+1
	if total > expectedMax {
		t.Errorf("total %d exceeds available %d: %v", total, expectedMax, result)
	}
}

func TestLimitWidths_TooNarrow(t *testing.T) {
	w := &Writer{termWidth: 20}
	result := w.limitWidths([]int{30, 30})
	if !widthsEq(result, []int{30, 30}) {
		t.Errorf("expected unchanged for term < 30, got %v", result)
	}
}

func TestLimitWidths_MinColWidth(t *testing.T) {
	w := &Writer{termWidth: 40}
	result := w.limitWidths([]int{50, 50})
	for _, cw := range result {
		if cw < 3 {
			t.Errorf("column width %d below minimum", cw)
		}
	}
}

func TestLimitWidths_SingleCol(t *testing.T) {
	w := &Writer{termWidth: 50}
	result := w.limitWidths([]int{60})
	total := result[0] + 2 // nCols+1
	if total > 50 {
		t.Errorf("total %d exceeds terminal %d", total, 50)
	}
}

func TestLimitWidths_ThreeCols(t *testing.T) {
	w := &Writer{termWidth: 50}
	result := w.limitWidths([]int{20, 20, 20})
	total := 4 // 3 cols + 1
	for _, cw := range result {
		total += cw
	}
	if total > 50 {
		t.Errorf("total %d exceeds terminal %d: %v", total, 50, result)
	}
}

func TestTableWidthLimit_Render(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)
	w.SetLive(false)
	w.SetTermWidth(40)

	_ = w.Handle(parser.Event{
		Type:   parser.TableStartEvent,
		Cells:  []string{"narrow", "wide column here"},
		Widths: []int{6, 16},
		Aligns: []int{0, 0},
	})
	_ = w.Handle(parser.Event{
		Type:  parser.TableRowEvent,
		Cells: []string{"a", "b"},
	})
	_ = w.Handle(parser.Event{Type: parser.TableEndEvent})

	out := buf.String()
	if len(out) == 0 {
		t.Fatal("no output")
	}
	if !strings.Contains(out, "┌") {
		t.Errorf("table border missing: %q", out)
	}
	if !strings.Contains(out, "narrow") {
		t.Errorf("narrow cell missing: %q", out)
	}
	if !strings.Contains(out, "wide") {
		t.Errorf("wide cell missing: %q", out)
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if VisibleLen(line) > 40 {
			t.Errorf("line exceeds term width 40: len=%d line=%q", VisibleLen(line), line)
		}
	}
}

func TestTableWrapMultiLine(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)
	w.SetLive(false)
	w.SetTermWidth(30)

	_ = w.Handle(parser.Event{
		Type:   parser.TableStartEvent,
		Cells:  []string{"col"},
		Widths: []int{25},
		Aligns: []int{0},
	})
	_ = w.Handle(parser.Event{
		Type:  parser.TableRowEvent,
		Cells: []string{"this is a long cell that should wrap"},
	})
	_ = w.Handle(parser.Event{Type: parser.TableEndEvent})

	out := buf.String()
	if !strings.Contains(out, "┌") {
		t.Fatal("table border missing")
	}
	if !strings.Contains(out, "wrap") {
		t.Errorf("wrapped word 'wrap' missing: %q", out)
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if VisibleLen(line) > 30 {
			t.Errorf("line exceeds term width 30: len=%d line=%q", VisibleLen(line), line)
		}
	}
	visibleLines := 0
	for _, line := range lines {
		if strings.Contains(line, "│") && len(strings.TrimSpace(stripANSI(line))) > 0 {
			visibleLines++
		}
	}
	if visibleLines < 2 {
		t.Errorf("expected at least 2 visible table lines, got %d: %q", visibleLines, out)
	}
}

func stripANSI(s string) string {
	result := ""
	runes := []rune(s)
	i := 0
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
		result += string(runes[i])
		i++
	}
	return result
}

func TestTableWrapAlignBold(t *testing.T) {
	var buf bytes.Buffer
	aw := NewAnsiWriter(&buf)
	w := NewWriter(aw, DefaultTheme)
	w.SetLive(false)
	w.SetTermWidth(40)

	_ = w.Handle(parser.Event{
		Type:   parser.TableStartEvent,
		Cells:  []string{"left", "center", "right"},
		Widths: []int{10, 10, 10},
		Aligns: []int{0, 1, 2},
	})
	_ = w.Handle(parser.Event{
		Type:  parser.TableRowEvent,
		Cells: []string{"short", "a longer cell", "**bold text that wraps**"},
	})
	_ = w.Handle(parser.Event{Type: parser.TableEndEvent})

	out := buf.String()
	if !strings.Contains(out, "short") {
		t.Errorf("left cell missing: %q", out)
	}
	if !strings.Contains(out, "longer") {
		t.Errorf("center cell missing: %q", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("right bold cell missing: %q", out)
	}
	if !strings.Contains(out, "\033[1m") {
		t.Errorf("bold style missing: %q", out)
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if VisibleLen(line) > 40 {
			t.Errorf("line exceeds term width 40: len=%d line=%q", VisibleLen(line), line)
		}
	}
}
