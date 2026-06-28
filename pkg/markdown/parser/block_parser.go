package parser

import "github.com/cjccjj/mdflow/pkg/markdown/tokenizer"

// blockParser holds the block-level parsing state shared across heading,
// code block, blockquote, and list constructs.
type blockParser struct {
	headerLvl          int
	fenceLen           int
	fenceChar          tokenizer.TokenType
	codeBlockFirst     bool
	codeBlockIndent    int
	blockquoteHadBlank bool
	p                  *Parser
}

func newBlockParser(p *Parser) *blockParser {
	return &blockParser{p: p}
}

func (bp *blockParser) reset() {
	bp.headerLvl = 0
	bp.fenceLen = 0
	bp.fenceChar = 0
	bp.codeBlockFirst = false
	bp.codeBlockIndent = 0
	bp.blockquoteHadBlank = false
}
