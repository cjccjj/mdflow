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
		if p.buf[0].Type == tokenizer.LeftBracketToken {
			p.linkBracketConsumed = false
			return p.processLinkRefLabel()
		}
		return p.flushLinkAsText()
	}

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			return p.flushLinkAsText()
		}
		if tok.Type == tokenizer.LeftBracketToken {
			p.consume(1)
			p.linkBuf = append(p.linkBuf, tok)
			p.linkDepth++
			continue
		}
		if tok.Type == tokenizer.BackslashToken {
			p.consume(1)
			if len(p.buf) > 0 {
				next := p.buf[0]
				if next.Type == tokenizer.BackslashToken {
					p.consume(1)
					p.linkBuf = append(p.linkBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
					continue
				}
				if next.Type == tokenizer.LeftBracketToken || next.Type == tokenizer.RightBracketToken {
					p.consume(1)
					p.linkBuf = append(p.linkBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: next.Value})
					continue
				}
			}
			p.linkBuf = append(p.linkBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
			continue
		}
		if tok.Type == tokenizer.RightBracketToken {
			if p.linkDepth > 0 {
				p.consume(1)
				p.linkBuf = append(p.linkBuf, tok)
				p.linkDepth--
				if p.linkDepth == 0 && len(p.buf) > 0 {
					if p.buf[0].Type == tokenizer.LeftParenToken {
						p.consume(1)
						p.state = LinkURLState
						return nil
					}
					if p.buf[0].Type == tokenizer.LeftBracketToken {
						return p.processLinkRefLabel()
					}
				}
				continue
			}
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
			if p.buf[0].Type == tokenizer.LeftBracketToken {
				return p.processLinkRefLabel()
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
			if p.urlHadNewline || p.urlAngleBracket {
				return p.flushLinkAsText()
			}
			p.consume(1)
			p.urlHadNewline = true
			continue
		}

		if p.urlDone {
			if tok.Type == tokenizer.TabToken || (tok.Type == tokenizer.TextToken && isPureWhitespaceToken(tok)) {
				p.consume(1)
				continue
			}
			if tok.Type == tokenizer.RightParenToken {
				p.consume(1)
				return p.emitLinkEvent()
			}
			if tok.Type == tokenizer.LeftParenToken {
				return p.collectTitle('(')
			}
			if tok.Type == tokenizer.TextToken && len(tok.Value) > 0 {
				q := tok.Value[0]
				if q == '"' || q == '\'' || q == '(' {
					return p.collectTitle(q)
				}
			}
			return p.flushLinkAsText()
		}

		if p.urlAngleBracket {
			if tok.Type == tokenizer.BackslashToken {
				p.consume(1)
				if len(p.buf) > 0 && p.buf[0].Type == tokenizer.GreaterToken {
					p.consume(1)
					p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\>"})
				} else {
					p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
				}
				continue
			}
			if tok.Type == tokenizer.GreaterToken {
				p.consume(1)
				p.urlAngleBracket = false
				p.urlDone = true
				if len(p.linkURLBuf) > 0 {
					first := p.linkURLBuf[0]
					if first.Type == tokenizer.TextToken && len(first.Value) > 0 && first.Value[0] == '<' {
						if len(first.Value) > 1 {
							p.linkURLBuf[0].Value = first.Value[1:]
						} else {
							p.linkURLBuf = p.linkURLBuf[1:]
						}
					}
				}
				continue
			}
			p.consume(1)
			p.linkURLBuf = append(p.linkURLBuf, tok)
			continue
		}

		if len(p.linkURLBuf) == 0 {
			if tok.Type == tokenizer.TabToken {
				p.consume(1)
				continue
			}
			if tok.Type == tokenizer.TextToken && isPureWhitespaceToken(tok) {
				p.consume(1)
				continue
			}
			if tok.Type == tokenizer.TextToken && len(tok.Value) > 0 && tok.Value[0] == '<' {
				p.urlAngleBracket = true
				p.consume(1)
				p.linkURLBuf = append(p.linkURLBuf, tok)
				continue
			}
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

		if tok.Type == tokenizer.LeftParenToken {
			p.consume(1)
			p.linkURLBuf = append(p.linkURLBuf, tok)
			p.urlParenDepth++
			continue
		}

		if tok.Type == tokenizer.RightParenToken {
			if p.urlParenDepth > 0 {
				p.consume(1)
				p.linkURLBuf = append(p.linkURLBuf, tok)
				p.urlParenDepth--
				continue
			}
			p.consume(1)
			return p.emitLinkEvent()
		}

		if tok.Type == tokenizer.TabToken || (tok.Type == tokenizer.TextToken && isPureWhitespaceToken(tok)) {
			p.consume(1)
			p.urlDone = true
			for len(p.buf) > 0 {
				nt := p.buf[0]
				if nt.Type == tokenizer.TabToken || (nt.Type == tokenizer.TextToken && isPureWhitespaceToken(nt)) {
					p.consume(1)
				} else if nt.Type == tokenizer.NewlineToken {
					if p.urlHadNewline {
						break
					}
					p.consume(1)
					p.urlHadNewline = true
				} else {
					break
				}
			}
			continue
		}

		if tok.Type == tokenizer.TextToken {
			spaceIdx := -1
			for i, r := range tok.Value {
				if r == ' ' || r == '\t' || (r >= 0x00 && r <= 0x1f) || r == 0x7f {
					spaceIdx = i
					break
				}
			}
			if spaceIdx >= 0 {
				rest := strings.TrimLeft(tok.Value[spaceIdx:], " \t")
				if spaceIdx == 0 && len(p.linkURLBuf) == 0 {
					if len(rest) > 0 && rest[0] == '<' {
						p.urlAngleBracket = true
						p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: rest})
					} else if rest != "" {
						p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: rest})
					}
					p.consume(1)
					continue
				}
				if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'' || rest[0] == '(') {
					if spaceIdx > 0 {
						p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: tok.Value[:spaceIdx]})
					}
					p.consume(1)
					p.urlDone = true
					p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: rest})
					p.skipURLWhitespace()
					continue
				}
				if rest != "" {
					return p.flushLinkAsText()
				}
				if spaceIdx > 0 {
					p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: tok.Value[:spaceIdx]})
				}
				p.consume(1)
				p.urlDone = true
				p.skipURLWhitespace()
				continue
			}
		}
		p.consume(1)
		p.linkURLBuf = append(p.linkURLBuf, tok)
	}
	return nil
}

func (p *Parser) collectTitle(q byte) []Event {
	tok := p.buf[0]
	p.consume(1)

	var titleBuilder strings.Builder

	closeQ := q
	if q == '(' {
		closeQ = ')'
	}

	if tok.Type == tokenizer.TextToken && len(tok.Value) > 1 {
		rest := tok.Value[1:]
		closeIdx := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				i++
				continue
			}
			if rest[i] == closeQ {
				closeIdx = i
				break
			}
		}
		if closeIdx >= 0 {
			titleBuilder.WriteString(rest[:closeIdx])
			trail := rest[closeIdx+1:]
			if trail != "" {
				p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: trail})
			}
			p.urlDone = true
			title := resolveEntities(titleBuilder.String())
			p.skipURLWhitespace()
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.RightParenToken {
				p.consume(1)
				events := p.emitLinkEvent()
				if len(events) > 0 {
					events[0].Title = title
				}
				return events
			}
			return p.flushLinkAsText()
		}
		titleBuilder.WriteString(rest)
	} else {
		// Single-char token opener (e.g., '"', '\'', '(' as separate token) — already consumed
	}

	for len(p.buf) > 0 {
		t := p.buf[0]
		if t.Type == tokenizer.NewlineToken {
			if p.urlHadNewline {
				return p.flushLinkAsText()
			}
			p.consume(1)
			p.urlHadNewline = true
			continue
		}
		if t.Type == tokenizer.BackslashToken {
			p.consume(1)
			if len(p.buf) > 0 {
				next := p.buf[0]
				if len(next.Value) > 0 {
					nextByte := next.Value[0]
					if q == '(' {
						if nextByte == closeQ {
							p.consume(1)
							titleBuilder.WriteByte(closeQ)
							continue
						}
					} else if next.Type == tokenizer.TextToken && nextByte == closeQ {
						p.consume(1)
						titleBuilder.WriteByte(closeQ)
						continue
					}
				}
			}
			titleBuilder.WriteByte('\\')
			continue
		}
		p.consume(1)
		consumed := false
		if q == '(' {
			if t.Type == tokenizer.RightParenToken {
				consumed = true
			}
		} else if t.Type == tokenizer.TextToken && len(t.Value) > 0 && t.Value[0] == closeQ {
			consumed = true
			if len(t.Value) > 1 {
				p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: t.Value[1:]})
			}
		}
		if consumed {
			break
		}
		titleBuilder.WriteString(t.Value)
	}

	p.urlDone = true
	title := resolveEntities(titleBuilder.String())
	p.skipURLWhitespace()
	if len(p.buf) > 0 && p.buf[0].Type == tokenizer.RightParenToken {
		p.consume(1)
		events := p.emitLinkEvent()
		if len(events) > 0 {
			events[0].Title = title
		}
		return events
	}

	return p.flushLinkAsText()
}

func (p *Parser) skipURLWhitespace() {
	for len(p.buf) > 0 {
		nt := p.buf[0]
		if nt.Type == tokenizer.TabToken || (nt.Type == tokenizer.TextToken && isPureWhitespaceToken(nt)) {
			p.consume(1)
		} else if nt.Type == tokenizer.NewlineToken {
			if p.urlHadNewline {
				break
			}
			p.consume(1)
			p.urlHadNewline = true
		} else {
			break
		}
	}
}

func (p *Parser) emitLinkEvent() []Event {
	var textBuilder strings.Builder
	for _, t := range p.linkBuf {
		textBuilder.WriteString(t.Value)
	}
	var urlBuilder strings.Builder
	for _, t := range p.linkURLBuf {
		urlBuilder.WriteString(t.Value)
	}
	textStr := resolveEntities(textBuilder.String())
	urlStr := resolveEntities(urlBuilder.String())

	p.linkBuf = nil
	p.linkURLBuf = nil
	p.linkBracketConsumed = false
	p.linkDepth = 0
	p.urlParenDepth = 0
	p.urlAngleBracket = false
	p.urlDone = false
	p.urlHadNewline = false
	p.linkTitleBuf = nil
	p.state = NormalState
	if len(textStr) > 0 {
		p.prevChar = textStr[len(textStr)-1]
	}
	return []Event{{Type: LinkEvent, Value: textStr, URL: urlStr}}
}

func (p *Parser) processLinkRefLabel() []Event {
	p.consume(1)

	var refLabel strings.Builder
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.prependTokens(tokenizer.Token{Type: tokenizer.LeftBracketToken, Value: "["})
			return p.flushLinkAsText()
		}
		if tok.Type == tokenizer.RightBracketToken {
			p.consume(1)
			var textBuilder strings.Builder
			for _, t := range p.linkBuf {
				textBuilder.WriteString(t.Value)
			}
			textStr := resolveEntities(textBuilder.String())
			labelStr := resolveEntities(refLabel.String())
			p.linkBuf = nil
			p.linkBracketConsumed = false
			p.state = NormalState
			if len(textStr) > 0 {
				p.prevChar = textStr[len(textStr)-1]
			}
			return []Event{{Type: LinkRefEvent, Value: textStr, URL: labelStr}}
		}
		p.consume(1)
		refLabel.WriteString(tok.Value)
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
	wasLinkURL := p.state == LinkURLState
	p.state = NormalState
	if wasLinkURL {
		p.prevChar = ')'
	} else {
		p.prevChar = ']'
	}
	return events
}
