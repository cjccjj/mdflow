# mdflow test suite

## Run order

Tests form a dependency chain. Run in this order to catch failures early:

```
1. verify-batch-parity.py    [PARSER] batch callbacks == MD4C (core + full flags)
2. verify-batch-source.py    [PARSER] batch build contains zero streaming code
3. run-parser-diff.py        [PARSER + REGRESSION] streaming vs batch callbacks
4. run-stream-pipeline.py    [PARSER] low-level streaming API correctness
5. run-stream-spec.py        [PARSER + REGRESSION] streaming feature behavior
6. run-ansi-tests.py         [RENDER SMOKE] CLI/parser/renderer no-crash checks
7. run-render-tests.py       [RENDER GOLDEN] exact and stripped ANSI output
8. run-eof-regress.py        [REGRESSION] CLI EOF/flush behavior
9. run-typewriter-tests.py   [REGRESSION] typewriter unit + CLI behavior
```

The labels identify the primary responsibility of each step. “Regression” means
a focused test that preserves a previously reviewed behavior; it can belong to
the parser, renderer, or CLI layer.

Step 1 is the prerequisite. If it fails, steps 2–5 are invalid (the batch oracle
has diverged from MD4C). Steps 1–5 are parser-focused tests. Steps 6–9 exercise
the CLI, renderer, and end-to-end behavior built on top of the parser.

## What each test validates

### Step 1: verify-batch-parity.py

```
python3 test/verify-batch-parity.py
python3 test/verify-batch-parity.py --full-flags
```

Builds two batch-mode binaries (our `md4cs.c` vs. MD4C's `md4c.c`) and runs
both on all spec examples. Must produce identical output. The default run
uses `flags=0` (core CommonMark); `--full-flags` repeats the check with the
mdflow CLI's full extension flag set (tables, permissive autolinks,
footnotes, admonitions, strikethrough, task lists, and the custom span
extensions), which is what catches streaming edits leaking into shared
parser code. CI runs both modes. Batch mode is for testing only — it is
never shipped with mdflow. Verifies that our batch code has not diverged
from MD4C — accidental edits to `#else` blocks are caught here.

Downloads `md4c.c` and `md4c.h` from the MD4C GitHub repository at the
specified ref (default: `master`). No local git remote or clone needed.

To compare against a specific MD4C version:

```
python3 test/verify-batch-parity.py --md4c-ref release-0.5.3
```

CI pins the MD4C master snapshot that mdflow currently tracks (see
`.github/workflows/ci-build.yml`) so upstream changes do not break CI
unexpectedly. Update the pin together with any parser sync.

### Step 2: verify-batch-source.py

```
python3 test/verify-batch-source.py
```

Preprocesses `src/md4cs.c` without `MD4C_STREAMING` and asserts the batch
build keeps the upstream 3-argument `md_process_inlines()` and contains
none of the streaming-only helpers, windowed walker state, or streaming-only
enum values. This is the static half of rule #1 (batch sees zero streaming
code); step 1 is the behavioral half.

### Step 3: run-parser-diff.py

```
python3 test/run-parser-diff.py              # all specs
python3 test/run-parser-diff.py -s <spec>    # single spec
python3 test/run-parser-diff.py -v           # print every streaming diff
python3 test/run-parser-diff.py --update-manifest   # re-record after review
```

Builds two binaries from `parser-diff.c` (batch + streaming) and compares
callback sequences for every spec example with `flags=0`. Most examples
match; the rest diverge because streaming intentionally emits differently in
documented cases. This step does not claim the streaming side is correct just
because a diff is "expected" — correctness is established by review:

- **Every divergence was inspected one by one during development.** For each
  divergent example we confirmed the input exercises a documented streaming
  behavior — reference links emitted as hints instead of resolved URLs,
  reference definitions and footnotes deferred to flush, the tight/loose
  first-item tradeoff, multi-line setext last-line lookahead,
  `[`-pinned paragraphs, or a similar hold-boundary rule — and then read the
  streaming event stream against the design in `DEVELOPMENT.md` to verify it
  is the intended output.
- **The reviewed set is recorded** in `test/parser-diff-expected.txt` (one
  `spec#example category` per line; categories are ref-hint,
  flush-ref-defs, footnotes, setext, p-order, tight-loose, or other). Exact
  batch and streaming callback counts and lines for those entries are stored
  in `test/parser-diff-expected.json`. The runner fails if a divergence appears
  outside the manifest, a reviewed divergence disappears, or any stored
  callback output changes. The diff count is therefore the size of the
  reviewed manifest, and the callback snapshot is the reviewed output contract.
- **Re-verifying or extending the set**: run `-v`, inspect each diff, classify
  it against the documented streaming behaviors, confirm the streaming output
  by reading it, then re-record with `--update-manifest` (full runs only).
  Never update the manifest or snapshots without reviewing every new, removed,
  or changed entry. A repeat run with the same reviewed divergence and the same
  callback output passes automatically.

The runner still treats a crash, timeout, or other hard error as a failure in
every mode; `-s <spec>` runs a single spec and skips the manifest/snapshot
check.

### Step 4: run-stream-pipeline.py

```
python3 test/run-stream-pipeline.py
```

Tests the streaming API (`md_stream_init/feed/flush/finish`) directly:
- CRLF across chunk boundaries
- Byte-by-byte feed equivalence
- Buffer compaction (footnote defs survive compaction)
- Empty input (DOC open/close only)
- Flush emits trailing partial line
- Fenced code and HTML line-by-line early emission
- Indented code emission across chunk boundaries
- Table conversion followed by paragraph emission after compaction

Requires: `gcc` (compiles `stream-event.c` internally).

### Step 5: run-stream-spec.py

Build the current event binary first; `run-stream-spec.py` does not compile
the program itself:

```
gcc -DMD4C_STREAMING -Wall -Wextra -Wshadow -Wdeclaration-after-statement -g -I src -o test/stream-event test/stream-event.c src/md4cs.c
```

```
python3 test/run-stream-spec.py -s test/stream-footnote.txt -p "test/stream-event --footnotes"
python3 test/run-stream-spec.py -s test/stream-ref.txt -p test/stream-event
python3 test/run-stream-spec.py -s test/stream-para.txt -p test/stream-event
python3 test/run-stream-spec.py -s test/stream-para.txt -p "test/stream-event --chunk 1"
python3 test/run-stream-spec.py -s test/stream-para.txt -p "test/stream-event --chunk 40"
```

Runs `stream-event.c` against streaming-specific spec files. Validates
footnote rendering (definition order, id assignment), reference link
hint emission, and incremental paragraph emission (cross-line spans,
setext lookahead, `[`-pinned paragraphs, quote paragraphs, unclosed code
spans). The `--chunk N` runs feed in fixed-size chunks so buffer
compaction runs between feeds; the event stream must be identical to the
single-shot feed.

Footnotes are a known streaming compromise: every `[^label]` emits a
footnote reference even without a matching definition (the definition
hashtable is unavailable during feed), and all definitions are emitted at
flush in definition order, including unreferenced ones. Batch instead
emits only referenced definitions in reference order. This is pinned by
`stream-footnote.txt`, not a regression target.

### Step 6: run-ansi-tests.py

```
python3 test/run-ansi-tests.py -p build/mdflow/mdflow
```

Runs `mdflow` on all spec examples (930+ inputs). Verifies:
- Exit code 0 (no crashes)
- Non-empty output for non-empty input

Does not verify output correctness — that is covered by parser-diff (step 3,
parser callbacks) and the render goldens (step 7, actual ANSI output).

The ANSI smoke suite also includes highlighter regression checks: keyword
SGRs, whole-word matching (non-keywords stay plain), string/comment SGRs,
cross-line strings and comments, division-vs-regex, and content
preservation (`strip_sgr(out) == code text`).

It also checks blockquote styling/bar placement, table safety with many or
empty cells, preferred wrapping and style continuation, and terminal-width
layout bounds.

### Step 7: run-render-tests.py

```
python3 test/run-render-tests.py -p build/mdflow/mdflow
```

Golden-output tests for the renderer (see `test/render-golden.txt`):

The renderer goldens focus on features with renderer-side logic, such as
tables, HTML, syntax highlighting, terminal widths, wrapping, and redraw
state. Simple callback-to-style mappings are also covered, but are lighter
snapshot checks rather than independent semantic oracles. The golden output
is an internal contract for the default theme, not a third-party oracle.

- **Exact mode** — expected output contains `␛` (U+241B, standing for the
  ESC byte): the output must match byte-for-byte, SGR codes included.
  Freezes the default theme's styling contract.
- **Stripped mode** — no `␛` in expected: SGR codes are stripped and the
  visible text must match.

Covers tables (borders, alignment, CJK/emoji widths, cursor-up redraw),
emphasis, code blocks/highlighting, links, entities (incl. malformed
numeric refs), HTML blocks (comments, CDATA, PIs, declarations, inline
tags), admonitions, blockquotes, lists (incl. deep nesting), footnotes,
reference links, task lists, spoilers, and the regression cases from past
bugs. Expected outputs are a contract with the default theme: theme
changes must update the goldens deliberately.

The CLI epilogue (trailing newline + SGR reset appended by
`mdflow/mdflow.c`) is stripped by the runner and must not appear in the
golden file.

### Step 8: run-eof-regress.py

```
python3 test/run-eof-regress.py -p build/mdflow/mdflow
```

Feeds exact bytes without a trailing newline to the CLI and compares the
visible text: a reference use after a consumed definition, blockquote text
after a definition, and the last list item after a definition. Guards the
flush path where a consumed streamed paragraph used to leave stale pending
lines behind, dropping the trailing line's content.

### Step 9: run-typewriter-tests.py

```
python3 test/run-typewriter-tests.py
```

Compiles `typewriter-test.c` against `mdflow/typewriter.c` and runs the
deterministic unit tests (tokenizer grapheme/ANSI grouping, pacer burst and
timeout rules, input-state rate detection) with a fake clock and a
recording writer, then exercises the built CLI end to end:

- typewriter pacing is on by default; `--help` lists only `--typewriter-off`
  and `--report`, and old tuning flags are rejected
- regular-file redirect bypasses with byte-identical output
- piped input is byte-identical after pacing
- tiny single-burst input stays passthrough (unclear means bypass)
- a slowly streamed input is measured once, switches to typewriter, and
  locks for the whole run
- sparse token-by-token streams and a long first-token wait still measure
  as slow (8 chars of evidence, span measured from the first byte)
- the fixed 1 second delay cap bursts overdue lines and reports the timeout
  count
- a long line paced while short lines arrive one at a time keeps the queue
  intact (regression: pacer queue must grow at `head + count == cap`)
- trailing empty lines are coalesced into the newest visible line and never
  grow the queue (1000+ blank lines stay a single run)
- pacing is always adaptive with `input_meta=on`; passthrough is locked for
  fast measured rates, file redirects, and EOF before measurement
- clustered reads (a big read right after a small one, caused by pacer
  backpressure) do not inflate the measured rate
- `test/typewriter-stress.md` (mixed Markdown, blank lines, and a long
  single line) matches the plain path

Requires: `gcc` (compiles `typewriter-test.c` internally).

## Spec data files (.txt)

| File | Source | Tests used by |
|---|---|---|
| `spec.txt` | CommonMark 0.31.2 | parser-diff, ansi-smoke |
| `spec-tables.txt` | GFM tables extension | ^ |
| `spec-strikethrough.txt` | Strikethrough extension | ^ |
| `spec-tasklists.txt` | Task list extension | ^ |
| `spec-footnotes.txt` | Footnote extension | ^ |
| `spec-highlight.txt` | Highlight extension | ^ |
| `spec-spoilers.txt` | Spoiler extension | ^ |
| `spec-subscripts.txt` | Subscript extension | ^ |
| `spec-superscripts.txt` | Superscript extension | ^ |
| `spec-underline.txt` | Underline extension | ^ |
| `spec-wiki-links.txt` | Wiki links extension | ^ |
| `spec-latex-math.txt` | LaTeX math extension | ^ |
| `spec-hard-soft-breaks.txt` | Hard/soft break extension | ^ |
| `spec-permissive-autolinks.txt` | Permissive autolinks | ^ |
| `spec-admonitions.txt` | Admonition extension | ^ |
| `coverage.txt` | Edge case coverage | ^ |
| `regressions.txt` | Historical bug regressions | ^ |
| `typewriter-stress.md` | Typewriter mixed/blank-line/long-line fixture | typewriter |
| `stream-footnote.txt` | Streaming footnote behavior | stream-spec |
| `stream-ref.txt` | Streaming reference links | stream-spec |
| `render-golden.txt` | ANSI renderer golden output | render-tests |
| `parser-diff-expected.txt` | Reviewed streaming/batch divergence manifest | parser-diff |
| `parser-diff-expected.json` | Exact reviewed parser callback snapshots | parser-diff |

## Architecture diagram

```
verify-batch-parity.py
    ↓        builds        ↓
┌───────────────┐   ┌─────────────────────┐
│  our md4cs.c  │   │  MD4C md4c.c        │   ← both batch mode
│  (batch)       │   │  (batch)            │
└───────┬───────┘   └──────────┬──────────┘
        │ parser-diff.c        │ parser-diff.c
        ↓                      ↓
   output (a)       ==      output (b)        ← must be identical

run-parser-diff.py
    ↓        builds        ↓
┌───────────────┐   ┌─────────────────────┐
│  our md4cs.c  │   │  our md4cs.c        │   ← streaming vs batch
│  (streaming)   │   │  (batch)            │
└───────┬───────┘   └──────────┬──────────┘
        │ parser-diff.c        │ parser-diff.c
        ↓                      ↓
   stream output      ≅     batch output       ← 650 match, 308 reviewed diffs

run-ansi-tests.py
    ↓ runs
┌───────────────┐
│    mdflow     │ ← libmd4cs + md4cs-ansi
└───────┬───────┘
        ↓
  exit code 0, non-empty output for all 930+ examples
```
