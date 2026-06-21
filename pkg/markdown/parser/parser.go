package parser

import (
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

type Parser struct {
	state           State
	buf             []tokenizer.Token
	lineStart       bool
	headerLvl       int
	fenceLen        int
	fenceChar       tokenizer.TokenType
	codeBlockFirst  bool
	tableHeaderBuf  []string
	tableColWidths  []int
	italicOpener    tokenizer.TokenType
	boldOpener      tokenizer.TokenType
	setextWaiting   bool
	setextBuf       []tokenizer.Token
	linkBuf         []tokenizer.Token
	linkBracketConsumed bool
	prevChar        byte
}

func New() *Parser {
	return &Parser{state: NormalState, lineStart: true}
}

func (p *Parser) Reset() {
	p.state = NormalState
	p.buf = nil
	p.lineStart = true
	p.headerLvl = 0
	p.fenceLen = 0
	p.fenceChar = 0
	p.codeBlockFirst = false
	p.tableHeaderBuf = nil
	p.tableColWidths = nil
	p.italicOpener = 0
	p.boldOpener = 0
	p.setextWaiting = false
	p.setextBuf = nil
	p.linkBuf = nil
	p.linkBracketConsumed = false
	p.prevChar = 0
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

	p.buf = append(p.buf, tokens...)
	return p.process()
}

func (p *Parser) process() []Event {
	var events []Event

	for len(p.buf) > 0 {
		prevLen := len(p.buf)
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
		}

		if len(p.buf) == prevLen && p.state == prevState {
			break
		}
	}

	return events
}

func (p *Parser) processNormal() []Event {
	var events []Event
	if len(p.buf) == 0 {
		return events
	}

	first := p.buf[0]

	if p.lineStart && first.Type == tokenizer.HashToken {
		return p.tryHeader()
	}

	if p.lineStart && first.Type == tokenizer.GreaterToken {
		return p.tryBlockquote()
	}

	if p.lineStart && first.Type == tokenizer.DashToken && p.hasConsecutive(tokenizer.DashToken, 3) {
		if p.isHorizontalRule(tokenizer.DashToken) {
			return []Event{{Type: HorizontalRuleEvent}}
		}
	}

	if p.lineStart && first.Type == tokenizer.DashToken {
		return p.tryBullet()
	}

	if p.lineStart && first.Type == tokenizer.TextToken {
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

	if p.lineStart && first.Type == tokenizer.StarToken && p.hasConsecutive(tokenizer.StarToken, 3) {
		if p.isHorizontalRule(tokenizer.StarToken) {
			return []Event{{Type: HorizontalRuleEvent}}
		}
	}

	if p.lineStart && first.Type == tokenizer.UnderscoreToken && p.hasConsecutive(tokenizer.UnderscoreToken, 3) {
		if p.isHorizontalRule(tokenizer.UnderscoreToken) {
			return []Event{{Type: HorizontalRuleEvent}}
		}
	}

	if p.lineStart && first.Type == tokenizer.StarToken {
		return p.tryBulletOrBold()
	}

	if first.Type == tokenizer.StarToken && p.hasConsecutive(tokenizer.StarToken, 2) {
		p.consume(2)
		p.state = BoldState
		p.boldOpener = tokenizer.StarToken
		return []Event{{Type: BoldStartEvent}}
	}

	if first.Type == tokenizer.BacktickToken && p.hasConsecutive(tokenizer.BacktickToken, 3) && p.lineStart {
		n := p.countConsecutive(tokenizer.BacktickToken)
		p.consume(n)
		p.state = CodeBlockState
		p.fenceLen = n
		p.fenceChar = tokenizer.BacktickToken
		p.codeBlockFirst = true
		return []Event{{Type: CodeBlockStartEvent}}
	}

	if first.Type == tokenizer.BacktickToken {
		n := p.countConsecutive(tokenizer.BacktickToken)
		p.consume(n)
		p.state = InlineCodeState
		p.fenceLen = n
		return []Event{{Type: InlineCodeStartEvent}}
	}

	if p.lineStart && first.Type == tokenizer.TildeToken && p.hasConsecutive(tokenizer.TildeToken, 3) {
		n := p.countConsecutive(tokenizer.TildeToken)
		p.consume(n)
		p.state = CodeBlockState
		p.fenceLen = n
		p.fenceChar = tokenizer.TildeToken
		p.codeBlockFirst = true
		return []Event{{Type: CodeBlockStartEvent}}
	}

	if first.Type == tokenizer.TildeToken && p.hasConsecutive(tokenizer.TildeToken, 2) {
		p.consume(2)
		p.state = StrikethroughState
		return []Event{{Type: StrikethroughStartEvent}}
	}

	if first.Type == tokenizer.StarToken && !p.lineStart {
		p.consume(1)
		p.state = ItalicState
		p.italicOpener = tokenizer.StarToken
		return []Event{{Type: ItalicStartEvent}}
	}

	if first.Type == tokenizer.UnderscoreToken && p.hasConsecutive(tokenizer.UnderscoreToken, 2) && !p.lineStart {
		p.consume(2)
		p.state = BoldState
		p.boldOpener = tokenizer.UnderscoreToken
		return []Event{{Type: BoldStartEvent}}
	}

	if first.Type == tokenizer.UnderscoreToken && !p.lineStart {
		p.consume(1)
		p.state = ItalicState
		p.italicOpener = tokenizer.UnderscoreToken
		return []Event{{Type: ItalicStartEvent}}
	}

	if p.lineStart && first.Type == tokenizer.PipeToken {
		return p.tryTableHeader()
	}

	if p.lineStart && first.Type == tokenizer.TabToken {
		p.consume(1)
		p.state = IndentedCodeBlockState
		return []Event{{Type: CodeBlockStartEvent}}
	}

	if p.lineStart && first.Type == tokenizer.TextToken && hasLeadingIndent(first.Value) {
		val := first.Value
		stripped := stripIndent(val)
		p.consume(1)
		p.state = IndentedCodeBlockState
		events := []Event{{Type: CodeBlockStartEvent}}
		if stripped != "" {
			events = append(events, Event{Type: TextEvent, Value: stripped})
		}
		return events
	}

	if p.lineStart && first.Type == tokenizer.TextToken {
		if p.hasNewline() {
			p.state = SetextPendingState
			return nil
		}
	}

	if first.Type == tokenizer.LeftBracketToken && p.prevChar != '!' {
		p.consume(1)
		p.state = LinkTextState
		p.linkBuf = nil
		p.linkBracketConsumed = false
		return nil
	}

	return p.emitTextOrSpecial()
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
	return false
}
