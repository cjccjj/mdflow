package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// tableParser handles GFM table detection and parsing.
// It owns table-specific state: the header buffer, column widths, and alignments.
type tableParser struct {
	tableHeaderBuf []string
	tableColWidths []int
	tableColAligns []int
	p              *Parser
}

func newTableParser(p *Parser) *tableParser {
	return &tableParser{p: p}
}

func (tp *tableParser) reset() {
	tp.tableHeaderBuf = nil
	tp.tableColWidths = nil
	tp.tableColAligns = nil
}

func (tp *tableParser) tryTableHeader() []Event {
	p := tp.p
	if !p.lineStart || len(p.buf) < 2 || !p.hasNewline() {
		return nil
	}
	cells, ok := tp.extractTableCells()
	if !ok || len(cells) == 0 {
		return p.emitTextOrSpecial()
	}
	tp.tableHeaderBuf = cells
	p.state = TablePendingState
	return nil
}

func (tp *tableParser) processTablePending() []Event {
	p := tp.p
	if len(p.buf) == 0 || !p.hasNewline() {
		return nil
	}
	if p.buf[0].Type != tokenizer.PipeToken {
		return tp.rejectTable()
	}
	cells, ok := tp.extractTableCells()
	if !ok || len(cells) == 0 {
		return tp.rejectTable()
	}
	aligns, ok := parseSeparatorAligns(cells)
	if !ok {
		return tp.rejectTable()
	}
	widths := make([]int, len(cells))
	for i, c := range cells {
		widths[i] = len(c)
	}
	if len(widths) != len(tp.tableHeaderBuf) {
		return tp.rejectTable()
	}
	tp.tableColWidths = widths
	tp.tableColAligns = aligns
	p.state = TableBodyState
	p.lineStart = true

	return []Event{{Type: TableStartEvent, Cells: tp.tableHeaderBuf, Widths: widths, Aligns: aligns}}
}

func (tp *tableParser) processTableBody() []Event {
	p := tp.p
	if len(p.buf) == 0 || !p.hasNewline() {
		return nil
	}
	if p.buf[0].Type != tokenizer.PipeToken {
		p.state = NormalState
		tp.tableColWidths = nil
		tp.tableColAligns = nil
		return []Event{{Type: TableEndEvent}}
	}
	cells, ok := tp.extractTableCells()
	if !ok {
		p.state = NormalState
		tp.tableColWidths = nil
		tp.tableColAligns = nil
		return []Event{{Type: TableEndEvent}}
	}
	p.lineStart = true

	return []Event{{Type: TableRowEvent, Cells: cells}}
}

func (tp *tableParser) rejectTable() []Event {
	p := tp.p
	cells := tp.tableHeaderBuf
	tp.tableHeaderBuf = nil
	p.state = NormalState
	if len(cells) > 0 {
		return []Event{
			{Type: TextEvent, Value: "| " + strings.Join(cells, " | ") + " |"},
			{Type: NewlineEvent},
		}
	}
	return nil
}

func (tp *tableParser) extractTableCells() ([]string, bool) {
	return tp.readTableCells(true, true)
}

func (tp *tableParser) flushBodyRow() ([]string, bool) {
	return tp.readTableCells(false, false)
}

func (tp *tableParser) readTableCells(requireLeadingPipe, requireNewline bool) ([]string, bool) {
	p := tp.p
	if len(p.buf) == 0 {
		return nil, false
	}

	hadPipe := p.buf[0].Type == tokenizer.PipeToken
	if requireLeadingPipe && !hadPipe {
		return nil, false
	}
	if hadPipe {
		p.consume(1)
	}

	var cells []string
	var current strings.Builder
	backtickDepth := 0

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			return normalizeTableCells(append(cells, strings.TrimSpace(current.String()))), hadPipe
		}
		if tok.Type == tokenizer.BacktickToken {
			backtickDepth = 1 - backtickDepth
			p.consume(1)
			current.WriteString(tok.Value)
			continue
		}
		if backtickDepth == 0 && tok.Type == tokenizer.PipeToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		p.consume(1)
		current.WriteString(tok.Value)
	}

	if requireNewline {
		return nil, false
	}
	return normalizeTableCells(append(cells, strings.TrimSpace(current.String()))), hadPipe
}

func normalizeTableCells(cells []string) []string {
	for len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

func parseSeparatorAligns(cells []string) (aligns []int, ok bool) {
	const (
		alignLeft   = 0
		alignCenter = 1
		alignRight  = 2
	)
	if len(cells) == 0 {
		return nil, false
	}
	aligns = make([]int, len(cells))
	for i, c := range cells {
		if c == "" {
			aligns[i] = alignLeft
			continue
		}
		for _, ch := range c {
			if ch != '-' && ch != ':' && ch != ' ' {
				return nil, false
			}
		}
		hasDash := false
		for _, ch := range c {
			if ch == '-' {
				hasDash = true
				break
			}
		}
		if !hasDash {
			return nil, false
		}
		leftColon := len(c) > 0 && c[0] == ':'
		rightColon := len(c) > 0 && c[len(c)-1] == ':'
		if leftColon && rightColon {
			aligns[i] = alignCenter
		} else if rightColon {
			aligns[i] = alignRight
		} else {
			aligns[i] = alignLeft
		}
	}
	return aligns, true
}
