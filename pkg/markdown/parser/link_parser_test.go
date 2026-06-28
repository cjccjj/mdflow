package parser

import (
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// Helper: parse inline tokens and return event type strings.
// Uses a full Parser so we test the link parser in its real context.
func linkEventTypes(input string) []string {
	p := New()
	// Only set eof when we have all tokens; then CloseStates to complete.
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

func linkEventValues(input string) []Event {
	p := New()
	tokens := tokenizer.Tokenize([]byte(input))
	p.eof = true
	events := p.Parse(tokens)
	events = append(events, p.CloseStates()...)
	return events
}

// --- Basic inline link tests ---

func TestLinkParser_BasicLink(t *testing.T) {
	types := linkEventTypes("[text](url)")
	if len(types) != 1 {
		t.Fatalf("expected 1 event, got %d: %v", len(types), types)
	}
	if types[0] != "Link" {
		t.Errorf("expected Link event, got %s", types[0])
	}
}

func TestLinkParser_LinkValueAndURL(t *testing.T) {
	events := linkEventValues("[hello](https://example.com)")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Value != "hello" {
		t.Errorf("expected Value='hello', got '%s'", events[0].Value)
	}
	if events[0].URL != "https://example.com" {
		t.Errorf("expected URL='https://example.com', got '%s'", events[0].URL)
	}
}

func TestLinkParser_LinkWithTitle(t *testing.T) {
	events := linkEventValues(`[text](url "title")`)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "title" {
		t.Errorf("expected Title='title', got '%s'", events[0].Title)
	}
}

func TestLinkParser_LinkWithSingleQuoteTitle(t *testing.T) {
	events := linkEventValues(`[text](url 'title')`)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "title" {
		t.Errorf("expected Title='title', got '%s'", events[0].Title)
	}
}

func TestLinkParser_LinkWithParenTitle(t *testing.T) {
	events := linkEventValues(`[text](url (title))`)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "title" {
		t.Errorf("expected Title='title', got '%s'", events[0].Title)
	}
}

func TestLinkParser_AngleBracketURL(t *testing.T) {
	events := linkEventValues("[text](<https://example.com>)")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].URL != "https://example.com" {
		t.Errorf("expected URL without angle brackets, got '%s'", events[0].URL)
	}
}

func TestLinkParser_BalancedParensInURL(t *testing.T) {
	events := linkEventValues("[text](/foo(and(bar)))")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].URL != "/foo(and(bar))" {
		t.Errorf("expected URL with balanced parens, got '%s'", events[0].URL)
	}
}

// --- Balanced brackets in link text ---

func TestLinkParser_BalancedBrackets(t *testing.T) {
	events := linkEventValues("[link [foo [bar]]](/uri)")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Value != "link [foo [bar]]" {
		t.Errorf("expected link text with balanced brackets, got '%s'", events[0].Value)
	}
}

// --- Reference links ---

func TestLinkParser_FullReferenceLink(t *testing.T) {
	events := linkEventValues("[text][label]")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != LinkRefEvent {
		t.Errorf("expected LinkRef event, got %s", events[0].Type.String())
	}
	if events[0].Value != "text" {
		t.Errorf("expected Value='text', got '%s'", events[0].Value)
	}
	if events[0].URL != "label" {
		t.Errorf("expected URL='label', got '%s'", events[0].URL)
	}
}

func TestLinkParser_CollapsedReferenceLink(t *testing.T) {
	events := linkEventValues("[text][]")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != LinkRefEvent {
		t.Errorf("expected LinkRef event, got %s", events[0].Type.String())
	}
}

// --- Not a link ---

func TestLinkParser_NotALink_TextBracket(t *testing.T) {
	// `]` without preceding `[` is just text
	events := linkEventValues("text ] not a link")
	// Should just be text events
	for _, e := range events {
		if e.Type == LinkEvent {
			t.Errorf("should not have found a link event: %+v", e)
		}
	}
}

func TestLinkParser_BareBrackets(t *testing.T) {
	// `[not a link` - unclosed bracket should flush as text (with brackets added by CloseStates)
	events := linkEventValues("[not a link")
	foundLink := false
	for _, e := range events {
		if e.Type == LinkEvent {
			foundLink = true
		}
	}
	if foundLink {
		t.Error("bare brackets should not become a link")
	}
	// The content appears as text (CloseStates adds `]` to unclosed links)
	text := ""
	for _, e := range events {
		if e.Type == TextEvent {
			text += e.Value
		}
	}
	if text != "[not a link]" {
		t.Errorf("expected '[not a link]' as text, got '%s'", text)
	}
}

// --- Streaming (cross-chunk boundary) ---

func TestLinkParser_StreamingLinkTextOpen(t *testing.T) {
	// Start link text, no closer yet
	p := New()
	tokens := tokenizer.Tokenize([]byte("[start"))
	events := p.Parse(tokens)
	// Should not emit link yet (waiting for "]")
	if len(events) != 0 {
		t.Errorf("expected 0 events while waiting for ']', got %d: %v", len(events), events)
	}
	if p.state != LinkTextState {
		t.Errorf("expected LinkTextState, got %v", p.state)
	}
}

func TestLinkParser_StreamingLinkAcrossChunks(t *testing.T) {
	// Full link across chunks: "[la" + "bel](u" + "rl)"
	p := New()

	// Chunk 1
	events := p.Parse(tokenizer.Tokenize([]byte("[la")))
	if len(events) != 0 {
		t.Errorf("chunk 1: expected 0 events, got %d", len(events))
	}

	// Chunk 2
	events = p.Parse(tokenizer.Tokenize([]byte("bel](u")))
	if len(events) != 0 {
		t.Errorf("chunk 2: expected 0 events, got %d", len(events))
	}

	// Chunk 3
	p.eof = true
	events = p.Parse(tokenizer.Tokenize([]byte("rl)")))
	events = append(events, p.CloseStates()...)
	if len(events) != 1 {
		t.Fatalf("chunk 3: expected 1 event, got %d: %v", len(events), events)
	}
	if events[0].Type != LinkEvent {
		t.Errorf("expected LinkEvent, got %s", events[0].Type.String())
	}
	if events[0].Value != "label" {
		t.Errorf("expected Value='label', got '%s'", events[0].Value)
	}
	if events[0].URL != "url" {
		t.Errorf("expected URL='url', got '%s'", events[0].URL)
	}
}

// --- Link text with escaped brackets ---

func TestLinkParser_EscapedBrackets(t *testing.T) {
	// `\[` produces literal `[` in link text, `\]` produces literal `]`
	// Input: [foo \\[bar\\] baz](/uri) — the escaped brackets stay in link text
	events := linkEventValues(`[foo \[bar\] baz](/uri)`)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Value != `foo [bar] baz` {
		t.Errorf("expected 'foo [bar] baz', got '%s'", events[0].Value)
	}
}

// --- Link flush (link as text when syntax fails) ---

func TestLinkParser_FlushLinkAsText(t *testing.T) {
	// Link text with newline flushes as text (brackets added)
	p := New()
	p.eof = true
	events := p.Parse(tokenizer.Tokenize([]byte("[text\n")))
	events = append(events, p.CloseStates()...)

	allText := ""
	for _, e := range events {
		if e.Type == TextEvent {
			allText += e.Value
		}
	}
	// The link is flushed as [text] (newline triggers flushLinkAsText which adds brackets)
	if allText != "[text]" {
		t.Errorf("expected '[text]' as text, got '%s'", allText)
	}
}

// --- Link with entity in text ---

func TestLinkParser_EntityInLinkText(t *testing.T) {
	events := linkEventValues("[foo &amp; bar](/uri)")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Value != "foo & bar" {
		t.Errorf("expected 'foo & bar', got '%s'", events[0].Value)
	}
}

// --- Link URL with various patterns ---

func TestLinkParser_URLWithSpacesFails(t *testing.T) {
	// URL with spaces should flush as text, not create a link
	events := linkEventValues("[text](/my uri)")
	hasLink := false
	for _, e := range events {
		if e.Type == LinkEvent {
			hasLink = true
		}
	}
	if hasLink {
		t.Error("URL with spaces should not become a link")
	}
}

func TestLinkParser_EmptyURL(t *testing.T) {
	events := linkEventValues("[text]()")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != LinkEvent {
		t.Errorf("expected LinkEvent, got %s", events[0].Type.String())
	}
	if events[0].URL != "" {
		t.Errorf("expected empty URL, got '%s'", events[0].URL)
	}
}

// --- Link reset ---

func TestLinkParser_Reset(t *testing.T) {
	p := New()
	// Start a link
	p.Parse(tokenizer.Tokenize([]byte("[text")))
	if p.state != LinkTextState {
		t.Fatalf("expected LinkTextState after opening bracket, got %v", p.state)
	}

	// Reset
	p.Reset()

	// Verify link state is cleared
	if p.state != NormalState {
		t.Errorf("expected NormalState after Reset, got %v", p.state)
	}
	// Verify link fields are zeroed
	if p.linkParser.linkBuf != nil {
		t.Error("linkBuf should be nil after Reset")
	}
	if p.linkParser.linkBracketConsumed {
		t.Error("linkBracketConsumed should be false after Reset")
	}
	if p.linkParser.linkDepth != 0 {
		t.Error("linkDepth should be 0 after Reset")
	}
}

// --- Test that link parser correctly handles empty buffer ---

func TestLinkParser_EmptyBuffer(t *testing.T) {
	p := New()
	p.state = LinkTextState
	events := p.Parse(tokenizer.Tokenize([]byte("")))
	if len(events) != 0 {
		t.Errorf("expected 0 events for empty buffer, got %d", len(events))
	}
}

// --- Test leading whitespace in URL ---

func TestLinkParser_LeadingWhitespaceInURL(t *testing.T) {
	events := linkEventValues("[link](   /uri)")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != LinkEvent {
		t.Errorf("expected LinkEvent, got %s", events[0].Type.String())
	}
	if events[0].URL != "/uri" {
		t.Errorf("expected URL='/uri', got '%s'", events[0].URL)
	}
}

// --- Test URL with backslash-escaped paren ---

func TestLinkParser_EscapedParenInURL(t *testing.T) {
	events := linkEventValues(`[text](\(foo\))`)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != LinkEvent {
		t.Errorf("expected LinkEvent, got %s", events[0].Type.String())
	}
}
