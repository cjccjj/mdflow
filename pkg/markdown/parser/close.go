package parser

import (
	"strings"
)

type finalizeMode int

const (
	finalizeClose finalizeMode = iota
	finalizeFlush
	finalizeSafe
)

func (p *Parser) CloseStates() []Event {
	return p.finalizeState(finalizeClose)
}

func (p *Parser) Flush() []Event {
	var out []Event

	if p.state == TableBodyState {
		return p.finalizeState(finalizeFlush)
	}

	switch p.state {
	case TablePendingState, SetextPendingState, LinkTextState, LinkURLState:
		out = append(out, p.finalizeState(finalizeFlush)...)
	}

	if len(p.buf) > 0 {
		p.eof = true
		out = append(out, p.process()...)
		for _, tok := range p.buf {
			out = append(out, Event{Type: TextEvent, Value: tok.Value})
		}
		p.buf = nil
		p.eof = false
	}
	return out
}

func (p *Parser) safeFlush() []Event {
	out := p.finalizeState(finalizeSafe)
	for _, tok := range p.buf {
		out = append(out, Event{Type: TextEvent, Value: tok.Value})
	}
	p.Reset()
	return out
}

func (p *Parser) finalizeState(mode finalizeMode) []Event {
	var out []Event

	if mode != finalizeFlush && p.emphasisParser.depth() > 0 {
		out = append(out, p.emphasisParser.drain()...)
	}

	switch p.state {
	case HeaderState:
		if mode != finalizeFlush {
			out = append(out, Event{Type: HeaderEndEvent})
		}
	// BoldState/ItalicState/StrikethroughState removed — emphasis is handled
	// by the emphasis stack drain above, not as parser states.
	case InlineCodeState:
		if mode != finalizeFlush {
			out = append(out, Event{Type: InlineCodeEndEvent})
		}
	case CodeBlockState, IndentedCodeBlockState:
		if mode != finalizeFlush {
			out = append(out, Event{Type: CodeBlockEndEvent})
		}
	case BlockquoteState:
		if mode != finalizeFlush {
			out = append(out, Event{Type: BlockquoteEndEvent})
		}
	case TableBodyState:
		cells, ok := p.tableParser.flushBodyRow()
		if ok {
			out = append(out, Event{Type: TableRowEvent, Cells: cells})
		}
		out = append(out, Event{Type: TableEndEvent})
		if mode == finalizeFlush {
			p.state = NormalState
		}
	case TablePendingState:
		if len(p.tableParser.tableHeaderBuf) > 0 {
			out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableParser.tableHeaderBuf, " | ") + " |"})
			out = append(out, Event{Type: NewlineEvent})
		}
		p.tableParser.tableHeaderBuf = nil
		if mode == finalizeFlush {
			p.state = NormalState
		}
	case SetextPendingState:
		out = append(out, p.flushSetext()...)
		if mode == finalizeFlush {
			p.state = NormalState
		}
	case LinkTextState, LinkURLState:
		out = append(out, p.linkParser.flushLinkAsText()...)

	case HTMLBlockState:
		if mode != finalizeFlush {
			out = append(out, Event{Type: HTMLBlockEndEvent})
		}

	case LinkRefDefState:
		out = append(out, p.linkRefDefParser.flushLinkRefDef()...)
	}

	return out
}

func (p *Parser) flushSetext() []Event {
	var out []Event
	content := stripTrailingWhitespace(p.setextBuf)
	out = append(out, p.parseInlineLine(content)...)
	out = append(out, Event{Type: NewlineEvent})
	p.setextBuf = nil
	p.setextWaiting = false
	return out
}
