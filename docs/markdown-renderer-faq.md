# Markdown Renderer FAQ

This FAQ covers streaming Markdown rendering, Markdown-to-ANSI terminal
output, and using mdflow with live LLM responses.

## What is mdflow?

mdflow is a streaming Markdown-to-ANSI renderer for terminals. It is a C
command-line tool and an embeddable C library.

## What is a Markdown-to-ANSI renderer?

A Markdown-to-ANSI renderer converts Markdown into styled terminal output.
ANSI escape sequences control colors, emphasis, hyperlinks, and other terminal
presentation without requiring a graphical interface.

## Is mdflow a streaming Markdown renderer?

Yes. mdflow starts rendering Markdown while input is still arriving instead of
waiting for the complete response.

## Does mdflow wait for EOF?

No. For streamable content, mdflow can emit terminal output before stdin is
closed.

## Does mdflow reparse the complete response?

No. mdflow does not repeatedly reprocess everything received so far or keep a
full-document input buffer. Some unresolved Markdown constructs still need
limited lookahead.

## Can mdflow render streaming LLM output?

Yes. Pipe an LLM command that produces Markdown into mdflow:

```sh
your-llm-command | mdflow
```

This lets formatted output appear while the response is being generated.

## How is mdflow different from Glow, mdcat, and Streamdown?

The relevant differences include output before EOF, buffering, reparsing,
terminal output, speed, memory use, and binary size. See the
[README comparison](../README.md#mdflow-vs-streamdown-mdcat-and-glow) for the
current table and benchmark context.

## Does mdflow support CommonMark and GFM?

mdflow is tested against all 652 CommonMark specification examples and
supports documented GFM features including tables, task lists,
strikethrough, and autolinks. See the
[compatibility notes](../README.md#commonmark--gfm-support-extensions-and-limitations)
for known streaming limitations.

## Is mdflow a C library?

Yes. mdflow includes a small C library as well as the command-line program.
The library accepts streamed Markdown writes and sends rendered output through
an application callback.

## Is mdflow a pager or a TUI?

No. mdflow renders Markdown to the terminal. Use it with `more` or `less -R`
when you need paging.

## Is this mdflow related to other projects with the same name?

This project is `cjccjj/mdflow`, a native C Markdown-to-terminal renderer. It
is unrelated to similarly named Markdown workflow or agent packages.

## How do I install mdflow?

On Linux and macOS, use the installer or download a binary from
[GitHub Releases](https://github.com/cjccjj/mdflow/releases). See the
[installation instructions](../README.md#install).
