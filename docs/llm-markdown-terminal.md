# Render LLM Markdown in a Terminal

mdflow is a streaming Markdown-to-ANSI terminal renderer for live LLM output.
Pipe an LLM command into mdflow to see formatted Markdown while the response
is still being generated.

## Pipe LLM output into mdflow

Use mdflow after any command that writes Markdown to standard output:

```sh
your-llm-command | mdflow
```

This works with local models, hosted-model CLI tools, shell scripts, and other
programs that produce Markdown responses.

## Render Ollama Markdown in a terminal

Ask Ollama for a Markdown response and pipe it into mdflow:

```sh
ollama run llama3 "Respond using Markdown" | mdflow
```

## Render output from Simon Willison's llm

The `llm` command can use mdflow as its terminal Markdown renderer:

```sh
llm "Explain mutexes using Markdown" | mdflow
```

## Render Markdown from curl

For an endpoint that returns Markdown directly, pipe the response to mdflow:

```sh
curl -sL https://example.com/markdown-response | mdflow
```

For a server-sent events endpoint whose `data:` lines contain plain Markdown,
extract those lines before rendering:

```sh
curl -Ns https://example.com/stream \
  | sed -n 's/^data: //p' \
  | mdflow
```

The extraction command depends on the service. JSON event payloads need to be
decoded before their text is sent to mdflow.

## Preserve command status in a shell pipeline

This Bash wrapper preserves the LLM command's failure status and otherwise
returns mdflow's status:

```bash
ask() {
  command your-llm-command "$@" | mdflow
  local status=("${PIPESTATUS[@]}")
  return "$((status[0] ? status[0] : status[1]))"
}
```

## Why use mdflow for LLM output?

- Formatted output can begin before the response is complete.
- Markdown is rendered directly for ANSI-compatible terminals.
- The complete response is not repeatedly reprocessed as it grows.
- mdflow is a lightweight C CLI and embeddable library.

See the [streaming Markdown renderer overview](streaming-markdown-renderer.md),
the [Markdown renderer FAQ](markdown-renderer-faq.md), and the
[installation instructions](../README.md#install).
