package parser

import (
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

const maxEmphasisDepth = 3

type emphasisFrame struct {
	state      State
	closerType tokenizer.TokenType
	closerLen  int
}

func (p *Parser) pushEmphasis(frame emphasisFrame) {
	p.emphStack = append(p.emphStack, frame)
}

func (p *Parser) popEmphasis() emphasisFrame {
	top := p.emphStack[len(p.emphStack)-1]
	p.emphStack = p.emphStack[:len(p.emphStack)-1]
	return top
}

func (p *Parser) topEmphasis() *emphasisFrame {
	if len(p.emphStack) == 0 {
		return nil
	}
	return &p.emphStack[len(p.emphStack)-1]
}

func (p *Parser) hasEmphasisState(s State) bool {
	for _, f := range p.emphStack {
		if f.state == s {
			return true
		}
	}
	return false
}

func (p *Parser) emphasisDepth() int {
	return len(p.emphStack)
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

func (p *Parser) drainEmphasisStack() []Event {
	var events []Event
	for len(p.emphStack) > 0 {
		frame := p.popEmphasis()
		events = append(events, emphasisEndEvent(frame))
	}
	return events
}

func (p *Parser) popAllEmphasis() []Event {
	return p.drainEmphasisStack()
}

func (p *Parser) tryEmphasisCloser(tt tokenizer.TokenType) ([]Event, bool) {
	top := p.topEmphasis()
	if top == nil || top.closerType != tt {
		return nil, false
	}

	count := p.countConsecutive(tt)
	if count < top.closerLen {
		if p.eof || len(p.buf) > count {
			return nil, false
		}
		return nil, true
	}

	if top.closerLen == 1 && count >= 2 && !p.hasEmphasisState(BoldState) {
		return nil, false
	}

	p.consume(top.closerLen)
	frame := p.popEmphasis()
	events := []Event{emphasisEndEvent(frame)}
	p.lineStart = false

	remaining := count - top.closerLen
	nextTop := p.topEmphasis()
	for remaining > 0 && nextTop != nil && nextTop.closerType == tt && remaining >= nextTop.closerLen {
		p.consume(nextTop.closerLen)
		frame2 := p.popEmphasis()
		events = append(events, emphasisEndEvent(frame2))
		remaining -= nextTop.closerLen
		nextTop = p.topEmphasis()
	}

	return events, true
}

func (p *Parser) tryEmphasisOpener(tt tokenizer.TokenType, state State, closerLen int) ([]Event, bool) {
	if p.emphasisDepth() >= maxEmphasisDepth {
		return nil, false
	}

	if p.hasEmphasisState(state) {
		return nil, false
	}

	if tt == tokenizer.UnderscoreToken && !isLeftFlanking(p.buf) {
		return nil, false
	}

	count := p.countConsecutive(tt)
	if count < closerLen {
		return nil, true
	}

	if state == BoldState && !hasMatchingCloser(p.buf[closerLen:], tt, closerLen, true) && hasNewlineIn(p.buf[closerLen:]) {
		return nil, false
	}

	if state == ItalicState && closerLen == 1 && !hasMatchingCloser(p.buf[closerLen:], tt, 1, true) {
		return nil, false
	}

	p.consume(closerLen)
	p.pushEmphasis(emphasisFrame{state: state, closerType: tt, closerLen: closerLen})
	p.lineStart = false
	return []Event{emphasisStartEvent(emphasisFrame{state: state})}, true
}

func (p *Parser) tryEmphasisStar() ([]Event, bool) {
	count := p.countConsecutive(tokenizer.StarToken)
	if count == 0 {
		return nil, false
	}

	if events, handled := p.tryEmphasisCloser(tokenizer.StarToken); handled {
		return events, true
	}

	depth := p.emphasisDepth()

	if count >= 3 && depth == 0 && !p.hasEmphasisState(BoldState) && !p.hasEmphasisState(ItalicState) &&
		hasMatchingCloser(p.buf[3:], tokenizer.StarToken, 3, true) {
		p.consume(3)
		p.pushEmphasis(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
		p.pushEmphasis(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}, {Type: BoldStartEvent}}, true
	}

	if count >= 2 && depth < maxEmphasisDepth && !p.hasEmphasisState(BoldState) {
		matched, waiting := p.checkConsecutive(tokenizer.StarToken, 2)
		if waiting {
			return nil, true
		}
		if matched {
			if !hasMatchingCloser(p.buf[2:], tokenizer.StarToken, 2, true) && hasNewlineIn(p.buf[2:]) {
				p.consume(2)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "**"}}, true
			}
			p.consume(2)
			p.pushEmphasis(emphasisFrame{state: BoldState, closerType: tokenizer.StarToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: BoldStartEvent}}, true
		}
	}

	if count >= 1 && depth < maxEmphasisDepth && !p.hasEmphasisState(ItalicState) {
		if !hasMatchingCloser(p.buf[1:], tokenizer.StarToken, 1, true) {
			p.consume(1)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "*"}}, true
		}
		p.consume(1)
		p.pushEmphasis(emphasisFrame{state: ItalicState, closerType: tokenizer.StarToken, closerLen: 1})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}, true
	}

	return nil, false
}

func (p *Parser) tryEmphasisUnderscore() ([]Event, bool) {
	count := p.countConsecutive(tokenizer.UnderscoreToken)
	if count == 0 {
		return nil, false
	}

	if events, handled := p.tryEmphasisCloser(tokenizer.UnderscoreToken); handled {
		return events, true
	}

	depth := p.emphasisDepth()

	if count >= 3 && depth == 0 && !p.hasEmphasisState(BoldState) && !p.hasEmphasisState(ItalicState) &&
		isLeftFlanking(p.buf) && hasMatchingCloser(p.buf[3:], tokenizer.UnderscoreToken, 3, true) {
		p.consume(3)
		p.pushEmphasis(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
		p.pushEmphasis(emphasisFrame{state: BoldState, closerType: tokenizer.UnderscoreToken, closerLen: 2})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}, {Type: BoldStartEvent}}, true
	}

	if count >= 2 && depth < maxEmphasisDepth && !p.hasEmphasisState(BoldState) {
		matched, waiting := p.checkConsecutive(tokenizer.UnderscoreToken, 2)
		if waiting {
			return nil, true
		}
		if matched {
			if !hasMatchingCloser(p.buf[2:], tokenizer.UnderscoreToken, 2, true) && hasNewlineIn(p.buf[2:]) {
				p.consume(2)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "__"}}, true
			}
			p.consume(2)
			p.pushEmphasis(emphasisFrame{state: BoldState, closerType: tokenizer.UnderscoreToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: BoldStartEvent}}, true
		}
	}

	if count >= 1 && depth < maxEmphasisDepth && !p.hasEmphasisState(ItalicState) {
		if !isLeftFlanking(p.buf) {
			p.consume(1)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "_"}}, true
		}
		if count == 1 && !hasMatchingCloser(p.buf[1:], tokenizer.UnderscoreToken, 1, true) {
			p.consume(1)
			p.lineStart = false
			return []Event{{Type: TextEvent, Value: "_"}}, true
		}
		p.consume(1)
		p.pushEmphasis(emphasisFrame{state: ItalicState, closerType: tokenizer.UnderscoreToken, closerLen: 1})
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}, true
	}

	return nil, false
}

func (p *Parser) tryEmphasisTilde() ([]Event, bool) {
	if events, handled := p.tryEmphasisCloser(tokenizer.TildeToken); handled {
		return events, true
	}

	if p.lineStart {
		_, waiting := p.checkConsecutive(tokenizer.TildeToken, 3)
		if waiting {
			return nil, true
		}
	}

	if !p.hasEmphasisState(StrikethroughState) && p.emphasisDepth() < maxEmphasisDepth {
		matched, waiting := p.checkConsecutive(tokenizer.TildeToken, 2)
		if waiting {
			return nil, true
		}
		if matched {
			p.consume(2)
			p.pushEmphasis(emphasisFrame{state: StrikethroughState, closerType: tokenizer.TildeToken, closerLen: 2})
			p.lineStart = false
			return []Event{{Type: StrikethroughStartEvent}}, true
		}
	}

	return nil, false
}
