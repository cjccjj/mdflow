package parser

import (
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

const maxEmphasisDepth = 3

// emphasisFrame tracks an open emphasis span on the stack.
type emphasisFrame struct {
	state      State
	closerType tokenizer.TokenType
	closerLen  int
}

// emphasisParser handles all emphasis (bold, italic, strikethrough) parsing.
// It owns the emphasis stack and all emphasis detection/closing logic.
type emphasisParser struct {
	emphStack []emphasisFrame
	p         *Parser // back-pointer for shared state (buf, lineStart, prevChar, etc.)
}

func newEmphasisParser(p *Parser) *emphasisParser {
	return &emphasisParser{p: p}
}

func (ep *emphasisParser) reset() {
	ep.emphStack = nil
}

func (ep *emphasisParser) push(frame emphasisFrame) {
	ep.emphStack = append(ep.emphStack, frame)
}

func (ep *emphasisParser) pop() emphasisFrame {
	top := ep.emphStack[len(ep.emphStack)-1]
	ep.emphStack = ep.emphStack[:len(ep.emphStack)-1]
	return top
}

func (ep *emphasisParser) top() *emphasisFrame {
	if len(ep.emphStack) == 0 {
		return nil
	}
	return &ep.emphStack[len(ep.emphStack)-1]
}

func (ep *emphasisParser) hasState(s State) bool {
	for _, f := range ep.emphStack {
		if f.state == s {
			return true
		}
	}
	return false
}

func (ep *emphasisParser) depth() int {
	return len(ep.emphStack)
}

func (ep *emphasisParser) drain() []Event {
	var events []Event
	for len(ep.emphStack) > 0 {
		frame := ep.pop()
		events = append(events, emphasisEndEvent(frame))
	}
	return events
}

func emphasisEndEvent(frame emphasisFrame) Event {
	switch frame.state {
	case BoldState:
		return Event{Type: BoldEndEvent}
	case ItalicState:
		return Event{Type: ItalicEndEvent}
	case StrikethroughState:
		return Event{Type: StrikethroughEndEvent}
	}
	return Event{}
}

func emphasisStartEvent(frame emphasisFrame) Event {
	switch frame.state {
	case BoldState:
		return Event{Type: BoldStartEvent}
	case ItalicState:
		return Event{Type: ItalicStartEvent}
	case StrikethroughState:
		return Event{Type: StrikethroughStartEvent}
	}
	return Event{}
}

func markerDelimiterByte(tt tokenizer.TokenType) byte {
	switch tt {
	case tokenizer.StarToken:
		return '*'
	case tokenizer.UnderscoreToken:
		return '_'
	case tokenizer.TildeToken:
		return '~'
	}
	return 0
}

func (ep *emphasisParser) canUnderscoreOpen(runLen int) bool {
	p := ep.p
	if !p.isLeftFlankingRun(tokenizer.UnderscoreToken, runLen) {
		return false
	}
	if !p.isRightFlankingRun(tokenizer.UnderscoreToken, runLen) {
		return true
	}
	return isPunctByte(p.prevChar)
}

func (ep *emphasisParser) canUnderscoreClose(runLen int) bool {
	p := ep.p
	if !p.isRightFlankingRun(tokenizer.UnderscoreToken, runLen) {
		return false
	}
	if !p.isLeftFlankingRun(tokenizer.UnderscoreToken, runLen) {
		return true
	}
	if runLen < len(p.buf) {
		_, followedByPunct := classifyNextChar(p.buf[runLen])
		return followedByPunct
	}
	return true
}

func (ep *emphasisParser) canOpen(tt tokenizer.TokenType, runLen int) bool {
	if tt == tokenizer.UnderscoreToken {
		return ep.canUnderscoreOpen(runLen)
	}
	return ep.p.isLeftFlankingRun(tt, runLen)
}

func multipleOf3RuleBlocks(openerRunLen, closerRunLen int) bool {
	sum := openerRunLen + closerRunLen
	if sum%3 != 0 {
		return false
	}
	return openerRunLen%3 != 0 || closerRunLen%3 != 0
}

func (ep *emphasisParser) tryCloser(tt tokenizer.TokenType) ([]Event, bool) {
	p := ep.p
	top := ep.top()
	if top == nil || top.closerType != tt {
		return nil, false
	}

	count := p.countConsecutive(tt)
	if tt == tokenizer.UnderscoreToken {
		if !ep.canUnderscoreClose(count) {
			return nil, false
		}
	} else if !p.isRightFlankingRun(tt, count) {
		return nil, false
	}

	if count < top.closerLen {
		if p.eof || len(p.buf) > count {
			return nil, false
		}
		return nil, true
	}

	if top.closerLen == 1 && count >= 2 && !ep.hasState(BoldState) {
		if hasMatchingCloser(p.buf[2:], tt, 2, true) {
			return nil, false
		}
		if hasMatchingCloser(p.buf[count:], tt, 1, true) {
			return nil, false
		}
		p.consume(1)
		frame := ep.pop()
		p.lineStart = false
		p.prevChar = markerDelimiterByte(tt)
		return []Event{emphasisEndEvent(frame)}, true
	}

	p.consume(top.closerLen)
	frame := ep.pop()
	events := []Event{emphasisEndEvent(frame)}
	p.lineStart = false
	p.prevChar = markerDelimiterByte(tt)

	remaining := count - top.closerLen
	nextTop := ep.top()
	for remaining > 0 && nextTop != nil && nextTop.closerType == tt && remaining >= nextTop.closerLen {
		p.consume(nextTop.closerLen)
		frame2 := ep.pop()
		events = append(events, emphasisEndEvent(frame2))
		remaining -= nextTop.closerLen
		nextTop = ep.top()
	}

	return events, true
}

func (ep *emphasisParser) tryStar() ([]Event, bool) {
	p := ep.p
	count := p.countConsecutive(tokenizer.StarToken)
	if count == 0 {
		return nil, false
	}

	depth := ep.depth()

	// count >= 3: combined opener (*** = italic + bold)
	if count >= 3 && depth == 0 && !ep.hasState(BoldState) && !ep.hasState(ItalicState) &&
		p.isLeftFlankingRun(tokenizer.StarToken, count) &&
		hasFlankingCloser(p.buf[3:], tokenizer.StarToken, 3, true) {
		p.consume(3)
		ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
		ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}, {Type: BoldStartEvent}}, true
	}

	// try closer for stars (after 3-star check)
	if events, handled := ep.tryCloser(tokenizer.StarToken); handled {
		return events, true
	}

	// count >= 2: bold opener
	if count >= 2 && depth < maxEmphasisDepth && !ep.hasState(BoldState) {
		matched, waiting := p.checkConsecutive(tokenizer.StarToken, 2)
		if waiting {
			return nil, true
		}
		if matched {
			if !p.isLeftFlankingRun(tokenizer.StarToken, count) {
				p.consume(2)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "**"}}, true
			}
			if !hasFlankingCloser(p.buf[2:], tokenizer.StarToken, 2, true) {
				if hasFlankingCloser(p.buf[2:], tokenizer.StarToken, 1, true) && multipleOf3RuleBlocks(count, 1) {
					p.consume(2)
					ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
					p.lineStart = false
					return []Event{{Type: TextEvent, Value: "*"}, {Type: ItalicStartEvent}}, true
				}
				if hasMatchingCloser(p.buf[2:], tokenizer.StarToken, 2, true) || hasNewlineIn(p.buf[2:]) {
					p.consume(2)
					p.lineStart = false
					return []Event{{Type: TextEvent, Value: "**"}}, true
				}
			}
			p.consume(2)
			ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: BoldStartEvent}}, true
		}
	}

	// count >= 2: cross-type bold opener (different closer type)
	if count >= 2 && depth < maxEmphasisDepth {
		top := ep.top()
		if top != nil && top.closerType != tokenizer.StarToken && p.isLeftFlankingRun(tokenizer.StarToken, count) {
			p.consume(2)
			ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: BoldStartEvent}}, true
		}
	}

	// count >= 1: italic opener
	if count >= 1 && depth < maxEmphasisDepth && !ep.hasState(ItalicState) {
		if !p.isLeftFlankingRun(tokenizer.StarToken, count) {
			p.consume(1)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "*"}}, true
		}
		if !hasFlankingCloser(p.buf[1:], tokenizer.StarToken, 1, true) {
			if hasMatchingCloser(p.buf[1:], tokenizer.StarToken, 1, true) || hasNewlineIn(p.buf[1:]) {
				p.consume(1)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "*"}}, true
			}
		}
		p.consume(1)
		ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}, true
	}

	// count >= 1: cross-type italic opener
	if count >= 1 && depth < maxEmphasisDepth {
		top := ep.top()
		if top != nil && top.closerType != tokenizer.StarToken && p.isLeftFlankingRun(tokenizer.StarToken, count) {
			p.consume(1)
			ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
			p.lineStart = false
			return []Event{{Type: ItalicStartEvent}}, true
		}
	}

	return nil, false
}

func (ep *emphasisParser) tryUnderscore() ([]Event, bool) {
	p := ep.p
	count := p.countConsecutive(tokenizer.UnderscoreToken)
	if count == 0 {
		return nil, false
	}

	// for underscores, try closer first (before combined opener check)
	if events, handled := ep.tryCloser(tokenizer.UnderscoreToken); handled {
		return events, true
	}

	depth := ep.depth()

	// count >= 3: combined opener (___ = italic + bold)
	if count >= 3 && depth == 0 && !ep.hasState(BoldState) && !ep.hasState(ItalicState) &&
		ep.canUnderscoreOpen(count) && hasFlankingCloser(p.buf[3:], tokenizer.UnderscoreToken, 3, true) {
		p.consume(3)
		ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
		ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.UnderscoreToken, closerLen: 2})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}, {Type: BoldStartEvent}}, true
	}

	// count >= 2: bold opener
	if count >= 2 && depth < maxEmphasisDepth && !ep.hasState(BoldState) {
		matched, waiting := p.checkConsecutive(tokenizer.UnderscoreToken, 2)
		if waiting {
			return nil, true
		}
		if matched {
			if !ep.canUnderscoreOpen(count) {
				p.consume(2)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "__"}}, true
			}
			if !hasFlankingCloser(p.buf[2:], tokenizer.UnderscoreToken, 2, true) {
				if hasFlankingCloser(p.buf[2:], tokenizer.UnderscoreToken, 1, true) && multipleOf3RuleBlocks(count, 1) {
					p.consume(2)
					ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
					p.lineStart = false
					return []Event{{Type: TextEvent, Value: "_"}, {Type: ItalicStartEvent}}, true
				}
				if hasMatchingCloser(p.buf[2:], tokenizer.UnderscoreToken, 2, true) || hasNewlineIn(p.buf[2:]) {
					p.consume(2)
					p.lineStart = false
					return []Event{{Type: TextEvent, Value: "__"}}, true
				}
			}
			p.consume(2)
			ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.UnderscoreToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: BoldStartEvent}}, true
		}
	}

	// count >= 2: cross-type bold opener
	if count >= 2 && depth < maxEmphasisDepth {
		top := ep.top()
		if top != nil && top.closerType != tokenizer.UnderscoreToken && ep.canUnderscoreOpen(count) {
			p.consume(2)
			ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.UnderscoreToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: BoldStartEvent}}, true
		}
	}

	// count >= 1: italic opener
	if count >= 1 && depth < maxEmphasisDepth && !ep.hasState(ItalicState) {
		if !ep.canUnderscoreOpen(count) {
			p.consume(1)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "_"}}, true
		}
		if count == 1 && !hasFlankingCloser(p.buf[1:], tokenizer.UnderscoreToken, 1, true) {
			if hasMatchingCloser(p.buf[1:], tokenizer.UnderscoreToken, 1, true) || hasNewlineIn(p.buf[1:]) {
				p.consume(1)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "_"}}, true
			}
		}
		p.consume(1)
		ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}, true
	}

	// count >= 1: cross-type italic opener
	if count >= 1 && depth < maxEmphasisDepth {
		top := ep.top()
		if top != nil && top.closerType != tokenizer.UnderscoreToken && ep.canUnderscoreOpen(count) {
			p.consume(1)
			ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
			p.lineStart = false
			return []Event{{Type: ItalicStartEvent}}, true
		}
		if top != nil && hasFlankingCloser(p.buf[1:], tokenizer.UnderscoreToken, 1, true) {
			p.consume(1)
			ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
			p.lineStart = false
			return []Event{{Type: ItalicStartEvent}}, true
		}
	}

	return nil, false
}

func (ep *emphasisParser) tryTilde() ([]Event, bool) {
	p := ep.p
	if events, handled := ep.tryCloser(tokenizer.TildeToken); handled {
		return events, true
	}

	if p.lineStart {
		_, waiting := p.checkConsecutive(tokenizer.TildeToken, 3)
		if waiting {
			return nil, true
		}
	}

	if !ep.hasState(StrikethroughState) && ep.depth() < maxEmphasisDepth {
		matched, waiting := p.checkConsecutive(tokenizer.TildeToken, 2)
		if waiting {
			return nil, true
		}
		if matched {
			p.consume(2)
			ep.push(emphasisFrame{state: StrikethroughState, closerType: tokenizer.TildeToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: StrikethroughStartEvent}}, true
		}
	}

	return nil, false
}

// tryBulletOrBold is called from the Parser to handle star-at-line-start ambiguity.
// If star is followed by structural whitespace, it's a bullet.
// Otherwise it delegates to the emphasis parser.
func (ep *emphasisParser) tryBulletOrBold() []Event {
	p := ep.p
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
			trimmed := trimLeadingSpace(second.Value)
			if hasFourLeadingSpaces(trimmed) {
				p.state = IndentedCodeBlockState
				codeContent := trimmed[4:]
				events = append(events, Event{Type: CodeBlockStartEvent})
				if codeContent != "" {
					events = append(events, Event{Type: TextEvent, Value: codeContent})
				}
				return events
			}
			if trimmed != "" {
				events = append(events, Event{Type: TextEvent, Value: trimmed})
			}
		}
		return events
	}
	// Not followed by whitespace — delegate to emphasis logic.
	starCount := p.countConsecutive(tokenizer.StarToken)
	if starCount >= 3 && len(p.buf) > 2 && p.buf[1].Type == tokenizer.StarToken && p.buf[2].Type == tokenizer.StarToken &&
		hasFlankingCloser(p.buf[3:], tokenizer.StarToken, 3, true) {
		p.consume(3)
		ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
		ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}, {Type: BoldStartEvent}}
	}

	matched, waiting := p.checkConsecutive(tokenizer.StarToken, 2)
	if matched {
		if !p.isLeftFlankingRun(tokenizer.StarToken, starCount) {
			p.consume(2)
			p.lineStart = false
			p.prevChar = '*'
			return []Event{{Type: TextEvent, Value: "**"}}
		}
		if !hasFlankingCloser(p.buf[2:], tokenizer.StarToken, 2, true) {
			if hasFlankingCloser(p.buf[2:], tokenizer.StarToken, 1, true) && multipleOf3RuleBlocks(starCount, 1) {
				p.consume(2)
				ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "*"}, {Type: ItalicStartEvent}}
			}
			if hasMatchingCloser(p.buf[2:], tokenizer.StarToken, 2, true) || hasNewlineIn(p.buf[2:]) {
				p.consume(2)
				p.lineStart = false
				p.prevChar = '*'
				return []Event{{Type: TextEvent, Value: "**"}}
			}
		}
		p.consume(2)
		ep.push(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
		p.lineStart = false
		return []Event{{Type: BoldStartEvent}}
	}
	if waiting {
		return nil
	}
	if len(p.buf) >= 3 && p.buf[1].Type == tokenizer.TextToken &&
		!hasLeadingSpace(p.buf[1].Value) && p.buf[2].Type == tokenizer.StarToken {
		if !p.isLeftFlankingRun(tokenizer.StarToken, starCount) {
			p.consume(1)
			p.lineStart = false
			p.prevChar = '*'
			return []Event{{Type: TextEvent, Value: "*"}}
		}
		if !hasFlankingCloser(p.buf[1:], tokenizer.StarToken, 1, true) {
			if hasMatchingCloser(p.buf[1:], tokenizer.StarToken, 1, true) || hasNewlineIn(p.buf[1:]) {
				p.consume(1)
				p.lineStart = false
				p.prevChar = '*'
				return []Event{{Type: TextEvent, Value: "*"}}
			}
		}
		p.consume(1)
		ep.push(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}
	}
	return nil
}

// Helper functions shared by tryBulletOrBold

func trimLeadingSpace(s string) string {
	if len(s) > 0 && s[0] == ' ' {
		return s[1:]
	}
	return s
}

func hasFourLeadingSpaces(s string) bool {
	return len(s) >= 4 && s[:4] == "    "
}

func hasLeadingSpace(s string) bool {
	return len(s) > 0 && s[0] == ' '
}
