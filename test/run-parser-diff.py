#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""
Build batch and streaming parser-diff binaries, then compare their
callback sequences across all spec examples.

Usage:
    python3 test/run-parser-diff.py
    python3 test/run-parser-diff.py -s test/spec.txt   (single spec)
"""

import argparse
import json
import os
import re
import subprocess
import sys


SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.dirname(SCRIPT_DIR)
BATCH_BIN = os.path.join(SCRIPT_DIR, "parser-diff-batch")
STREAM_BIN = os.path.join(SCRIPT_DIR, "parser-diff-stream")
MANIFEST = os.path.join(SCRIPT_DIR, "parser-diff-expected.txt")
SNAPSHOTS = os.path.join(SCRIPT_DIR, "parser-diff-expected.json")

CFLAGS = "-Wall -Wextra -Wshadow -Wdeclaration-after-statement -g -O2"
CFLAGS += " -I " + os.path.join(PROJECT_DIR, "src")
CFLAGS += " -DMD_VERSION_MAJOR=0 -DMD_VERSION_MINOR=5 -DMD_VERSION_RELEASE=0"


def build_binaries():
    src_dir = os.path.join(PROJECT_DIR, "src")
    parser_src = os.path.join(SCRIPT_DIR, "parser-diff.c")

    # Always rebuild the objects: stale build artifacts in src/ silently
    # skew the comparison if md4cs.c has changed since they were produced.
    stream_obj = os.path.join(src_dir, "md4cs.o")
    batch_obj = os.path.join(src_dir, "md4cs-batch.o")
    subprocess.run(
        f"gcc {CFLAGS} -DMD4C_STREAMING -c -o {stream_obj} {src_dir}/md4cs.c",
        shell=True, check=True, cwd=PROJECT_DIR)
    subprocess.run(
        f"gcc {CFLAGS} -c -o {batch_obj} {src_dir}/md4cs.c",
        shell=True, check=True, cwd=PROJECT_DIR)

    subprocess.run(
        f"gcc {CFLAGS} -DMD4C_STREAMING -o {STREAM_BIN} {parser_src} {stream_obj}",
        shell=True, check=True, cwd=PROJECT_DIR)

    subprocess.run(
        f"gcc {CFLAGS} -o {BATCH_BIN} {parser_src} {batch_obj}",
        shell=True, check=True, cwd=PROJECT_DIR)


def load_manifest():
    """Load the reviewed set of expected streaming-vs-batch divergences."""
    expected = {}
    if os.path.exists(MANIFEST):
        with open(MANIFEST, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    fields = line.split()
                    expected[fields[0]] = fields[1] if len(fields) > 1 else ""
    return expected


def load_snapshots():
    """Load exact reviewed batch/stream callback snapshots."""
    if not os.path.exists(SNAPSHOTS):
        return {}
    with open(SNAPSHOTS, encoding="utf-8") as f:
        data = json.load(f)
    if data.get("version") != 1 or not isinstance(data.get("entries"), dict):
        raise ValueError("unsupported parser-diff snapshot format")
    return data["entries"]


def classify_diff(batch_lines, stream_lines):
    """Classify a reviewed divergence by its event signature."""
    s = "\n".join(stream_lines)
    b = "\n".join(batch_lines)
    if "MD_SPAN_REFERENCE_LINK" in s or "MD_SPAN_REFERENCE_IMAGE" in s:
        return "ref-hint"
    if "REFERENCE_SECTION" in s:
        return "flush-ref-defs"
    if "FOOTNOTE_DEF_SECTION" in s:
        return "footnotes"
    if any("+BLOCK   H" in x for x in batch_lines + stream_lines):
        return "setext"
    nb = [x for x in batch_lines if "+BLOCK   P" not in x and "-BLOCK   P" not in x]
    ns = [x for x in stream_lines if "+BLOCK   P" not in x and "-BLOCK   P" not in x]
    if nb == ns:
        return "p-order"
    if any("+BLOCK   LI" in x for x in batch_lines + stream_lines):
        return "tight-loose"
    if "MD_SPAN_CODE" in s or "MD_SPAN_CODE" in b:
        return "code-span"
    if "  (BR)" in s or "  (BR)" in b:
        return "hardbreak"
    return "other"


def write_manifest(entries):
    """Persist the reviewed divergence set (spec#example category per line)."""
    with open(MANIFEST, "w", encoding="utf-8") as f:
        f.write("# Reviewed streaming-vs-batch divergences.\n")
        f.write("# Each entry was inspected one by one: the input exercises\n")
        f.write("# a documented streaming behavior (reference hints, flush-time\n")
        f.write("# emission, tight/loose first-item tradeoff, hold-boundary\n")
        f.write("# lookahead) and the streaming event stream was verified\n")
        f.write("# against the design in DEVELOPMENT.md.\n")
        f.write("# Re-run with --update-manifest only after reviewing every\n")
        f.write("# new or removed entry.\n")
        f.write("# Exact callback traces are stored in parser-diff-expected.json.\n")
        for entry in sorted(entries, key=lambda item: item["name"]):
            f.write(f"{entry['name']} {entry['category']}\n")


def write_snapshots(entries):
    """Persist exact reviewed batch and streaming callback outputs."""
    data = {
        "version": 1,
        "entries": {},
    }
    for entry in sorted(entries, key=lambda item: item["name"]):
        data["entries"][entry["name"]] = {
            "category": entry["category"],
            "batch_count": entry["batch_count"],
            "stream_count": entry["stream_count"],
            "batch": entry["batch_lines"],
            "stream": entry["stream_lines"],
        }
    with open(SNAPSHOTS, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2, sort_keys=True)
        f.write("\n")


def extract_examples(spec_path):
    """Yield (name, markdown_str) tuples from spec file."""
    with open(spec_path, "r", encoding="utf-8") as f:
        content = f.read()

    # Match ```````` example ... ```````` blocks
    pattern = re.compile(
        r"`{32,} example\n(.*?)\n\.\n(.*?)`{32,}",
        re.DOTALL
    )

    # Get line numbers for naming
    lines = content.split("\n")

    for m in pattern.finditer(content):
        markdown = m.group(1)
        # Count which example by scanning up to this match
        ex_num = content[:m.start()].count("```````````````````````````````` example") + 1
        yield (f"{os.path.basename(spec_path)}#{ex_num}", markdown.rstrip("\n"))


def run_parser_diff(binary, markdown):
    """Run a parser-diff binary, return (count, lines) or raise."""
    p = subprocess.run(
        [binary],
        input=markdown.encode("utf-8"),
        capture_output=True,
        timeout=10,
    )
    if p.returncode != 0:
        raise RuntimeError(f"{binary} exited {p.returncode}: {p.stderr.decode()}")

    output = p.stdout.decode("utf-8").strip()
    if not output:
        return (0, [])

    first_line = output.split("\n")[0]
    m = re.match(r"(\d+) events", first_line)
    if not m:
        raise RuntimeError(f"Unexpected output from {binary}: {output[:200]}")

    count = int(m.group(1))
    lines = output.split("\n")[1:]
    return (count, lines)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-s", "--spec", default=None,
                        help="Single spec file to test")
    parser.add_argument("-v", "--verbose", action="store_true",
                        help="Print every streaming difference")
    parser.add_argument("--update-manifest", action="store_true",
                        help="Rewrite parser-diff-expected.txt with the current "
                             "diff set (full runs only; review every entry first)")
    args = parser.parse_args()
    if args.update_manifest and args.spec:
        parser.error("--update-manifest requires a full run (no -s)")

    build_binaries()

    stats = {"match": 0, "diff": 0, "error": 0, "skip": 0}
    diff_entries = []

    if args.spec:
        spec_files = [args.spec]
    else:
        spec_dir = SCRIPT_DIR
        all_files = sorted(os.listdir(spec_dir))
        spec_files = []
        for f in all_files:
            if f.startswith("spec-") or f in ("spec.txt", "coverage.txt", "regressions.txt"):
                spec_files.append(os.path.join(spec_dir, f))

    for spec_path in spec_files:
        for name, markdown in extract_examples(spec_path):
            try:
                batch_count, batch_lines = run_parser_diff(BATCH_BIN, markdown)
                stream_count, stream_lines = run_parser_diff(STREAM_BIN, markdown)

                if batch_count == stream_count and batch_lines == stream_lines:
                    stats["match"] += 1
                else:
                    stats["diff"] += 1
                    diff_entries.append({
                        "name": name,
                        "category": classify_diff(batch_lines, stream_lines),
                        "batch_count": batch_count,
                        "stream_count": stream_count,
                        "batch_lines": batch_lines,
                        "stream_lines": stream_lines,
                    })
                    if args.verbose:
                        print(f"\nDIFF {name}")
                        for i, (b, s) in enumerate(zip(batch_lines, stream_lines)):
                            if b != s:
                                print(f"  batch[{i}]: {b}")
                                print(f"  stream[{i}]: {s}")
                        if len(batch_lines) != len(stream_lines):
                            print(f"  (batch={batch_count}, stream={stream_count})")

            except Exception as e:
                stats["error"] += 1
                print(f"ERROR {name}: {e}")

    total = sum(stats.values())
    print()
    print(f"Results: {stats['match']} match, {stats['diff']} diff, "
          f"{stats['error']} error, {stats['skip']} skip (total {total})")

    if args.spec is None:
        expected = load_manifest()
        actual = {entry["name"]: entry["category"] for entry in diff_entries}
        unexpected = sorted(set(actual) - set(expected))
        missing = sorted(set(expected) - set(actual))
        category_changed = sorted(
            name for name in set(actual) & set(expected)
            if actual[name] != expected[name]
        )
        if args.update_manifest:
            write_manifest(diff_entries)
            write_snapshots(diff_entries)
            print(f"Manifest and callback snapshots updated: {len(diff_entries)} "
                  "divergences (review before committing)")
            return 1 if stats["error"] > 0 else 0

        from collections import Counter
        cats = Counter(entry["category"] for entry in diff_entries)
        cat_summary = ", ".join(f"{k}={v}" for k, v in sorted(cats.items()))
        print(f"Expected: {len(set(actual) & set(expected))} reviewed divergences "
              f"({cat_summary})")
        if unexpected:
            print(f"UNEXPECTED: {len(unexpected)} divergence(s) outside the "
                  "reviewed manifest:")
            for name in unexpected:
                print(f"  {name}")
        if missing:
            print(f"MISSING: {len(missing)} reviewed divergence(s) no longer "
                  "differ:")
            for name in missing:
                print(f"  {name}")

        for name in category_changed:
            print(f"CATEGORY CHANGED: {name} ({expected[name]} -> "
                  f"{actual[name]})")

        try:
            expected_snapshots = load_snapshots()
            snapshot_format_error = False
        except (OSError, ValueError, json.JSONDecodeError) as e:
            print(f"SNAPSHOT ERROR: {e}")
            expected_snapshots = {}
            snapshot_format_error = True

        snapshot_missing = sorted(set(actual) - set(expected_snapshots))
        snapshot_unexpected = sorted(set(expected_snapshots) - set(actual))
        snapshot_changed = []
        for entry in diff_entries:
            name = entry["name"]
            expected_snapshot = expected_snapshots.get(name)
            if expected_snapshot is None:
                continue
            actual_snapshot = {
                "category": entry["category"],
                "batch_count": entry["batch_count"],
                "stream_count": entry["stream_count"],
                "batch": entry["batch_lines"],
                "stream": entry["stream_lines"],
            }
            if expected_snapshot != actual_snapshot:
                snapshot_changed.append(name)

        print(f"Callback snapshots: {len(set(actual) & set(expected_snapshots))} "
              "exact")
        if snapshot_missing:
            print(f"SNAPSHOT MISSING: {len(snapshot_missing)} divergence(s) "
                  "have no reviewed callback snapshot:")
            for name in snapshot_missing:
                print(f"  {name}")
        if snapshot_unexpected:
            print(f"SNAPSHOT UNEXPECTED: {len(snapshot_unexpected)} snapshot(s) "
                  "have no current divergence:")
            for name in snapshot_unexpected:
                print(f"  {name}")
        if snapshot_changed:
            print(f"SNAPSHOT CHANGED: {len(snapshot_changed)} reviewed "
                  "callback output(s) changed:")
            for name in snapshot_changed:
                print(f"  {name}")

        if (unexpected or missing or category_changed or snapshot_format_error
                or snapshot_missing or snapshot_unexpected or snapshot_changed):
            print("\nEvery streaming-vs-batch divergence must match the reviewed "
                  "manifest and exact callback snapshots in test/. Use "
                  "--update-manifest only after manually reviewing every "
                  "new, removed, or changed entry (see TESTING.md step 3).")
            return 1

    # Differences inside the reviewed manifest and exact snapshots are
    # expected streaming behavior. Only crashes/timeouts or a changed
    # reviewed contract fail the run.
    return 1 if stats["error"] > 0 else 0


if __name__ == "__main__":
    sys.exit(main())
