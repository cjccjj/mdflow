package markdown

import (
	"bytes"
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
	n := len(data)
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		var chunk []byte
		if idx >= 0 {
			chunk = data[:idx+1]
			data = data[idx+1:]
		} else {
			chunk = data
			data = nil
		}
		tokens := tokenizer.Tokenize(chunk)
		events := p.parser.Parse(tokens)
		for _, e := range events {
			if err := p.writer.Handle(e); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

func (p *Pipeline) Flush() error {
	events := p.parser.Flush()
	for _, e := range events {
		if err := p.writer.Handle(e); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pipeline) Reset() {
	p.writer.ResetStyles()
	p.parser.Reset()
}

func (p *Pipeline) Close() error {
	if err := p.Flush(); err != nil {
		return err
	}
	for _, e := range p.parser.CloseStates() {
		if err := p.writer.Handle(e); err != nil {
			return err
		}
	}
	p.Reset()
	return nil
}
