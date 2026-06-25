# mdflow

Streaming Markdown-to-ANSI renderer for the terminal. Built for AI output pipelines — renders as bytes arrive, works across chunk boundaries, never buffers the full document. Not a full-featured reader like [glow](https://github.com/charmbracelet/glow) / [glamour](https://github.com/charmbracelet/glamour).

## Install

Binary download (linux amd64 / arm64):
```bash
curl -sSfL https://raw.githubusercontent.com/cjccjj/mdflow/main/install.sh | sh
```
Or pick a binary from [releases](https://github.com/cjccjj/mdflow/releases).

Go toolchain (any platform):
```bash
go install github.com/cjccjj/mdflow/cmd/mdflow@latest
```

## Usage

**Pipe** (stdin → stdout):
```bash
echo '# Hello **world**' | mdflow
llm "explain goroutines" | mdflow
```

**Go library**:
```go
import "github.com/cjccjj/mdflow/pkg/markdown"

r := markdown.NewRenderer(os.Stdout)
r.Write([]byte("**bold** and *italic*"))
r.Close()
```

## Supported

`#`–`######` headings, setext headings (`===`/`---`), `**bold**`, `*italic*`, `__bold__`, `_italic_`, `~~strikethrough~~`, `` `inline code` ``, fenced/indented code blocks, `-`/`*` bullets, `1.` ordered lists, `---`/`***`/`___` horizontal rules, `> blockquotes`, `\| tables \|`, inline links `[text](url)`, reference links `[text][label]`, backslash escapes.

## Partially supported

HTML blocks (`<pre>`, `<script>`, `<style>`, `<!-- -->`, `<? ?>`, `<!DOCTYPE>`, `<![CDATA[` — printed dimmed as raw). Link reference definitions (`[label]: url` — shown dimmed). Some HTML block types and multi-line definitions deferred.

## Not supported (printed as-is)

`![images](url)`, `<div>` / `<table>` / other generic HTML blocks, GFM task lists, autolinks.

## Roadmap

*Done:*
- CommonMark 2.1–2.4 (characters, tabs, insecure chars, backslash escapes)
- CommonMark 4.1–4.5, 4.8–4.9 (thematic breaks, ATX headings, setext headings, indented/fenced code blocks, paragraphs, blank lines)
- CommonMark 5.1–5.2 (block quotes, list items with ordered lists)
- CommonMark 6.1–6.3, 6.7 (code spans, emphasis with `_`, inline links, hard line breaks)
- GFM: tables, strikethrough

*Planned:*
- Terminal width wrapping, syntax highlighting in code blocks

*Deferred / not practical for streaming:*
- HTML blocks types 6-7 (4.6), raw HTML inline (6.6), nested lists (5.3), full emphasis algorithm (6.2), autolinks (6.5), images (6.4)

## Compliance

mdflow aims to cover CommonMark 0.31.2 and commonly-used GFM extensions. Coverage: ~78% of spec sections supported or partially supported. Streaming architecture means some features (shortcut reference links, nested structures) are deliberately omitted or simplified.

## API

```go
import (
    "github.com/cjccjj/mdflow/pkg/markdown"
    "github.com/cjccjj/mdflow/pkg/markdown/render"
)

r := markdown.NewRenderer(os.Stdout)

r.Write(data)   // parse & render chunk
r.Flush()       // drain buffered tokens (keep state)
r.Reset()       // reset for a new document
r.Close()       // flush + close open styles + reset

// Custom theme
t := render.DefaultTheme
t.Bold = render.Style{Prefix: "\033[1;31m", Suffix: "\033[0m"}
r := markdown.NewRenderer(os.Stdout, markdown.WithTheme(t))
```
