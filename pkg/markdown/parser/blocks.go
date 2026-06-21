package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func (p *Parser) tryHeader() []Event {
	matched, waiting := p.checkConsecutive(tokenizer.HashToken, 1)
	if waiting || (matched && len(p.buf) < 2) {
		return nil
	}
	if !matched {
		return nil
	}

	level := 0
	for i, tok := range p.buf {
		if tok.Type == tokenizer.HashToken {
			level++
		} else {
			if i > 0 && tok.Type == tokenizer.TextToken && strings.HasPrefix(tok.Value, " ") {
				p.headerLvl = level
				p.consume(i + 1)
				trimmed := strings.TrimPrefix(tok.Value, " ")
				p.state = HeaderState
				events := []Event{{Type: HeaderStartEvent, Level: level}}
				if trimmed != "" {
					events = append(events, Event{Type: TextEvent, Value: trimmed})
				}
				return events
			}
			return p.emitHashAsText(level, i)
		}
		if level > 6 {
			remaining := i + 1
			if remaining < len(p.buf) && p.buf[remaining].Type == tokenizer.TextToken && strings.HasPrefix(p.buf[remaining].Value, " ") {
				return p.emitHashAsText(level, remaining+1)
			}
			return p.emitHashAsText(level, remaining)
		}
	}

	return nil
}

func (p *Parser) emitHashAsText(level, idx int) []Event {
	p.consume(idx)
	var events []Event
	for i := 0; i < level; i++ {
		events = append(events, Event{Type: TextEvent, Value: "#"})
	}
	return events
}

func (p *Parser) tryBullet() []Event {
	if len(p.buf) < 2 {
		return nil
	}
	second := p.buf[1]
	if second.Type == tokenizer.TextToken && strings.HasPrefix(second.Value, " ") {
		trimmed := strings.TrimPrefix(second.Value, " ")
		p.consume(2)
		p.lineStart = false
		events := []Event{{Type: BulletItemEvent}}
		if trimmed != "" {
			events = append(events, Event{Type: TextEvent, Value: trimmed})
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
	if p.buf[1].Type == tokenizer.TextToken && strings.HasPrefix(p.buf[1].Value, " ") {
		trimmed := strings.TrimPrefix(p.buf[1].Value, " ")
		p.consume(2)
		p.lineStart = false
		events := []Event{{Type: BulletItemEvent}}
		if trimmed != "" {
			events = append(events, Event{Type: TextEvent, Value: trimmed})
		}
		return events
	}
	matched, waiting := p.checkConsecutive(tokenizer.StarToken, 2)
	if matched {
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
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		switch tok.Type {
		case tokenizer.NewlineToken:
			p.consume(1)
			p.state = NormalState
			p.lineStart = true
			events = append(events, Event{Type: HeaderEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		default:
			p.consume(1)
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		}
	}
	return events
}

func (p *Parser) processCodeBlock() []Event {
	var events []Event
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
			p.codeBlockFirst = false
			p.lineStart = true
			events = append(events, Event{Type: NewlineEvent})
		} else if tok.Type == tokenizer.TextToken && p.codeBlockFirst && strings.TrimSpace(tok.Value) != "" {
			p.codeBlockFirst = false
			events = append(events, Event{Type: CodeBlockLangEvent, Value: tok.Value})
		} else if tok.Type == tokenizer.TextToken && strings.TrimSpace(tok.Value) == "" {
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		} else {
			p.codeBlockFirst = false
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		}
	}
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
		if tok.Type == tokenizer.TabToken {
			p.consume(1)
			p.lineStart = false
			continue
		}
		if tok.Type == tokenizer.TextToken && hasLeadingIndent(tok.Value) {
			val := stripIndent(tok.Value)
			p.consume(1)
			p.lineStart = false
			if val != "" {
				events = append(events, Event{Type: TextEvent, Value: val})
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
	if len(p.buf) > 0 && p.buf[0].Type == tokenizer.TextToken && strings.HasPrefix(p.buf[0].Value, " ") {
		val := p.buf[0].Value
		p.consume(1)
		p.state = BlockquoteState
		p.lineStart = false
		events := []Event{{Type: BlockquoteStartEvent}}
		trimmed := strings.TrimPrefix(val, " ")
		if trimmed != "" {
			events = append(events, Event{Type: TextEvent, Value: trimmed})
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
				if len(p.buf) > 0 && p.buf[0].Type == tokenizer.TextToken && strings.HasPrefix(p.buf[0].Value, " ") {
					val := strings.TrimPrefix(p.buf[0].Value, " ")
					p.consume(1)
					p.lineStart = false
					if val != "" {
						events = append(events, Event{Type: TextEvent, Value: val})
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
