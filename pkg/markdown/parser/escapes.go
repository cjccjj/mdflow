package parser

import (
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func isASCIIPunctByte(b byte) bool {
	return (b >= 0x21 && b <= 0x2F) ||
		(b >= 0x3A && b <= 0x40) ||
		(b >= 0x5B && b <= 0x60) ||
		(b >= 0x7B && b <= 0x7E)
}

func hasMatchingCloser(tokens []tokenizer.Token, tt tokenizer.TokenType, n int, spanNewlines bool) bool {
	runLen := 0
	newlineStreak := 0
	for _, tok := range tokens {
		if tok.Type == tokenizer.NewlineToken {
			if !spanNewlines {
				return false
			}
			newlineStreak++
			if newlineStreak >= 2 {
				return false
			}
			runLen = 0
			continue
		}
		newlineStreak = 0
		if tok.Type == tt {
			runLen++
			if runLen == n {
				return true
			}
		} else {
			runLen = 0
		}
	}
	return false
}

func (p *Parser) handleBackslash() []Event {
	if len(p.buf) < 2 {
		p.consume(1)
		p.lineStart = false
		return []Event{{Type: TextEvent, Value: "\\"}}
	}

	second := p.buf[1]
	// Hard line break: backslash immediately followed by a newline.
	if second.Type == tokenizer.NewlineToken {
		p.consume(2)
		p.lineStart = true
		return []Event{{Type: NewlineEvent}}
	}
	if len(second.Value) == 0 || !isASCIIPunctByte(second.Value[0]) {
		p.consume(1)
		p.lineStart = false
		return []Event{{Type: TextEvent, Value: "\\"}}
	}

	p.consume(1)
	escaped := p.buf[0].Value[:1]
	if len(p.buf[0].Value) > 1 {
		p.buf[0].Value = p.buf[0].Value[1:]
	} else {
		p.consume(1)
	}
	p.lineStart = false
	return []Event{{Type: TextEvent, Value: escaped}}
}
