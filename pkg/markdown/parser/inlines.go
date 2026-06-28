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
	prevWasBacktick := false
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.BacktickToken {
			matched, waiting := p.checkConsecutive(tokenizer.BacktickToken, p.fenceLen)
			if matched {
				if prevWasBacktick || (p.fenceLen < len(p.buf) && p.buf[p.fenceLen].Type == tokenizer.BacktickToken) {
					p.consume(1)
					events = append(events, Event{Type: TextEvent, Value: "`"})
					prevWasBacktick = true
					continue
				}
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
						lastIdx := len(events) - 1
						hasLeadingSpace := events[0].Type == TextEvent && strings.HasPrefix(events[0].Value, " ")
						hasTrailingSpace := events[lastIdx].Type == TextEvent && strings.HasSuffix(events[lastIdx].Value, " ")
						if hasLeadingSpace && hasTrailingSpace {
							events[0].Value = strings.TrimPrefix(events[0].Value, " ")
							events[lastIdx].Value = strings.TrimSuffix(events[lastIdx].Value, " ")
						}
					}
				}
				events = append(events, Event{Type: InlineCodeEndEvent})
				if len(events) > 1 {
					last := events[len(events)-2]
					if last.Type == TextEvent && len(last.Value) > 0 {
						p.prevChar = last.Value[len(last.Value)-1]
					}
				}
				return events
			}
			if waiting {
				break
			}
		}
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			prevWasBacktick = false
			events = append(events, Event{Type: TextEvent, Value: " "})
			p.lineStart = true
			continue
		}
		p.consume(1)
		prevWasBacktick = false
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		p.lineStart = false
	}
	return events
}

// link-related methods (processLinkText, processLinkURL, processLinkRefLabel,
// flushLinkAsText, emitLinkEvent, collectTitle, skipURLWhitespace) moved to link_parser.go
