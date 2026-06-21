package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func (p *Parser) tryTableHeader() []Event {
	if !p.lineStart || len(p.buf) < 2 || !p.hasNewline() {
		return nil
	}
	cells, ok := p.extractTableCells()
	if !ok || len(cells) == 0 {
		return p.emitTextOrSpecial()
	}
	p.tableHeaderBuf = cells
	p.state = TablePendingState
	return nil
}

func (p *Parser) processTablePending() []Event {
	if len(p.buf) == 0 || !p.hasNewline() {
		return nil
	}
	if p.buf[0].Type != tokenizer.PipeToken {
		return p.rejectTable()
	}
	cells, ok := p.extractTableCells()
	if !ok || len(cells) == 0 {
		return p.rejectTable()
	}
	if !isSeparatorCells(cells) {
		return p.rejectTable()
	}
	widths := make([]int, len(cells))
	for i, c := range cells {
		widths[i] = len(c)
	}
	if len(widths) != len(p.tableHeaderBuf) {
		return p.rejectTable()
	}
	p.tableColWidths = widths
	p.state = TableBodyState
	p.lineStart = true

	headerVal := strings.Join(p.tableHeaderBuf, "\x00")
	return []Event{{Type: TableStartEvent, Value: headerVal, Extra: widths}}
}

func (p *Parser) processTableBody() []Event {
	if len(p.buf) == 0 || !p.hasNewline() {
		return nil
	}
	if p.buf[0].Type != tokenizer.PipeToken {
		p.state = NormalState
		p.tableColWidths = nil
		return []Event{{Type: TableEndEvent}}
	}
	cells, ok := p.extractTableCells()
	if !ok || len(cells) == 0 {
		p.state = NormalState
		p.tableColWidths = nil
		return []Event{{Type: TableEndEvent}}
	}
	p.lineStart = true

	rowVal := strings.Join(cells, "\x00")
	return []Event{{Type: TableRowEvent, Value: rowVal}}
}

func (p *Parser) rejectTable() []Event {
	cells := p.tableHeaderBuf
	p.tableHeaderBuf = nil
	p.state = NormalState
	if len(cells) > 0 {
		return []Event{
			{Type: TextEvent, Value: "| " + strings.Join(cells, " | ") + " |"},
			{Type: NewlineEvent},
		}
	}
	return nil
}

func (p *Parser) extractTableCells() ([]string, bool) {
	if len(p.buf) == 0 || p.buf[0].Type != tokenizer.PipeToken {
		return nil, false
	}
	p.consume(1)
	var cells []string
	var current strings.Builder
	backtickDepth := 0

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			for len(cells) > 0 && cells[len(cells)-1] == "" {
				cells = cells[:len(cells)-1]
			}
			return cells, len(cells) > 0
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
	return nil, false
}

func (p *Parser) flushBodyRow() ([]string, bool) {
	if len(p.buf) == 0 {
		return nil, false
	}
	if p.buf[0].Type == tokenizer.PipeToken {
		p.consume(1)
	}
	var cells []string
	var current strings.Builder
	backtickDepth := 0

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			for len(cells) > 0 && cells[len(cells)-1] == "" {
				cells = cells[:len(cells)-1]
			}
			return cells, len(cells) > 0
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
	cells = append(cells, strings.TrimSpace(current.String()))
	for len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells, len(cells) > 0
}

func isSeparatorCells(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if c == "" {
			continue
		}
		for _, ch := range c {
			if ch != '-' && ch != ':' && ch != ' ' {
				return false
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
			return false
		}
	}
	return true
}
