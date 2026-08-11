#!/usr/bin/env python3
"""Typewriter mode tests for the mdflow CLI.

Two layers:
  1. Deterministic C unit tests (test/typewriter-test.c) for the tokenizer,
     pacer, rate estimator and input state, driven with a fake clock.
  2. End-to-end CLI tests. Pacing is on by default but starts in
     passthrough: the first real measurement (>= 8 chars over >= 50 ms,
     span measured from the first byte) decides once and locks. Slow
     streams switch to typewriter; fast streams, file redirects, and tiny
     single-burst inputs stay in passthrough. The only options are
     --typewriter-off and --report.
"""

from __future__ import annotations

import os
from pathlib import Path
import re
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[1]
BUILD = ROOT / "build"
MDFLOW = BUILD / "mdflow" / "mdflow"
UNIT_BIN = BUILD / "typewriter-test"
CC = os.environ.get("CC", "gcc")

STRESS_FIXTURE = ROOT / "test" / "typewriter-stress.md"


def run_unit_tests() -> bool:
    cmd = [
        CC,
        "-std=c99",
        "-Wall", "-Wextra", "-Wshadow", "-Wdeclaration-after-statement",
        "-I", str(ROOT / "mdflow"),
        str(ROOT / "test" / "typewriter-test.c"),
        str(ROOT / "mdflow" / "typewriter.c"),
        "-o", str(UNIT_BIN),
    ]
    proc = subprocess.run(cmd)
    if proc.returncode != 0:
        print("FAIL: could not compile typewriter-test")
        return False
    proc = subprocess.run([str(UNIT_BIN)])
    if proc.returncode != 0:
        print("FAIL: deterministic unit tests")
        return False
    print("PASS: deterministic unit tests")
    return True


def clean_env() -> dict[str, str]:
    env = dict(os.environ)
    env.pop("COLUMNS", None)
    return env


def run_cli(args: list[str], data: bytes | None = None,
            stdin_file: Path | None = None) -> subprocess.CompletedProcess:
    if stdin_file is not None:
        with open(stdin_file, "rb") as fh:
            return subprocess.run(
                [str(MDFLOW), *args],
                stdin=fh,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=clean_env(),
            )
    return subprocess.run(
        [str(MDFLOW), *args],
        input=data,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=clean_env(),
    )


def slow_pipe(data: bytes, chunk: int = 32, delay: float = 0.015,
              args: list[str] | None = None) -> subprocess.CompletedProcess:
    """Stream data slowly enough that the first measurement classifies it
    as a slow stream (well under 5000 chars/sec)."""
    proc = subprocess.Popen(
        [str(MDFLOW), *(args or [])],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=clean_env(),
    )
    try:
        assert proc.stdin is not None and proc.stdout is not None
        for i in range(0, len(data), chunk):
            proc.stdin.write(data[i:i + chunk])
            proc.stdin.flush()
            time.sleep(delay)
        proc.stdin.close()
        out = proc.stdout.read()
        proc.wait(timeout=30)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()
    return subprocess.CompletedProcess(proc.args, proc.returncode,
                                       out, proc.stderr.read())


def test_help() -> bool:
    proc = subprocess.run(
        [str(MDFLOW), "--help"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    ok = proc.returncode == 0
    ok = ok and b"--typewriter-off" in proc.stderr
    ok = ok and b"--report" not in proc.stderr
    ok = ok and b"Disable typewriter effect" in proc.stderr
    ok = ok and b"--cps" not in proc.stderr
    ok = ok and b"--adaptive" not in proc.stderr
    print(("PASS" if ok else "FAIL")
          + ": --help lists --typewriter-off and hides --report")
    return ok


def test_unknown_options_rejected() -> bool:
    ok = True
    for args in (["--cps", "10"], ["--adaptive"], ["--max-delay", "1"]):
        proc = run_cli(args, data=b"")
        good = proc.returncode != 0 and b"unknown option" in proc.stderr
        ok = ok and good
        print(("PASS" if good else "FAIL") + f": {args[0]} is rejected")
    return ok


def test_unclear_input_passes_through() -> bool:
    """A tiny single-burst input has no timing evidence, so it stays in
    passthrough (unclear means bypass)."""
    data = b"hi\n"
    plain = run_cli(["--typewriter-off"], data=data)
    paced = run_cli(["--report"], data=data)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    ok = ok and b"bypass=on" in paced.stderr
    ok = ok and b"input_meta=on" in paced.stderr
    print(("PASS" if ok else "FAIL")
          + ": tiny single-burst input stays passthrough")
    return ok


def test_slow_stream_activates_typewriter() -> bool:
    """A stream measured well under 5000 chars/sec switches to typewriter
    once and locks."""
    data = b"x" * 100 + b"\n"
    plain = run_cli(["--typewriter-off"], data=data)
    start = time.monotonic()
    paced = slow_pipe(data, chunk=8, delay=0.03, args=["--report"])
    elapsed = time.monotonic() - start
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    ok = ok and b"bypass=off" in paced.stderr
    ok = ok and b"input_meta=on" in paced.stderr
    match = re.search(rb"output_chars=(\d+)", paced.stderr)
    ok = ok and match is not None and int(match.group(1)) > 0
    print(("PASS" if ok else "FAIL")
          + f": slow stream activates typewriter (elapsed {elapsed:.2f}s)")
    return ok


def test_bypass_file_parity() -> bool:
    demo = ROOT / "assets" / "demo.md"
    plain = run_cli(["--typewriter-off"], stdin_file=demo)
    paced = run_cli(["--report"], stdin_file=demo)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    ok = ok and b"bypass=on" in paced.stderr
    print(("PASS" if ok else "FAIL")
          + ": regular-file redirect auto-bypasses with byte parity")
    return ok


def test_piped_parity_default() -> bool:
    data = b"# title\n\nsome **bold** and *italic* text\n\n- one\n- two\n"
    plain = run_cli(["--typewriter-off"], data=data)
    paced = run_cli([], data=data)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    print(("PASS" if ok else "FAIL") + ": piped input byte parity")
    return ok


def test_max_delay_cap() -> bool:
    # 200 chars fed slowly: measured ~160/s, so pacing would take ~1.2 s and
    # the fixed 1 s cap bursts the overdue line.
    data = b"b" * 200 + b"\n"
    plain = run_cli(["--typewriter-off"], data=data)
    start = time.monotonic()
    paced = slow_pipe(data, chunk=2, delay=0.012, args=["--report"])
    elapsed = time.monotonic() - start
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    match = re.search(rb"timeout_burst_count=(\d+)", paced.stderr)
    ok = ok and match is not None and int(match.group(1)) >= 1
    print(("PASS" if ok else "FAIL")
          + f": fixed 1 s max-delay cap bursts overdue line ({elapsed:.2f}s)")
    return ok


def test_slow_stream_queue_growth() -> bool:
    """A long line paced while short lines arrive one at a time must not
    corrupt the pacer queue (the head advances while the tail grows)."""
    data = b"z" * 60 + b"\n" + b"y\n" * 20
    plain = run_cli(["--typewriter-off"], data=data)
    paced = slow_pipe(data, chunk=2, delay=0.01)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    print(("PASS" if ok else "FAIL")
          + ": slow stream with advancing queue head stays intact")
    return ok


def test_code_block_blank_lines_parity() -> bool:
    """A fenced code block emits consecutive blank rendered lines; they must
    be coalesced (not queued per line) and still match the plain path."""
    data = b"```\na\n" + b"\n" * 2000 + b"b\n```\n"
    plain = run_cli(["--typewriter-off"], data=data)
    paced = slow_pipe(data, chunk=32, delay=0.02)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    print(("PASS" if ok else "FAIL")
          + ": code-block blank lines coalesce with byte parity")
    return ok


def test_stress_parity() -> bool:
    if not STRESS_FIXTURE.is_file():
        print(f"FAIL: stress fixture not found: {STRESS_FIXTURE}")
        return False
    data = STRESS_FIXTURE.read_bytes()
    plain = run_cli(["--typewriter-off"], data=data)
    paced = slow_pipe(data)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    print(("PASS" if ok else "FAIL")
          + f": {STRESS_FIXTURE.name} byte parity")
    return ok


def test_rate_bypass_pipe() -> bool:
    # Large enough to arrive in several pipe reads, so the first measurement
    # is far above the fixed 5000 chars/sec threshold and the run stays in
    # passthrough.
    data = b"line with some **markdown** here\n" * 30000
    plain = run_cli(["--typewriter-off"], data=data)
    paced = run_cli(["--report"], data=data)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    ok = ok and b"bypass=on" in paced.stderr
    print(("PASS" if ok else "FAIL")
          + ": measured rate above 5000 chars/sec stays passthrough")
    return ok


def test_report_fields() -> bool:
    proc = run_cli(["--report"], data=b"hello world\n")
    ok = proc.returncode == 0
    ok = ok and b"report: input_chars=" in proc.stderr
    ok = ok and b"max_delay_cap=1.0s" in proc.stderr
    ok = ok and b"input_meta=on" in proc.stderr
    print(("PASS" if ok else "FAIL") + ": --report prints expected fields")
    return ok


def main() -> int:
    if not MDFLOW.is_file():
        print(f"error: mdflow binary not found: {MDFLOW}")
        return 1

    results = [
        run_unit_tests(),
        test_help(),
        test_unknown_options_rejected(),
        test_unclear_input_passes_through(),
        test_slow_stream_activates_typewriter(),
        test_bypass_file_parity(),
        test_piped_parity_default(),
        test_max_delay_cap(),
        test_slow_stream_queue_growth(),
        test_code_block_blank_lines_parity(),
        test_stress_parity(),
        test_rate_bypass_pipe(),
        test_report_fields(),
    ]

    if all(results):
        print("ALL TYPEWRITER TESTS PASSED")
        return 0
    print("TYPEWRITER TESTS FAILED")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
