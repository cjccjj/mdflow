#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""Verify the batch build contains no streaming-only parser code.

Rule #1 of this project: the batch-mode compilation of src/md4cs.c must see
zero streaming code and stay behaviorally identical to upstream MD4C. This
test preprocesses src/md4cs.c without MD4C_STREAMING and asserts the
effective batch source keeps the upstream 3-argument md_process_inlines()
and contains none of the windowed walker, streaming helpers, streaming-only
state, or streaming-only enum values.

    python3 test/verify-batch-source.py
"""

import os
import re
import subprocess
import sys


SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SRC_DIR = os.path.join(SCRIPT_DIR, "..", "src")

# Streaming-only constructs that must never be compiled into the batch build.
STREAMING_MARKERS = [
    "md_stream_process_inlines",
    "md_inline_text",
    "emit_from",
    "emit_to",
    "owns_text",
    "owns_substr_types",
    "owns_substr_offsets",
    "md_resolve_reference",
    "md_store_link_attr",
    "md_process_verbatim_line",
    "md_process_table_underline",
    "md_route_table_row",
    "md_process_footnote_def_body",
    "md_process_doc_end",
    "md_append_line_into_current_block",
    "OFFtask_mark_off;CHARtask_mark;",
    "MD_BLOCK_REFERENCE_SECTION",
    "MD_SPAN_REFERENCE_LINK",
    "MD_SPAN_REFERENCE_IMAGE",
]

# The batch build must keep the upstream signature.
UPSTREAM_SIGNATURE = "md_process_inlines(MD_CTX*ctx,constMD_LINE*lines,MD_SIZEn_lines)"


def preprocess_batch():
    p = subprocess.run(
        ["gcc", "-E", "-fdirectives-only", "-P",
         "-I", SRC_DIR, os.path.join(SRC_DIR, "md4cs.c")],
        capture_output=True, text=True)
    if p.returncode != 0:
        print(p.stderr)
        sys.exit("preprocessing src/md4cs.c failed")
    return p.stdout


def main():
    text = preprocess_batch()
    # Comments are kept by -fdirectives-only; strip them before token checks.
    text = re.sub(r'/\*.*?\*/', '', text, flags=re.S)
    text = re.sub(r'//[^\n]*', '', text)
    compact = re.sub(r'\s+', '', text)

    found = [m for m in STREAMING_MARKERS if m in compact]
    if re.search(r'md_stream_[a-z_]+', compact):
        found.append("md_stream_* identifier")
    if UPSTREAM_SIGNATURE not in compact:
        found.append("missing upstream 3-arg md_process_inlines signature")

    if found:
        print("FAIL: streaming code leaked into the batch build:")
        for item in found:
            print(f"  - {item}")
        return 1

    print("batch source contains no streaming-only code")
    return 0


if __name__ == "__main__":
    sys.exit(main())
