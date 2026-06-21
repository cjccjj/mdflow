package parser

import (
	"strings"
)

func (p *Parser) CloseStates() []Event {
	var out []Event
	switch p.state {
	case HeaderState:
		out = append(out, Event{Type: HeaderEndEvent})
	case BoldState:
		out = append(out, Event{Type: BoldEndEvent})
	case ItalicState:
		out = append(out, Event{Type: ItalicEndEvent})
	case StrikethroughState:
		out = append(out, Event{Type: StrikethroughEndEvent})
	case InlineCodeState:
		out = append(out, Event{Type: InlineCodeEndEvent})
	case CodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case IndentedCodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case BlockquoteState:
		out = append(out, Event{Type: BlockquoteEndEvent})
	case TableBodyState:
		cells, ok := p.flushBodyRow()
		if ok {
			out = append(out, Event{Type: TableRowEvent, Cells: cells})
		}
		out = append(out, Event{Type: TableEndEvent})
	case TablePendingState:
		if len(p.tableHeaderBuf) > 0 {
			out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableHeaderBuf, " | ") + " |"})
			out = append(out, Event{Type: NewlineEvent})
		}
		p.tableHeaderBuf = nil
	case SetextPendingState:
		out = append(out, p.flushSetext()...)
	}
	return out
}

func (p *Parser) Flush() []Event {
	var out []Event
	if p.state == TableBodyState {
		cells, ok := p.flushBodyRow()
		if ok {
			out = append(out, Event{Type: TableRowEvent, Cells: cells})
		}
		out = append(out, Event{Type: TableEndEvent})
		p.state = NormalState
		return out
	}
	if p.state == TablePendingState && len(p.tableHeaderBuf) > 0 {
		out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableHeaderBuf, " | ") + " |"})
		out = append(out, Event{Type: NewlineEvent})
		p.tableHeaderBuf = nil
		p.state = NormalState
	}
	if p.state == SetextPendingState {
		out = append(out, p.flushSetext()...)
		p.state = NormalState
	}
	if len(p.buf) > 0 {
		for _, tok := range p.buf {
			out = append(out, Event{Type: TextEvent, Value: tok.Value})
		}
		p.buf = nil
	}
	return out
}

func (p *Parser) safeFlush() []Event {
	var out []Event
	switch p.state {
	case HeaderState:
		out = append(out, Event{Type: HeaderEndEvent})
	case BoldState:
		out = append(out, Event{Type: BoldEndEvent})
	case ItalicState:
		out = append(out, Event{Type: ItalicEndEvent})
	case StrikethroughState:
		out = append(out, Event{Type: StrikethroughEndEvent})
	case InlineCodeState:
		out = append(out, Event{Type: InlineCodeEndEvent})
	case CodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case IndentedCodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case BlockquoteState:
		out = append(out, Event{Type: BlockquoteEndEvent})
	case TableBodyState:
		cells, ok := p.flushBodyRow()
		if ok {
			out = append(out, Event{Type: TableRowEvent, Cells: cells})
		}
		out = append(out, Event{Type: TableEndEvent})
	case TablePendingState:
		if len(p.tableHeaderBuf) > 0 {
			out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableHeaderBuf, " | ") + " |"})
			out = append(out, Event{Type: NewlineEvent})
		}
	case SetextPendingState:
		out = append(out, p.flushSetext()...)
	}
	for _, tok := range p.buf {
		out = append(out, Event{Type: TextEvent, Value: tok.Value})
	}
	p.Reset()
	return out
}

func (p *Parser) flushSetext() []Event {
	var out []Event
	out = append(out, p.parseInlineLine(p.setextBuf)...)
	out = append(out, Event{Type: NewlineEvent})
	p.setextBuf = nil
	p.setextWaiting = false
	return out
}
