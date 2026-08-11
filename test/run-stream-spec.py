#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""Run streaming parser spec tests.

Usage:
    python3 test/run-stream-spec.py -s test/stream-footnote.txt -p "test/stream-event --footnotes"
    python3 test/run-stream-spec.py -s test/stream-ref.txt -p test/stream-event
"""

import argparse, re, shlex, subprocess, sys


def parse_spec(path):
    """Parse spec file, return list of (markdown, expected) tuples."""
    examples = []
    with open(path) as f:
        text = f.read()

    pattern = re.compile(r"````+ example\n(.*?)\n\.\n(.*?)\n````+", re.DOTALL)
    for m in pattern.finditer(text):
        examples.append((m.group(1), m.group(2)))
    return examples


def run_test(program, markdown):
    """Run markdown through the test binary, return stdout."""
    proc = subprocess.run(
        program,
        input=markdown + "\n",
        capture_output=True,
        text=True,
    )
    return proc.stdout.strip()


def main():
    argparse_parser = argparse.ArgumentParser(description="Run streaming spec tests.")
    argparse_parser.add_argument("-s", "--spec", required=True, help="path to spec file")
    argparse_parser.add_argument("-p", "--program", required=True,
                                  help="test binary with optional arguments (e.g. 'stream-event --footnotes')")
    args = argparse_parser.parse_args()

    program = shlex.split(args.program)
    examples = parse_spec(args.spec)
    passed = 0
    failed = 0

    for i, (markdown, expected) in enumerate(examples):
        actual = run_test(program, markdown)
        expected = expected.strip()
        if actual == expected:
            passed += 1
            print(f"  example {i+1}  PASSED")
        else:
            failed += 1
            print(f"  example {i+1}  FAILED")
            exp_lines = expected.split("\n")
            act_lines = actual.split("\n")
            for j in range(max(len(exp_lines), len(act_lines))):
                ex = exp_lines[j] if j < len(exp_lines) else "(missing)"
                ac = act_lines[j] if j < len(act_lines) else "(missing)"
                if ex != ac:
                    print(f"    line {j}:")
                    print(f"      expected: {repr(ex)}")
                    print(f"      got:      {repr(ac)}")

    print(f"\n{passed} passed, {failed} failed")
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
