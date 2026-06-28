package parser

import (
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// linkParser handles inline links [text](url) and reference links [text][label].
// It owns all link-specific state and methods, keeping the main Parser struct lean.
type linkParser struct {
	linkBuf             []tokenizer.Token
	linkURLBuf          []tokenizer.Token
	linkBracketConsumed bool
	linkDepth           int
	urlParenDepth       int
	urlAngleBracket     bool
	urlDone             bool
	urlHadNewline       bool
	linkTitleBuf        []tokenizer.Token

	p *Parser // back-pointer for shared state (buf, state, lineStart, etc.)
}

func newLinkParser(p *Parser) *linkParser {
	return &linkParser{p: p}
}

func (lp *linkParser) reset() {
	lp.linkBuf = nil
	lp.linkURLBuf = nil
	lp.linkBracketConsumed = false
	lp.linkDepth = 0
	lp.urlParenDepth = 0
	lp.urlAngleBracket = false
	lp.urlDone = false
	lp.urlHadNewline = false
	lp.linkTitleBuf = nil
}

// startLinkText is called when a '[' is encountered in processInlineStart.
// It initializes link state and transitions to LinkTextState.
func (lp *linkParser) startLinkText() {
	lp.reset()
	lp.p.enterState(LinkTextState)
}

// AppendURLToken is used by entity handlers that need to push tokens into the
// URL buffer when parsing inside a LinkURLState context.
func (lp *linkParser) AppendURLToken(tok tokenizer.Token) {
	lp.linkURLBuf = append(lp.linkURLBuf, tok)
}

// processLinkText handles LinkTextState: parsing the link text between [ and ].
func (lp *linkParser) processLinkText() []Event {
	p := lp.p
	if len(p.buf) == 0 {
		return nil
	}

	if lp.linkBracketConsumed {
		if p.buf[0].Type == tokenizer.LeftParenToken {
			p.consume(1)
			lp.linkBracketConsumed = false
			p.state = LinkURLState
			return nil
		}
		if p.buf[0].Type == tokenizer.LeftBracketToken {
			lp.linkBracketConsumed = false
			return lp.processLinkRefLabel()
		}
		return lp.flushLinkAsText()
	}

	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			return lp.flushLinkAsText()
		}
		if tok.Type == tokenizer.LeftBracketToken {
			p.consume(1)
			lp.linkBuf = append(lp.linkBuf, tok)
			lp.linkDepth++
			continue
		}
		if tok.Type == tokenizer.BackslashToken {
			p.consume(1)
			if len(p.buf) > 0 {
				next := p.buf[0]
				if next.Type == tokenizer.BackslashToken {
					p.consume(1)
					lp.linkBuf = append(lp.linkBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
					continue
				}
				if next.Type == tokenizer.LeftBracketToken || next.Type == tokenizer.RightBracketToken {
					p.consume(1)
					lp.linkBuf = append(lp.linkBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: next.Value})
					continue
				}
			}
			lp.linkBuf = append(lp.linkBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
			continue
		}
		if tok.Type == tokenizer.RightBracketToken {
			if lp.linkDepth > 0 {
				p.consume(1)
				lp.linkBuf = append(lp.linkBuf, tok)
				lp.linkDepth--
				if lp.linkDepth == 0 && len(p.buf) > 0 {
					if p.buf[0].Type == tokenizer.LeftParenToken {
						p.consume(1)
						p.state = LinkURLState
						return nil
					}
					if p.buf[0].Type == tokenizer.LeftBracketToken {
						return lp.processLinkRefLabel()
					}
				}
				continue
			}
			p.consume(1)
			if len(p.buf) == 0 {
				lp.linkBracketConsumed = true
				return nil
			}
			if p.buf[0].Type == tokenizer.LeftParenToken {
				p.consume(1)
				p.state = LinkURLState
				return nil
			}
			if p.buf[0].Type == tokenizer.LeftBracketToken {
				return lp.processLinkRefLabel()
			}
			return lp.flushLinkAsText()
		}
		p.consume(1)
		lp.linkBuf = append(lp.linkBuf, tok)
	}
	return nil
}

// processLinkURL handles LinkURLState: parsing the URL between ( and ).
func (lp *linkParser) processLinkURL() []Event {
	p := lp.p
	if len(p.buf) == 0 {
		return nil
	}

	for len(p.buf) > 0 {
		tok := p.buf[0]

		if tok.Type == tokenizer.NewlineToken {
			if lp.urlHadNewline || lp.urlAngleBracket {
				return lp.flushLinkAsText()
			}
			p.consume(1)
			lp.urlHadNewline = true
			continue
		}

		if lp.urlDone {
			if tok.Type == tokenizer.TabToken || (tok.Type == tokenizer.TextToken && isPureWhitespaceToken(tok)) {
				p.consume(1)
				continue
			}
			if tok.Type == tokenizer.RightParenToken {
				p.consume(1)
				return lp.emitLinkEvent()
			}
			if tok.Type == tokenizer.LeftParenToken {
				return lp.collectTitle('(')
			}
			if tok.Type == tokenizer.TextToken && len(tok.Value) > 0 {
				q := tok.Value[0]
				if q == '"' || q == '\'' || q == '(' {
					return lp.collectTitle(q)
				}
			}
			return lp.flushLinkAsText()
		}

		if lp.urlAngleBracket {
			if tok.Type == tokenizer.BackslashToken {
				p.consume(1)
				if len(p.buf) > 0 && p.buf[0].Type == tokenizer.GreaterToken {
					p.consume(1)
					lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\>"})
				} else {
					lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
				}
				continue
			}
			if tok.Type == tokenizer.GreaterToken {
				p.consume(1)
				lp.urlAngleBracket = false
				lp.urlDone = true
				if len(lp.linkURLBuf) > 0 {
					first := lp.linkURLBuf[0]
					if first.Type == tokenizer.TextToken && len(first.Value) > 0 && first.Value[0] == '<' {
						if len(first.Value) > 1 {
							lp.linkURLBuf[0].Value = first.Value[1:]
						} else {
							lp.linkURLBuf = lp.linkURLBuf[1:]
						}
					}
				}
				continue
			}
			p.consume(1)
			lp.linkURLBuf = append(lp.linkURLBuf, tok)
			continue
		}

		if len(lp.linkURLBuf) == 0 {
			if tok.Type == tokenizer.TabToken {
				p.consume(1)
				continue
			}
			if tok.Type == tokenizer.TextToken && isPureWhitespaceToken(tok) {
				p.consume(1)
				continue
			}
			if tok.Type == tokenizer.TextToken && len(tok.Value) > 0 && tok.Value[0] == '<' {
				lp.urlAngleBracket = true
				p.consume(1)
				lp.linkURLBuf = append(lp.linkURLBuf, tok)
				continue
			}
		}

		if tok.Type == tokenizer.BackslashToken {
			p.consume(1)
			if len(p.buf) > 0 {
				next := p.buf[0]
				if len(next.Value) > 0 && isASCIIPunctByte(next.Value[0]) {
					escaped := next.Value[:1]
					if len(next.Value) > 1 {
						p.buf[0].Value = next.Value[1:]
					} else {
						p.consume(1)
					}
					lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: escaped})
					continue
				}
			}
			lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: "\\"})
			continue
		}

		if tok.Type == tokenizer.LeftParenToken {
			p.consume(1)
			lp.linkURLBuf = append(lp.linkURLBuf, tok)
			lp.urlParenDepth++
			continue
		}

		if tok.Type == tokenizer.RightParenToken {
			if lp.urlParenDepth > 0 {
				p.consume(1)
				lp.linkURLBuf = append(lp.linkURLBuf, tok)
				lp.urlParenDepth--
				continue
			}
			p.consume(1)
			return lp.emitLinkEvent()
		}

		if tok.Type == tokenizer.TabToken || (tok.Type == tokenizer.TextToken && isPureWhitespaceToken(tok)) {
			p.consume(1)
			lp.urlDone = true
			for len(p.buf) > 0 {
				nt := p.buf[0]
				if nt.Type == tokenizer.TabToken || (nt.Type == tokenizer.TextToken && isPureWhitespaceToken(nt)) {
					p.consume(1)
				} else if nt.Type == tokenizer.NewlineToken {
					if lp.urlHadNewline {
						break
					}
					p.consume(1)
					lp.urlHadNewline = true
				} else {
					break
				}
			}
			continue
		}

		if tok.Type == tokenizer.TextToken {
			spaceIdx := -1
			for i, r := range tok.Value {
				if r == ' ' || r == '\t' || (r >= 0x00 && r <= 0x1f) || r == 0x7f {
					spaceIdx = i
					break
				}
			}
			if spaceIdx >= 0 {
				rest := strings.TrimLeft(tok.Value[spaceIdx:], " \t")
				if spaceIdx == 0 && len(lp.linkURLBuf) == 0 {
					if len(rest) > 0 && rest[0] == '<' {
						lp.urlAngleBracket = true
						lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: rest})
					} else if rest != "" {
						lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: rest})
					}
					p.consume(1)
					continue
				}
				if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'' || rest[0] == '(') {
					if spaceIdx > 0 {
						lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: tok.Value[:spaceIdx]})
					}
					p.consume(1)
					lp.urlDone = true
					p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: rest})
					lp.skipURLWhitespace()
					continue
				}
				if rest != "" {
					return lp.flushLinkAsText()
				}
				if spaceIdx > 0 {
					lp.linkURLBuf = append(lp.linkURLBuf, tokenizer.Token{Type: tokenizer.TextToken, Value: tok.Value[:spaceIdx]})
				}
				p.consume(1)
				lp.urlDone = true
				lp.skipURLWhitespace()
				continue
			}
		}
		p.consume(1)
		lp.linkURLBuf = append(lp.linkURLBuf, tok)
	}
	return nil
}

// processLinkRefLabel handles reference links: [text][label] and [text][].
func (lp *linkParser) processLinkRefLabel() []Event {
	p := lp.p
	p.consume(1)

	var refLabel strings.Builder
	for len(p.buf) > 0 {
		tok := p.buf[0]
		if tok.Type == tokenizer.NewlineToken {
			p.prependTokens(tokenizer.Token{Type: tokenizer.LeftBracketToken, Value: "["})
			return lp.flushLinkAsText()
		}
		if tok.Type == tokenizer.RightBracketToken {
			p.consume(1)
			var textBuilder strings.Builder
			for _, t := range lp.linkBuf {
				textBuilder.WriteString(t.Value)
			}
			textStr := resolveEntities(textBuilder.String())
			labelStr := resolveEntities(refLabel.String())
			lp.linkBuf = nil
			lp.linkBracketConsumed = false
			p.state = NormalState
			if len(textStr) > 0 {
				p.prevChar = textStr[len(textStr)-1]
			}
			return []Event{{Type: LinkRefEvent, Value: textStr, URL: labelStr}}
		}
		p.consume(1)
		refLabel.WriteString(tok.Value)
	}

	return nil
}

// collectTitle parses a link title: "title", 'title', or (title).
func (lp *linkParser) collectTitle(q byte) []Event {
	p := lp.p
	tok := p.buf[0]
	p.consume(1)

	var titleBuilder strings.Builder

	closeQ := q
	if q == '(' {
		closeQ = ')'
	}

	if tok.Type == tokenizer.TextToken && len(tok.Value) > 1 {
		rest := tok.Value[1:]
		closeIdx := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				i++
				continue
			}
			if rest[i] == closeQ {
				closeIdx = i
				break
			}
		}
		if closeIdx >= 0 {
			titleBuilder.WriteString(rest[:closeIdx])
			trail := rest[closeIdx+1:]
			if trail != "" {
				p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: trail})
			}
			lp.urlDone = true
			title := resolveEntities(titleBuilder.String())
			lp.skipURLWhitespace()
			if len(p.buf) > 0 && p.buf[0].Type == tokenizer.RightParenToken {
				p.consume(1)
				events := lp.emitLinkEvent()
				if len(events) > 0 {
					events[0].Title = title
				}
				return events
			}
			return lp.flushLinkAsText()
		}
		titleBuilder.WriteString(rest)
	}

	for len(p.buf) > 0 {
		t := p.buf[0]
		if t.Type == tokenizer.NewlineToken {
			if lp.urlHadNewline {
				return lp.flushLinkAsText()
			}
			p.consume(1)
			lp.urlHadNewline = true
			continue
		}
		if t.Type == tokenizer.BackslashToken {
			p.consume(1)
			if len(p.buf) > 0 {
				next := p.buf[0]
				if len(next.Value) > 0 {
					nextByte := next.Value[0]
					if q == '(' {
						if nextByte == closeQ {
							p.consume(1)
							titleBuilder.WriteByte(closeQ)
							continue
						}
					} else if next.Type == tokenizer.TextToken && nextByte == closeQ {
						p.consume(1)
						titleBuilder.WriteByte(closeQ)
						continue
					}
				}
			}
			titleBuilder.WriteByte('\\')
			continue
		}
		p.consume(1)
		consumed := false
		if q == '(' {
			if t.Type == tokenizer.RightParenToken {
				consumed = true
			}
		} else if t.Type == tokenizer.TextToken && len(t.Value) > 0 && t.Value[0] == closeQ {
			consumed = true
			if len(t.Value) > 1 {
				p.prependTokens(tokenizer.Token{Type: tokenizer.TextToken, Value: t.Value[1:]})
			}
		}
		if consumed {
			break
		}
		titleBuilder.WriteString(t.Value)
	}

	lp.urlDone = true
	title := resolveEntities(titleBuilder.String())
	lp.skipURLWhitespace()
	if len(p.buf) > 0 && p.buf[0].Type == tokenizer.RightParenToken {
		p.consume(1)
		events := lp.emitLinkEvent()
		if len(events) > 0 {
			events[0].Title = title
		}
		return events
	}

	return lp.flushLinkAsText()
}

// skipURLWhitespace skips whitespace, tabs, and newlines after the URL
// and before the title or closing paren.
func (lp *linkParser) skipURLWhitespace() {
	p := lp.p
	for len(p.buf) > 0 {
		nt := p.buf[0]
		if nt.Type == tokenizer.TabToken || (nt.Type == tokenizer.TextToken && isPureWhitespaceToken(nt)) {
			p.consume(1)
		} else if nt.Type == tokenizer.NewlineToken {
			if lp.urlHadNewline {
				break
			}
			p.consume(1)
			lp.urlHadNewline = true
		} else {
			break
		}
	}
}

// emitLinkEvent builds and emits a LinkEvent from the collected link text and URL,
// resets link state, and transitions back to NormalState.
func (lp *linkParser) emitLinkEvent() []Event {
	p := lp.p
	var textBuilder strings.Builder
	for _, t := range lp.linkBuf {
		textBuilder.WriteString(t.Value)
	}
	var urlBuilder strings.Builder
	for _, t := range lp.linkURLBuf {
		urlBuilder.WriteString(t.Value)
	}
	textStr := resolveEntities(textBuilder.String())
	urlStr := resolveEntities(urlBuilder.String())

	lp.reset()
	p.state = NormalState
	if len(textStr) > 0 {
		p.prevChar = textStr[len(textStr)-1]
	}
	return []Event{{Type: LinkEvent, Value: textStr, URL: urlStr}}
}

// flushLinkAsText emits the incomplete link as literal text ([text] or [text](url)).
// This is called when link syntax is invalid or when the document ends during link parsing.
func (lp *linkParser) flushLinkAsText() []Event {
	p := lp.p
	var events []Event
	events = append(events, Event{Type: TextEvent, Value: "["})
	for _, tok := range lp.linkBuf {
		events = append(events, Event{Type: TextEvent, Value: resolveEntities(tok.Value)})
	}
	events = append(events, Event{Type: TextEvent, Value: "]"})
	if p.state == LinkURLState {
		events = append(events, Event{Type: TextEvent, Value: "("})
		for _, tok := range lp.linkURLBuf {
			events = append(events, Event{Type: TextEvent, Value: resolveEntities(tok.Value)})
		}
		lp.linkURLBuf = nil
	}
	lp.linkBuf = nil
	lp.linkBracketConsumed = false
	wasLinkURL := p.state == LinkURLState
	p.state = NormalState
	if wasLinkURL {
		p.prevChar = ')'
	} else {
		p.prevChar = ']'
	}
	return events
}
