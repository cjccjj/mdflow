package parser

import (
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

func (p *Parser) checkHorizontalRule(tt tokenizer.TokenType) (matched bool, waiting bool) {
	i := 0
	for i < len(p.buf) && p.buf[i].Type == tt {
		i++
	}
	if i == len(p.buf) {
		if p.eof {
			return false, false
		}
		return false, true
	}
	if i < 3 {
		return false, false
	}
	if p.buf[i].Type == tokenizer.NewlineToken {
		p.consume(i)
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
