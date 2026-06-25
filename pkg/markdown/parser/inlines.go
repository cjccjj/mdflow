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
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
			p.lineStart = true
			continue
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
					contentOnlySpaces := true
					for _, e := range events {
						if e.Type == TextEvent {
							for _, r := range e.Value {
								if r != ' ' {
									contentOnlySpaces = false
									break
								}
							}
						} else {
							contentOnlySpaces = false
							break
						}
						if !contentOnlySpaces {
							break
						}
					}
					if !contentOnlySpaces {
						if events[0].Type == TextEvent && strings.HasPrefix(events[0].Value, " ") {
							events[0].Value = strings.TrimPrefix(events[0].Value, " ")
						}
						lastIdx := len(events) - 1
						if events[lastIdx].Type == TextEvent && strings.HasSuffix(events[lastIdx].Value, " ") {
							events[lastIdx].Value = strings.TrimSuffix(events[lastIdx].Value, " ")
						}
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
			var urlBuilder strings.Builder
			for _, t := range p.linkURLBuf {
				urlBuilder.WriteString(t.Value)
			}
			var textBuilder strings.Builder
			for _, t := range p.linkBuf {
				textBuilder.WriteString(t.Value)
			}
			urlStr := resolveEntities(urlBuilder.String())
			textStr := resolveEntities(textBuilder.String())
			p.linkBuf = nil
			p.linkURLBuf = nil
			p.linkBracketConsumed = false
			p.state = NormalState
			return []Event{{Type: LinkEvent, Value: textStr, URL: urlStr}}
		}
		if tok.Type == tokenizer.BackslashToken {
			p.consume(1)
			if len(p.buf) > 0 {
				next := p.buf[0]
				if len(next.Value) > 0 && isASCIIPunctByte(next.Value[0]) {
					escaped := next.Value[:1]
					if len(next.Value) > 1 {
						p.buf[0].Value = next.Value[1:]
					} else {
						p.consume(1)
					}
					p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: escaped})
					continue
				}
			}
			p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
			continue
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
		events = append(events, Event{Type: TextEvent, Value: resolveEntities(tok.Value)})
	}
	events = append(events, Event{Type: TextEvent, Value: "]"})
	if p.state == LinkURLState {
		events = append(events, Event{Type: TextEvent, Value: "("})
		for _, tok := range p.linkURLBuf {
			events = append(events, Event{Type: TextEvent, Value: resolveEntities(tok.Value)})
		}
		p.linkURLBuf = nil
	}
	p.linkBuf = nil
	p.linkBracketConsumed = false
	p.state = NormalState
	return events
}
