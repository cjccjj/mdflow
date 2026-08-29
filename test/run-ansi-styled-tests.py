#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""Focused DEFAULT checks for renderer control sequences.

The general renderer suites use --plain. This small suite covers the DEFAULT
ANSI behavior that plain mode intentionally suppresses: incremental table redraws
and OSC 8 hyperlink metadata. It never treats a particular SGR color as a
contract.
"""

import argparse
import os
import re
import subprocess
import sys


SGR_RE = re.compile(r'\033\[[0-9;]*[a-zA-Z]')
OSC8_RE = re.compile(r'\033\]8;;[^\033\x07]*(?:\033\\|\x07)')


def strip_sgr(text):
    return SGR_RE.sub('', text)


def strip_osc8(text):
    return OSC8_RE.sub('', text)


def run_mdflow(program, markdown, env=None):
    return subprocess.run(
        [program, '--typewriter-off'],
        input=markdown.encode('utf-8'), stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, env=env)


def table_rewrite_check(program):
    markdown = ('| 日本語 | ✅ |\n'
                '|---|---|\n'
                '| abc | def |\n')
    expected = ('┌────────┬─────┐\n'
                '│ 日本語 │ ✅  │\n'
                '├────────┼─────┤\n'
                '│ abc    │ def │\n'
                '└────────┴─────┘')
    env = dict(os.environ)
    env['COLUMNS'] = '80'
    proc = run_mdflow(program, markdown, env)
    output = proc.stdout.decode('utf-8', errors='replace')
    if proc.returncode != 0:
        print('FAIL [table-rewrite]: exit {}: {}'.format(
            proc.returncode, proc.stderr.decode(errors='replace').strip()))
        return False

    final_start = output.rfind('┌')
    final = strip_sgr(output[final_start:]).rstrip('\n') \
        if final_start >= 0 else ''
    checks = [
        ('cursor-up', '\033[3A' in output),
        ('erase', '\033[J' in output),
        ('incremental-redraw', output.count('┌') >= 2),
        ('final-visible-table', final == expected),
    ]
    ok = True
    for name, condition in checks:
        if condition:
            print('PASS [table-rewrite-{}]'.format(name))
        else:
            print('FAIL [table-rewrite-{}]: {!r}'.format(name, output))
            ok = False
    return ok


def osc8_check(program):
    destination = 'https://example.test/docs'
    markdown = '[label]({})\n'.format(destination)
    proc = run_mdflow(program, markdown)
    output = proc.stdout.decode('utf-8', errors='replace')
    opener = '\033]8;;{}\033\\'.format(destination)
    closer = '\033]8;;\033\\'
    visible = strip_osc8(strip_sgr(output))
    checks = [
        ('opener', opener in output),
        ('closer', closer in output),
        ('destination', destination in output),
        ('visible-label', 'label' in visible),
        ('url-hidden-from-visible-output', destination not in visible),
    ]
    ok = proc.returncode == 0
    if not ok:
        print('FAIL [osc8]: exit {}: {}'.format(
            proc.returncode, proc.stderr.decode(errors='replace').strip()))
    for name, condition in checks:
        if condition:
            print('PASS [osc8-{}]'.format(name))
        else:
            print('FAIL [osc8-{}]: {!r}'.format(name, output))
            ok = False
    return ok


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('-p', '--program', default='build/mdflow/mdflow',
                        help='path to mdflow binary')
    args = parser.parse_args()
    results = [table_rewrite_check(args.program),
               osc8_check(args.program)]
    if all(results):
        print('ALL FOCUSED ANSI TESTS PASSED')
        return 0
    print('FOCUSED ANSI TESTS FAILED')
    return 1


if __name__ == '__main__':
    sys.exit(main())
