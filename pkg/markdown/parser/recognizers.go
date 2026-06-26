package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

type Recognizer func(p *Parser) (events []Event, handled bool)

func (p *Parser) enterState(state State) {
	p.state = state
}

func (p *Parser) exitToNormal() {
	p.state = NormalState
}

// tryThematicBreak checks for a horizontal rule using all three marker types.
func (p *Parser) tryThematicBreak() ([]Event, bool) {
	for _, marker := range []tokenizer.TokenType{tokenizer.DashToken, tokenizer.StarToken, tokenizer.UnderscoreToken} {
		matched, waiting := p.checkHorizontalRule(marker)
		if matched {
			return []Event{{Type: HorizontalRuleEvent}}, true
		}
		if waiting {
			return nil, true
		}
	}
	return nil, false
}

// tryStarEmphasis handles *-based bold and italic.
func (p *Parser) tryStarEmphasis() ([]Event, bool) {
	matched, waiting := p.checkConsecutive(tokenizer.StarToken, 2)
	if matched {
		if !hasMatchingCloser(p.buf[2:], tokenizer.StarToken, 2, true) && hasNewlineIn(p.buf[2:]) {
			p.consume(2)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "**"}}, true
		}
		p.consume(2)
		p.enterState(BoldState)
		p.boldOpener = tokenizer.StarToken
		return []Event{{Type: BoldStartEvent}}, true
	}
	if waiting {
		return nil, true
	}

	if !hasMatchingCloser(p.buf[1:], tokenizer.StarToken, 1, true) {
		p.consume(1)
		p.lineStart = false
		return []Event{{Type: TextEvent, Value: "*"}}, true
	}
	p.consume(1)
	p.enterState(ItalicState)
	p.italicOpener = tokenizer.StarToken
	return []Event{{Type: ItalicStartEvent}}, true
}

// tryUnderscoreEmphasis handles _-based bold and italic.
func (p *Parser) tryUnderscoreEmphasis() ([]Event, bool) {
	matched, waiting := p.checkConsecutive(tokenizer.UnderscoreToken, 2)
	if matched {
		if !hasMatchingCloser(p.buf[2:], tokenizer.UnderscoreToken, 2, true) && hasNewlineIn(p.buf[2:]) {
			p.consume(2)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "__"}}, true
		}
		p.consume(2)
		p.enterState(BoldState)
		p.boldOpener = tokenizer.UnderscoreToken
		return []Event{{Type: BoldStartEvent}}, true
	}
	if waiting {
		return nil, true
	}

	if !hasMatchingCloser(p.buf[1:], tokenizer.UnderscoreToken, 1, true) {
		p.consume(1)
		p.lineStart = false
		return []Event{{Type: TextEvent, Value: "_"}}, true
	}
	if !isLeftFlanking(p.buf) {
		p.consume(1)
		p.lineStart = false
		return []Event{{Type: TextEvent, Value: "_"}}, true
	}
	p.consume(1)
	p.enterState(ItalicState)
	p.italicOpener = tokenizer.UnderscoreToken
	return []Event{{Type: ItalicStartEvent}}, true
}

// tryOrderedList checks for an ordered list prefix (like "1. " or "2) ").
func (p *Parser) tryOrderedList() ([]Event, bool) {
	if p.buf[0].Type != tokenizer.TextToken {
		return nil, false
	}
	fullValue := p.buf[0].Value
	prefix, ok := orderedListPrefix(fullValue)
	if !ok {
		return nil, false
	}
	p.consume(1)
	p.lineStart = false
	events := []Event{{Type: BulletItemEvent, Value: prefix}}
	rest := fullValue[len(prefix):]
	if strings.HasPrefix(rest, "    ") {
		p.enterState(IndentedCodeBlockState)
		codeContent := rest[4:]
		events = append(events, Event{Type: CodeBlockStartEvent})
		if codeContent != "" {
			events = append(events, Event{Type: TextEvent, Value: codeContent})
		}
		return events, true
	}
	if rest != "" {
		events = append(events, Event{Type: TextEvent, Value: rest})
	}
	return events, true
}

// tryIndentedCodeOrList handles 4+ spaces of indentation.
func (p *Parser) tryIndentedCodeOrList() ([]Event, bool) {
	satisfied, consumeCount, remaining := p.peekEquivIndent()
	if !satisfied {
		return nil, false
	}
	if isListStartAfterIndent(p.buf[consumeCount:], remaining) {
		p.consume(consumeCount)
		p.enterState(IndentedCodeBlockState)
		p.lineStart = false
		events := []Event{{Type: CodeBlockStartEvent}}
		if remaining != "" {
			events = append(events, Event{Type: TextEvent, Value: remaining})
		}
		return events, true
	}
	p.consume(consumeCount)
	p.enterState(IndentedCodeBlockState)
	p.lineStart = false
	events := []Event{{Type: CodeBlockStartEvent}}
	if remaining != "" {
		events = append(events, Event{Type: TextEvent, Value: remaining})
	}
	return events, true
}

// trySetextCandidate buffers a text line as a potential setext heading.
func (p *Parser) trySetextCandidate() ([]Event, bool) {
	if p.buf[0].Type != tokenizer.TextToken {
		return nil, false
	}
	if !p.hasNewline() {
		return nil, false
	}
	p.enterState(SetextPendingState)
	return nil, true
}

// tryBacktickInline handles inline code via backticks.
func (p *Parser) tryBacktickInline() ([]Event, bool) {
	return p.processBacktickStart()
}

// tryTildeInline handles strikethrough via tildes.
func (p *Parser) tryTildeInline() ([]Event, bool) {
	return p.processTildeStart()
}

// tryEscapeOrEntity handles backslash escapes and & entities.
func (p *Parser) tryEscapeOrEntity() ([]Event, bool) {
	first := p.buf[0]
	if first.Type == tokenizer.BackslashToken {
		return p.handleBackslash(), true
	}
	if first.Type == tokenizer.AmpersandToken {
		return p.handleEntity(), true
	}
	return nil, false
}

// tryATXHeading handles all ATX heading cases.
func (p *Parser) tryATXHeading() ([]Event, bool) {
	first := p.buf[0]

	if first.Type == tokenizer.HashToken {
		hashCount := 0
		for hashCount < len(p.buf) && p.buf[hashCount].Type == tokenizer.HashToken {
			hashCount++
		}
		if hashCount > 6 {
			p.consume(hashCount)
			p.lineStart = false
			events := make([]Event, hashCount)
			for i := 0; i < hashCount; i++ {
				events[i] = Event{Type: TextEvent, Value: "#"}
			}
			return events, true
		}
		events := p.tryHeader()
		return events, events != nil
	}

	if first.Type == tokenizer.TextToken && len(p.buf) > 1 {
		if sp := leadingSpaceCount(first.Value); sp >= 1 && sp <= 3 && p.buf[1].Type == tokenizer.HashToken {
			p.consume(1)
			events := p.tryHeader()
			return events, events != nil
		}
	}

	return nil, false
}

// tryBlockquoteR adapts tryBlockquote for the Recognizer signature.
func (p *Parser) tryBlockquoteR() ([]Event, bool) {
	events := p.tryBlockquote()
	return events, events != nil
}

// tryBulletDash adapts tryBullet for the Recognizer signature.
func (p *Parser) tryBulletDash() ([]Event, bool) {
	events := p.tryBullet()
	return events, events != nil
}

// tryBulletStarOrOrdered adapts tryBulletOrBold for the Recognizer signature.
func (p *Parser) tryBulletStarOrOrdered() ([]Event, bool) {
	events := p.tryBulletOrBold()
	return events, events != nil
}

// tryTableR adapts tryTableHeader for the Recognizer signature.
func (p *Parser) tryTableR() ([]Event, bool) {
	events := p.tryTableHeader()
	return events, events != nil
}

func (p *Parser) bufferHasPattern() bool {
	if len(p.buf) == 0 {
		return false
	}
	t := p.buf[0].Type
	if p.lineStart {
		switch t {
		case tokenizer.HashToken, tokenizer.GreaterToken, tokenizer.DashToken,
			tokenizer.StarToken, tokenizer.UnderscoreToken, tokenizer.PipeToken,
			tokenizer.TabToken, tokenizer.LeftBracketToken:
			return true
		case tokenizer.TextToken:
			return hasLineStartTextPattern(p.buf[0].Value)
		}
	}
	switch t {
	case tokenizer.StarToken, tokenizer.BacktickToken, tokenizer.TildeToken,
		tokenizer.UnderscoreToken, tokenizer.LeftBracketToken,
		tokenizer.BackslashToken, tokenizer.AmpersandToken,
		tokenizer.HashToken, tokenizer.DashToken, tokenizer.GreaterToken,
		tokenizer.PipeToken:
		return true
	case tokenizer.TextToken:
		if p.lineStart {
			return hasLineStartTextPattern(p.buf[0].Value)
		}
	}
	return false
}

func hasLineStartTextPattern(v string) bool {
	if len(v) == 0 {
		return false
	}
	if v[0] == '<' {
		return true
	}
	if v[0] == ' ' {
		for _, c := range v {
			if c >= '0' && c <= '9' {
				return true
			}
			if c != ' ' {
				break
			}
		}
	}
	if _, ok := orderedListPrefix(v); ok {
		return true
	}
	return false
}
