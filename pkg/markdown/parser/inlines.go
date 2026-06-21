package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func (p *Parser) processBold() []Event {
	var events []Event
	for len(p.buf) > 0 {
		if p.lineStart && p.isBlockLevel(p.buf[0].Type) {
			events = append(events, Event{Type: BoldEndEvent})
			p.state = NormalState
			return events
		}
		if p.hasConsecutive(tokenizer.StarToken, 2) {
			p.consume(2)
			p.state = NormalState
			events = append(events, Event{Type: BoldEndEvent})
			return events
		}
		tok := p.buf[0]
		p.consume(1)
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		if tok.Type == tokenizer.NewlineToken {
			p.lineStart = true
		} else {
			p.lineStart = false
		}
	}
	return events
}

func (p *Parser) processItalic() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if p.lineStart && p.isBlockLevel(tok.Type) {
			events = append(events, Event{Type: ItalicEndEvent})
			p.state = NormalState
			return events
		}
		if tok.Type == tokenizer.StarToken {
			p.consume(1)
			p.state = NormalState
			events = append(events, Event{Type: ItalicEndEvent})
			return events
		}
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.state = NormalState
			p.lineStart = true
			events = append(events, Event{Type: ItalicEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		}
		p.consume(1)
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		p.lineStart = false
	}
	return events
}

func (p *Parser) processStrikethrough() []Event {
	var events []Event
	for len(p.buf) > 0 {
		if p.lineStart && p.isBlockLevel(p.buf[0].Type) {
			events = append(events, Event{Type: StrikethroughEndEvent})
			p.state = NormalState
			return events
		}
		if p.hasConsecutive(tokenizer.TildeToken, 2) {
			p.consume(2)
			p.state = NormalState
			events = append(events, Event{Type: StrikethroughEndEvent})
			return events
		}
		tok := p.buf[0]
		p.consume(1)
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		if tok.Type == tokenizer.NewlineToken {
			p.lineStart = true
		}
	}
	return events
}

func (p *Parser) processInlineCode() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.BacktickToken && p.hasConsecutive(tokenizer.BacktickToken, p.fenceLen) {
			p.consume(p.fenceLen)
			p.state = NormalState
			p.fenceLen = 0
			if len(events) > 0 {
				first := events[0]
				last := events[len(events)-1]
				if first.Type == TextEvent && strings.HasPrefix(first.Value, " ") {
					first.Value = strings.TrimPrefix(first.Value, " ")
				}
				if last.Type == TextEvent && strings.HasSuffix(last.Value, " ") {
					last.Value = strings.TrimSuffix(last.Value, " ")
				}
			}
			events = append(events, Event{Type: InlineCodeEndEvent})
			return events
		}
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.state = NormalState
			p.lineStart = true
			p.fenceLen = 0
			events = append(events, Event{Type: InlineCodeEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		}
		p.consume(1)
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		p.lineStart = false
	}
	return events
}
