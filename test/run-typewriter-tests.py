#!/usr/bin/env python3
"""Typewriter mode tests for the mdflow CLI.

Two layers:
  1. Deterministic C unit tests (test/typewriter-test.c) for the tokenizer,
     pacer, rate estimator and input state, driven with a fake clock.
  2. End-to-end CLI tests. Pacing is on by default but starts in
     passthrough: the first real measurement (>= 8 chars over >= 50 ms,
     span measured from the first byte) decides once and locks. Slow
     streams switch to typewriter; fast streams, file redirects, and tiny
     single-burst inputs stay in passthrough. The options are
     --typewriter-off, --report, and --osc8-off. Renderer parity checks use
     --plain so they test typewriter behavior independently of styling.
"""

from __future__ import annotations

import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
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
            stdin_file: Path | None = None,
            cwd: Path | None = None) -> subprocess.CompletedProcess:
    if stdin_file is not None:
        with open(stdin_file, "rb") as fh:
            return subprocess.run(
                [str(MDFLOW), *args],
                stdin=fh,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=clean_env(),
                cwd=cwd,
            )
    return subprocess.run(
        [str(MDFLOW), *args],
        input=data,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=clean_env(),
        cwd=cwd,
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
    ok = ok and b"--osc8-off" in proc.stderr
    ok = ok and b"Disable typewriter effect" in proc.stderr
    ok = ok and b"Disable OSC 8 hyperlinks" in proc.stderr
    ok = ok and b"--cps" not in proc.stderr
    ok = ok and b"--adaptive" not in proc.stderr
    print(("PASS" if ok else "FAIL")
          + ": --help lists --typewriter-off and --osc8-off, hides --report")
    return ok


def test_unknown_options_rejected() -> bool:
    ok = True
    for args in (["--cps", "10"], ["--adaptive"], ["--max-delay", "1"]):
        proc = run_cli(["--plain", *args], data=b"")
        good = proc.returncode != 0 and b"unknown option" in proc.stderr
        ok = ok and good
        print(("PASS" if good else "FAIL") + f": {args[0]} is rejected")
    return ok


def test_unclear_input_passes_through() -> bool:
    """A tiny single-burst input has no timing evidence, so it stays in
    passthrough (unclear means bypass)."""
    data = b"hi\n"
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = run_cli(["--plain", "--report"], data=data)
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
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    start = time.monotonic()
    paced = slow_pipe(data, chunk=8, delay=0.03,
                      args=["--plain", "--report"])
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
    plain = run_cli(["--plain", "--typewriter-off"], stdin_file=demo)
    paced = run_cli(["--plain", "--report"], stdin_file=demo)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    ok = ok and b"bypass=on" in paced.stderr
    print(("PASS" if ok else "FAIL")
          + ": regular-file redirect auto-bypasses with byte parity")
    return ok


def test_filename_input() -> bool:
    """A filename operand must use the same input path as a redirect."""
    demo = ROOT / "assets" / "demo.md"
    redirected = run_cli(["--plain", "--typewriter-off"], stdin_file=demo)
    plain = run_cli(["--plain", str(demo), "--typewriter-off"],
                    data=b"# ignored stdin\n")
    paced = run_cli(["--plain", "--report", str(demo)],
                    data=b"# ignored stdin\n")
    ok = redirected.returncode == 0 and plain.returncode == 0
    ok = ok and paced.returncode == 0
    ok = ok and plain.stdout == redirected.stdout
    ok = ok and paced.stdout == redirected.stdout
    ok = ok and b"bypass=on" in paced.stderr
    print(("PASS" if ok else "FAIL")
          + ": filename input matches redirect and bypasses typewriter")
    return ok


def test_standard_filename_operands() -> bool:
    """Keep '-' stdin and '--' delimiter behavior conventional."""
    data = b"# file operand\n\ncontent\n"
    piped = run_cli(["--plain", "--typewriter-off"], data=data)
    dash = run_cli(["--plain", "--typewriter-off", "-"], data=data)
    with tempfile.TemporaryDirectory() as temp_dir:
        directory = Path(temp_dir)
        filename = "-not-markdown.bin"
        (directory / filename).write_bytes(data)
        delimited = run_cli(["--plain", "--typewriter-off", "--", filename],
                            data=b"# ignored stdin\n", cwd=directory)
        missing = run_cli(["--plain", "--typewriter-off",
                           str(directory / "missing-input.md")], data=b"")
        extra = run_cli(["--plain", "--typewriter-off", "one.md", "two.md"],
                        data=b"")
    ok = piped.returncode == 0 and dash.returncode == 0
    ok = ok and delimited.returncode == 0
    ok = ok and dash.stdout == piped.stdout
    ok = ok and delimited.stdout == piped.stdout
    ok = ok and missing.returncode != 0 and b"cannot open" in missing.stderr
    ok = ok and extra.returncode != 0 and b"too many input files" in extra.stderr
    print(("PASS" if ok else "FAIL")
          + ": '-', '--', missing, and extra filename operands behave")
    return ok


def test_piped_parity_default() -> bool:
    data = b"# title\n\nsome **bold** and *italic* text\n\n- one\n- two\n"
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = run_cli(["--plain"], data=data)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    print(("PASS" if ok else "FAIL") + ": piped input byte parity")
    return ok


def test_rendering_option_parity() -> bool:
    """Plain output must match through direct and paced CLI paths."""
    data = (b"# title\n\n"
            b"some **bold** text and a [link](https://e.com)\n\n"
            b"| a | b |\n|---|---|\n| one | two |\n")
    direct = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = run_cli(["--plain"], data=data)
    ok = (direct.returncode == 0 and paced.returncode == 0
          and direct.stdout == paced.stdout)
    print(("PASS" if ok else "FAIL")
          + ": plain rendering preserves typewriter parity")
    return ok


def test_max_delay_cap() -> bool:
    # 200 chars fed slowly: measured ~160/s, so pacing would take ~1.2 s and
    # the fixed 1 s cap bursts the overdue line.
    data = b"b" * 200 + b"\n"
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    start = time.monotonic()
    paced = slow_pipe(data, chunk=2, delay=0.012,
                      args=["--plain", "--report"])
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
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = slow_pipe(data, chunk=2, delay=0.01, args=["--plain"])
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    print(("PASS" if ok else "FAIL")
          + ": slow stream with advancing queue head stays intact")
    return ok


def test_code_block_blank_lines_parity() -> bool:
    """A fenced code block emits consecutive blank rendered lines; they must
    be coalesced (not queued per line) and still match the plain path."""
    data = b"```\na\n" + b"\n" * 2000 + b"b\n```\n"
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = slow_pipe(data, chunk=32, delay=0.02, args=["--plain"])
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
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = slow_pipe(data, args=["--plain"])
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
    plain = run_cli(["--plain", "--typewriter-off"], data=data)
    paced = run_cli(["--plain", "--report"], data=data)
    ok = plain.returncode == 0 and paced.returncode == 0
    ok = ok and plain.stdout == paced.stdout
    ok = ok and b"bypass=on" in paced.stderr
    print(("PASS" if ok else "FAIL")
          + ": measured rate above 5000 chars/sec stays passthrough")
    return ok


def test_report_fields() -> bool:
    proc = run_cli(["--plain", "--report"], data=b"hello world\n")
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
        test_filename_input(),
        test_standard_filename_operands(),
        test_piped_parity_default(),
        test_rendering_option_parity(),
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
