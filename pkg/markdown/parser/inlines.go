package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// processBold, processItalic, processStrikethrough removed (dead code).
// Emphasis is now handled entirely by emphasisParser via the emphasis stack.

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
