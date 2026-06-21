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
			p.boldOpener = 0
			p.state = NormalState
			return events
		}
		matched, waiting := p.checkConsecutive(p.boldOpener, 2)
		if matched {
			p.consume(2)
			p.boldOpener = 0
			p.state = NormalState
			events = append(events, Event{Type: BoldEndEvent})
			return events
		}
		if waiting {
			break
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
			p.italicOpener = 0
			p.state = NormalState
			return events
		}
		if tok.Type == p.italicOpener {
			p.consume(1)
			p.italicOpener = 0
			p.state = NormalState
			events = append(events, Event{Type: ItalicEndEvent})
			return events
		}
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.italicOpener = 0
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
		matched, waiting := p.checkConsecutive(tokenizer.TildeToken, 2)
		if matched {
			p.consume(2)
			p.state = NormalState
			events = append(events, Event{Type: StrikethroughEndEvent})
			return events
		}
		if waiting {
			break
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
		if tok.Type == tokenizer.BacktickToken {
			matched, waiting := p.checkConsecutive(tokenizer.BacktickToken, p.fenceLen)
			if matched {
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
			if waiting {
				break
			}
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

func (p *Parser) processLinkText() []Event {
	if len(p.buf) == 0 {
		return nil
	}

	if p.linkBracketConsumed {
		if p.buf[0].Type == tokenizer.LeftParenToken {
			p.consume(1)
			p.linkBracketConsumed = false
			p.state = LinkURLState
			return nil
		}
		return p.flushLinkAsText()
	}

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			return p.flushLinkAsText()
		}
		if tok.Type == tokenizer.RightBracketToken {
			p.consume(1)
			if len(p.buf) == 0 {
				p.linkBracketConsumed = true
				return nil
			}
			if p.buf[0].Type == tokenizer.LeftParenToken {
				p.consume(1)
				p.state = LinkURLState
				return nil
			}
			return p.flushLinkAsText()
		}
		p.consume(1)
		p.linkBuf = append(p.linkBuf, tok)
	}
	return nil
}

func (p *Parser) processLinkURL() []Event {
	if len(p.buf) == 0 {
		return nil
	}

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			return p.flushLinkAsText()
		}
		if tok.Type == tokenizer.RightParenToken {
			p.consume(1)
			var urlStr strings.Builder
			for _, t := range p.linkURLBuf {
				urlStr.WriteString(t.Value)
			}
			var textStr strings.Builder
			for _, t := range p.linkBuf {
				textStr.WriteString(t.Value)
			}
			p.linkBuf = nil
			p.linkURLBuf = nil
			p.linkBracketConsumed = false
			p.state = NormalState
			return []Event{{Type: LinkEvent, Value: textStr.String(), URL: urlStr.String()}}
		}
		p.consume(1)
		p.linkURLBuf = append(p.linkURLBuf, tok)
	}
	return nil
}

func (p *Parser) flushLinkAsText() []Event {
	var events []Event
	events = append(events, Event{Type: TextEvent, Value: "["})
	for _, tok := range p.linkBuf {
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
	}
	events = append(events, Event{Type: TextEvent, Value: "]"})
	if p.state == LinkURLState {
		events = append(events, Event{Type: TextEvent, Value: "("})
		for _, tok := range p.linkURLBuf {
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		}
		p.linkURLBuf = nil
	}
	p.linkBuf = nil
	p.linkBracketConsumed = false
	p.state = NormalState
	return events
}
