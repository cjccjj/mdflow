package markdown

import (
	"io"

	"github.com/cjccjj/mdflow/pkg/markdown/render"
)

type Option func(*Renderer)

func WithTheme(theme render.Theme) Option {
	return func(r *Renderer) {
		r.theme = theme
	}
}

type Renderer struct {
	pipeline *Pipeline
	theme    render.Theme
	w        io.Writer
}

func NewRenderer(w io.Writer, opts ...Option) *Renderer {
	r := &Renderer{
		w:     w,
		theme: render.DefaultTheme,
	}
	for _, opt := range opts {
		opt(r)
	}
	r.pipeline = NewPipeline(w, r.theme)
	return r
}

func (r *Renderer) Write(p []byte) (int, error) {
	return r.pipeline.Write(p)
}

func (r *Renderer) Flush() error {
	return r.pipeline.Flush()
}

func (r *Renderer) Reset() {
	r.pipeline.Reset()
}

func (r *Renderer) Close() error {
	return r.pipeline.Close()
}
