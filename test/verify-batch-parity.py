#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""
Verify that our batch-mode md4cs.c produces identical parser output
to MD4C's md4c.c. This guards against accidental corruption of
the batch (non-streaming) code paths during MD4C sync or editing.

Builds two parser-diff binaries:
  - ours:     test/parser-diff.c + our src/md4cs.c (batch mode)
  - MD4C:     test/parser-diff.c + MD4C md4c.c  (batch mode)

Both use our md4cs.h for enum definitions. If the outputs differ
on any spec example, our batch code has diverged from MD4C.

By default the parser runs with flags=0 (core CommonMark). Pass
--full-flags (or --flags N) to also verify extension modes such as
tables and permissive autolinks.
"""

import argparse
import hashlib
import os
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.request


SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.dirname(SCRIPT_DIR)
SRC_DIR = os.path.join(PROJECT_DIR, "src")
PARSER_DIFF_SRC = os.path.join(SCRIPT_DIR, "parser-diff.c")

CFLAGS = ["-Wall", "-Wextra", "-Wshadow", "-Wdeclaration-after-statement", "-g", "-O2"]

MD4C_URL = "https://raw.githubusercontent.com/mity/md4c/{ref}/src/{filename}"

# Full parser extension set for upstream parity. The mdflow CLI enables a
# subset (see src/mdflow.c): MD_DIALECT_GITHUB plus highlight.
FULL_FLAGS_EXPR = ("MD_DIALECT_GITHUB | MD_FLAG_HIGHLIGHT | MD_FLAG_SPOILERS "
                   "| MD_FLAG_SUPERSCRIPTS | MD_FLAG_SUBSCRIPTS "
                   "| MD_FLAG_UNDERLINE")


def eval_flags(expr):
    """Evaluate a flag expression from the MD_FLAG_* defines in md4cs.h."""
    ns = {}
    with open(os.path.join(SRC_DIR, "md4cs.h"), encoding="utf-8") as f:
        for line in f:
            m = re.match(r'#define\s+(MD_FLAG_\w+|MD_DIALECT_GITHUB)\s+(.+)', line)
            if m:
                rhs = re.sub(r'/\*.*?\*/', '', m.group(2)).strip()
                try:
                    ns[m.group(1)] = eval(rhs, {}, ns)
                except Exception:
                    pass
    return eval(expr, {}, ns)


def download_raw(ref, filename):
    """Download a raw file from MD4C GitHub at the given ref."""
    url = MD4C_URL.format(ref=ref, filename=filename)
    try:
        with urllib.request.urlopen(url) as r:
            return r.read().decode("utf-8")
    except urllib.error.HTTPError as e:
        raise RuntimeError(
            f"Failed to download {url}: HTTP {e.code} {e.reason}"
        )
    except urllib.error.URLError as e:
        raise RuntimeError(
            f"Failed to download {url}: {e.reason}"
        )


def build_binary(name, src_md4cs_c, include_dirs):
    """Compile parser-diff.c against a specific md4c source in batch mode.
    include_dirs is a list of paths for -I flags."""
    exe = os.path.join(tempfile.gettempdir(), f"pd-{name}")
    args = ["gcc"] + CFLAGS
    for d in include_dirs:
        args.append(f"-I{d}")
    args += ["-o", exe, PARSER_DIFF_SRC, src_md4cs_c]
    env = os.environ.copy()
    subprocess.run(args, check=True, capture_output=True, cwd=PROJECT_DIR, env=env)
    return exe


def extract_examples(spec_path):
    """Yield (name, markdown_str) from a spec file."""
    with open(spec_path, "r", encoding="utf-8") as f:
        content = f.read()

    pattern = re.compile(r"`{32,} example\n(.*?)\n\.\n(.*?)`{32,}", re.DOTALL)
    for m in pattern.finditer(content):
        yield (os.path.basename(spec_path), m.group(1).rstrip("\n"))


def run_binary(binary, flags, markdown):
    """Run a parser-diff binary, return (returncode, stdout_lines)."""
    args = [binary]
    if flags:
        args.append(str(flags))
    p = subprocess.run(args, input=markdown.encode("utf-8"),
                       capture_output=True, timeout=10)
    return (p.returncode, p.stdout.decode("utf-8").strip())


def spec_files():
    """Return list of spec files in test/."""
    sd = SCRIPT_DIR
    files = sorted(os.listdir(sd))
    return [
        os.path.join(sd, f)
        for f in files
        if f.startswith("spec-") or f in ("spec.txt", "coverage.txt", "regressions.txt")
    ]


def main():
    parser = argparse.ArgumentParser(
        description="Verify batch-mode parity with MD4C"
    )
    parser.add_argument(
        "--md4c-ref", default="master",
        help="MD4C git ref to compare against "
             "(branch name, tag, or commit hash, default: master)"
    )
    parser.add_argument(
        "--flags", type=lambda v: int(v, 0), default=0,
        help="numeric MD_PARSER::flags to verify (default: 0)"
    )
    parser.add_argument(
        "--full-flags", action="store_true",
        help="verify with the mdflow CLI's full extension flag set"
    )
    args = parser.parse_args()
    ref = args.md4c_ref
    flags = eval_flags(FULL_FLAGS_EXPR) if args.full_flags else args.flags

    print(f"=== Batch-mode parity check: ours vs MD4C ({ref}) "
          f"flags=0x{flags:x} ===")

    # Download MD4C parser source into a temp dir
    print("Downloading MD4C source (md4c.c, md4c.h)...")
    md4c_dir = tempfile.mkdtemp()
    try:
        for filename in ("md4c.c", "md4c.h"):
            content = download_raw(ref, filename)
            filepath = os.path.join(md4c_dir, filename)
            with open(filepath, "w") as f:
                f.write(content)
            print(f"  {filename} ({len(content)} bytes)")
    except RuntimeError as e:
        print(f"ERROR: {e}")
        sys.exit(1)

    md4c_c = os.path.join(md4c_dir, "md4c.c")

    # Build both binaries.
    # - ours: compile md4cs.c with our md4cs.h from src/
    # - MD4C:  compile MD4C's md4c.c with its md4c.h from temp dir
    # Both use our parser-diff.c which includes md4cs.h.
    # We include BOTH paths so parser-diff.c finds our md4cs.h and
    # MD4C's md4c.c finds its md4c.h.
    print("Building parser-diff (ours)...")
    our_bin = build_binary("ours", os.path.join(SRC_DIR, "md4cs.c"), [SRC_DIR])
    print("Building parser-diff (MD4C)...")
    md4c_bin = build_binary("md4c", md4c_c, [SRC_DIR, md4c_dir])

    print("")

    total = 0
    passed = 0
    failed = 0
    errors = 0

    for spec_path in spec_files():
        for name, markdown in extract_examples(spec_path):
            total += 1
            try:
                rc_up, out_up = run_binary(md4c_bin, flags, markdown)
                rc_our, out_our = run_binary(our_bin, flags, markdown)

                if rc_up != rc_our:
                    print(f"  FAIL rc mismatch [{total}]: "
                          f"md4c={rc_up} ours={rc_our}")
                    failed += 1
                    continue

                if out_up == out_our:
                    passed += 1
                else:
                    failed += 1
                    md_hash = hashlib.sha256(markdown.encode("utf-8")).hexdigest()[:8]
                    print(f"  FAIL divergence [{total}] {spec_path} (hash={md_hash})")
                    # Show first differing section
                    up_lines = out_up.split("\n")
                    our_lines = out_our.split("\n")
                    for i, (u, o) in enumerate(zip(up_lines, our_lines)):
                        if u != o:
                            print(f"    line {i}:")
                            print(f"      md4c:  {repr(u)}")
                            print(f"      ours:  {repr(o)}")
                            break
                    if len(up_lines) != len(our_lines):
                        print(f"    length: md4c={len(up_lines)} ours={len(our_lines)}")

                if total % 100 == 0:
                    print(f"  ...{total} examples checked...")

            except Exception as e:
                errors += 1
                print(f"  ERROR [{total}]: {e}")

    print(f"\n{passed} passed, {failed} failed, {errors} errors (total {total})")

    # Cleanup
    for f in [our_bin, md4c_bin]:
        try:
            os.unlink(f)
        except OSError:
            pass
    try:
        shutil.rmtree(md4c_dir)
    except OSError:
        pass

    if failed or errors:
        print("\nBatch code has diverged from MD4C!")
        sys.exit(1)
    else:
        print("\nBatch code is identical to MD4C.")
        sys.exit(0)


if __name__ == "__main__":
    main()
