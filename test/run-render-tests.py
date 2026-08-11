#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""Golden-output tests for the ANSI renderer.

Runs mdflow on each example in render-golden.txt and compares the output
against the frozen expected output:

  - if the expected output contains the ESC placeholder '␛' (U+241B),
    the comparison is EXACT (byte-for-byte ANSI, including SGR codes);
  - otherwise the actual output is stripped of SGR codes first and the
    visible text must match.

The expected outputs are a contract with the default theme: changing the
theme or renderer output means updating the goldens deliberately.
"""
import sys
import re
import glob
import argparse
from subprocess import Popen, PIPE


SGR_RE = re.compile(r'\033\[[0-9;]*[a-zA-Z]')
ESC_MARK = '\u241b'          # ␛ stands for ESC in golden files
CR_MARK = '\u240d'           # ␍ stands for carriage return
ESC = '\x1b'
CLI_EPILOGUE = '\n' + ESC + '[0m'   # mdflow CLI appends this after close


def strip_sgr(text):
    return SGR_RE.sub('', text)


def strip_cli_epilogue(text):
    """Remove the CLI's trailing newline + SGR reset (not renderer output)."""
    if text.endswith(CLI_EPILOGUE):
        return text[:-len(CLI_EPILOGUE)]
    return text


def parse_examples(specfile):
    """CommonMark-style ```````` example blocks: markdown, '.', expected."""
    tests = []
    state = 0
    md = []
    exp = []
    example = 0
    with open(specfile, 'r', encoding='utf-8', newline='\n') as f:
        for line in f:
            l = line.strip()
            if re.match(r"`{32} example", l):
                state = 1
                md = []
                exp = []
            elif state >= 2 and l == "`" * 32:
                state = 0
                example += 1
                tests.append({
                    'example': example,
                    'markdown': ''.join(md).replace('→', '\t'),
                    'expected': ''.join(exp).replace('→', '\t'),
                })
            elif l == ".":
                state += 1
            elif state == 1:
                md.append(line)
            elif state == 2:
                exp.append(line)
    return tests


def run_mdflow(md_text, program):
    p = Popen([program, "--typewriter-off"],
              stdout=PIPE, stdin=PIPE, stderr=PIPE)
    out, err = p.communicate(input=md_text.encode('utf-8'))
    # Normalize CRLF line endings so exact-string checks stay portable.
    return p.returncode, out.decode('utf-8').replace('\r\n', '\n'), \
           err.decode('utf-8')


def main():
    ap = argparse.ArgumentParser(
        description='Golden-output tests for the ANSI renderer.')
    ap.add_argument('-p', '--program', default='mdflow/mdflow',
                    help='path to mdflow binary')
    ap.add_argument('-g', '--golden', default='test/render-golden.txt',
                    help='golden spec file')
    args = ap.parse_args()

    failed = 0
    total = 0

    for t in parse_examples(args.golden):
        total += 1
        rc, out, err = run_mdflow(t['markdown'], args.program)
        if rc != 0:
            print(f"CRASH [example {t['example']}] exit {rc}: {err.strip()}")
            failed += 1
            continue

        expected = (t['expected'].replace(ESC_MARK, ESC)
                                 .replace(CR_MARK, '\r'))
        actual = strip_cli_epilogue(out)
        if ESC not in expected:
            actual = strip_sgr(actual)

        if actual != expected:
            failed += 1
            print(f"FAIL [example {t['example']}]")
            print(f"  input:    {t['markdown']!r}")
            print(f"  expected: {expected!r}")
            print(f"  actual:   {actual!r}")

    print(f"\n{total - failed}/{total} render golden checks passed")
    return 0 if failed == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
