package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

type Parser struct {
	state State
	tokenBuffer
	lineContext
	blockContext
	emphasisContext
	tableContext
	setextContext
	linkContext
	htmlBlockContext
	linkRefDefContext
}

type tokenBuffer struct {
	buf []tokenizer.Token
	eof bool
}

type lineContext struct {
	lineStart     bool
	prevChar      byte
	contentIndent int
}

type blockContext struct {
	headerLvl       int
	fenceLen        int
	fenceChar       tokenizer.TokenType
	codeBlockFirst  bool
	codeBlockIndent int
}

type emphasisContext struct {
	italicOpener tokenizer.TokenType
	boldOpener   tokenizer.TokenType
}

type tableContext struct {
	tableHeaderBuf []string
	tableColWidths []int
	tableColAligns []int
}

type setextContext struct {
	setextWaiting bool
	setextBuf     []tokenizer.Token
}

type linkContext struct {
	linkBuf             []tokenizer.Token
	linkURLBuf          []tokenizer.Token
	linkBracketConsumed bool
}

type htmlBlockContext struct {
	htmlBlockType int
	htmlIndent    int
}

type linkRefDefContext struct {
	lrdWaiting bool
	lrdBuf     []tokenizer.Token
}

func New() *Parser {
	return &Parser{state: NormalState, lineContext: lineContext{lineStart: true}}
}

func (p *Parser) Reset() {
	p.state = NormalState
	p.tokenBuffer = tokenBuffer{}
	p.lineContext = lineContext{lineStart: true}
	p.blockContext = blockContext{}
	p.emphasisContext = emphasisContext{}
	p.tableContext = tableContext{}
	p.setextContext = setextContext{}
	p.linkContext = linkContext{}
	p.htmlBlockContext = htmlBlockContext{}
	p.linkRefDefContext = linkRefDefContext{}
}

func (p *Parser) Parse(tokens []tokenizer.Token) (events []Event) {
	if len(tokens) == 0 {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			events = p.safeFlush()
		}
	}()

	p.appendTokens(tokens)
	return p.process()
}

func (p *Parser) process() []Event {
	var events []Event

	for p.hasBufferedTokens() {
		prevLen := p.bufferedLen()
		prevState := p.state

		switch p.state {

		case NormalState:
			events = append(events, p.processNormal()...)

		case HeaderState:
			events = append(events, p.processHeader()...)

		case BoldState:
			events = append(events, p.processBold()...)

		case ItalicState:
			events = append(events, p.processItalic()...)

		case StrikethroughState:
			events = append(events, p.processStrikethrough()...)

		case InlineCodeState:
			events = append(events, p.processInlineCode()...)

		case CodeBlockState:
			events = append(events, p.processCodeBlock()...)

		case IndentedCodeBlockState:
			events = append(events, p.processIndentedCodeBlock()...)

		case BlockquoteState:
			events = append(events, p.processBlockquote()...)

		case TablePendingState:
			events = append(events, p.processTablePending()...)

		case TableBodyState:
			events = append(events, p.processTableBody()...)

		case SetextPendingState:
			events = append(events, p.processSetextPending()...)

		case LinkTextState:
			events = append(events, p.processLinkText()...)

		case LinkURLState:
			events = append(events, p.processLinkURL()...)

		case HTMLBlockState:
			events = append(events, p.processHTMLBlock()...)

		case LinkRefDefState:
			events = append(events, p.processLinkRefDef()...)
		}

		if p.bufferedLen() == prevLen && p.state == prevState {
			break
		}
	}

	return events
}

func (p *Parser) processNormal() []Event {
	if len(p.buf) == 0 {
		return nil
	}

	first := p.buf[0]

	if events, handled := p.processEscapeOrEntity(first); handled {
		return events
	}

	if p.lineStart {
		if events, handled := p.processLineStartBlock(first); handled {
			return events
		}
	}

	if events, handled := p.processInlineStart(first); handled {
		return events
	}

	if p.lineStart {
		if events, handled := p.processDeferredLineStart(first); handled {
			return events
		}
	}

	return p.emitTextOrSpecial()
}

func (p *Parser) processEscapeOrEntity(first tokenizer.Token) ([]Event, bool) {
	if first.Type == tokenizer.BackslashToken {
		return p.handleBackslash(), true
	}
	if first.Type == tokenizer.AmpersandToken {
		return p.handleEntity(), true
	}
	return nil, false
}

func (p *Parser) processLineStartBlock(first tokenizer.Token) ([]Event, bool) {
	if events, handled := p.tryFencedCodeBlock(); handled {
		return events, true
	}

	for _, marker := range []tokenizer.TokenType{tokenizer.DashToken, tokenizer.StarToken, tokenizer.UnderscoreToken} {
		matched, waiting := p.checkHorizontalRule(marker)
		if matched {
			return []Event{{Type: HorizontalRuleEvent}}, true
		}
		if waiting {
			return nil, true
		}
	}

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
		return p.tryHeader(), true
	}

	if first.Type == tokenizer.TextToken && len(p.buf) > 1 {
		if sp := leadingSpaceCount(first.Value); sp >= 1 && sp <= 3 && p.buf[1].Type == tokenizer.HashToken {
			p.consume(1)
			return p.tryHeader(), true
		}
	}

	if first.Type == tokenizer.GreaterToken {
		return p.tryBlockquote(), true
	}

	if first.Type == tokenizer.DashToken {
		matched, waiting := p.checkHorizontalRule(tokenizer.DashToken)
		if matched {
			return []Event{{Type: HorizontalRuleEvent}}, true
		}
		if waiting {
			return nil, true
		}
		return p.tryBullet(), true
	}

	if first.Type == tokenizer.TextToken {
		if prefix, ok := orderedListPrefix(first.Value); ok {
			p.consume(1)
			p.lineStart = false
			events := []Event{{Type: BulletItemEvent, Value: prefix}}
			rest := first.Value[len(prefix):]
			if rest != "" {
				events = append(events, Event{Type: TextEvent, Value: rest})
			}
			return events, true
		}
	}

	if first.Type == tokenizer.StarToken {
		matched, waiting := p.checkHorizontalRule(tokenizer.StarToken)
		if matched {
			return []Event{{Type: HorizontalRuleEvent}}, true
		}
		if waiting {
			return nil, true
		}
		return p.tryBulletOrBold(), true
	}

	if first.Type == tokenizer.TextToken {
		if events, handled := p.tryHTMLBlock(); handled {
			return events, true
		}
	}

	if first.Type == tokenizer.LeftBracketToken {
		if events, handled := p.tryLinkRefDef(); handled {
			return events, true
		}
	}

	return nil, false
}

func (p *Parser) processInlineStart(first tokenizer.Token) ([]Event, bool) {
	if first.Type == tokenizer.StarToken && !p.lineStart {
		matched, waiting := p.checkHorizontalRule(tokenizer.StarToken)
		if matched {
			return []Event{{Type: HorizontalRuleEvent}}, true
		}
		if waiting {
			return nil, true
		}
		matched, waiting = p.checkConsecutive(tokenizer.StarToken, 2)
		if matched {
			if !hasMatchingCloser(p.buf[2:], tokenizer.StarToken, 2, true) && hasNewlineIn(p.buf[2:]) {
				p.consume(2)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "**"}}, true
			}
			p.consume(2)
			p.state = BoldState
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
		p.state = ItalicState
		p.italicOpener = tokenizer.StarToken
		return []Event{{Type: ItalicStartEvent}}, true
	}

	if first.Type == tokenizer.UnderscoreToken {
		matched, waiting := p.checkHorizontalRule(tokenizer.UnderscoreToken)
		if matched {
			return []Event{{Type: HorizontalRuleEvent}}, true
		}
		if waiting {
			return nil, true
		}
		matched, waiting = p.checkConsecutive(tokenizer.UnderscoreToken, 2)
		if matched {
			if !hasMatchingCloser(p.buf[2:], tokenizer.UnderscoreToken, 2, true) && hasNewlineIn(p.buf[2:]) {
				p.consume(2)
				p.lineStart = false
				return []Event{{Type: TextEvent, Value: "__"}}, true
			}
			p.consume(2)
			p.state = BoldState
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
		p.state = ItalicState
		p.italicOpener = tokenizer.UnderscoreToken
		return []Event{{Type: ItalicStartEvent}}, true
	}

	if first.Type == tokenizer.BacktickToken {
		return p.processBacktickStart()
	}

	if first.Type == tokenizer.TildeToken {
		return p.processTildeStart()
	}

	if first.Type == tokenizer.LeftBracketToken && p.prevChar != '!' {
		p.consume(1)
		p.state = LinkTextState
		p.linkBuf = nil
		p.linkBracketConsumed = false
		return nil, true
	}

	return nil, false
}

func (p *Parser) processBacktickStart() ([]Event, bool) {
	if p.lineStart {
		matched, waiting := p.checkConsecutive(tokenizer.BacktickToken, 3)
		if matched {
			n := p.countConsecutive(tokenizer.BacktickToken)
			if n == len(p.buf) && !p.eof {
				return nil, true
			}

			hasBacktickInInfo := false
			for i := n; i < len(p.buf); i++ {
				if p.buf[i].Type == tokenizer.NewlineToken {
					break
				}
				if p.buf[i].Type == tokenizer.BacktickToken {
					hasBacktickInInfo = true
					break
				}
			}
			if hasBacktickInInfo {
				p.consume(n)
				p.state = InlineCodeState
				p.fenceLen = n
				return []Event{{Type: InlineCodeStartEvent}}, true
			}

			p.consume(n)
			p.state = CodeBlockState
			p.fenceLen = n
			p.fenceChar = tokenizer.BacktickToken
			p.codeBlockFirst = true
			return []Event{{Type: CodeBlockStartEvent}}, true
		}
		if waiting {
			return nil, true
		}
	}
	n := p.countConsecutive(tokenizer.BacktickToken)
	if n == len(p.buf) && !p.eof {
		return nil, true
	}
	if n == 1 && !hasMatchingCloser(p.buf[n:], tokenizer.BacktickToken, n, false) {
		p.consume(n)
		p.lineStart = false
		var events []Event
		for i := 0; i < n; i++ {
			events = append(events, Event{Type: TextEvent, Value: "`"})
		}
		return events, true
	}
	p.consume(n)
	p.state = InlineCodeState
	p.fenceLen = n
	return []Event{{Type: InlineCodeStartEvent}}, true
}

func (p *Parser) processTildeStart() ([]Event, bool) {
	if p.lineStart {
		matched, waiting := p.checkConsecutive(tokenizer.TildeToken, 3)
		if matched {
			n := p.countConsecutive(tokenizer.TildeToken)
			if n == len(p.buf) && !p.eof {
				return nil, true
			}
			p.consume(n)
			p.state = CodeBlockState
			p.fenceLen = n
			p.fenceChar = tokenizer.TildeToken
			p.codeBlockFirst = true
			return []Event{{Type: CodeBlockStartEvent}}, true
		}
		if waiting {
			return nil, true
		}
	}
	matched, waiting := p.checkConsecutive(tokenizer.TildeToken, 2)
	if matched {
		p.consume(2)
		p.state = StrikethroughState
		return []Event{{Type: StrikethroughStartEvent}}, true
	}
	if waiting {
		return nil, true
	}
	return nil, false
}

func (p *Parser) processDeferredLineStart(first tokenizer.Token) ([]Event, bool) {
	if first.Type == tokenizer.PipeToken {
		return p.tryTableHeader(), true
	}

	if satisfied, consumeCount, remaining := p.peekEquivIndent(); satisfied {
		if isListStartAfterIndent(p.buf[consumeCount:], remaining) {
			p.consume(consumeCount)
			if remaining != "" {
				return p.handleIndentedListRemaining(remaining), true
			}
			return p.handleIndentedList(), true
		}
		p.consume(consumeCount)
		p.state = IndentedCodeBlockState
		p.lineStart = false
		events := []Event{{Type: CodeBlockStartEvent}}
		if remaining != "" {
			events = append(events, Event{Type: TextEvent, Value: remaining})
		}
		return events, true
	}

	if first.Type == tokenizer.TextToken {
		if p.hasNewline() {
			p.state = SetextPendingState
			return nil, true
		}
	}

	return nil, false
}

func (p *Parser) handleIndentedList() []Event {
	if len(p.buf) == 0 {
		return nil
	}
	first := p.buf[0]
	switch first.Type {
	case tokenizer.DashToken:
		return p.tryBullet()
	case tokenizer.StarToken:
		return p.tryBulletOrBold()
	case tokenizer.TextToken:
		if prefix, ok := orderedListPrefix(first.Value); ok {
			p.consume(1)
			p.lineStart = false
			events := []Event{{Type: BulletItemEvent, Value: prefix}}
			rest := first.Value[len(prefix):]
			if rest != "" {
				events = append(events, Event{Type: TextEvent, Value: rest})
			}
			return events
		}
	}
	return nil
}

func (p *Parser) handleIndentedListRemaining(remaining string) []Event {
	if prefix, ok := orderedListPrefix(remaining); ok {
		p.lineStart = false
		events := []Event{{Type: BulletItemEvent, Value: prefix}}
		rest := remaining[len(prefix):]
		if rest != "" {
			events = append(events, Event{Type: TextEvent, Value: rest})
		}
		return events
	}

	if isDigitsOnly(remaining) && len(p.buf) > 0 {
		next := p.buf[0]
		if next.Type == tokenizer.RightParenToken || next.Type == tokenizer.LeftParenToken {
			p.consume(1)
			prefix := remaining + next.Value
			if len(p.buf) > 0 && hasStructuralWhitespace(p.buf[0]) {
				tok := p.buf[0]
				p.consume(1)
				p.lineStart = false
				events := []Event{{Type: BulletItemEvent, Value: prefix + " "}}
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
			p.lineStart = false
			return []Event{{Type: BulletItemEvent, Value: prefix + " "}}
		}
		if next.Type == tokenizer.TextToken && len(next.Value) > 0 && next.Value[0] == '.' {
			dot := next.Value
			if len(dot) > 1 && dot[1] == ' ' {
				p.consume(1)
				prefix := remaining + dot
				p.lineStart = false
				events := []Event{{Type: BulletItemEvent, Value: prefix}}
				rest := dot[len(prefix)-len(remaining):]
				if rest != "" {
					events = append(events, Event{Type: TextEvent, Value: rest})
				}
				return events
			}
			if len(dot) == 1 && len(p.buf) > 1 && hasStructuralWhitespace(p.buf[1]) {
				p.consume(1)
				tok := p.buf[0]
				p.consume(1)
				p.lineStart = false
				events := []Event{{Type: BulletItemEvent, Value: remaining + ". "}}
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
		}
	}
	return nil
}

func (p *Parser) emitTextOrSpecial() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		switch tok.Type {
		case tokenizer.TextToken:
			p.consume(1)
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
			if len(tok.Value) > 0 {
				p.prevChar = tok.Value[len(tok.Value)-1]
			}
		case tokenizer.NewlineToken:
			p.consume(1)
			p.lineStart = true
			events = append(events, Event{Type: NewlineEvent})
			p.prevChar = '\n'
		default:
			p.consume(1)
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
			if len(tok.Value) > 0 {
				p.prevChar = tok.Value[len(tok.Value)-1]
			}
		}
		if p.state != NormalState || p.bufferHasPattern() {
			break
		}
	}
	return events
}

func (p *Parser) bufferHasPattern() bool {
	if len(p.buf) == 0 {
		return false
	}
	if p.lineStart {
		if p.buf[0].Type == tokenizer.HashToken {
			return true
		}
		if p.buf[0].Type == tokenizer.GreaterToken {
			return true
		}
		if p.buf[0].Type == tokenizer.DashToken {
			return true
		}
		if p.buf[0].Type == tokenizer.StarToken {
			return true
		}
		if p.buf[0].Type == tokenizer.UnderscoreToken {
			return true
		}
		if p.buf[0].Type == tokenizer.PipeToken {
			return true
		}
		if p.buf[0].Type == tokenizer.TabToken {
			return true
		}
	}
	if p.buf[0].Type == tokenizer.StarToken {
		return true
	}
	if p.buf[0].Type == tokenizer.BacktickToken {
		return true
	}
	if p.buf[0].Type == tokenizer.TildeToken {
		return true
	}
	if p.buf[0].Type == tokenizer.UnderscoreToken {
		return true
	}
	if p.buf[0].Type == tokenizer.LeftBracketToken {
		return true
	}
	if p.buf[0].Type == tokenizer.BackslashToken {
		return true
	}
	if p.buf[0].Type == tokenizer.AmpersandToken {
		return true
	}
	if p.buf[0].Type == tokenizer.TextToken && p.lineStart && len(p.buf[0].Value) > 0 {
		// Check for potential HTML block start patterns.
		if p.buf[0].Value[0] == '<' {
			return true
		}
		// Check for text token starting with spaces followed by digits (ordered list).
		if p.buf[0].Value[0] == ' ' {
			for _, c := range p.buf[0].Value {
				if c >= '0' && c <= '9' {
					return true
				}
				if c != ' ' {
					break
				}
			}
		}
	}
	return false
}
