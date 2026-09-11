# Streaming Markdown Renderer for LLM Output

mdflow is a true streaming Markdown renderer for live LLM output. It converts
Markdown to styled ANSI terminal output as text arrives, making it a
Markdown-to-ANSI renderer for command-line programs and terminal applications.

## Why use a streaming Markdown renderer?

When an LLM sends a response a piece at a time, waiting for the complete
response makes the terminal feel slow. mdflow starts rendering while the
response is still arriving.

- Output can begin before stdin reaches EOF.
- The complete response is not kept in memory.
- Previously received Markdown is not repeatedly reprocessed.
- The output is designed for terminals and ANSI-compatible applications.
- The project includes a C command-line tool and an embeddable library.

## Use mdflow with streamed Markdown

```sh
your-llm-command | mdflow
```

mdflow also accepts files, logs, and any other Markdown source that can be
sent to standard input.

## mdflow at a glance

- **Category:** streaming Markdown renderer and Markdown-to-ANSI terminal tool
- **Primary use:** live LLM output in command-line programs
- **Input:** Markdown from stdin or streamed library writes
- **Output:** styled ANSI terminal output
- **Language:** C
- **Interfaces:** CLI and embeddable C library
- **Markdown:** CommonMark tested against 652 examples, plus documented GFM features
- **License:** MIT

## mdflow and other terminal Markdown renderers

People comparing mdflow with Glow, mdcat, or Streamdown often care about
output before EOF, full-document buffering, reparsing, terminal output,
performance, and memory use. The [README comparison](../README.md#mdflow-vs-streamdown-mdcat-and-glow)
summarizes those differences and links to the benchmark details already in the
repository.

## CommonMark, GFM, and streaming limitations

mdflow is tested against the CommonMark specification examples and supports
documented GFM features such as tables, task lists, strikethrough, and
autolinks. This is not a claim of perfect support for every Markdown feature
in every streaming situation. Some constructs need future input, and the
[README compatibility notes](../README.md#commonmark--gfm-support-extensions-and-limitations)
describe the known limitations.

## Learn more

- [Markdown renderer FAQ](markdown-renderer-faq.md)
- [mdflow README](../README.md)
- [Install and use mdflow](../README.md#install)
