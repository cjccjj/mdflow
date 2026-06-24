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

func isLeftFlanking(tokens []tokenizer.Token) bool {
	if len(tokens) < 2 {
		return true
	}
	next := tokens[1]
	if next.Type == tokenizer.NewlineToken || next.Type == tokenizer.TabToken {
		return false
	}
	if next.Type == tokenizer.TextToken && len(next.Value) > 0 && (next.Value[0] == ' ' || next.Value[0] == '\t') {
		return false
	}
	return true
}
