package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

type Parser struct {
	state State
	tokenBuffer
	lineContext
		blockParser      *blockParser
	linkRefDefParser *linkRefDefParser
	setextParser     *setextParser
	htmlBlockParser  *htmlBlockParser

	linkParser      *linkParser
	emphasisParser  *emphasisParser
	tableParser     *tableParser
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



func New() *Parser {
	p := &Parser{state: NormalState, lineContext: lineContext{lineStart: true}}
	p.linkParser = newLinkParser(p)
	p.emphasisParser = newEmphasisParser(p)
	p.tableParser = newTableParser(p)
	p.blockParser = newBlockParser(p)
	p.linkRefDefParser = newLinkRefDefParser(p)
	p.setextParser = newSetextParser(p)
	p.htmlBlockParser = newHTMLBlockParser(p)
	return p
}

func (p *Parser) Reset() {
	p.state = NormalState
	p.tokenBuffer = tokenBuffer{}
	p.lineContext = lineContext{lineStart: true}
	p.blockParser.reset()
	p.linkParser.reset()
	p.emphasisParser.reset()
	p.tableParser.reset()
	p.linkRefDefParser.reset()
	p.setextParser.reset()
	p.htmlBlockParser.reset()
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

		case InlineCodeState:
			events = append(events, p.processInlineCode()...)

		case CodeBlockState:
			events = append(events, p.processCodeBlock()...)

		case IndentedCodeBlockState:
			events = append(events, p.processIndentedCodeBlock()...)

		case BlockquoteState:
			events = append(events, p.processBlockquote()...)

		case TablePendingState:
			events = append(events, p.tableParser.processTablePending()...)

		case TableBodyState:
			events = append(events, p.tableParser.processTableBody()...)

		case SetextPendingState:
			events = append(events, p.processSetextPending()...)

		case LinkTextState:
			events = append(events, p.linkParser.processLinkText()...)

		case LinkURLState:
			events = append(events, p.linkParser.processLinkURL()...)

		case HTMLBlockState:
			events = append(events, p.processHTMLBlock()...)

		case LinkRefDefState:
			events = append(events, p.linkRefDefParser.processLinkRefDef()...)
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

	if events, handled := p.tryThematicBreak(); handled {
		return events, true
	}

	if events, handled := p.tryATXHeading(); handled {
		return events, true
	}

	if first.Type == tokenizer.GreaterToken {
		return p.tryBlockquote(), true
	}

	if first.Type == tokenizer.DashToken {
		return p.tryBullet(), true
	}

	if first.Type == tokenizer.StarToken {
		if events := p.emphasisParser.tryBulletOrBold(); events != nil {
			return events, true
		}
		return nil, false
	}

	if first.Type == tokenizer.TextToken {
		if events, handled := p.tryOrderedList(); handled {
			return events, true
		}
	}

	if events, handled := p.tryHTMLBlock(); handled {
		return events, true
	}

	if events, handled := p.linkRefDefParser.tryLinkRefDef(); handled {
		return events, true
	}

	return nil, false
}

func (p *Parser) processInlineStart(first tokenizer.Token) ([]Event, bool) {
	if first.Type == tokenizer.BacktickToken {
		return p.processBacktickStart()
	}

	if first.Type == tokenizer.LeftBracketToken && p.prevChar != '!' {
		p.consume(1)
		p.linkParser.startLinkText()
		return nil, true
	}

	if first.Type == tokenizer.TildeToken {
		return p.emphasisParser.tryTilde()
	}

	if first.Type == tokenizer.StarToken {
		return p.emphasisParser.tryStar()
	}

	if first.Type == tokenizer.UnderscoreToken {
		return p.emphasisParser.tryUnderscore()
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
				p.blockParser.fenceLen = n
				return []Event{{Type: InlineCodeStartEvent}}, true
			}

			p.consume(n)
			p.state = CodeBlockState
			p.blockParser.fenceLen = n
			p.blockParser.fenceChar = tokenizer.BacktickToken
			p.blockParser.codeBlockFirst = true
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
	p.blockParser.fenceLen = n
	return []Event{{Type: InlineCodeStartEvent}}, true
}

// processTildeStart removed (dead code). Tilde emphasis is handled by emphasisParser.tryTilde().

func (p *Parser) processDeferredLineStart(first tokenizer.Token) ([]Event, bool) {
	if first.Type == tokenizer.PipeToken {
		return p.tableParser.tryTableHeader(), true
	}

	if events, handled := p.tryIndentedCodeOrList(); handled {
		return events, true
	}

	if events, handled := p.trySetextCandidate(); handled {
		return events, true
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
		return p.emphasisParser.tryBulletOrBold()
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


