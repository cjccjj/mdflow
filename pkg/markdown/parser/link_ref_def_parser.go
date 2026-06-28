package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// linkRefDefParser handles link reference definitions: [label]: url "title".
type linkRefDefParser struct {
	lrdWaiting bool
	lrdBuf     []tokenizer.Token
	p          *Parser
}

func newLinkRefDefParser(p *Parser) *linkRefDefParser {
	return &linkRefDefParser{p: p}
}

func (lrd *linkRefDefParser) reset() {
	lrd.lrdWaiting = false
	lrd.lrdBuf = nil
}

func (lrd *linkRefDefParser) tryLinkRefDef() ([]Event, bool) {
	p := lrd.p
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

	if idx >= len(p.buf) || p.buf[idx].Type != tokenizer.LeftBracketToken {
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

	label, url, _, ok := parseLinkRefDefLine(fullText)
	if !ok {
		return nil, false
	}

	p.consume(lineEnd + 1)
	p.lineStart = true

	if url == "" {
		p.state = LinkRefDefState
		lrd.lrdBuf = append(lrd.lrdBuf, lineTokens...)
		lrd.lrdBuf = append(lrd.lrdBuf, tokenizer.Token{Type: tokenizer.NewlineToken, Value: "\n"})
		lrd.lrdWaiting = false
		return nil, true
	}

			lrd.p.state = NormalState
	return []Event{{Type: LinkRefDefEvent, Value: label, URL: url}}, true
}

func (lrd *linkRefDefParser) processLinkRefDef() []Event {
	p := lrd.p
	if !p.hasNewline() {
		return nil
	}

	tokens := collectTokensUpToNewline(p.buf)
	if len(tokens) == 0 || isBlankLineTokens(tokens) {
		if len(p.buf) > 0 && p.buf[0].Type == tokenizer.NewlineToken {
			p.consume(1)
		}
		return lrd.flushLinkRefDef()
	}

	p.consume(len(tokens))
	if len(p.buf) > 0 && p.buf[0].Type == tokenizer.NewlineToken {
		p.consume(1)
	}
	p.lineStart = true
	lrd.lrdBuf = append(lrd.lrdBuf, tokens...)
	lrd.lrdBuf = append(lrd.lrdBuf, tokenizer.Token{Type: tokenizer.NewlineToken, Value: "\n"})

	fullText := rebuildLineText(lrd.lrdBuf)
	label, url, _, ok := parseLinkRefDefLine(fullText)
	if ok {
		lrd.lrdBuf = nil
		lrd.lrdWaiting = false
			lrd.p.state = NormalState
		return []Event{{Type: LinkRefDefEvent, Value: label, URL: url}}
	}

	return nil
}

func (lrd *linkRefDefParser) flushLinkRefDef() []Event {
	var events []Event
	for _, tok := range lrd.lrdBuf {
		events = append(events, Event{Type: TextEvent, Value: tok.Value})
	}
	lrd.lrdBuf = nil
	lrd.lrdWaiting = false
	return events
}

// parseLinkRefDefLine parses a single [label]: url "title" line.
// Returns label, url, title, ok.
func parseLinkRefDefLine(line string) (label, url, title string, ok bool) {
	line = strings.TrimLeft(line, " \t")
	if len(line) == 0 || line[0] != '[' {
		return "", "", "", false
	}

	closeBracket := findUnescapedBracket(line[1:])
	if closeBracket < 0 {
		return "", "", "", false
	}
	closeBracket++
	label = line[1:closeBracket]

	afterLabel := strings.TrimLeft(line[closeBracket+1:], " \t")
	if len(afterLabel) == 0 || afterLabel[0] != ':' {
		return "", "", "", false
	}

	rest := strings.TrimLeft(afterLabel[1:], " \t")
	if len(rest) == 0 {
		return label, "", "", true
	}

	urlEnd := 0
	if len(rest) > 0 && rest[0] == '<' {
		closeAngle := strings.IndexByte(rest[1:], '>')
		if closeAngle < 0 {
			return "", "", "", false
		}
		url = rest[1 : 1+closeAngle]
		urlEnd = 1 + closeAngle + 1
	} else {
		for urlEnd < len(rest) {
			c := rest[urlEnd]
			if c == ' ' || c == '\t' || c == '\n' {
				break
			}
			urlEnd++
		}
		url = rest[:urlEnd]
	}

	titlePart := strings.TrimLeft(rest[urlEnd:], " \t")
	if len(titlePart) == 0 {
		return label, url, "", true
	}

	q := titlePart[0]
	if q == '"' || q == '\'' || q == '(' {
		var close byte
		switch q {
		case '"':
			close = '"'
		case '\'':
			close = '\''
		case '(':
			close = ')'
		}
		closeIdx := -1
		for i := 1; i < len(titlePart); i++ {
			if titlePart[i] == close && (i == 1 || titlePart[i-1] != '\\') {
				closeIdx = i
				break
			}
		}
		if closeIdx < 0 {
			return "", "", "", false
		}
		title = titlePart[1:closeIdx]
		afterTitle := strings.TrimLeft(titlePart[closeIdx+1:], " \t")
		if len(afterTitle) > 0 {
			return "", "", "", false
		}
		return label, url, title, true
	}

	return label, url, "", true
}

func findUnescapedBracket(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == ']' {
			return i
		}
	}
	return -1
}
