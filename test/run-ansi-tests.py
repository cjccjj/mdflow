#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE)
import sys
import re
import glob
import os
import argparse
from subprocess import Popen, PIPE


SGR_RE = re.compile(r'\033\[[0-9;]*[a-zA-Z]')
OSC8_RE = re.compile('\x1b\\]8;;[^\x1b\x07]*(\x1b\\\\|\x07)')


def strip_sgr(text):
    return OSC8_RE.sub('', SGR_RE.sub('', text))


def collapse_ws(text):
    return re.sub(r'\s+', ' ', text).strip()


def parse_spec_examples(specfile):
    line_number = 0
    example_number = 0
    markdown_lines = []
    state = 0
    tests = []

    with open(specfile, 'r', encoding='utf-8', newline='\n') as f:
        for line in f:
            line_number += 1
            l = line.strip()
            if re.match(r"`{32} example( [a-z]{1,})?", l):
                state = 1
            elif state >= 2 and l == "`" * 32:
                state = 0
                example_number += 1
                md = ''.join(markdown_lines).replace('→', '\t')
                tests.append({
                    'markdown': md,
                    'example': example_number,
                    'file': specfile,
                })
                markdown_lines = []
            elif l == ".":
                state += 1
            elif state == 1:
                markdown_lines.append(line)
    return tests


def run_mdflow(md_text, program="mdflow/mdflow", env=None):
    p = Popen([program, "--plain", "--typewriter-off"],
              stdout=PIPE, stdin=PIPE, stderr=PIPE, env=env)
    out, err = p.communicate(input=md_text.encode('utf-8'))
    # Normalize CRLF line endings so exact-string checks stay portable.
    out = out.decode('utf-8').replace('\r\n', '\n')
    return p.returncode, out, err.decode('utf-8')


def run_quote_checks(program):
    """Regression checks: inline spans that open at the start of a
    blockquote line must render behind the │ bar, not before/over it.
    Returns the number of failures."""
    failures = 0

    def run(md):
        rc, out, err = run_mdflow(md, program)
        if rc != 0:
            raise RuntimeError(f"mdflow exit {rc}: {err}")
        return out

    bar = '│'

    # Inline spans must render AFTER the bar, not before or over it.
    out = run('> ==foo== at start\n')
    if out.find(bar) < 0 or out.find('foo') < out.find(bar):
        print(f"FAIL [quote-highlight]: {out!r}")
        failures += 1

    # Plain mode keeps the visible link destination after the bar.
    out = run('> [x](https://e.com) at start\n')
    if out.find(bar) < 0 or out.find('x (https://e.com)') < out.find(bar):
        print(f"FAIL [quote-link]: {out!r}")
        failures += 1

    # Footnote ref label renders after the bar.
    out = run('> [^1] starts line\n')
    if '[1]' not in out or out.find('[1]') < out.find(bar):
        print(f"FAIL [quote-footnote]: {out!r}")
        failures += 1

    # <br> inside a quote re-arms the bar for the following line.
    out = run('> line1<br>line2\n')
    if out.count(bar) < 2:
        print(f"FAIL [quote-br]: {out!r}")
        failures += 1

    return failures


def run_reference_checks(program):
    """Check deferred reference definitions and visible reference output."""
    failures = 0
    md = ('[uri]: https://example.com/docs "remote"\n'
          '[section]: #intro\n'
          '[file]: docs/readme.md\n')
    expected_visible = ('References\n'
                        '[uri] https://example.com/docs (remote)\n'
                        '[section] #intro\n'
                        '[file] docs/readme.md')

    rc, out, err = run_mdflow(md, program)
    if rc != 0:
        print(f"FAIL [reference]: exit {rc}: {err.strip()}")
        failures += 1
    elif '\033' in out or out.strip() != expected_visible:
        print(f"FAIL [reference]: {out!r}")
        failures += 1

    return failures


def run_highlight_checks(program):
    """Check code-block labels and content preservation in plain mode."""
    failures = 0

    def run(md):
        rc, out, err = run_mdflow(md, program)
        if rc != 0:
            raise RuntimeError(f"mdflow exit {rc}: {err}")
        return out

    def check(name, cond, out):
        nonlocal failures
        if not cond:
            print(f"FAIL [hl-{name}]: {out!r}")
            failures += 1

    def body(out):
        """Drop the fenced language label line (first line) from output."""
        lines = out.split('\n', 1)
        return lines[1] if len(lines) > 1 else ''

    # Unlabeled blocks have no language label and preserve their content.
    out = run('```\nint x = 42; // note\n```\n')
    check('nolabel-preserve',
          strip_sgr(out).strip() == 'int x = 42; // note', out)

    out = run('```mysterylang\nint x = 42;\n```\n')
    check('unknown-lang-label',
          'mysterylang' in strip_sgr(out), out)

    # Recognized labels, aliases, and token boundaries must preserve every
    # code character even though plain mode intentionally suppresses styles.
    cases = [
        ('c-content', '```c\nint x = 42; // note\n```\n',
         'int x = 42; // note'),
        ('alias-content', '```cpp\nint x = 42;\n```\n',
         'int x = 42;'),
        ('case-content', '```Python\nint x = 42;\n```\n',
         'int x = 42;'),
        ('whole-word-content',
         '```c\nmargin serif color username users\n'
         'inbox info insist\nif in\n```\n',
         'margin serif color username users\ninbox info insist\nif in'),
        ('string-content', '```c\ns = "int" + "hi"\n```\n',
         's = "int" + "hi"'),
        ('multiline-string-content', '```c\ns = "l1\nl2"\n```\n',
         's = "l1\nl2"'),
        ('block-comment-content', '```c\n/* a\nb */\n```\n',
         '/* a\nb */'),
        ('hash-comment-content', '```c\n# config\nx = 1\n```\n',
         '# config\nx = 1'),
        ('division-content', '```c\nx = a / b;\nlet rx = /a+b/;\n```\n',
         'x = a / b;\nlet rx = /a+b/;'),
    ]
    for name, markdown, expected in cases:
        out = run(markdown)
        check(name, strip_sgr(body(out)).strip() == expected, out)

    # non-alphanumeric backtick line is unformatted (scanner type-0
    # fallback), content preserved verbatim
    out = run('```c\n```c\nint main()\n```\n')
    check('backtick-visible', '```c' in strip_sgr(out), out)
    check('backtick-content',
          strip_sgr(body(out)).strip() == '```c\nint main()', out)

    # blockquote-wrapped code block: bar re-armed per line
    out = run('> ```sh\n> echo hi\n> ```\n')
    if out.count('│') < 2:
        print(f"FAIL [hl-quote-bar]: {out!r}")
        failures += 1

    return failures


def run_table_safety_checks(program):
    """Regression checks for renderer table bounds and empty cells."""
    failures = 0

    def run(md):
        rc, out, err = run_mdflow(md, program)
        if rc != 0:
            print(f"FAIL [table-safety-crash]: exit {rc}: {err.strip()}")
            return None
        return strip_sgr(out).replace('\r', '')

    # More columns than the old fixed renderer arrays supported must remain
    # safe and renderable.
    row = '|' + '|'.join(['x'] * 33) + '|\n'
    underline = '|' + '|'.join(['---'] * 33) + '|\n'
    body = '|' + '|'.join(['y'] * 33) + '|\n'
    out = run(row + underline + body)
    if out is not None and ('┌' not in out or 'y' not in out):
        print(f"FAIL [table-safety-columns]: {out!r}")
        failures += 1
    elif out is None:
        failures += 1

    # An empty first cell must not dereference a null cell buffer.
    out = run('| | b |\n|---|---|\n| | c |\n')
    if out is not None and ('┌' not in out or 'c' not in out):
        print(f"FAIL [table-safety-empty-cell]: {out!r}")
        failures += 1
    elif out is None:
        failures += 1

    return failures


def run_table_wrap_checks(program):
    """Regression checks for preferred breaks and content preservation."""
    failures = 0
    env = os.environ.copy()
    env['COLUMNS'] = '24'

    def run(md):
        rc, out, err = run_mdflow(md, program, env=env)
        if rc != 0:
            print(f"FAIL [table-wrap-crash]: exit {rc}: {err.strip()}")
            return None, None
        return out, strip_sgr(out).replace('\r', '')

    out, plain = run(
        '| h | b |\n'
        '|---|---|\n'
        '| **one two three four five six** | *alpha beta gamma delta* |\n')
    if out is None:
        failures += 1
    else:
        if 'one two' not in plain or 'three' not in plain:
            print(f"FAIL [table-wrap-word-boundary]: {plain!r}")
            failures += 1

    # Emergency wrapping must preserve every character of an unbreakable
    # token instead of stopping at a fixed slice count.
    word = 'x' * 300
    out, plain = run('| h | b |\n|---|---|\n| ' + word + ' | z |\n')
    if out is None:
        failures += 1
    elif plain.count('x') != len(word):
        print(f"FAIL [table-wrap-long-word]: got {plain.count('x')} x's")
        failures += 1

    # A hard break emitted by an inline HTML <br> must become a table line,
    # not a literal newline embedded inside one drawn row.
    out, plain = run('| h | b |\n|---|---|\n| one<br>two | alpha beta |\n')
    if out is None:
        failures += 1
    elif 'one\ntwo' in plain:
        print(f"FAIL [table-wrap-hard-break]: {plain!r}")
        failures += 1

    return failures


def run_table_layout_checks(program):
    """Regression checks for wrap-aware terminal width allocation."""
    failures = 0
    env = os.environ.copy()
    env['COLUMNS'] = '32'
    md = ('| a | b |\n'
          '|---|---|\n'
          '| a very long sentence with many words | short |\n')
    rc, out, err = run_mdflow(md, program, env=env)
    if rc != 0:
        print(f"FAIL [table-layout-crash]: exit {rc}: {err.strip()}")
        return 1

    plain = strip_sgr(out).replace('\r', '')
    final = plain[plain.rfind('┌'):]
    if 'short' not in final:
        print(f"FAIL [table-layout-short-cell]: {final!r}")
        failures += 1
    if re.search(r'shor\s*│\n.*\bt\b', final):
        print(f"FAIL [table-layout-narrow-column]: {final!r}")
        failures += 1
    for line in final.splitlines():
        if len(line) > 32:
            print(f"FAIL [table-layout-terminal-width]: {line!r}")
            failures += 1

    # A word wrap that breaks at a space must not make the row one column
    # narrower than the border it is drawn under.
    env['COLUMNS'] = '84'
    md = ('| Flag | Massive body of text explaining the full scenario in detail |\n'
          '|:-----|:------------------------------------------------------------|\n'
          '| ✳ | A comprehensive breakdown covering background, causes, effects, and recommended next steps. |\n'
          '| ✳ | ok |\n')
    rc, out, err = run_mdflow(md, program, env=env)
    if rc != 0:
        print(f"FAIL [table-layout-space-wrap-crash]: exit {rc}: {err.strip()}")
        return failures + 1

    plain = strip_sgr(out).replace('\r', '')
    final = plain[plain.rfind('┌'):]
    final_lines = final.splitlines()
    border_len = len(final_lines[0]) if final_lines else 0
    for line in final_lines:
        if line and len(line) != border_len:
            print(f"FAIL [table-layout-space-wrap-width]: {line!r}")
            failures += 1

    return failures


def main():
    ap = argparse.ArgumentParser(
        description='Test mdflow: run all spec examples through mdflow '
                    'and verify no crashes.')
    ap.add_argument('-p', '--program', default='mdflow/mdflow',
                    help='path to mdflow binary')
    ap.add_argument('-s', '--spec', default=None,
                    help='single spec file to test (default: all spec*.txt)')
    args = ap.parse_args()

    if args.spec:
        spec_files = [args.spec]
    else:
        spec_dir = 'test'
        spec_files = (glob.glob(f'{spec_dir}/spec*.txt')
                      + glob.glob(f'{spec_dir}/coverage.txt')
                      + glob.glob(f'{spec_dir}/regressions.txt'))

    total = 0
    passed = 0
    failed = 0
    errored = 0

    failures = run_quote_checks(args.program)
    if failures:
        print(f"quote regression checks: {failures} failed")
        failed += failures
    total += failures

    failures = run_reference_checks(args.program)
    if failures:
        print(f"reference renderer checks: {failures} failed")
        failed += failures
    total += failures

    failures = run_highlight_checks(args.program)
    if failures:
        print(f"highlight smoke checks: {failures} failed")
        failed += failures
    total += failures

    failures = run_table_safety_checks(args.program)
    if failures:
        print(f"table safety checks: {failures} failed")
        failed += failures
    total += failures

    failures = run_table_wrap_checks(args.program)
    if failures:
        print(f"table wrapping checks: {failures} failed")
        failed += failures
    total += failures

    failures = run_table_layout_checks(args.program)
    if failures:
        print(f"table layout checks: {failures} failed")
        failed += failures
    total += failures

    for sf in sorted(spec_files):
        tests = parse_spec_examples(sf)
        for test in tests:
            total += 1
            md = test['markdown']
            if not md.strip():
                passed += 1
                continue
            try:
                rc, _ansi_out, ansi_err = run_mdflow(md, args.program)
                if rc != 0:
                    print(f"CRASH [{test['file']}:{test['example']}] "
                          f"exit code {rc}: {ansi_err.strip()}")
                    errored += 1
                    failed += 1
                    continue
                passed += 1
            except Exception as e:
                print(f"ERROR [{test['file']}:{test['example']}]: {e}")
                errored += 1

    print(f"\n{passed} passed, {failed} failed, {errored} errored "
          f"(total {total})")

    return 0 if failed == 0 and errored == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
