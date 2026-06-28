package parser

import (
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

// setextParser handles setext headings (=== underline for H1, --- underline for H2).
// It buffers a candidate line and validates the next line is a proper underline.
type setextParser struct {
	setextWaiting bool
	setextBuf     []tokenizer.Token
	p             *Parser
}

func newSetextParser(p *Parser) *setextParser {
	return &setextParser{p: p}
}

func (sp *setextParser) reset() {
	sp.setextWaiting = false
	sp.setextBuf = nil
}

// flushSetext emits the buffered setext content. Used by Flush/CloseStates.
func (sp *setextParser) flushSetext() []Event {
	p := sp.p
	content := stripTrailingWhitespace(sp.setextBuf)
	events := p.parseInlineLine(content)
	events = append(events, Event{Type: NewlineEvent})
	sp.setextBuf = nil
	sp.setextWaiting = false
	return events
}
