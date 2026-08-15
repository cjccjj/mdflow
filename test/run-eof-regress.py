#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""Regression tests for content at end of input without a trailing newline.

The streaming parser closes the last block at flush. A streamed paragraph
that was consumed as a reference/footnote definition left stale pending
lines behind, so a following line without a trailing newline lost its
content (ref use, blockquote text, or the last list item). This test feeds
exact bytes (no trailing newline) to the CLI and compares the visible text.

    python3 test/run-eof-regress.py -p build/mdflow/mdflow
"""

import argparse
import re
import subprocess
import sys


SGR_RE = re.compile(r'\033\[[0-9;]*[a-zA-Z]')
OSC8_RE = re.compile('\x1b\\]8;;[^\x1b\x07]*(\x1b\\\\|\x07)')

CASES = [
    # Ref use after a consumed reference definition.
    ('[foo]: /url "title"\n\n[foo]',
     'foo ref:[foo]\n\n\nReferences\n[foo]: /url "title"'),
    # Blockquote text after a consumed reference definition.
    ('# [Foo]\n[foo]: /url\n> bar',
     'Foo ref:[Foo]\n\n│ bar\n\n\n\nReferences\n[foo]: /url'),
    # Last list item after a reference definition inside the list.
    ('- a\n- b\n\n  [ref]: /url\n- d',
     '• a\n• b\n\n• d\n\n\nReferences\n[ref]: /url'),
]


def visible(text):
    return OSC8_RE.sub('', SGR_RE.sub('', text)).strip()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('-p', '--program', default='build/mdflow/mdflow',
                    help='path to mdflow binary')
    args = ap.parse_args()

    failed = 0
    for i, (markdown, expected) in enumerate(CASES, 1):
        p = subprocess.run([args.program, '--typewriter-off'],
                           input=markdown.encode('utf-8'),
                           capture_output=True)
        if p.returncode != 0:
            print(f"FAIL [case {i}] exit {p.returncode}: {p.stderr.decode()}")
            failed += 1
            continue
        actual = visible(p.stdout.decode())
        if actual != expected:
            failed += 1
            print(f"FAIL [case {i}] input={markdown!r}")
            print(f"  expected: {expected!r}")
            print(f"  actual:   {actual!r}")

    print(f"\n{len(CASES) - failed}/{len(CASES)} EOF regression checks passed")
    return 0 if failed == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
