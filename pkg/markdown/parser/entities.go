package parser

import (
	"html"
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

func (p *Parser) handleEntity() []Event {
	if len(p.buf) < 1 {
		return nil
	}

	if len(p.buf) < 2 {
		if !p.eof {
			return nil
		}
		p.consume(1)
		return []Event{{Type: TextEvent, Value: "&"}}
	}

	second := p.buf[1]

	if second.Type == tokenizer.HashToken {
		return p.handleNumericEntity()
	}

	if second.Type == tokenizer.TextToken {
		return p.handleNamedEntity()
	}

	p.consume(1)
	return []Event{{Type: TextEvent, Value: "&"}}
}

func (p *Parser) handleNumericEntity() []Event {
	p.consume(1)
	if len(p.buf) < 1 {
		if !p.eof {
			return nil
		}
		return []Event{{Type: TextEvent, Value: "&#"}}
	}

	hashTok := p.buf[0]
	p.consume(1)

	if len(p.buf) < 1 {
		if !p.eof {
			p.linkURLBuf = append(p.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: hashTok.Value})
			return nil
		}
		return []Event{{Type: TextEvent, Value: "&" + hashTok.Value}}
	}

	tok := p.buf[0]
	if tok.Type != tokenizer.TextToken {
		p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
		return []Event{{Type: TextEvent, Value: "&"}}
	}

	fullValue := tok.Value
	semiIdx := strings.IndexByte(fullValue, ';')
	if semiIdx < 0 {
		p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
		return []Event{{Type: TextEvent, Value: "&"}}
	}

	digits := fullValue[:semiIdx]
	var rest string
	if semiIdx+1 < len(fullValue) {
		rest = fullValue[semiIdx+1:]
	}

	var entityStr string
	if len(digits) >= 1 && (digits[0] == 'x' || digits[0] == 'X') {
		hexDigits := digits[1:]
		if len(hexDigits) < 1 || len(hexDigits) > 6 {
			p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
			return []Event{{Type: TextEvent, Value: "&"}}
		}
		for _, r := range hexDigits {
			if !isHexDigit(byte(r)) {
				p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
				return []Event{{Type: TextEvent, Value: "&"}}
			}
		}
		entityStr = "&#" + string(digits) + ";"
	} else {
		if len(digits) < 1 || len(digits) > 7 {
			p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
			return []Event{{Type: TextEvent, Value: "&"}}
		}
		for _, r := range digits {
			if r < '0' || r > '9' {
				p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
				return []Event{{Type: TextEvent, Value: "&"}}
			}
		}
		entityStr = "&#" + string(digits) + ";"
	}

	decoded := html.UnescapeString(entityStr)
	if decoded == entityStr {
		p.buf = append([]tokenizer.Token{hashTok, tok}, p.buf[1:]...)
		return []Event{{Type: TextEvent, Value: "&"}}
	}

	p.consume(1)

	if rest != "" {
		p.buf = append([]tokenizer.Token{{Type: tokenizer.TextToken, Value: rest}}, p.buf...)
	}

	return []Event{{Type: TextEvent, Value: decoded}}
}

func (p *Parser) handleNamedEntity() []Event {
	p.consume(1)
	tok := p.buf[0]
	p.consume(1)

	fullValue := tok.Value
	semiIdx := strings.IndexByte(fullValue, ';')
	if semiIdx < 0 {
		p.buf = append([]tokenizer.Token{tok}, p.buf...)
		return []Event{{Type: TextEvent, Value: "&"}}
	}

	name := fullValue[:semiIdx]
	var rest string
	if semiIdx+1 < len(fullValue) {
		rest = fullValue[semiIdx+1:]
	}

	for _, r := range name {
		if !isAlphaNum(byte(r)) {
			p.buf = append([]tokenizer.Token{tok}, p.buf...)
			return []Event{{Type: TextEvent, Value: "&"}}
		}
	}

	entityStr := "&" + name + ";"
	decoded := html.UnescapeString(entityStr)
	if decoded == entityStr {
		p.buf = append([]tokenizer.Token{tok}, p.buf...)
		return []Event{{Type: TextEvent, Value: "&"}}
	}

	if rest != "" {
		p.buf = append([]tokenizer.Token{{Type: tokenizer.TextToken, Value: rest}}, p.buf...)
	}

	return []Event{{Type: TextEvent, Value: decoded}}
}

func resolveEntities(s string) string {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] != '&' {
			result = append(result, s[i])
			i++
			continue
		}
		semi := strings.IndexByte(s[i+1:], ';')
		if semi < 0 {
			result = append(result, '&')
			i++
			continue
		}
		inner := s[i+1 : i+1+semi]
		if !isValidEntityContent(inner) {
			result = append(result, '&')
			i++
			continue
		}
		full := s[i : i+1+semi+1]
		decoded := html.UnescapeString(full)
		if decoded == full {
			result = append(result, '&')
			i++
			continue
		}
		result = append(result, []byte(decoded)...)
		i = i + 1 + semi + 1
	}
	result = append(result, s[i:]...)
	return string(result)
}

func isValidEntityContent(inner string) bool {
	if len(inner) == 0 {
		return false
	}
	if inner[0] == '#' {
		if len(inner) >= 3 && (inner[1] == 'x' || inner[1] == 'X') {
			hex := inner[2:]
			if len(hex) < 1 || len(hex) > 6 {
				return false
			}
			for _, r := range hex {
				if !isHexDigit(byte(r)) {
					return false
				}
			}
			return true
		}
		dec := inner[1:]
		if len(dec) < 1 || len(dec) > 7 {
			return false
		}
		for _, r := range dec {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	for _, r := range inner {
		if !isAlphaNum(byte(r)) {
			return false
		}
	}
	return true
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}


