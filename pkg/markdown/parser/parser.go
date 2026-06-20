package parser

import (
	"strings"

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
}

func (p *Parser) CloseStates() []Event {
	var out []Event
	switch p.state {
	case HeaderState:
		out = append(out, Event{Type: HeaderEndEvent})
	case BoldState:
		out = append(out, Event{Type: BoldEndEvent})
	case ItalicState:
		out = append(out, Event{Type: ItalicEndEvent})
	case StrikethroughState:
		out = append(out, Event{Type: StrikethroughEndEvent})
	case InlineCodeState:
		out = append(out, Event{Type: InlineCodeEndEvent})
	case CodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case IndentedCodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case BlockquoteState:
		out = append(out, Event{Type: BlockquoteEndEvent})
	case TableBodyState:
		cells, ok := p.flushBodyRow()
		if ok {
			rowVal := strings.Join(cells, "\x00")
			out = append(out, Event{Type: TableRowEvent, Value: rowVal})
		}
		out = append(out, Event{Type: TableEndEvent})
	case TablePendingState:
		if len(p.tableHeaderBuf) > 0 {
			out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableHeaderBuf, " | ") + " |"})
			out = append(out, Event{Type: NewlineEvent})
		}
		p.tableHeaderBuf = nil
	}
	return out
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

func (p *Parser) Flush() []Event {
	var out []Event
	if p.state == TableBodyState {
		cells, ok := p.flushBodyRow()
		if ok {
			rowVal := strings.Join(cells, "\x00")
			out = append(out, Event{Type: TableRowEvent, Value: rowVal})
		}
		out = append(out, Event{Type: TableEndEvent})
		p.state = NormalState
		return out
	}
	if p.state == TablePendingState && len(p.tableHeaderBuf) > 0 {
		out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableHeaderBuf, " | ") + " |"})
		out = append(out, Event{Type: NewlineEvent})
		p.tableHeaderBuf = nil
		p.state = NormalState
	}
	if len(p.buf) > 0 {
		for _, tok := range p.buf {
			out = append(out, Event{Type: TextEvent, Value: tok.Value})
		}
		p.buf = nil
	}
	return out
}

func (p *Parser) safeFlush() []Event {
	var out []Event
	switch p.state {
	case HeaderState:
		out = append(out, Event{Type: HeaderEndEvent})
	case BoldState:
		out = append(out, Event{Type: BoldEndEvent})
	case ItalicState:
		out = append(out, Event{Type: ItalicEndEvent})
	case StrikethroughState:
		out = append(out, Event{Type: StrikethroughEndEvent})
	case InlineCodeState:
		out = append(out, Event{Type: InlineCodeEndEvent})
	case CodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case IndentedCodeBlockState:
		out = append(out, Event{Type: CodeBlockEndEvent})
	case BlockquoteState:
		out = append(out, Event{Type: BlockquoteEndEvent})
	case TableBodyState:
		cells, ok := p.flushBodyRow()
		if ok {
			rowVal := strings.Join(cells, "\x00")
			out = append(out, Event{Type: TableRowEvent, Value: rowVal})
		}
		out = append(out, Event{Type: TableEndEvent})
	case TablePendingState:
		if len(p.tableHeaderBuf) > 0 {
			out = append(out, Event{Type: TextEvent, Value: "| " + strings.Join(p.tableHeaderBuf, " | ") + " |"})
			out = append(out, Event{Type: NewlineEvent})
		}
	}
	for _, tok := range p.buf {
		out = append(out, Event{Type: TextEvent, Value: tok.Value})
	}
	p.Reset()
	return out
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
		}

		if len(p.buf) == prevLen && p.state == prevState {
			break
		}
	}

	return events
}

// ---- normal state ----

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
		return []Event{{Type: ItalicStartEvent}}
	}

	if first.Type == tokenizer.UnderscoreToken && p.hasConsecutive(tokenizer.UnderscoreToken, 2) && !p.lineStart {
		p.consume(2)
		p.state = BoldState
		return []Event{{Type: BoldStartEvent}}
	}

	if first.Type == tokenizer.UnderscoreToken && !p.lineStart {
		p.consume(1)
		p.state = ItalicState
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

	// fall through: treat as text
	return p.emitTextOrSpecial()
}

func (p *Parser) tryHeader() []Event {
	if !p.hasConsecutive(tokenizer.HashToken, 1) || len(p.buf) < 2 {
		return nil
	}

	// count consecutive hashes
	level := 0
	for i, tok := range p.buf {
		if tok.Type == tokenizer.HashToken {
			level++
		} else {
			if i > 0 && tok.Type == tokenizer.TextToken && strings.HasPrefix(tok.Value, " ") {
				p.headerLvl = level
				p.consume(i + 1)
				// trim leading space from the text token
				trimmed := strings.TrimPrefix(tok.Value, " ")
				p.state = HeaderState
				events := []Event{{Type: HeaderStartEvent, Value: headerLabel(level)}}
				if trimmed != "" {
					events = append(events, Event{Type: TextEvent, Value: trimmed})
				}
				return events
			}
			// # not followed by space -> treat as text
			return p.emitHashAsText(level, i)
		}
		if level > 6 {
			// more than 6 hashes is not a valid heading
			remaining := i + 1
			if remaining < len(p.buf) && p.buf[remaining].Type == tokenizer.TextToken && strings.HasPrefix(p.buf[remaining].Value, " ") {
				return p.emitHashAsText(level, remaining+1)
			}
			return p.emitHashAsText(level, remaining)
		}
	}

	// ran out of buffer — can't decide yet
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
	// dash not followed by space — treat as text
	p.consume(1)
	return []Event{{Type: TextEvent, Value: "-"}}
}

func (p *Parser) tryBulletOrBold() []Event {
	if len(p.buf) < 2 {
		return nil
	}
	// Check for bullet: * followed by space-text
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
	// Check for bold: **
	if p.hasConsecutive(tokenizer.StarToken, 2) {
		p.consume(2)
		p.state = BoldState
		p.lineStart = false
		return []Event{{Type: BoldStartEvent}}
	}
	// Check for italic: *text*
	if len(p.buf) >= 3 && p.buf[1].Type == tokenizer.TextToken &&
		!strings.HasPrefix(p.buf[1].Value, " ") && p.buf[2].Type == tokenizer.StarToken {
		p.consume(1)
		p.state = ItalicState
		p.lineStart = false
		return []Event{{Type: ItalicStartEvent}}
	}
	// single star — treat as text
	if len(p.buf) >= 2 {
		p.consume(1)
		p.lineStart = false
		return []Event{{Type: TextEvent, Value: "*"}}
	}
	// can't decide yet
	return nil
}

// ---- emit text, handling special chars that didn't match a pattern ----

func (p *Parser) emitTextOrSpecial() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		switch tok.Type {
		case tokenizer.TextToken:
			p.consume(1)
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		case tokenizer.NewlineToken:
			p.consume(1)
			p.lineStart = true
			events = append(events, Event{Type: NewlineEvent})
		default:
			// single special char treated as text
			p.consume(1)
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
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
	return false
}

// ---- header state ----

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

// ---- bold state ----

func (p *Parser) processBold() []Event {
	var events []Event
	for len(p.buf) > 0 {
		if p.lineStart && p.isBlockLevel(p.buf[0].Type) {
			events = append(events, Event{Type: BoldEndEvent})
			p.state = NormalState
			return events
		}
		if p.hasConsecutive(tokenizer.StarToken, 2) {
			p.consume(2)
			p.state = NormalState
			events = append(events, Event{Type: BoldEndEvent})
			return events
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

// ---- italic state ----

func (p *Parser) processItalic() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if p.lineStart && p.isBlockLevel(tok.Type) {
			events = append(events, Event{Type: ItalicEndEvent})
			p.state = NormalState
			return events
		}
		if tok.Type == tokenizer.StarToken {
			p.consume(1)
			p.state = NormalState
			events = append(events, Event{Type: ItalicEndEvent})
			return events
		}
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.state = NormalState
			p.lineStart = true
			events = append(events, Event{Type: ItalicEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		}
		p.consume(1)
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		p.lineStart = false
	}
	return events
}

// ---- strikethrough state ----

func (p *Parser) processStrikethrough() []Event {
	var events []Event
	for len(p.buf) > 0 {
		if p.lineStart && p.isBlockLevel(p.buf[0].Type) {
			events = append(events, Event{Type: StrikethroughEndEvent})
			p.state = NormalState
			return events
		}
		if p.hasConsecutive(tokenizer.TildeToken, 2) {
			p.consume(2)
			p.state = NormalState
			events = append(events, Event{Type: StrikethroughEndEvent})
			return events
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

// ---- inline code state ----

func (p *Parser) processInlineCode() []Event {
	var events []Event
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.BacktickToken && p.hasConsecutive(tokenizer.BacktickToken, p.fenceLen) {
			p.consume(p.fenceLen)
			p.state = NormalState
			p.fenceLen = 0
			if len(events) > 0 {
				first := events[0]
				last := events[len(events)-1]
				if first.Type == TextEvent && strings.HasPrefix(first.Value, " ") {
					first.Value = strings.TrimPrefix(first.Value, " ")
				}
				if last.Type == TextEvent && strings.HasSuffix(last.Value, " ") {
					last.Value = strings.TrimSuffix(last.Value, " ")
				}
			}
			events = append(events, Event{Type: InlineCodeEndEvent})
			return events
		}
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.state = NormalState
			p.lineStart = true
			p.fenceLen = 0
			events = append(events, Event{Type: InlineCodeEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		}
		p.consume(1)
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
		p.lineStart = false
	}
	return events
}

// ---- code block state ----

func (p *Parser) processCodeBlock() []Event {
	var events []Event
	for len(p.buf) > 0 {
		if p.lineStart && p.hasConsecutive(p.fenceChar, p.fenceLen) {
			n := p.countConsecutive(p.fenceChar)
			p.consume(n)
			p.state = NormalState
			p.fenceLen = 0
			p.fenceChar = 0
			p.codeBlockFirst = false
			events = append(events, Event{Type: CodeBlockEndEvent})
			return events
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

// ---- indented code block state ----

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
		// at line start
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
		// non-blank, non-indented line — end code block
		p.state = NormalState
		events = append(events, Event{Type: CodeBlockEndEvent})
		return events
	}
	return events
}

// ---- blockquote state ----

func (p *Parser) tryBlockquote() []Event {
	if len(p.buf) < 2 {
		return nil
	}
	p.consume(1)
	// skip optional space after >
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
	// > followed by text without space or just >
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
			// check if next line continues blockquote
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.GreaterToken {
				p.consume(1)
				// skip optional space
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
			// next line doesn't start with > — end blockquote
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

func orderedListPrefix(s string) (string, bool) {
	if len(s) == 0 || (s[0] < '0' || s[0] > '9') {
		return "", false
	}
	i := 0
	for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) {
		return "", false
	}
	if s[i] != '.' && s[i] != ')' {
		return "", false
	}
	end := i + 1
	if end < len(s) && s[end] == ' ' {
		end++
	}
	return s[:end], true
}

func hasLeadingIndent(s string) bool {
	spaces := 0
	for _, r := range s {
		if r == ' ' {
			spaces++
		} else if r == '\t' {
			spaces += 4
		} else {
			break
		}
		if spaces >= 4 {
			return true
		}
	}
	return spaces >= 4
}

func stripIndent(s string) string {
	spaces := 0
	for i, r := range s {
		if r == ' ' {
			spaces++
		} else if r == '\t' {
			spaces += 4
		} else {
			if spaces >= 4 {
				return s[i:]
			}
			break
		}
	}
	return ""
}

// ---- table state ----

func (p *Parser) tryTableHeader() []Event {
	if !p.lineStart || len(p.buf) < 2 || !p.hasNewline() {
		return nil
	}
	cells, ok := p.extractTableCells()
	if !ok || len(cells) == 0 {
		return p.emitTextOrSpecial()
	}
	p.tableHeaderBuf = cells
	p.state = TablePendingState
	return nil
}

func (p *Parser) processTablePending() []Event {
	if len(p.buf) == 0 || !p.hasNewline() {
		return nil
	}
	if p.buf[0].Type != tokenizer.PipeToken {
		return p.rejectTable()
	}
	cells, ok := p.extractTableCells()
	if !ok || len(cells) == 0 {
		return p.rejectTable()
	}
	if !isSeparatorCells(cells) {
		return p.rejectTable()
	}
	widths := make([]int, len(cells))
	for i, c := range cells {
		widths[i] = len(c)
	}
	if len(widths) != len(p.tableHeaderBuf) {
		return p.rejectTable()
	}
	p.tableColWidths = widths
	p.state = TableBodyState
	p.lineStart = true

	headerVal := strings.Join(p.tableHeaderBuf, "\x00")
	return []Event{{Type: TableStartEvent, Value: headerVal, Extra: widths}}
}

func (p *Parser) processTableBody() []Event {
	if len(p.buf) == 0 || !p.hasNewline() {
		return nil
	}
	if p.buf[0].Type != tokenizer.PipeToken {
		p.state = NormalState
		p.tableColWidths = nil
		return []Event{{Type: TableEndEvent}}
	}
	cells, ok := p.extractTableCells()
	if !ok || len(cells) == 0 {
		p.state = NormalState
		p.tableColWidths = nil
		return []Event{{Type: TableEndEvent}}
	}
	p.lineStart = true

	rowVal := strings.Join(cells, "\x00")
	return []Event{{Type: TableRowEvent, Value: rowVal}}
}

func (p *Parser) rejectTable() []Event {
	cells := p.tableHeaderBuf
	p.tableHeaderBuf = nil
	p.state = NormalState
	if len(cells) > 0 {
		return []Event{
			{Type: TextEvent, Value: "| " + strings.Join(cells, " | ") + " |"},
			{Type: NewlineEvent},
		}
	}
	return nil
}

func (p *Parser) extractTableCells() ([]string, bool) {
	if len(p.buf) == 0 || p.buf[0].Type != tokenizer.PipeToken {
		return nil, false
	}
	p.consume(1)
	var cells []string
	var current strings.Builder
	backtickDepth := 0

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			for len(cells) > 0 && cells[len(cells)-1] == "" {
				cells = cells[:len(cells)-1]
			}
			return cells, len(cells) > 0
		}
		if tok.Type == tokenizer.BacktickToken {
			backtickDepth = 1 - backtickDepth
			p.consume(1)
			current.WriteString(tok.Value)
			continue
		}
		if backtickDepth == 0 && tok.Type == tokenizer.PipeToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		p.consume(1)
		current.WriteString(tok.Value)
	}
	return nil, false
}

func (p *Parser) flushBodyRow() ([]string, bool) {
	if len(p.buf) == 0 {
		return nil, false
	}
	if p.buf[0].Type == tokenizer.PipeToken {
		p.consume(1)
	}
	var cells []string
	var current strings.Builder
	backtickDepth := 0

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			for len(cells) > 0 && cells[len(cells)-1] == "" {
				cells = cells[:len(cells)-1]
			}
			return cells, len(cells) > 0
		}
		if tok.Type == tokenizer.BacktickToken {
			backtickDepth = 1 - backtickDepth
			p.consume(1)
			current.WriteString(tok.Value)
			continue
		}
		if backtickDepth == 0 && tok.Type == tokenizer.PipeToken {
			p.consume(1)
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		p.consume(1)
		current.WriteString(tok.Value)
	}
	cells = append(cells, strings.TrimSpace(current.String()))
	for len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells, len(cells) > 0
}

func isSeparatorCells(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if c == "" {
			continue
		}
		for _, ch := range c {
			if ch != '-' && ch != ':' && ch != ' ' {
				return false
			}
		}
		hasDash := false
		for _, ch := range c {
			if ch == '-' {
				hasDash = true
				break
			}
		}
		if !hasDash {
			return false
		}
	}
	return true
}

// ---- helpers ----

func (p *Parser) hasNewline() bool {
	for _, tok := range p.buf {
		if tok.Type == tokenizer.NewlineToken {
			return true
		}
	}
	return false
}

func (p *Parser) isHorizontalRule(tt tokenizer.TokenType) bool {
	i := 0
	for i < len(p.buf) && p.buf[i].Type == tt {
		i++
	}
	if i < 3 {
		return false
	}
	// All consecutive dashes/stars followed immediately by newline
	if i < len(p.buf) && p.buf[i].Type == tokenizer.NewlineToken {
		p.consume(i)
		return true
	}
	return false
}

func (p *Parser) hasConsecutive(tt tokenizer.TokenType, n int) bool {
	if len(p.buf) < n {
		return false
	}
	for i := 0; i < n; i++ {
		if p.buf[i].Type != tt {
			return false
		}
	}
	return true
}

func (p *Parser) countConsecutive(tt tokenizer.TokenType) int {
	n := 0
	for n < len(p.buf) && p.buf[n].Type == tt {
		n++
	}
	return n
}

func (p *Parser) isBlockLevel(tt tokenizer.TokenType) bool {
	switch tt {
	case tokenizer.HashToken, tokenizer.DashToken, tokenizer.StarToken, tokenizer.BacktickToken, tokenizer.UnderscoreToken, tokenizer.GreaterToken:
		return true
	}
	return false
}

func (p *Parser) consume(n int) {
	if n > len(p.buf) {
		n = len(p.buf)
	}
	p.buf = p.buf[n:]
}

func headerLabel(level int) string {
	switch level {
	case 1:
		return "h1"
	case 2:
		return "h2"
	case 3:
		return "h3"
	case 4:
		return "h4"
	case 5:
		return "h5"
	case 6:
		return "h6"
	default:
		return "h1"
	}
}
