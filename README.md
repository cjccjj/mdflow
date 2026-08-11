# mdflow

#### mdflow is a small, fast, true 〰️streaming〰️ Markdown-to-terminal renderer

*Parser derived from the proven [MD4C](https://github.com/mity/md4c), restructured for true streaming.*

### See it in action

![Demo](assets/demo.gif)

## Why mdflow

- Real-time Streaming - must have for AI
- CommonMark + GFM tested - tested against CommonMark spec (652 examples)
- Very Fast
- Flat memory - use little memory and stays flat as input grows
- Lightweight - tiny binary easy to embed in an app or resource-constrained devices

### Comparison

| | | <ins> mdflow </ins> | streamdown | mdcat | glow (glamour)  |
|:--- |:---|:---|:---|:---|:---|
| Capabilities | Streaming | ✅ | ✅ | ❌ | ❌ |
| | Buffering | Single line | Single line | Whole doc | Whole doc |
| | CommonMark | ✅ Full | ❌ | ✅ Full | ❌ |
| | GFM tables | ✅ | ⚠️ Limited | ⚠️ Limited | ✅ |
| Render time | 1 MB input | $\color{green}{\mathsf{0.03\ s}}$ | $\mathsf{8.09\ s}$ | $\mathsf{0.34\ s}$ | $\mathsf{0.64\ s}$ |
| | 10 MB input | $\color{green}{\mathsf{0.37\ s}}$ | $\mathsf{78.72\ s}$ | $\mathsf{2.85\ s}$ | $\mathsf{12.33\ s}$ |
| | 100 MB input | $\color{green}{\mathsf{3.56\ s}}$ | — | $\mathsf{27.38\ s}$ | — |
| Peak RAM | 1 MB input | $\color{green}{\mathsf{2.3\ MB}}$ | $\mathsf{24.8\ MB}$ | $\mathsf{48.7\ MB}$ | $\mathsf{118.3\ MB}$ |
| | 10 MB input | $\color{green}{\mathsf{2.3\ MB}}$ | $\mathsf{24.8\ MB}$ | $\mathsf{102.3\ MB}$ | $\mathsf{972.8\ MB}$ |
| | 100 MB input | $\color{green}{\mathsf{2.4\ MB}}$ | — | $\mathsf{624.6\ MB}$ | — |
| Binary | Language | C | Python | Rust | Go |
| | Size | $\color{green}{\mathsf{284.8\ KB}}$ | — | $\mathsf{11.0\ MB}$ | $\mathsf{17.2\ MB}$ |

<sub>1. mdflow handles Reference definitions pragmatically because they cannot be fully streamed.</sub> <br>
<sub>2. streamdown and glow did not finish the 100 MB test within 100 seconds.</sub> <br>
<sub>3. Input consisted of mixed Markdown. Performance varies by content.</sub> <br>
<sub>4. Benchmarked on GitHub Actions (Ubuntu 24.04, AMD EPYC 7763, 4 vCPUs)</sub> <br>
<sub>5. mdflow v0.1.0, streamdown 0.36.6, glow v2.1.2, and mdcat v2.7.1.</sub>

## Features

- **Full CommonMark + GFM** - tables, strikethrough, task lists, autolinks, footnotes, and admonitions.
- **Extras on top** - highlights, spoilers, and underlines.
- **Tables** - box-drawing borders, alignment, automatic layout, and wrapping that preserves styling.
- **Unicode-correct** - tested with CJK and emoji.
- **Syntax highlighting** - simplified, generic highlighting using five styles, applied only to code blocks whose fence label names a major programming language (unlabeled and unknown labels render plain).
- **Inline HTML** - tags and entities styled for the terminal, with comments hidden.
- **HTML blocks** - raw HTML scanned and styled, entities decoded, comments hidden, and Markdown inside left literal.

### Limitations / TBD

- **Reference links** - cannot be resolved inline while streaming. mdflow shows references as dim hints, then collects and prints their definitions at the end.
- **Syntax highlighting** - provides lightweight, generic highlighting for readability rather than full language-specific palettes; code blocks without a recognized major-language label are left unstyled.
- **No pager or TUI** - mdflow renders; scrolling is left to `more` or `less -R`.
- **Customizable styling** - a clean, separate theme layer; user-facing theme configuration is still to come.

## Install

For Linux and macOS, installs to `~/.local/bin`:

```
curl -fsSL https://raw.githubusercontent.com/cjccjj/mdflow/main/install.sh | sh
```

Or download a binary from [Releases](https://github.com/cjccjj/mdflow/releases) (Linux x86_64/arm64, macOS arm64).

## Use

Pipe Markdown into mdflow - logs, files, LLM output, anything that streams:

```
cat README.md | mdflow
llm "show me a markdown demo" | mdflow
```

Or read a file with a redirect (mdflow takes stdin only, no filename arguments):

```
mdflow < README.md
```

For paging long documents, pipe to `more` or `less -R` - they render ANSI colors and stream input as it arrives. Plain `less` shows raw escape codes.

```
mdflow < README.md | more
```
Typewriter (paced) output is on when live streams are detected, making the output appear smoother. It adds no delay before the first character appears, and may add a total delay during streaming less than 1 second.

Typewriter mode will not be on for non-live-stream input and can be forcibly disabled with:

```bash
--typewriter-off
```

## Development

### Library

mdflow is also a small C library - three functions, libc only:

```c
#include "mdflow.h"

mdflow_t* mf = mdflow_open(80, my_output_callback, my_userdata);
mdflow_write(mf, "# Hello\n", 8);
mdflow_close(mf);
```

`mdflow_open()` takes a terminal width (0 = unlimited), an output callback, and userdata. 
Feed chunks with `mdflow_write()` - styled output arrives through the callback as blocks close. `mdflow_close()` flushes and frees everything. Link with `-lmdflow`.

### Build

```sh
cmake -S . -B build
cmake --build build
```

Building requires only a C compiler and CMake. GCC and Clang builds enable `-Wall`, `-Wextra`, and `-Wshadow`.

### Architecture

```
stdin → parser (streaming) → renderer (streaming) → stdout
```
Two components, both streaming, bundled into one library - no AST, no document buffer.

**Parser (MD4CS)** - [MD4C](https://github.com/mity/md4c) is a fast SAX-like Markdown parser with a flat-buffer design, though it still buffers in full and fires all callbacks at the end, because many features depend on input that has not arrived yet.
When analyzed feature by feature, some require only one line of lookahead; some require unbounded lookahead but style can be determined earlier. mdflow's parser MD4CS, builds on top of MD4C reconstructs the features that require handling to enable true streaming. It can thus emit callbacks in the first pass and free memory immediately.

**Renderer (md4cs-ansi)** - maps parser callbacks to styled terminal output.

Sub-modules:
- `highlight.c` - single-pass lightweight code highlighting, derived from [microlight](https://github.com/asvd/microlight) (MIT)
- `html.c` - HTML tag/entity scanner that styles raw HTML
- tables - box-drawing layout that redraws when column widths change mid-stream

### Acknowledgments

- [MD4C](https://github.com/mity/md4c) by Martin Mitáš (MIT) - the parser mdflow is built on.
- [microlight](https://github.com/asvd/microlight) by asvd (MIT) - the code highlighter is derived from it.

### License

MIT. See [LICENSE.md](LICENSE.md) and the license comments in the source file headers.
