package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func (p *Parser) tryHeader() []Event {
	level := 0
	for level < len(p.buf) && p.buf[level].Type == tokenizer.HashToken {
		level++
	}

	if level == 0 || level > 6 {
		return nil
	}

	if level >= len(p.buf) {
		return nil
	}

	delim := p.buf[level]

	if delim.Type == tokenizer.NewlineToken {
		p.consume(level)
		p.headerLvl = level
		p.state = HeaderState
		return []Event{{Type: HeaderStartEvent, Level: level}}
	}

	if delim.Type == tokenizer.TabToken {
		p.consume(level + 1)
		p.headerLvl = level
		p.state = HeaderState
		p.contentIndent = tabRemainingEquiv(level)
		return []Event{{Type: HeaderStartEvent, Level: level}}
	}

	if delim.Type == tokenizer.TextToken && len(delim.Value) > 0 && delim.Value[0] == ' ' {
		p.consume(level)
		if len(delim.Value) == 1 {
			p.consume(1)
		} else {
			p.buf[0] = tokenizer.Token{Type: tokenizer.TextToken, Value: delim.Value[1:]}
		}
		p.headerLvl = level
		p.state = HeaderState
		return []Event{{Type: HeaderStartEvent, Level: level}}
	}

	return nil
}

func (p *Parser) tryBullet() []Event {
	if len(p.buf) < 2 {
		return nil
	}
	second := p.buf[1]
	if hasStructuralWhitespace(second) {
		p.consume(2)
		p.lineStart = false
		events := []Event{{Type: BulletItemEvent}}
		if second.Type == tokenizer.TabToken {
			p.contentIndent = tabRemainingEquiv(1)
		} else {
			trimmed := strings.TrimPrefix(second.Value, " ")
			if trimmed != "" {
				events = append(events, Event{Type: TextEvent, Value: trimmed})
			}
		}
		return events
	}
	p.consume(1)
	return []Event{{Type: TextEvent, Value: "-"}}
}

func (p *Parser) tryBulletOrBold() []Event {
	if len(p.buf) < 2 {
		return nil
	}
	second := p.buf[1]
	if hasStructuralWhitespace(second) {
		p.consume(2)
		p.lineStart = false
		events := []Event{{Type: BulletItemEvent}}
		if second.Type == tokenizer.TabToken {
			p.contentIndent = tabRemainingEquiv(1)
		} else if second.Type == tokenizer.TextToken {
			trimmed := strings.TrimPrefix(second.Value, " ")
			if trimmed != "" {
				events = append(events, Event{Type: TextEvent, Value: trimmed})
			}
		}
		return events
	}
	matched, waiting := p.checkConsecutive(tokenizer.StarToken, 2)
	if matched {
		if !hasMatchingCloser(p.buf[2:], tokenizer.StarToken, 2) && hasNewlineIn(p.buf[2:]) {
			p.consume(2)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "**"}}
		}
		p.consume(2)
		p.state = BoldState
		p.boldOpener = tokenizer.StarToken
		p.lineStart = false
		return []Event{{Type: BoldStartEvent}}
	}
	if waiting {
		return nil
	}
	if len(p.buf) >= 3 && p.buf[1].Type == tokenizer.TextToken &&
		!strings.HasPrefix(p.buf[1].Value, " ") && p.buf[2].Type == tokenizer.StarToken {
		p.consume(1)
		p.state = ItalicState
		p.italicOpener = tokenizer.StarToken
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}
	}
	p.consume(1)
	p.lineStart = false
	return []Event{{Type: TextEvent, Value: "*"}}
}

func (p *Parser) processHeader() []Event {
	if !p.hasNewline() {
		return nil
	}

	var content []tokenizer.Token
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			break
		}
		p.consume(1)
		content = append(content, tok)
	}

	content = stripLeadingWhitespaceTokens(content)
	content = stripTrailingWhitespaceTokens(content)
	content = stripClosingHashSequence(content)
	content = stripTrailingWhitespaceTokens(content)

	var events []Event
	if len(content) > 0 {
		events = append(events, p.parseInlineLine(content)...)
	}
	events = append(events, Event{Type: HeaderEndEvent})

	p.state = NormalState
	p.lineStart = true
	events = append(events, Event{Type: NewlineEvent})
	return events
}

func stripClosingHashSequence(tokens []tokenizer.Token) []tokenizer.Token {
	if len(tokens) == 0 {
		return tokens
	}

	hashCount := 0
	i := len(tokens) - 1
	for i >= 0 && tokens[i].Type == tokenizer.HashToken {
		if i > 0 && tokens[i-1].Type == tokenizer.BackslashToken {
			break
		}
		hashCount++
		i--
	}

	if hashCount == 0 {
		return tokens
	}

	if i < 0 {
		return nil
	}

	tok := tokens[i]
	if tok.Type == tokenizer.TabToken {
		return tokens[:i]
	}
	if tok.Type == tokenizer.TextToken && len(tok.Value) > 0 && tok.Value[len(tok.Value)-1] == ' ' {
		if len(tok.Value) == 1 {
			return tokens[:i]
		}
		tokens[i] = tokenizer.Token{Type: tokenizer.TextToken, Value: tok.Value[:len(tok.Value)-1]}
		return tokens[:i+1]
	}

	return tokens
}

func (p *Parser) processCodeBlock() []Event {
	var events []Event
	var infoBuf strings.Builder
	flushInfo := func() {
		if infoBuf.Len() > 0 {
			lang := strings.TrimSpace(resolveEntities(infoBuf.String()))
			if lang != "" {
				events = append(events, Event{Type: CodeBlockLangEvent, Value: lang})
			}
			infoBuf.Reset()
		}
	}
	for len(p.buf) > 0 {
		if p.lineStart && p.buf[0].Type == p.fenceChar {
			matched, waiting := p.checkConsecutive(p.fenceChar, p.fenceLen)
			if matched {
				n := p.countConsecutive(p.fenceChar)
				if n == len(p.buf) {
					break
				}
				p.consume(n)
				p.state = NormalState
				p.fenceLen = 0
				p.fenceChar = 0
				p.codeBlockFirst = false
				flushInfo()
				events = append(events, Event{Type: CodeBlockEndEvent})
				return events
			}
			if waiting {
				break
			}
		}
		tok := p.buf[0]
		p.consume(1)
		if tok.Type == tokenizer.NewlineToken {
			if p.codeBlockFirst {
				flushInfo()
				p.codeBlockFirst = false
			}
			p.lineStart = true
			events = append(events, Event{Type: NewlineEvent})
		} else if p.codeBlockFirst {
			if tok.Type == tokenizer.BackslashToken && len(p.buf) > 0 {
				next := p.buf[0]
				if len(next.Value) > 0 && isASCIIPunctByte(next.Value[0]) {
					p.consume(1)
					escaped := next.Value[:1]
					if len(next.Value) > 1 {
						p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: next.Value[1:]})
					}
					infoBuf.WriteString(escaped)
					continue
				}
			}
			if tok.Type == tokenizer.TextToken {
				infoBuf.WriteString(tok.Value)
				p.lineStart = false
				continue
			}
			if tok.Type == tokenizer.AmpersandToken {
				infoBuf.WriteString("&")
				p.lineStart = false
				continue
			}
			flushInfo()
			p.codeBlockFirst = false
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		} else if tok.Type == tokenizer.TextToken && strings.TrimSpace(tok.Value) == "" {
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		} else {
			p.codeBlockFirst = false
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		}
	}
	flushInfo()
	return events
}

func (p *Parser) processIndentedCodeBlock() []Event {
	var events []Event
	for len(p.buf) > 0 {
		if !p.lineStart {
			tok := p.buf[0]
			p.consume(1)
			if tok.Type == tokenizer.NewlineToken {
				p.lineStart = true
				events = append(events, Event{Type: NewlineEvent})
				return events
			}
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
			continue
		}
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			events = append(events, Event{Type: NewlineEvent})
			continue
		}
		if ok, remaining := p.equivIndent(); ok {
			p.lineStart = false
			if remaining != "" {
				events = append(events, Event{Type: TextEvent, Value: remaining})
			}
			continue
		}
		p.state = NormalState
		events = append(events, Event{Type: CodeBlockEndEvent})
		return events
	}
	return events
}

func (p *Parser) tryBlockquote() []Event {
	if len(p.buf) < 2 {
		return nil
	}
	p.consume(1)
	if len(p.buf) > 0 && hasStructuralWhitespace(p.buf[0]) {
		tok := p.buf[0]
		p.consume(1)
		p.state = BlockquoteState
		p.lineStart = false
		events := []Event{{Type: BlockquoteStartEvent}}
		if tok.Type == tokenizer.TabToken {
			p.contentIndent = tabRemainingEquiv(1)
		} else if tok.Type == tokenizer.TextToken {
			trimmed := strings.TrimPrefix(tok.Value, " ")
			if trimmed != "" {
				events = append(events, Event{Type: TextEvent, Value: trimmed})
			}
		}
		return events
	}
	p.state = BlockquoteState
	p.lineStart = false
	events := []Event{{Type: BlockquoteStartEvent}}
	if len(p.buf) > 0 && p.buf[0].Type == tokenizer.NewlineToken {
		return events
	}
	return events
}

func (p *Parser) processBlockquote() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.lineStart = true
			events = append(events, Event{Type: NewlineEvent})
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.GreaterToken {
				p.consume(1)
				if len(p.buf) > 0 && hasStructuralWhitespace(p.buf[0]) {
					tok := p.buf[0]
					p.consume(1)
					p.lineStart = false
					if tok.Type == tokenizer.TabToken {
						p.contentIndent = tabRemainingEquiv(1)
					} else if tok.Type == tokenizer.TextToken {
						val := strings.TrimPrefix(tok.Value, " ")
						if val != "" {
							events = append(events, Event{Type: TextEvent, Value: val})
						}
					}
				} else {
					p.lineStart = false
				}
				return events
			}
			p.state = NormalState
			events = append(events, Event{Type: BlockquoteEndEvent})
			return events
		}
		p.consume(1)
		p.lineStart = false
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
	}
	return events
}

func (p *Parser) processSetextPending() []Event {
	if !p.setextWaiting {
		if !p.hasNewline() {
			return nil
		}
		p.setextBuf = p.collectLineTokens()
		p.setextWaiting = true
		return nil
	}

	if !p.hasNewline() {
		return nil
	}

	level, ok := p.checkSetextUnderline()
	if ok {
		if !hasTextContent(p.setextBuf) {
			events := p.parseInlineLine(p.setextBuf)
			events = append(events, Event{Type: NewlineEvent})
			p.setextBuf = nil
			p.setextWaiting = false
			p.state = NormalState
			return events
		}
		var events []Event
		events = append(events, Event{Type: HeaderStartEvent, Level: level})
		events = append(events, p.parseInlineLine(p.setextBuf)...)
		events = append(events, Event{Type: HeaderEndEvent})
		events = append(events, Event{Type: NewlineEvent})
		p.setextBuf = nil
		p.setextWaiting = false
		p.state = NormalState
		return events
	}

	events := p.parseInlineLine(p.setextBuf)
	events = append(events, Event{Type: NewlineEvent})
	p.setextBuf = nil
	p.setextWaiting = false
	p.state = NormalState
	return events
}

func (p *Parser) collectLineTokens() []tokenizer.Token {
	var tokens []tokenizer.Token
	for len(p.buf) > 0 {
		tok := p.buf[0]
		p.consume(1)
		if tok.Type == tokenizer.NewlineToken {
			p.lineStart = true
			return tokens
		}
		tokens = append(tokens, tok)
		p.lineStart = false
	}
	return tokens
}

func (p *Parser) parseInlineLine(tokens []tokenizer.Token) []Event {
	if len(tokens) == 0 {
		return nil
	}
	tmp := New()
	tmp.lineStart = false
	events := tmp.Parse(tokens)
	events = append(events, tmp.CloseStates()...)
	return events
}

func hasTextContent(tokens []tokenizer.Token) bool {
	for _, tok := range tokens {
		if tok.Type == tokenizer.TextToken {
			for _, r := range tok.Value {
				if r != ' ' && r != '\t' {
					return true
				}
			}
		} else {
			return true
		}
	}
	return false
}

func (p *Parser) checkSetextUnderline() (int, bool) {
	if len(p.buf) < 2 {
		return 0, false
	}

	first := p.buf[0]

	if first.Type == tokenizer.TextToken {
		t := strings.TrimRight(first.Value, " \t")
		if len(t) >= 3 {
			allEq := true
			for _, ch := range t {
				if ch != '=' {
					allEq = false
					break
				}
			}
			if allEq {
				p.consume(1)
				if len(p.buf) > 0 && p.buf[0].Type == tokenizer.NewlineToken {
					p.consume(1)
					p.lineStart = true
					return 1, true
				}
			}
		}
	}

	if first.Type == tokenizer.DashToken {
		matched, waiting := p.checkConsecutive(tokenizer.DashToken, 3)
		if waiting {
			return 0, false
		}
		if matched {
			n := p.countConsecutive(tokenizer.DashToken)
			p.consume(n)
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.TextToken && strings.TrimSpace(p.buf[0].Value) == "" {
				p.consume(1)
			}
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.NewlineToken {
				p.consume(1)
				p.lineStart = true
				return 2, true
			}
		}
	}

	return 0, false
}
