package markdown

import (
	"io"

	"github.com/cjccjj/mdflow/pkg/markdown/parser"
	"github.com/cjccjj/mdflow/pkg/markdown/render"
	"github.com/cjccjj/mdflow/pkg/markdown/tokenizer"
)

type Pipeline struct {
	parser *parser.Parser
	writer *render.Writer
	w      io.Writer
}

func NewPipeline(w io.Writer, theme render.Theme) *Pipeline {
	aw := render.NewAnsiWriter(w)
	wr := render.NewWriter(aw, theme)
	wr.SetLive(render.IsTerminal(w))
	return &Pipeline{
		parser: parser.New(),
		writer: wr,
		w:      w,
	}
}

func (p *Pipeline) Write(data []byte) (int, error) {
	tokens := tokenizer.Tokenize(data)
	events := p.parser.Parse(tokens)
	for _, e := range events {
		p.writer.Handle(e)
	}
	return len(data), nil
}

func (p *Pipeline) Flush() error {
	events := p.parser.Flush()
	for _, e := range events {
		p.writer.Handle(e)
	}
	return nil
}

func (p *Pipeline) Reset() {
	p.parser.Reset()
}

func (p *Pipeline) Close() error {
	p.Flush()
	for _, e := range p.parser.CloseStates() {
		p.writer.Handle(e)
	}
	p.Reset()
	return nil
}
