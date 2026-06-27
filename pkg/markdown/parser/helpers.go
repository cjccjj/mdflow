package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

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

func tabRemainingEquiv(col int) int {
	next := ((col + 4) / 4) * 4
	equiv := next - col
	if equiv > 1 {
		return equiv - 1
	}
	return 0
}

func (p *Parser) equivIndent() (bool, string) {
	col := 0
	for i, tok := range p.buf {
		switch tok.Type {
		case tokenizer.TabToken:
			col = ((col + 4) / 4) * 4
			if col >= 4 {
				p.buf = p.buf[i+1:]
				return true, ""
			}
		case tokenizer.TextToken:
			for j, r := range tok.Value {
				if r == ' ' {
					col++
					if col >= 4 {
						p.buf = p.buf[i+1:]
						return true, tok.Value[j+1:]
					}
				} else {
					return false, ""
				}
			}
		default:
			return false, ""
		}
	}
	return false, ""
}

func (p *Parser) peekEquivIndent() (satisfied bool, consumeCount int, remaining string) {
	col := 0
	for i, tok := range p.buf {
		switch tok.Type {
		case tokenizer.TabToken:
			col = ((col + 4) / 4) * 4
			if col >= 4 {
				return true, i + 1, ""
			}
		case tokenizer.TextToken:
			for j, r := range tok.Value {
				if r == ' ' {
					col++
					if col >= 4 {
						return true, i + 1, tok.Value[j+1:]
					}
				} else {
					return false, 0, ""
				}
			}
		default:
			return false, 0, ""
		}
	}
	return false, 0, ""
}

func (p *Parser) hasNewline() bool {
	for _, tok := range p.buf {
		if tok.Type == tokenizer.NewlineToken {
			return true
		}
	}
	return false
}

func (p *Parser) checkConsecutive(tt tokenizer.TokenType, n int) (matched bool, waiting bool) {
	for i := 0; i < n && i < len(p.buf); i++ {
		if p.buf[i].Type != tt {
			return false, false
		}
	}
	if len(p.buf) < n {
		if p.eof {
			return false, false
		}
		return false, true
	}
	return true, false
}

func isPureWhitespaceToken(tok tokenizer.Token) bool {
	if tok.Type == tokenizer.TabToken {
		return true
	}
	if tok.Type == tokenizer.TextToken {
		for _, r := range tok.Value {
			if r != ' ' {
				return false
			}
		}
		return true
	}
	return false
}

func isBlankLineTokens(tokens []tokenizer.Token) bool {
	for _, tok := range tokens {
		if tok.Type == tokenizer.NewlineToken {
			return true
		}
		if !isPureWhitespaceToken(tok) {
			return false
		}
	}
	return false
}

func hasStructuralWhitespace(tok tokenizer.Token) bool {
	if tok.Type == tokenizer.TabToken {
		return true
	}
	if tok.Type == tokenizer.TextToken && strings.HasPrefix(tok.Value, " ") {
		return true
	}
	return false
}

func (p *Parser) checkHorizontalRule(tt tokenizer.TokenType) (matched bool, waiting bool) {
	markerCount := 0
	consumed := 0
	// Skip up to 3 spaces of leading indent (CommonMark 4.1)
	indent := 0
	for consumed < len(p.buf) {
		tok := p.buf[consumed]
		if tok.Type == tokenizer.TabToken {
			indent = ((indent + 4) / 4) * 4
			consumed++
			if indent > 3 {
				return false, false
			}
		} else if tok.Type == tokenizer.TextToken {
			spaceCount := 0
			for _, r := range tok.Value {
				if r == ' ' {
					spaceCount++
				} else {
					break
				}
			}
			if spaceCount == len(tok.Value) && spaceCount > 0 {
				indent += spaceCount
				consumed++
				if indent > 3 {
					return false, false
				}
			} else {
				break
			}
		} else {
			break
		}
	}
	// Count markers (whitespace between markers is allowed)
	for consumed < len(p.buf) {
		tok := p.buf[consumed]
		if tok.Type == tt {
			markerCount++
			consumed++
		} else if isPureWhitespaceToken(tok) {
			consumed++
		} else {
			break
		}
	}
	if consumed == len(p.buf) {
		if p.eof {
			return false, false
		}
		return false, true
	}
	if markerCount < 3 {
		return false, false
	}
	if p.buf[consumed].Type == tokenizer.NewlineToken {
		p.consume(consumed)
		return true, false
	}
	return false, false
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

func leadingSpaceCount(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func stripLeadingWhitespaceTokens(tokens []tokenizer.Token) []tokenizer.Token {
	for len(tokens) > 0 {
		tok := tokens[0]
		if tok.Type == tokenizer.TabToken {
			tokens = tokens[1:]
		} else if tok.Type == tokenizer.TextToken {
			trimmed := strings.TrimLeft(tok.Value, " \t")
			if trimmed != tok.Value {
				if trimmed == "" {
					tokens = tokens[1:]
				} else {
					tokens[0] = tokenizer.Token{Type: tokenizer.TextToken, Value: trimmed}
					return tokens
				}
			} else {
				break
			}
		} else {
			break
		}
	}
	return tokens
}

func stripTrailingWhitespaceTokens(tokens []tokenizer.Token) []tokenizer.Token {
	for len(tokens) > 0 {
		tok := tokens[len(tokens)-1]
		if tok.Type == tokenizer.TabToken {
			tokens = tokens[:len(tokens)-1]
		} else if tok.Type == tokenizer.TextToken {
			trimmed := strings.TrimRight(tok.Value, " \t")
			if trimmed != tok.Value {
				if trimmed == "" {
					tokens = tokens[:len(tokens)-1]
				} else {
					tokens[len(tokens)-1] = tokenizer.Token{Type: tokenizer.TextToken, Value: trimmed}
					return tokens
				}
			} else {
				break
			}
		} else {
			break
		}
	}
	return tokens
}

func (p *Parser) appendTokens(tokens []tokenizer.Token) {
	p.buf = append(p.buf, tokens...)
}

func (p *Parser) bufferedLen() int {
	return len(p.buf)
}

func (p *Parser) hasBufferedTokens() bool {
	return len(p.buf) > 0
}

func (p *Parser) prependTokens(tokens ...tokenizer.Token) {
	if len(tokens) == 0 {
		return
	}
	p.buf = append(append([]tokenizer.Token(nil), tokens...), p.buf...)
}

func (p *Parser) consume(n int) {
	if n > len(p.buf) {
		n = len(p.buf)
	}
	p.buf = p.buf[n:]
}

func hasNewlineIn(tokens []tokenizer.Token) bool {
	for _, tok := range tokens {
		if tok.Type == tokenizer.NewlineToken {
			return true
		}
	}
	return false
}

func isListStartAfterIndent(tokens []tokenizer.Token, remaining string) bool {
	if remaining != "" {
		if _, ok := orderedListPrefix(remaining); ok {
			return true
		}
		if isDigitsOnly(remaining) && len(tokens) > 0 {
			next := tokens[0]
			if (next.Type == tokenizer.LeftParenToken || next.Type == tokenizer.RightParenToken) && len(tokens) > 1 && hasStructuralWhitespace(tokens[1]) {
				return true
			}
			if next.Type == tokenizer.TextToken && len(next.Value) > 0 && next.Value[0] == '.' {
				if len(next.Value) > 1 && next.Value[1] == ' ' {
					return true
				}
				if len(next.Value) == 1 && len(tokens) > 1 && hasStructuralWhitespace(tokens[1]) {
					return true
				}
			}
		}
		return false
	}
	if len(tokens) == 0 {
		return false
	}
	first := tokens[0]
	if first.Type == tokenizer.DashToken || first.Type == tokenizer.StarToken {
		return true
	}
	if first.Type == tokenizer.TextToken {
		if _, ok := orderedListPrefix(first.Value); ok {
			return true
		}
	}
	return false
}

func isDigitsOnly(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\xa0'
}

func isPunctByte(b byte) bool {
	return (b >= 0x21 && b <= 0x2F) ||
		(b >= 0x3A && b <= 0x40) ||
		(b >= 0x5B && b <= 0x60) ||
		(b >= 0x7B && b <= 0x7E)
}

func (p *Parser) isLeftFlankingRun(tt tokenizer.TokenType, runLen int) bool {
	followedByWhitespace := false
	followedByPunct := false

	if runLen < len(p.buf) {
		next := p.buf[runLen]
		followedByWhitespace, followedByPunct = classifyNextChar(next)
	} else if !p.eof {
		return true
	} else {
		followedByWhitespace = true
	}

	if followedByWhitespace {
		return false
	}

	if !followedByPunct {
		return true
	}

	return p.lineStart || isWhitespaceByte(p.prevChar) || isPunctByte(p.prevChar)
}

func (p *Parser) isRightFlankingRun(tt tokenizer.TokenType, runLen int) bool {
	precededByWhitespace := p.lineStart || isWhitespaceByte(p.prevChar)
	if precededByWhitespace {
		return false
	}

	precededByPunct := isPunctByte(p.prevChar) && p.prevChar != markerDelimiterByte(tt)
	if !precededByPunct {
		return true
	}

	if runLen < len(p.buf) {
		next := p.buf[runLen]
		followedByWhitespace, followedByPunct := classifyNextChar(next)
		return followedByWhitespace || followedByPunct
	}

	if p.eof {
		return true
	}

	return true
}

func classifyNextChar(tok tokenizer.Token) (isWhitespace bool, isPunct bool) {
	switch tok.Type {
	case tokenizer.NewlineToken, tokenizer.TabToken:
		return true, false
	case tokenizer.TextToken:
		if len(tok.Value) > 0 {
			b := tok.Value[0]
			if isWhitespaceByte(b) {
				return true, false
			}
			if isPunctByte(b) {
				return false, true
			}
		}
		return false, false
	default:
		return false, true
	}
}
