package parser

import (
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func tableEventTypes(input string) []string {
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

func TestTable_Basic(t *testing.T) {
	types := tableEventTypes("| A | B |\n| --- | --- |\n| 1 | 2 |\n")
	// Should be TableStart, TableRow, TableEnd
	hasStart := false
	hasRow := false
	hasEnd := false
	for _, ty := range types {
		switch ty {
		case "TableStart":
			hasStart = true
		case "TableRow":
			hasRow = true
		case "TableEnd":
			hasEnd = true
		}
	}
	if !hasStart || !hasRow || !hasEnd {
		t.Errorf("expected TableStart+TableRow+TableEnd, got %v", types)
	}
}

func TestTable_Alignments(t *testing.T) {
	// Left, center, right alignment
	types := tableEventTypes("| A | B | C |\n| :--- | :---: | ---: |\n| 1 | 2 | 3 |\n")
	hasStart := false
	for _, ty := range types {
		if ty == "TableStart" {
			hasStart = true
		}
	}
	if !hasStart {
		t.Errorf("expected TableStart, got %v", types)
	}
}

func TestTable_MultipleRows(t *testing.T) {
	types := tableEventTypes("| A |\n| --- |\n| 1 |\n| 2 |\n| 3 |\n")
	rowCount := 0
	for _, ty := range types {
		if ty == "TableRow" {
			rowCount++
		}
	}
	if rowCount != 3 {
		t.Errorf("expected 3 TableRow events, got %d: %v", rowCount, types)
	}
}

func TestTable_SingleColumn(t *testing.T) {
	types := tableEventTypes("| A |\n| --- |\n| 1 |\n")
	hasStart := false
	hasRow := false
	hasEnd := false
	for _, ty := range types {
		switch ty {
		case "TableStart":
			hasStart = true
		case "TableRow":
			hasRow = true
		case "TableEnd":
			hasEnd = true
		}
	}
	if !hasStart || !hasRow || !hasEnd {
		t.Errorf("expected TableStart+TableRow+TableEnd, got %v", types)
	}
}

func TestTable_NotATable_NoSeparator(t *testing.T) {
	// Pipe at line start but no valid separator = not a table
	types := tableEventTypes("| not a table\n")
	for _, ty := range types {
		if ty == "TableStart" {
			t.Errorf("should not be a table, got %v", types)
		}
	}
}

func TestTable_NotATable_NoTrailingNewline(t *testing.T) {
	// Incomplete first line (no newline) — parser waits for more data
	p := New()
	events := p.Parse(tokenizer.Tokenize([]byte("| A | B |")))
	if len(events) != 0 {
		t.Errorf("expected 0 events (waiting for newline), got %d: %v", len(events), events)
	}
	// State stays NormalState because hasNewline() returned false;
	// the parser loop breaks waiting for more tokens
	if p.state != NormalState {
		t.Errorf("expected NormalState, got %v", p.state)
	}
}

func TestTable_StreamingAcrossChunks(t *testing.T) {
	p := New()
	// Header
	events := p.Parse(tokenizer.Tokenize([]byte("| A | B |\n")))
	if len(events) != 0 {
		t.Logf("header events: %v", events)
	}
	// Separator
	events = p.Parse(tokenizer.Tokenize([]byte("| --- | --- |\n")))
	if len(events) < 1 || events[0].Type != TableStartEvent {
		t.Errorf("expected TableStart, got %v", events)
	}
	// Data row
	events = p.Parse(tokenizer.Tokenize([]byte("| 1 | 2 |\n")))
	if len(events) < 1 || events[0].Type != TableRowEvent {
		t.Errorf("expected TableRow, got %v", events)
	}
}

func TestTable_FlushPendingRejects(t *testing.T) {
	p := New()
	p.Parse(tokenizer.Tokenize([]byte("| A | B |\n")))
	// Flush should emit as text
	events := p.Flush()
	text := ""
	for _, e := range events {
		if e.Type == TextEvent {
			text += e.Value
		}
	}
	if text != "| A | B |" {
		t.Errorf("expected '| A | B |' as text from flush, got '%s'", text)
	}
}

func TestTable_Reset(t *testing.T) {
	p := New()
	p.Parse(tokenizer.Tokenize([]byte("| A |\n")))
	if p.state != TablePendingState {
		t.Fatalf("expected TablePendingState, got %v", p.state)
	}
	p.Reset()
	if p.state != NormalState {
		t.Errorf("expected NormalState after Reset, got %v", p.state)
	}
	if p.tableParser.tableHeaderBuf != nil {
		t.Error("tableHeaderBuf should be nil after Reset")
	}
}

func TestTable_BodyEndsWithoutPipe(t *testing.T) {
	// Table body ends when line doesn't start with pipe
	types := tableEventTypes("| A |\n| --- |\n| 1 |\nnot a table row\n")
	hasEnd := false
	for _, ty := range types {
		if ty == "TableEnd" {
			hasEnd = true
		}
	}
	if !hasEnd {
		t.Errorf("expected TableEnd when body line doesn't start with pipe, got %v", types)
	}
}

func TestTable_EmptyCells(t *testing.T) {
	types := tableEventTypes("| A | | C |\n| --- | --- | --- |\n| 1 | | 3 |\n")
	hasStart := false
	hasRow := false
	for _, ty := range types {
		if ty == "TableStart" {
			hasStart = true
		}
		if ty == "TableRow" {
			hasRow = true
		}
	}
	if !hasStart || !hasRow {
		t.Errorf("expected TableStart+TableRow for table with empty cells, got %v", types)
	}
}

// parseSeparatorAligns tests already exist in parser_test.go
