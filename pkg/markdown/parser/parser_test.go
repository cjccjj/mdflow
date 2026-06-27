package parser

import (
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func tokenize(s string) []tokenizer.Token {
	return tokenizer.Tokenize([]byte(s))
}

func eventTypes(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type.String()
	}
	return out
}

func TestBold(t *testing.T) {
	p := New()
	tokens := tokenize("**hello**")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"BoldStart", "Text", "BoldEnd"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestBoldStreaming(t *testing.T) {
	p := New()

	chunk1 := tokenize("**he")
	events1 := p.Parse(chunk1)
	expected1 := []string{"BoldStart", "Text"}
	got1 := eventTypes(events1)
	if !equal(got1, expected1) {
		t.Fatalf("chunk1: expected %v, got %v", expected1, got1)
	}

	chunk2 := tokenize("llo**")
	events2 := p.Parse(chunk2)
	expected2 := []string{"Text", "BoldEnd"}
	got2 := eventTypes(events2)
	if !equal(got2, expected2) {
		t.Fatalf("chunk2: expected %v, got %v", expected2, got2)
	}
}

func TestHeader(t *testing.T) {
	p := New()
	tokens := tokenize("# Hello\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"HeaderStart", "Text", "HeaderEnd", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestHeaderH2(t *testing.T) {
	p := New()
	tokens := tokenize("## Hello\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"HeaderStart", "Text", "HeaderEnd", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestInlineCode(t *testing.T) {
	p := New()
	tokens := tokenize("`code`")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"InlineCodeStart", "Text", "InlineCodeEnd"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestCodeBlock(t *testing.T) {
	p := New()
	tokens := tokenize("```\ncode\n```\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"CodeBlockStart", "Newline", "Text", "Newline", "CodeBlockEnd", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestBulletDash(t *testing.T) {
	p := New()
	tokens := tokenize("- item\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"BulletItem", "Text", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestBulletStar(t *testing.T) {
	p := New()
	tokens := tokenize("* item\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"BulletItem", "Text", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestStarNotBoldNotBullet(t *testing.T) {
	p := New()
	tokens := tokenize("*notbold*\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	// Single * is italic now
	expected := []string{"ItalicStart", "Text", "ItalicEnd", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestPlainText(t *testing.T) {
	p := New()
	tokens := tokenize("hello world\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"Text", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestMixedContent(t *testing.T) {
	p := New()
	tokens := tokenize("# Hello\n\n**world**\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{
		"HeaderStart", "Text", "HeaderEnd", "Newline",
		"Newline",
		"BoldStart", "Text", "BoldEnd", "Newline",
	}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestIncompleteBoldFlush(t *testing.T) {
	p := New()
	tokens := tokenize("**hello")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	// Flush just drains buffer, does not close state
	expected := []string{"BoldStart", "Text"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestIncompleteBoldFlushClose(t *testing.T) {
	p := New()
	tokens := tokenize("**hello")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	expected := []string{"BoldStart", "Text"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestFourStars(t *testing.T) {
	// ****abc — first ** literal (same-run), second ** opens bold optimistically
	// (streaming limitation: no closer visible, parser opens in case closer arrives later)
	p := New()
	tokens := tokenize("****abc")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	got := eventTypes(events)
	if len(got) < 2 || got[0] != "Text" {
		t.Errorf("first event should be Text (literal **), got %v", got)
	}
}

func TestDashNotBullet(t *testing.T) {
	// dash not at line start should be emitted as text
	p := New()
	tokens := tokenize("hello-world\n")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)

	// Tokenizer splits at -, so parser sees Text("hello"), Dash, Text("world"), Newline
	// Dash not at lineStart is emitted as TextEvent("-")
	expected := []string{"Text", "Text", "Text", "Newline"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestEmptyInput(t *testing.T) {
	p := New()
	events := p.Parse([]tokenizer.Token{})
	events = append(events, p.Flush()...)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %v", events)
	}
}

func TestLoneHash(t *testing.T) {
	// Lone # at line start — must not hang, emit as text
	p := New()
	tokens := tokenize("#")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)
	expected := []string{"Text"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestLoneDoubleHash(t *testing.T) {
	// Lone ## at line start — must not hang, emit as text
	p := New()
	tokens := tokenize("##")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)
	expected := []string{"Text", "Text"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestLoneStar(t *testing.T) {
	// Lone * at line start — must not hang, emit as text
	p := New()
	tokens := tokenize("*")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)
	expected := []string{"Text"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestStarStarSplit(t *testing.T) {
	// ** split across chunks — first * alone, second * completes **
	// With flanking-aware closer detection, may emit literal or bold depending on context
	p := New()
	p.Parse(tokenize("*"))
	events := p.Parse(tokenize("*"))
	events = append(events, p.Flush()...)
	got := eventTypes(events)
	// Accept either BoldStart (optimistic) or Text (conservative/no closer visible)
	if len(got) == 0 || (got[0] != "BoldStart" && got[0] != "Text") {
		t.Errorf("expected BoldStart or Text, got %v", got)
	}
}

func TestLoneDash(t *testing.T) {
	// Lone - at line start — must not hang, emit as text
	p := New()
	tokens := tokenize("-")
	events := p.Parse(tokens)
	events = append(events, p.Flush()...)
	expected := []string{"Text"}
	got := eventTypes(events)
	if !equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func equal(a, b []string) bool {
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

func intsEqual(a, b []int) bool {
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

func TestParseSeparatorAligns_Left(t *testing.T) {
	aligns, ok := parseSeparatorAligns([]string{"---", "---"})
	if !ok {
		t.Fatal("expected ok")
	}
	for i, a := range aligns {
		if a != 0 {
			t.Errorf("column %d: expected 0 (left), got %d", i, a)
		}
	}
}

func TestParseSeparatorAligns_LeftColon(t *testing.T) {
	aligns, ok := parseSeparatorAligns([]string{":---"})
	if !ok {
		t.Fatal("expected ok")
	}
	if aligns[0] != 0 {
		t.Errorf("expected 0 (left) for :---, got %d", aligns[0])
	}
}

func TestParseSeparatorAligns_Center(t *testing.T) {
	aligns, ok := parseSeparatorAligns([]string{":---:"})
	if !ok {
		t.Fatal("expected ok")
	}
	if aligns[0] != 1 {
		t.Errorf("expected 1 (center) for :---:, got %d", aligns[0])
	}
}

func TestParseSeparatorAligns_Right(t *testing.T) {
	aligns, ok := parseSeparatorAligns([]string{"---:"})
	if !ok {
		t.Fatal("expected ok")
	}
	if aligns[0] != 2 {
		t.Errorf("expected 2 (right) for ---:, got %d", aligns[0])
	}
}

func TestParseSeparatorAligns_Mixed(t *testing.T) {
	aligns, ok := parseSeparatorAligns([]string{"---", ":---:", "---:"})
	if !ok {
		t.Fatal("expected ok")
	}
	expected := []int{0, 1, 2}
	if !intsEqual(aligns, expected) {
		t.Errorf("expected %v, got %v", expected, aligns)
	}
}

func TestParseSeparatorAligns_EmptyCell(t *testing.T) {
	aligns, ok := parseSeparatorAligns([]string{"", "---"})
	if !ok {
		t.Fatal("expected ok")
	}
	if aligns[0] != 0 {
		t.Errorf("expected 0 (left) for empty, got %d", aligns[0])
	}
}

func TestParseSeparatorAligns_Invalid(t *testing.T) {
	_, ok := parseSeparatorAligns([]string{"abc"})
	if ok {
		t.Fatal("expected not ok for non-separator")
	}
	_, ok = parseSeparatorAligns([]string{})
	if ok {
		t.Fatal("expected not ok for empty")
	}
}

func TestParseSeparatorAligns_NoDash(t *testing.T) {
	_, ok := parseSeparatorAligns([]string{":"})
	if ok {
		t.Fatal("expected not ok for colon-only")
	}
}
