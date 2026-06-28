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
			if strings.HasPrefix(trimmed, "    ") {
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
	p.consume(1)
	p.lineStart = false
	return []Event{{Type: TextEvent, Value: "-"}}
}

// tryBulletOrBold moved to emphasis.go as emphasisParser.tryBulletOrBold()

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

func (p *Parser) tryFencedCodeBlock() ([]Event, bool) {
	idx := 0
	indent := 0

	for idx < len(p.buf) {
		tok := p.buf[idx]
		if tok.Type == tokenizer.TextToken {
			v := tok.Value
			sp := 0
			for sp < len(v) && v[sp] == ' ' {
				sp++
			}
			if sp < len(v) {
				break
			}
			indent += sp
			idx++
			if indent > 3 {
				return nil, false
			}
		} else if tok.Type == tokenizer.TabToken {
			indent = ((indent + 4) / 4) * 4
			idx++
			if indent > 3 {
				return nil, false
			}
		} else {
			break
		}
	}

	if idx >= len(p.buf) {
		return nil, false
	}

	firstAfterIndent := p.buf[idx]

	if firstAfterIndent.Type == tokenizer.BacktickToken {
		count := 0
		for idx+count < len(p.buf) && p.buf[idx+count].Type == tokenizer.BacktickToken {
			count++
		}
		if count < 3 {
			return nil, false
		}

		hasBacktickInInfo := false
		for i := idx + count; i < len(p.buf); i++ {
			if p.buf[i].Type == tokenizer.NewlineToken {
				break
			}
			if p.buf[i].Type == tokenizer.BacktickToken {
				hasBacktickInInfo = true
				break
			}
		}
		if hasBacktickInInfo {
			return nil, false
		}

		p.consume(idx + count)
		p.state = CodeBlockState
		p.fenceLen = count
		p.fenceChar = tokenizer.BacktickToken
		p.codeBlockFirst = true
		p.codeBlockIndent = indent
		return []Event{{Type: CodeBlockStartEvent}}, true
	}

	if firstAfterIndent.Type == tokenizer.TildeToken {
		count := 0
		for idx+count < len(p.buf) && p.buf[idx+count].Type == tokenizer.TildeToken {
			count++
		}
		if count < 3 {
			return nil, false
		}

		p.consume(idx + count)
		p.state = CodeBlockState
		p.fenceLen = count
		p.fenceChar = tokenizer.TildeToken
		p.codeBlockFirst = true
		p.codeBlockIndent = indent
		return []Event{{Type: CodeBlockStartEvent}}, true
	}

	return nil, false
}

func stripLineIndent(text string, indent int) string {
	if indent <= 0 {
		return text
	}
	runes := []rune(text)
	stripped := 0
	for stripped < len(runes) && stripped < indent && runes[stripped] == ' ' {
		stripped++
	}
	return string(runes[stripped:])
}

func (p *Parser) tryCloseCodeFence() (handled bool, closing bool) {
	idx := 0
	indent := 0

	for idx < len(p.buf) {
		tok := p.buf[idx]
		if tok.Type == tokenizer.TextToken {
			v := tok.Value
			sp := 0
			for sp < len(v) && v[sp] == ' ' {
				sp++
			}
			if sp < len(v) {
				break
			}
			indent += sp
			idx++
			if indent > 3 {
				return false, false
			}
		} else if tok.Type == tokenizer.TabToken {
			indent = ((indent + 4) / 4) * 4
			idx++
			if indent > 3 {
				return false, false
			}
		} else {
			break
		}
	}

	if idx >= len(p.buf) || p.buf[idx].Type != p.fenceChar {
		return false, false
	}

	count := 0
	for idx+count < len(p.buf) && p.buf[idx+count].Type == p.fenceChar {
		count++
	}
	if count < p.fenceLen {
		return false, false
	}

	afterFence := idx + count
	for i := afterFence; i < len(p.buf); i++ {
		tok := p.buf[i]
		if tok.Type == tokenizer.NewlineToken {
			break
		}
		if tok.Type == tokenizer.TabToken {
			continue
		}
		if tok.Type == tokenizer.TextToken && strings.TrimSpace(tok.Value) == "" {
			continue
		}
		return false, false
	}

	p.consume(idx + count)

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			break
		}
		if isPureWhitespaceToken(tok) {
			p.consume(1)
			continue
		}
		break
	}

	p.state = NormalState
	p.lineStart = true
	p.fenceLen = 0
	p.fenceChar = 0
	p.codeBlockFirst = false
	p.codeBlockIndent = 0
	return true, true
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
		if p.lineStart {
			if handled, closing := p.tryCloseCodeFence(); handled && closing {
				flushInfo()
				events = append(events, Event{Type: CodeBlockEndEvent})
				return events
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
			if tok.Type == tokenizer.HashToken {
				infoBuf.WriteString("#")
				p.lineStart = false
				continue
			}
			flushInfo()
			p.codeBlockFirst = false
			p.lineStart = false
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
		} else if p.codeBlockIndent > 0 {
			text := tok.Value
			if tok.Type == tokenizer.TextToken || tok.Type == tokenizer.TabToken {
				stripped := stripLineIndent(text, p.codeBlockIndent)
				events = append(events, Event{Type: TextEvent, Value: stripped})
			} else {
				events = append(events, Event{Type: TextEvent, Value: text})
			}
			p.lineStart = false
		} else if tok.Type == tokenizer.TextToken && strings.TrimSpace(tok.Value) == "" {
			events = append(events, Event{Type: TextEvent, Value: tok.Value})
			p.lineStart = false
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
		if isBlankLineTokens(p.buf) {
			for len(p.buf) > 0 && p.buf[0].Type != tokenizer.NewlineToken {
				t := p.buf[0]
				p.consume(1)
				events = append(events, Event{Type: TextEvent, Value: t.Value})
			}
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.NewlineToken {
				p.consume(1)
				events = append(events, Event{Type: NewlineEvent})
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

	// Handle re-entry after a chunk boundary: we are still in BlockquoteState
	// and at lineStart. Decide whether this line continues or terminates the blockquote.
	if p.lineStart && len(p.buf) > 0 {
		first := p.buf[0]

		// An empty blockquote line (> followed by nothing) followed by a
		// non-> line ends the blockquote (CommonMark Ex 249).
		if p.blockquoteHadBlank && first.Type != tokenizer.GreaterToken {
			p.blockquoteHadBlank = false
			p.state = NormalState
			events = append(events, Event{Type: BlockquoteEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		}
		p.blockquoteHadBlank = false

		if first.Type == tokenizer.GreaterToken {
			// Check for empty blockquote line: >\n
			if len(p.buf) >= 2 && p.buf[1].Type == tokenizer.NewlineToken {
				p.consume(2) // consume > and newline
				p.lineStart = true
				p.blockquoteHadBlank = true
				events = append(events, Event{Type: NewlineEvent})
				return events
			}
			// Explicit blockquote marker on continuation line (cross-chunk).
			events = append(events, Event{Type: NewlineEvent})
			events = append(events, p.consumeBlockquoteMarker()...)
			if len(p.buf) == 0 {
				return events
			}
		} else if first.Type == tokenizer.NewlineToken {
			// Blank line — ends the blockquote (BlockquoteEndEvent before
			// NewlineEvent so the renderer does not prefix the next line).
			p.state = NormalState
			events = append(events, Event{Type: BlockquoteEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		} else if isBlockquoteTerminator(first) {
			// Block construct (heading, HR, list, code fence, etc.) — ends the blockquote.
			p.state = NormalState
			events = append(events, Event{Type: BlockquoteEndEvent})
			events = append(events, Event{Type: NewlineEvent})
			return events
		} else {
			// Lazy continuation: paragraph continuation text.
			events = append(events, Event{Type: NewlineEvent})
			p.lineStart = false
		}
	}

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.consume(1)
			p.lineStart = true
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.GreaterToken {
				// Check for empty blockquote line: >\n
				if len(p.buf) >= 2 && p.buf[1].Type == tokenizer.NewlineToken {
					p.consume(2) // consume > and newline
					p.lineStart = true
					events = append(events, Event{Type: NewlineEvent})
					events = append(events, Event{Type: NewlineEvent})
					// Check whether the next line is non-> (terminates blockquote).
					if len(p.buf) > 0 && p.buf[0].Type != tokenizer.GreaterToken {
						if isBlockquoteTerminator(p.buf[0]) {
							p.state = NormalState
							events = append(events, Event{Type: BlockquoteEndEvent})
							events = append(events, Event{Type: NewlineEvent})
							return events
						}
						// Otherwise lazy continuation — but after an empty
						// blockquote line, any non-> line ends the blockquote.
						p.state = NormalState
						events = append(events, Event{Type: BlockquoteEndEvent})
						events = append(events, Event{Type: NewlineEvent})
						return events
					}
					return events
				}
				events = append(events, Event{Type: NewlineEvent})
				events = append(events, p.consumeBlockquoteMarker()...)
				return events
			}
			// Buffer is empty (chunk boundary) — keep blockquote open, wait for next chunk.
			if len(p.buf) == 0 {
				return events
			}
			// Next token is not >.  Check whether the line terminates the blockquote.
			if p.buf[0].Type == tokenizer.NewlineToken {
				// Blank line ends the blockquote (BlockquoteEndEvent before
				// NewlineEvent so the renderer does not prefix the next line).
				p.state = NormalState
				events = append(events, Event{Type: BlockquoteEndEvent})
				events = append(events, Event{Type: NewlineEvent})
				return events
			}
			if isBlockquoteTerminator(p.buf[0]) {
				p.state = NormalState
				events = append(events, Event{Type: BlockquoteEndEvent})
				events = append(events, Event{Type: NewlineEvent})
				return events
			}
			// Lazy continuation: the next line is paragraph continuation text.
			events = append(events, Event{Type: NewlineEvent})
			// Fall through to consume continuation tokens.
			continue
		}
		p.consume(1)
		p.lineStart = false
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
	}
	return events
}

// consumeBlockquoteMarker consumes a > token (and optional following space/tab)
// and returns the appropriate events. Used both for the initial tryBlockquote path
// and for explicit > markers on continuation lines when re-entering after a chunk
// boundary.
func (p *Parser) consumeBlockquoteMarker() []Event {
	var events []Event
	p.consume(1) // consume >
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

// isBlockquoteTerminator reports whether tok starts a line that would end a
// blockquote (i.e. a block-level construct, not paragraph continuation text).
func isBlockquoteTerminator(tok tokenizer.Token) bool {
	switch tok.Type {
	case tokenizer.HashToken, tokenizer.DashToken, tokenizer.PipeToken,
		tokenizer.BacktickToken, tokenizer.TildeToken, tokenizer.TabToken,
		tokenizer.GreaterToken:
		return true
	case tokenizer.StarToken, tokenizer.UnderscoreToken:
		// These could be emphasis or HR/bullet.  Conservatively treat as
		// terminators — emphasis-at-line-start inside a lazy continuation is
		// an edge case that can be refined later.
		return true
	case tokenizer.LeftBracketToken:
		return true // link reference definition
	case tokenizer.TextToken:
		v := tok.Value
		if len(v) > 0 && v[0] == '<' {
			return true // HTML block
		}
		if sp := leadingSpaceCount(v); sp >= 4 {
			rest := v[sp:]
			if rest == "" {
				return false // pure whitespace — not a terminator
			}
			// If the text after indentation is a list marker, it is
			// paragraph continuation text, not an indented code block
			// (indented code blocks cannot interrupt paragraphs).
			if _, ok := orderedListPrefix(rest); ok {
				return false
			}
			if len(rest) > 0 && (rest[0] == '-' || rest[0] == '*') && len(rest) > 1 && rest[1] == ' ' {
				return false // bullet-like → paragraph continuation
			}
			return true // indented code block
		}
		if _, ok := orderedListPrefix(v); ok {
			return true // ordered list
		}
	}
	return false
}

func (p *Parser) processSetextPending() []Event {
	// First call: collect the first content line.
	if !p.setextParser.setextWaiting {
		if !p.hasNewline() {
			return nil
		}
		p.setextParser.setextBuf = stripLeadingSpaces(p.collectLineTokens(), 3)
		p.setextParser.setextWaiting = true
		return nil
	}

	if !p.hasNewline() {
		return nil
	}

	// A line matching a list item marker takes precedence over a setext underline.
	if len(p.buf) > 0 {
		first := p.buf[0]
		if first.Type == tokenizer.DashToken || first.Type == tokenizer.StarToken {
			if len(p.buf) < 2 || p.buf[1].Type == tokenizer.NewlineToken || hasStructuralWhitespace(p.buf[1]) {
				events := p.parseInlineLine(p.setextParser.setextBuf)
				events = append(events, Event{Type: NewlineEvent})
				p.setextParser.setextBuf = nil
				p.setextParser.setextWaiting = false
				p.state = NormalState
				return events
			}
		}
	}

	level, ok := p.checkSetextUnderline()
	if ok {
		content := stripTrailingWhitespace(p.setextParser.setextBuf)
		var events []Event
		if hasTextContent(content) {
			events = append(events, Event{Type: HeaderStartEvent, Level: level})
		}
		events = append(events, p.parseInlineLine(content)...)
		if hasTextContent(content) {
			events = append(events, Event{Type: HeaderEndEvent})
		}
		events = append(events, Event{Type: NewlineEvent})
		p.setextParser.setextBuf = nil
		p.setextParser.setextWaiting = false
		p.state = NormalState
		return events
	}

	// Not an underline. A blank line ends the setext attempt.
	if p.buf[0].Type == tokenizer.NewlineToken {
		p.consume(1)
		p.lineStart = true
		events := p.parseInlineLine(p.setextParser.setextBuf)
		events = append(events, Event{Type: NewlineEvent})
		events = append(events, Event{Type: NewlineEvent})
		p.setextParser.setextBuf = nil
		p.setextParser.setextWaiting = false
		p.state = NormalState
		return events
	}

	// A line starting with a block construct interrupts the setext heading.
	first := p.buf[0]
	if !isContinuationText(first) {
		events := p.parseInlineLine(p.setextParser.setextBuf)
		events = append(events, Event{Type: NewlineEvent})
		p.setextParser.setextBuf = nil
		p.setextParser.setextWaiting = false
		p.state = NormalState
		return events
	}

	// Otherwise, append this line to the heading content (multi-line setext).
	lineTokens := p.collectLineTokens()
	lineTokens = stripLeadingWhitespaceTokens(lineTokens)
	p.setextParser.setextBuf = append(p.setextParser.setextBuf, tokenizer.Token{Type: tokenizer.NewlineToken, Value: "\n"})
	p.setextParser.setextBuf = append(p.setextParser.setextBuf, lineTokens...)
	return nil
}

func isContinuationText(tok tokenizer.Token) bool {
	if tok.Type != tokenizer.TextToken {
		return false
	}
	for _, r := range tok.Value {
		if r != ' ' && r != '\t' {
			return true
		}
	}
	return false
}

func stripLeadingSpaces(tokens []tokenizer.Token, max int) []tokenizer.Token {
	if len(tokens) == 0 {
		return tokens
	}
	first := tokens[0]
	if first.Type != tokenizer.TextToken {
		return tokens
	}
	v := first.Value
	trimmed := 0
	for trimmed < len(v) && trimmed < max && v[trimmed] == ' ' {
		trimmed++
	}
	if trimmed == 0 {
		return tokens
	}
	rest := v[trimmed:]
	if rest == "" {
		return tokens[1:]
	}
	tokens[0] = tokenizer.Token{Type: tokenizer.TextToken, Value: rest}
	return tokens
}

func stripTrailingWhitespace(tokens []tokenizer.Token) []tokenizer.Token {
	for len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		if last.Type == tokenizer.TabToken {
			tokens = tokens[:len(tokens)-1]
			continue
		}
		if last.Type == tokenizer.TextToken {
			t := strings.TrimRight(last.Value, " \t")
			if t == "" {
				tokens = tokens[:len(tokens)-1]
				continue
			}
			if t != last.Value {
				tokens[len(tokens)-1] = tokenizer.Token{Type: tokenizer.TextToken, Value: t}
			}
		}
		break
	}
	return tokens
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
	tmp.eof = true
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

func (p *Parser) tryHTMLBlock() ([]Event, bool) {
	idx := 0
	indent := 0

	for idx < len(p.buf) {
		tok := p.buf[idx]
		if tok.Type == tokenizer.TextToken {
			v := tok.Value
			sp := 0
			for sp < len(v) && v[sp] == ' ' {
				sp++
			}
			if sp < len(v) {
				break
			}
			indent += sp
			idx++
			if indent > 3 {
				return nil, false
			}
		} else if tok.Type == tokenizer.TabToken {
			indent = ((indent + 4) / 4) * 4
			idx++
			if indent > 3 {
				return nil, false
			}
		} else {
			break
		}
	}

	if idx >= len(p.buf) {
		return nil, false
	}

	first := p.buf[idx]
	if first.Type != tokenizer.TextToken {
		return nil, false
	}
	if len(first.Value) == 0 || first.Value[0] != '<' {
		return nil, false
	}

	lineEnd := -1
	for i := idx; i < len(p.buf); i++ {
		if p.buf[i].Type == tokenizer.NewlineToken {
			lineEnd = i
			break
		}
	}
	if lineEnd < 0 {
		return nil, false
	}

	lineTokens := p.buf[idx:lineEnd]
	fullText := rebuildLineText(lineTokens)

	bt := getHTMLBlockType(fullText)
	if bt == 0 {
		return nil, false
	}

	p.consume(lineEnd + 1)

	p.state = HTMLBlockState
	p.htmlBlockParser.htmlBlockType = bt
	p.htmlBlockParser.htmlIndent = indent
	p.lineStart = true

	var events []Event
	events = append(events, Event{Type: HTMLBlockStartEvent})
	for _, tok := range lineTokens {
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
	}
	events = append(events, Event{Type: TextEvent, Value: "\n"})

	if checkHTMLEndCondition(fullText, bt) {
		events = append(events, Event{Type: HTMLBlockEndEvent})
		p.state = NormalState
		p.htmlBlockParser.htmlBlockType = 0
		p.htmlBlockParser.htmlIndent = 0
	}

	return events, true
}

func (p *Parser) processHTMLBlock() []Event {
	if !p.hasNewline() {
		return nil
	}

	tokens := collectTokensUpToNewline(p.buf)
	trail := p.buf[len(tokens):]
	hasNewline := len(trail) > 0 && trail[0].Type == tokenizer.NewlineToken

	lineText := rebuildLineText(tokens)
	endInfo := checkHTMLEndCondition(lineText, p.htmlBlockParser.htmlBlockType)

	p.consume(len(tokens))
	if hasNewline {
		p.consume(1)
	}

	var events []Event
	for _, tok := range tokens {
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
	}
	if hasNewline {
		events = append(events, Event{Type: TextEvent, Value: "\n"})
	}

	if endInfo {
		events = append(events, Event{Type: HTMLBlockEndEvent})
		p.state = NormalState
		p.lineStart = true
		p.htmlBlockParser.htmlBlockType = 0
		p.htmlBlockParser.htmlIndent = 0
	}

	return events
}

func collectTokensUpToNewline(tokens []tokenizer.Token) []tokenizer.Token {
	for i, tok := range tokens {
		if tok.Type == tokenizer.NewlineToken {
			return tokens[:i]
		}
	}
	return tokens
}

func rebuildLineText(tokens []tokenizer.Token) string {
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteString(tok.Value)
	}
	return b.String()
}

func getHTMLBlockType(text string) int {
	lower := strings.ToLower(text)

	if strings.HasPrefix(text, "<![CDATA[") {
		return 5
	}

	if strings.HasPrefix(text, "<!--") {
		return 2
	}

	if strings.HasPrefix(text, "<?") {
		return 3
	}

	if strings.HasPrefix(text, "<!") && len(text) > 2 && isASCIILetter(text[2]) {
		return 4
	}

	blockStarters := []string{"<pre", "<script", "<style", "<textarea"}
	for _, bs := range blockStarters {
		if strings.HasPrefix(lower, bs) {
			after := text[len(bs):]
			if len(after) == 0 {
				return 1
			}
			c := after[0]
			if c == ' ' || c == '\t' || c == '>' {
				return 1
			}
			return 0
		}
	}

	return 0
}

func isASCIILetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func checkHTMLEndCondition(lineText string, blockType int) bool {
	lower := strings.ToLower(lineText)

	switch blockType {
	case 1:
		return strings.Contains(lower, "</pre>") ||
			strings.Contains(lower, "</script>") ||
			strings.Contains(lower, "</style>") ||
			strings.Contains(lower, "</textarea>")
	case 2:
		return strings.Contains(lineText, "-->")
	case 3:
		return strings.Contains(lineText, "?>")
	case 4:
		return strings.Contains(lineText, ">")
	case 5:
		return strings.Contains(lineText, "]]>")
	}
	return true
}

// tryLinkRefDef, parseLinkRefDefLine, findUnescapedBracket,
// processLinkRefDef, and flushLinkRefDef moved to link_ref_def_parser.go

func (p *Parser) checkSetextUnderline() (int, bool) {
	n := len(p.buf)
	if n < 2 {
		return 0, false
	}

	// Scan the underline line without consuming (consume only on success).
	idx := 0
	// Skip up to 3 spaces / tabs of leading indentation on the underline.
	indent := 0
	for idx < n {
		tok := p.buf[idx]
		if tok.Type == tokenizer.TabToken {
			indent = ((indent + 4) / 4) * 4
			if indent > 3 {
				return 0, false
			}
			idx++
			continue
		}
		if tok.Type == tokenizer.TextToken {
			sp := 0
			for sp < len(tok.Value) && tok.Value[sp] == ' ' {
				sp++
			}
			if sp == len(tok.Value) && sp > 0 {
				indent += sp
				if indent > 3 {
					return 0, false
				}
				idx++
				continue
			}
			break
		}
		break
	}
	if idx >= n {
		return 0, false
	}

	// Determine the underline marker and verify the rest of the line.
	level := 0
	markerEnd := idx
	tok := p.buf[idx]
	if tok.Type == tokenizer.TextToken {
		// '=' underline: leading spaces may be embedded in this token.
		v := tok.Value
		lead := 0
		for lead < len(v) && v[lead] == ' ' {
			lead++
		}
		if lead > 3-indent {
			return 0, false
		}
		rest := strings.TrimRight(v[lead:], " \t")
		if rest == "" {
			return 0, false
		}
		for _, c := range rest {
			if c != '=' {
				return 0, false
			}
		}
		level = 1
		markerEnd = idx + 1
	} else if tok.Type == tokenizer.DashToken {
		count := 0
		for markerEnd < n && p.buf[markerEnd].Type == tokenizer.DashToken {
			markerEnd++
			count++
		}
		if count == 0 {
			return 0, false
		}
		level = 2
	} else {
		return 0, false
	}

	// Skip trailing spaces/tabs after the marker.
	for markerEnd < n {
		t := p.buf[markerEnd]
		if t.Type == tokenizer.TabToken {
			markerEnd++
			continue
		}
		if t.Type == tokenizer.TextToken && len(strings.TrimRight(t.Value, " \t")) == 0 {
			markerEnd++
			continue
		}
		break
	}

	// The line must end with a newline.
	if markerEnd >= n || p.buf[markerEnd].Type != tokenizer.NewlineToken {
		return 0, false
	}

	// Success: consume up to and including the newline.
	p.consume(markerEnd + 1)
	p.lineStart = true
	return level, true
}
