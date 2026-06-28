package parser

import (
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// emphasisEventTypes parses input through a full parser and returns event type strings.
func emphasisEventTypes(input string) []string {
	p := New()
	tokens := tokenizer.Tokenize([]byte(input))
	p.eof = true
	events := p.Parse(tokens)
	events = append(events, p.CloseStates()...)
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type.String()
	}
	return types
}

// --- Bold (star) tests ---

func TestEmphasis_BoldStar(t *testing.T) {
	types := emphasisEventTypes("**bold**")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "BoldStart" || types[1] != "Text" || types[2] != "BoldEnd" {
		t.Errorf("expected [BoldStart Text BoldEnd], got %v", types)
	}
}

func TestEmphasis_BoldUnderscore(t *testing.T) {
	types := emphasisEventTypes("__bold__")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "BoldStart" || types[1] != "Text" || types[2] != "BoldEnd" {
		t.Errorf("expected [BoldStart Text BoldEnd], got %v", types)
	}
}

// --- Italic (star) tests ---

func TestEmphasis_ItalicStar(t *testing.T) {
	types := emphasisEventTypes("*italic*")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" || types[1] != "Text" || types[2] != "ItalicEnd" {
		t.Errorf("expected [ItalicStart Text ItalicEnd], got %v", types)
	}
}

func TestEmphasis_ItalicUnderscore(t *testing.T) {
	types := emphasisEventTypes("_italic_")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" || types[1] != "Text" || types[2] != "ItalicEnd" {
		t.Errorf("expected [ItalicStart Text ItalicEnd], got %v", types)
	}
}

// --- Strikethrough tests ---

func TestEmphasis_Strikethrough(t *testing.T) {
	types := emphasisEventTypes("~~strike~~")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "StrikethroughStart" || types[1] != "Text" || types[2] != "StrikethroughEnd" {
		t.Errorf("expected [StrikethroughStart Text StrikethroughEnd], got %v", types)
	}
}

// --- Combined bold+italic (3 stars) ---

func TestEmphasis_BoldItalicCombo(t *testing.T) {
	types := emphasisEventTypes("***bold italic***")
	// Should be ItalicStart, BoldStart, Text, BoldEnd, ItalicEnd
	if len(types) != 5 {
		t.Fatalf("expected 5 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" || types[1] != "BoldStart" {
		t.Errorf("expected [ItalicStart BoldStart ...], got %v", types[:2])
	}
}

// --- Nested emphasis ---

func TestEmphasis_NestedBoldInItalic(t *testing.T) {
	types := emphasisEventTypes("*italic **bold** italic*")
	// ItalicStart, Text, BoldStart, Text, BoldEnd, Text, ItalicEnd
	if len(types) != 7 {
		t.Fatalf("expected 7 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" {
		t.Errorf("expected ItalicStart first, got %s", types[0])
	}
	// Find BoldStart inside
	hasBold := false
	for _, ty := range types {
		if ty == "BoldStart" {
			hasBold = true
		}
	}
	if !hasBold {
		t.Error("expected BoldStart inside italic")
	}
}

func TestEmphasis_NestedItalicInBold(t *testing.T) {
	types := emphasisEventTypes("**bold *italic* bold**")
	if len(types) != 7 {
		t.Fatalf("expected 7 events, got %d: %v", len(types), types)
	}
	if types[0] != "BoldStart" {
		t.Errorf("expected BoldStart first, got %s", types[0])
	}
	hasItalic := false
	for _, ty := range types {
		if ty == "ItalicStart" {
			hasItalic = true
		}
	}
	if !hasItalic {
		t.Error("expected ItalicStart inside bold")
	}
}

// --- Cross-character nesting ---

func TestEmphasis_StarInUnderscore(t *testing.T) {
	types := emphasisEventTypes("*_foo_*")
	if len(types) != 5 {
		t.Fatalf("expected 5 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" || types[1] != "ItalicStart" {
		t.Errorf("expected two ItalicStarts, got %v", types[:2])
	}
}

func TestEmphasis_UnderscoreInStar(t *testing.T) {
	types := emphasisEventTypes("_*foo*_")
	if len(types) != 5 {
		t.Fatalf("expected 5 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" || types[1] != "ItalicStart" {
		t.Errorf("expected two ItalicStarts, got %v", types[:2])
	}
}

// --- Intraword underscore prevention ---

func TestEmphasis_IntrawordUnderscore(t *testing.T) {
	// foo_bar_ should NOT be italic (intraword)
	types := emphasisEventTypes("foo_bar_")
	for _, ty := range types {
		if ty == "ItalicStart" {
			t.Errorf("intraword underscore should not open italic, got events: %v", types)
		}
	}
}

func TestEmphasis_IntrawordDoubleUnderscore(t *testing.T) {
	// foo__bar__ should NOT be bold (intraword)
	types := emphasisEventTypes("foo__bar__")
	for _, ty := range types {
		if ty == "BoldStart" {
			t.Errorf("intraword double underscore should not open bold, got events: %v", types)
		}
	}
}

// --- Left-flanking: not followed by whitespace ---

func TestEmphasis_NotLeftFlanking_StarFollowedBySpace(t *testing.T) {
	// ** followed by space is not left-flanking
	types := emphasisEventTypes("** foo bar**")
	for _, ty := range types {
		if ty == "BoldStart" {
			t.Error("** followed by space should not open bold")
		}
	}
}

// --- Right-flanking: closer not preceded by whitespace ---

func TestEmphasis_StarCloserPrecededBySpace(t *testing.T) {
	// *foo bar * - closer preceded by space → not right-flanking
	types := emphasisEventTypes("*foo bar *")
	hasItalicStart := false
	for _, ty := range types {
		if ty == "ItalicStart" {
			hasItalicStart = true
		}
	}
	if hasItalicStart {
		t.Error("* opener should not open if only closer is preceded by space")
	}
}

// --- Multiple-of-3 rule ---

func TestEmphasis_MultipleOf3(t *testing.T) {
	// **foo* should produce *<em>foo</em> (italic, not bold)
	types := emphasisEventTypes("**foo*")
	if len(types) < 3 {
		t.Fatalf("expected at least 3 events, got %d: %v", len(types), types)
	}
	// Should start with Text("*") then ItalicStart
	if types[0] != "Text" || types[1] != "ItalicStart" {
		t.Errorf("expected [Text ItalicStart ...], got %v", types[:2])
	}
}

// --- Line-start star: emphasis, not bullet ---

func TestEmphasis_LineStartStar(t *testing.T) {
	// *foo* at line start should be italic, not bullet
	types := emphasisEventTypes("*foo*")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" {
		t.Errorf("expected ItalicStart at line start, got %s", types[0])
	}
}

// --- Strikethrough with other emphasis ---

func TestEmphasis_StrikethroughWithBold(t *testing.T) {
	types := emphasisEventTypes("~~**bold strike**~~")
	if len(types) != 5 {
		t.Fatalf("expected 5 events, got %d: %v", len(types), types)
	}
	if types[0] != "StrikethroughStart" || types[1] != "BoldStart" {
		t.Errorf("expected [StrikethroughStart BoldStart ...], got %v", types[:2])
	}
	if types[len(types)-2] != "BoldEnd" || types[len(types)-1] != "StrikethroughEnd" {
		t.Errorf("expected [... BoldEnd StrikethroughEnd], got %v", types[len(types)-2:])
	}
}

// --- Streaming: emphasis across chunk boundaries ---

func TestEmphasis_StreamingBoldAcrossChunks(t *testing.T) {
	p := New()
	events := p.Parse(tokenizer.Tokenize([]byte("**bo")))
	// Gets BoldStart + Text("bo") — text after emphasis opener is emitted immediately
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d: %v", len(events), events)
	}
	if events[0].Type != BoldStartEvent {
		t.Fatalf("expected BoldStart first, got %s", events[0].Type)
	}
	events = p.Parse(tokenizer.Tokenize([]byte("ld**")))
	// Should get Text("ld") + BoldEnd
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d: %v", len(events), events)
	}
	last := events[len(events)-1]
	if last.Type != BoldEndEvent {
		t.Errorf("expected BoldEnd last, got %s", last.Type)
	}
}

// --- Emphasis stack drain on close ---

func TestEmphasis_UnclosedBoldAtClose(t *testing.T) {
	p := New()
	p.eof = true
	events := p.Parse(tokenizer.Tokenize([]byte("**unclosed")))
	events = append(events, p.CloseStates()...)
	// Should have BoldStart, Text, BoldEnd
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d: %v", len(events), events)
	}
	if events[len(events)-1].Type != BoldEndEvent {
		t.Errorf("expected BoldEnd last (from stack drain), got %s", events[len(events)-1].Type)
	}
}

// --- Emphasis not confused by punctuation ---

func TestEmphasis_StarAroundQuotes(t *testing.T) {
	types := emphasisEventTypes(`*"foo"*`)
	// The * here is left-flanking (not followed by whitespace, followed by punct, preceded by nothing/ws)
	// Actually at document start, prevChar is \n, which is whitespace.
	// * followed by " — " is punctuation. Left-flanking: (not followed by ws) AND
	// (not followed by punct OR preceded by ws/punct). Since preceded by ws(\n) and
	// followed by punct("), it IS left-flanking.
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "ItalicStart" {
		t.Errorf("expected ItalicStart, got %s", types[0])
	}
}

// --- Emphasis stack: depth cap ---

func TestEmphasis_DepthCap(t *testing.T) {
	// 4 levels of nesting should be capped at 3
	types := emphasisEventTypes("***~~**bold**~~***")
	// Should produce events without crash
	if len(types) == 0 {
		t.Error("expected some events")
	}
}

// --- Bold underscore at line start ---

func TestEmphasis_BoldUnderscoreLineStart(t *testing.T) {
	types := emphasisEventTypes("__bold__")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	if types[0] != "BoldStart" {
		t.Errorf("expected BoldStart, got %s", types[0])
	}
}

// --- Star not a bullet when not followed by whitespace ---

func TestEmphasis_StarAtLineStart_NotBullet(t *testing.T) {
	types := emphasisEventTypes("*not a bullet*")
	if len(types) != 3 {
		t.Fatalf("expected 3 events, got %d: %v", len(types), types)
	}
	// Should be italic, not bullet
	hasBullet := false
	for _, ty := range types {
		if ty == "BulletItem" {
			hasBullet = true
		}
	}
	if hasBullet {
		t.Error("star at line start without trailing space should not be a bullet")
	}
}

// --- Star bullet: yes when followed by space ---

func TestEmphasis_StarAtLineStart_Bullet(t *testing.T) {
	types := emphasisEventTypes("* bullet")
	if len(types) < 2 {
		t.Fatalf("expected at least 2 events, got %d: %v", len(types), types)
	}
	if types[0] != "BulletItem" {
		t.Errorf("expected BulletItem, got %s", types[0])
	}
}
