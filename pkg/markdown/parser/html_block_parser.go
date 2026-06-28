package parser

// htmlBlockParser handles HTML block detection and processing.
type htmlBlockParser struct {
	htmlBlockType int
	htmlIndent    int
	p             *Parser
}

func newHTMLBlockParser(p *Parser) *htmlBlockParser {
	return &htmlBlockParser{p: p}
}

func (hp *htmlBlockParser) reset() {
	hp.htmlBlockType = 0
	hp.htmlIndent = 0
}
