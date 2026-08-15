#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
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
    p = Popen([program, "--typewriter-off"],
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

    # Highlight must open AFTER the bar: the bar stays dim and the content
    # receives the reverse-video styling.
    out = run('> ==foo== at start\n')
    if '\033[7m\033[2m' in out or '\033[7m' not in out \
            or out.find('\033[7m') < out.find(bar):
        print(f"FAIL [quote-highlight]: {out!r}")
        failures += 1

    # Link underline/blue must start after the bar, not on the bar.
    out = run('> [x](https://e.com) at start\n')
    if '\033[1;4;38;5;75m' not in out \
            or out.find('\033[1;4;38;5;75m') < out.find(bar):
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


def run_highlight_checks(program):
    """Smoke checks for the code-block highlighter:
    the 4 style SGRs, token finalization rules (EOL comments, cross-line
    strings/comments), division-vs-regex, and content preservation.
    Returns the number of failures."""
    failures = 0

    def run(md):
        rc, out, err = run_mdflow(md, program)
        if rc != 0:
            raise RuntimeError(f"mdflow exit {rc}: {err}")
        return out

    kw = '\033[38;5;176m'    # hl_keyword
    pu = '\033[2m'            # hl_punct (no style: plain normal)
    st = '\033[38;5;114m'     # hl_string
    cm = '\033[3;38;5;245m'   # hl_comment
    reset = '\033[0m'

    def check(name, cond, out):
        nonlocal failures
        if not cond:
            print(f"FAIL [hl-{name}]: {out!r}")
            failures += 1

    def body(out):
        """Drop the fenced language label line (first line) from output."""
        lines = out.split('\n', 1)
        return lines[1] if len(lines) > 1 else ''

    # Highlighting is gated on the fence language label: only recognized
    # major languages are highlighted; unknown and unlabeled blocks
    # render plain (label line still shown for fenced blocks).
    out = run('```\nint x = 42; // note\n```\n')
    check('nolabel-plain',
          kw not in out and st not in out and cm not in out, out)
    check('nolabel-preserve',
          strip_sgr(out).strip() == 'int x = 42; // note', out)

    out = run('```mysterylang\nint x = 42;\n```\n')
    check('unknown-lang-plain',
          kw not in out and st not in out and cm not in out, out)
    check('unknown-lang-label',
          'mysterylang' in strip_sgr(out), out)

    # keyword + EOL line comment; punctuation renders plain (no SGR)
    out = run('```c\nint x = 42; // note\n```\n')
    check('keyword', kw + 'int' in out, out)
    check('punct-plain', pu not in out and '\033[38;5;252m=' not in out
          and '\033[38;5;176m=' not in out, out)
    check('comment-line', cm + '// note' + reset in out, out)
    check('preserve-1',
          strip_sgr(body(out)).strip() == 'int x = 42; // note', out)

    # common aliases and case-insensitive labels also enable highlighting
    out = run('```cpp\nint x = 42;\n```\n')
    check('alias-lang', kw + 'int' in out, out)
    out = run('```Python\nint x = 42;\n```\n')
    check('case-insensitive-lang', kw + 'int' in out, out)

    # whole-word keyword matching: substrings of real keywords or words
    # containing keyword fragments must NOT be styled (the keyword list is
    # anchored with ^( ... )$)
    out = run('```c\nmargin serif color username users\ninbox info insist\nif in\n```\n')
    check('non-keywords-plain',
          kw + 'margin' not in out and kw + 'serif' not in out
          and kw + 'color' not in out and kw + 'username' not in out
          and kw + 'users' not in out,
          out)
    check('fragment-words', kw + 'inbox' not in out
          and kw + 'info' not in out and kw + 'insist' not in out, out)
    check('real-keywords', kw + 'if' in out and kw + 'in' in out, out)

    # string token; keywords inside a string must NOT be re-scanned
    out = run('```c\ns = "int" + "hi"\n```\n')
    check('string', st + '"int"' + reset in out, out)
    check('no-kw-in-string', kw + 'int' not in out, out)
    check('preserve-2',
          strip_sgr(body(out)).strip() == 's = "int" + "hi"', out)

    # string spanning two lines
    out = run('```c\ns = "l1\nl2"\n```\n')
    check('string-multiline', st + '"l1\nl2"' + reset in out, out)

    # block comment spanning two lines
    out = run('```c\n/* a\nb */\n```\n')
    check('comment-block', cm + '/* a\nb */' + reset in out, out)

    # hash comment ends at newline; next line stays plain
    out = run('```c\n# config\nx = 1\n```\n')
    check('comment-hash', cm + '# config' + reset + '\n' in out, out)
    check('hash-eol', cm + '# config\n' not in out, out)
    check('preserve-3',
          strip_sgr(body(out)).strip() == '# config\nx = 1', out)

    # division is plain punctuation; a regex literal after '=' is string-styled
    out = run('```c\nx = a / b;\nlet rx = /a+b/;\n```\n')
    check('division', pu not in body(out).split('\n')[0], out)
    check('regex', st + '/a+b/' + reset in out, out)

    # non-alphanumeric backtick line is unformatted (scanner type-0
    # fallback), content preserved verbatim
    out = run('```c\n```c\nint main()\n```\n')
    check('backtick-plain', '```c' in strip_sgr(out), out)
    check('preserve-4',
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
    """Regression checks for preferred breaks and style continuation."""
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
        if '\033[1mthree' not in out or '\033[3mbeta' not in out:
            print(f"FAIL [table-wrap-style-continuation]: {out!r}")
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
                rc, ansi_out, ansi_err = run_mdflow(md, args.program)
                if rc != 0:
                    print(f"CRASH [{test['file']}:{test['example']}] "
                          f"exit code {rc}: {ansi_err.strip()}")
                    errored += 1
                    failed += 1
                    continue
                if not ansi_out.strip():
                    print(f"EMPTY [{test['file']}:{test['example']}] "
                          f"non-empty input produced empty output")
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
