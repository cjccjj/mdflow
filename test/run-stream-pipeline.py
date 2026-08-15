#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md)
"""Pipeline tests for the streaming parser API.

Tests: CRLF splitting, byte-feed equivalence, buffer compaction,
duplicate-definition handling, empty input, and trailing-line flush.
"""

import subprocess, os, sys, tempfile


PROJECT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(PROJECT_DIR, "src", "md4cs.c")
EVENT_SRC = os.path.join(PROJECT_DIR, "test", "stream-event.c")
EVENT_BIN = os.path.join(tempfile.gettempdir(), "stream-event")


def build_event():
    if not os.path.exists(EVENT_BIN):
        subprocess.run(
            ["gcc", "-DMD4C_STREAMING", "-Wall", "-Wextra", "-g",
             f"-I{os.path.join(PROJECT_DIR, 'src')}", "-o", EVENT_BIN,
             EVENT_SRC, SRC, "-w"],
            check=True, capture_output=True)


def run_event(markdown, footnotes=False):
    """Run markdown through stream-event, return stripped stdout."""
    build_event()
    args = [EVENT_BIN, "--footnotes"] if footnotes else [EVENT_BIN]
    proc = subprocess.run(args, input=markdown + "\n", capture_output=True, text=True)
    return proc.stdout.strip()


def build_and_run(name, source_code, input_data=""):
    """Compile a C driver, run it with input on stdin, return stripped stdout."""
    src = os.path.join(tempfile.gettempdir(), f"pipeline-{name}.c")
    exe = os.path.join(tempfile.gettempdir(), f"pipeline-{name}")
    with open(src, "w") as f:
        f.write(source_code)
    subprocess.run(
        ["gcc", "-DMD4C_STREAMING", "-Wall", "-Wextra", "-g",
         f"-I{os.path.join(PROJECT_DIR, 'src')}", "-o", exe, src, SRC, "-w"],
        check=True, capture_output=True)
    proc = subprocess.run([exe], input=input_data, capture_output=True, text=True)
    return proc.stdout.strip()


passed = 0
failed = 0

# ── Test 1: CRLF \r + \n across chunk boundary → single line ──

output = build_and_run("crlf", r'''
#include <stdio.h>
#include <string.h>
#include "md4cs.h"
static int tx(MD_TEXTTYPE t, const MD_CHAR* s, MD_SIZE n, void* u) { return 0; }
static int eb(MD_BLOCKTYPE t, void* d, void* u) { return 0; }
static int lb(MD_BLOCKTYPE t, void* d, void* u) { return 0; }
static int es(MD_SPANTYPE t, void* d, void* u) { return 0; }
static int ls(MD_SPANTYPE t, void* d, void* u) { return 0; }
int main() {
    MD_PARSER p = {0};
    p.enter_block = eb; p.leave_block = lb;
    p.enter_span  = es; p.leave_span  = ls;
    p.text = tx;
    MD_PARSER_CTX* ctx;
    if(md_stream_init(&p, NULL, &ctx) != 0) { printf("FAIL\n"); return 1; }
    if(md_stream_feed(ctx, "hello\r", 6) != 0)  { printf("FAIL\n"); return 1; }
    if(md_stream_feed(ctx, "\nworld\n", 7) != 0) { printf("FAIL\n"); return 1; }
    if(md_stream_flush(ctx) != 0) { printf("FAIL\n"); return 1; }
    md_stream_finish(ctx);
    printf("PASS\n");
    return 0;
}
''')

if output == "PASS":
    print("  PASS: CRLF split across chunks")
    passed += 1
else:
    print(f"  FAIL: CRLF — {output}")
    failed += 1

# ── Test 2: byte-by-byte feed == single feed ─────────────────

single = run_event("hello **bold**\n")

output = build_and_run("bytefeed", '''
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <string.h>
#include "md4cs.h"
#define MAX_EVENTS 8192
#define MAX_LINE 256
static char ev[MAX_EVENTS][MAX_LINE];
static int n;
static void a(const char* f, ...) {
    char b[MAX_LINE]; va_list ap; va_start(ap,f); vsnprintf(b,MAX_LINE,f,ap); va_end(ap);
    if(n < MAX_EVENTS) snprintf(ev[n++], MAX_LINE, "%s", b);
}
static int eb(MD_BLOCKTYPE t,void*d,void*u){ a("+BLOCK  %d",t); return 0; }
static int lb(MD_BLOCKTYPE t,void*d,void*u){ a("-BLOCK  %d",t); return 0; }
static int es(MD_SPANTYPE t,void*d,void*u){ a(">SPAN   %d",t); return 0; }
static int ls(MD_SPANTYPE t,void*d,void*u){ a("<SPAN   %d",t); return 0; }
static int tx(MD_TEXTTYPE t,const MD_CHAR*s,MD_SIZE sz,void*u){
    if(t==0) a("TEXT \\"%.*s\\" (NORMAL)", (int)sz, s); return 0;
}
int main() {
    MD_PARSER p = {0};
    p.enter_block=eb; p.leave_block=lb; p.enter_span=es; p.leave_span=ls; p.text=tx;
    MD_PARSER_CTX* ctx;
    md_stream_init(&p,0,&ctx);
    const char* in = "hello **bold**\\n";
    int len = (int)strlen(in), i;
    for(i = 0; i < len; i++)
        md_stream_feed(ctx, in + i, 1);
    md_stream_flush(ctx);
    md_stream_finish(ctx);
    for(i = 0; i < n; i++) printf("%s\\n", ev[i]);
    return 0;
}
''')

# Parse both outputs into event type sequences, compare
def event_types(output):
    """Extract ordered list of event types from output lines."""
    lines = [l.strip() for l in output.split("\n") if l.strip()]
    # Strip lines like "N events" header
    lines = [l for l in lines if not (l.endswith("events") and l[0].isdigit())]
    result = []
    for l in lines:
        # Take the first token (e.g. "+BLOCK", "-BLOCK", ">SPAN", "<SPAN", "TEXT")
        parts = l.split()
        if parts:
            result.append(parts[0])
    return result

if event_types(single) == event_types(output):
    print("  PASS: byte-by-byte feed == single feed")
    passed += 1
else:
    print(f"  FAIL: byte-by-byte feed")
    print(f"    single: {event_types(single)}")
    print(f"    byte:   {event_types(output)}")
    failed += 1


# ── Test 3: buffer compaction preserves footnote defs ────────

output = build_and_run("compact", '''
#include <stdio.h>
#include <string.h>
#include "md4cs.h"
static int tx(MD_TEXTTYPE t, const MD_CHAR* s, MD_SIZE n, void* u) { return 0; }
static int eb(MD_BLOCKTYPE t, void* d, void* u) { return 0; }
static int lb(MD_BLOCKTYPE t, void* d, void* u) { return 0; }
static int es(MD_SPANTYPE t, void* d, void* u) { return 0; }
static int ls(MD_SPANTYPE t, void* d, void* u) { return 0; }
int main() {
    MD_PARSER p = {0};
    p.flags = 0x100000; /* MD_FLAG_FOOTNOTES */
    p.enter_block = eb; p.leave_block = lb;
    p.enter_span  = es; p.leave_span  = ls;
    p.text = tx;
    MD_PARSER_CTX* ctx;
    if(md_stream_init(&p, NULL, &ctx) != 0) { printf("FAIL init\\n"); return 1; }
    /* Feed many complete lines to trigger buffer compaction */
    if(md_stream_feed(ctx, "[^a]: persisted note text that is long enough\\n", 47) != 0)
        { printf("FAIL feed1\\n"); return 1; }
    if(md_stream_feed(ctx, "\\n", 1) != 0)
        { printf("FAIL feed2\\n"); return 1; }
    if(md_stream_feed(ctx, "[^b]: also survives compaction across chunks\\n", 48) != 0)
        { printf("FAIL feed3\\n"); return 1; }
    if(md_stream_flush(ctx) != 0)
        { printf("FAIL flush\\n"); return 1; }
    md_stream_finish(ctx);
    printf("PASS\\n");
    return 0;
}
''')

if output == "PASS":
    print("  PASS: buffer compaction preserves defs")
    passed += 1
else:
    print(f"  FAIL: compaction — {output}")
    failed += 1


# ── Test 4: duplicate definitions keep the first label ───────

output = build_and_run("duplicate-defs", r'''
#include <stdio.h>
#include <string.h>
#include "md4cs.h"
static int footnote_defs;
static int in_footnote;
static int in_refs;
static char footnote_text[128];
static int footnote_len;
static char ref_text[256];
static int ref_len;
static int eb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)d; (void)u;
    if(t == MD_BLOCK_FOOTNOTE_DEF) { footnote_defs++; in_footnote = 1; }
    if(t == MD_BLOCK_REFERENCE_SECTION) in_refs = 1;
    return 0;
}
static int lb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)d; (void)u;
    if(t == MD_BLOCK_FOOTNOTE_DEF) in_footnote = 0;
    if(t == MD_BLOCK_REFERENCE_SECTION) in_refs = 0;
    return 0;
}
static int es(MD_SPANTYPE t, void* d, void* u) { (void)t; (void)d; (void)u; return 0; }
static int ls(MD_SPANTYPE t, void* d, void* u) { (void)t; (void)d; (void)u; return 0; }
static int tx(MD_TEXTTYPE t, const MD_CHAR* s, MD_SIZE n, void* u) {
    int take;
    (void)t; (void)u;
    if(in_footnote) {
        take = (int)n;
        if(take > (int)sizeof(footnote_text) - footnote_len - 1)
            take = (int)sizeof(footnote_text) - footnote_len - 1;
        if(take > 0) { memcpy(footnote_text + footnote_len, s, (size_t)take); footnote_len += take; }
        footnote_text[footnote_len] = '\0';
    }
    if(in_refs) {
        take = (int)n;
        if(take > (int)sizeof(ref_text) - ref_len - 1)
            take = (int)sizeof(ref_text) - ref_len - 1;
        if(take > 0) { memcpy(ref_text + ref_len, s, (size_t)take); ref_len += take; }
        ref_text[ref_len] = '\0';
    }
    return 0;
}
int main() {
    MD_PARSER p = {0};
    MD_PARSER_CTX* ctx;
    p.flags = MD_DIALECT_GITHUB;
    p.enter_block = eb; p.leave_block = lb;
    p.enter_span = es; p.leave_span = ls; p.text = tx;
    if(md_stream_init(&p, NULL, &ctx) != 0) { printf("FAIL init\n"); return 1; }
    if(md_stream_feed(ctx, "[Alpha]: /first\n", 16) != 0
            || md_stream_feed(ctx, "[alpha]: /second\n\n", 18) != 0
            || md_stream_feed(ctx, "[^Note]: first footnote\n", 24) != 0
            || md_stream_feed(ctx, "[^note]: second footnote\n\n", 26) != 0
            || md_stream_flush(ctx) != 0) {
        printf("FAIL feed\n"); md_stream_finish(ctx); return 1;
    }
    md_stream_finish(ctx);
    if(footnote_defs != 1 || strstr(footnote_text, "first footnote") == NULL
            || strstr(footnote_text, "second footnote") != NULL
            || strstr(ref_text, "/first") == NULL
            || strstr(ref_text, "/second") != NULL) {
        printf("FAIL defs=%d footnote=%s refs=%s\n", footnote_defs,
               footnote_text, ref_text);
        return 1;
    }
    printf("PASS\n");
    return 0;
}
''')

if output == "PASS":
    print("  PASS: duplicate definitions keep first label")
    passed += 1
else:
    print(f"  FAIL: duplicate definitions — {output}")
    failed += 1


# ── Test 5: empty input → just DOC open/close ────────────────

events = run_event("")
has_doc_open = "+BLOCK   DOC" in events
has_doc_close = "-BLOCK   DOC" in events
if has_doc_open and has_doc_close:
    print("  PASS: empty input -> DOC open/close")
    passed += 1
else:
    print(f"  FAIL: empty input — {events}")
    failed += 1


# ── Test 6: flush emits trailing partial line ────────────────

events = run_event("hello")
needs = ["+BLOCK   DOC", "+BLOCK   P", 'TEXT  "hello"  (NORMAL)', "-BLOCK   P", "-BLOCK   DOC"]
if all(n in events for n in needs):
    print("  PASS: flush emits trailing partial line")
    passed += 1
else:
    print(f"  FAIL: trailing line — {events}")
    failed += 1


# ── Test 7: fenced code and HTML blocks emit line by line ─────

output = build_and_run("earlyemit", '''
#include <stdio.h>
#include <string.h>
#include "md4cs.h"
#define MAXE 64
static char ev[MAXE][128];
static int n;
static void a(const char* f, const char* s) {
    if(n < MAXE) snprintf(ev[n++], 128, "%s %s", f, s);
}
static int eb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)d; (void)u;
    if(t == MD_BLOCK_CODE) a("+BLOCK", "CODE");
    if(t == MD_BLOCK_HTML) a("+BLOCK", "HTML");
    return 0;
}
static int lb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)d; (void)u;
    if(t == MD_BLOCK_CODE) a("-BLOCK", "CODE");
    if(t == MD_BLOCK_HTML) a("-BLOCK", "HTML");
    return 0;
}
static int es(MD_SPANTYPE t, void* d, void* u) { (void)t;(void)d;(void)u; return 0; }
static int ls(MD_SPANTYPE t, void* d, void* u) { (void)t;(void)d;(void)u; return 0; }
static int tx(MD_TEXTTYPE t, const MD_CHAR* s, MD_SIZE sz, void* u) {
    (void)s; (void)sz; (void)u;
    if(t == MD_TEXT_CODE) a("TEXT", "(CODE)");
    if(t == MD_TEXT_HTML) a("TEXT", "(HTML)");
    return 0;
}
int main() {
    MD_PARSER p; MD_PARSER_CTX* ctx;
    memset(&p, 0, sizeof(p));
    p.enter_block = eb; p.leave_block = lb; p.enter_span = es; p.leave_span = ls; p.text = tx;
    if(md_stream_init(&p, NULL, &ctx) != 0) { printf("FAIL init\\n"); return 1; }
    /* Feed the opening fence + first line only: the code block is still
     * open, but enter_block must already have fired. */
    if(md_stream_feed(ctx, "```c\\nline1\\n", 10) != 0) { printf("FAIL f1\\n"); return 1; }
    a("MARK", "");
    if(md_stream_feed(ctx, "line2\\n```\\n", 10) != 0) { printf("FAIL f2\\n"); return 1; }
    a("MARK", "");
    if(md_stream_feed(ctx, "<div>\\n", 6) != 0) { printf("FAIL f3\\n"); return 1; }
    if(md_stream_flush(ctx) != 0) { printf("FAIL fl\\n"); return 1; }
    md_stream_finish(ctx);
    for(n = 0; n < 64 && ev[n][0]; n++) printf("%s\\n", ev[n]);
    return 0;
}
''')

lines = output.split("\n")
before_first_mark = []
after = []
seen_mark = 0
for l in lines:
    if l.strip() == "MARK":
        seen_mark = 1
        continue
    if seen_mark == 0:
        before_first_mark.append(l.strip())
    else:
        after.append(l.strip())

early_ok = ("+BLOCK CODE" in before_first_mark
            and "TEXT (CODE)" in after
            and "MARK" in output)
# Code block must be closed before the HTML block starts; the HTML
# enter must fire before its content leaves (early), with leave at close.
html_ok = ("+BLOCK HTML" in after and "-BLOCK HTML" in after
           and after.index("+BLOCK HTML") < after.index("-BLOCK HTML"))

if early_ok and html_ok:
    print("  PASS: fenced code / HTML emit line by line (early emission)")
    passed += 1
else:
    print(f"  FAIL: early emission")
    print(f"    before first MARK: {before_first_mark}")
    print(f"    after: {after}")
    failed += 1


# ── Test 7: indented code emits per line across chunk boundaries ─

# Driver: records every callback (all blocks, all text), feeds the document
# across chunk boundaries chosen to exercise the trailing-blank holdback.
indented_driver = r'''
#include <stdio.h>
#include <string.h>
#include "md4cs.h"
#define MAXE 64
static char ev[MAXE][128];
static int n;
static const char* bname(MD_BLOCKTYPE t) {
    switch(t) {
        case MD_BLOCK_DOC:  return "DOC";
        case MD_BLOCK_P:    return "P";
        case MD_BLOCK_CODE: return "CODE";
        case MD_BLOCK_HTML: return "HTML";
        default:            return "OTHER";
    }
}
static void mark(void) {
    if(n < MAXE) snprintf(ev[n++], 128, "MARK");
}
static int eb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)d;(void)u;
    if(n < MAXE) snprintf(ev[n++], 128, "+BLOCK %s", bname(t));
    return 0;
}
static int lb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)d;(void)u;
    if(n < MAXE) snprintf(ev[n++], 128, "-BLOCK %s", bname(t));
    return 0;
}
static int es(MD_SPANTYPE t, void* d, void* u) { (void)t;(void)d;(void)u; return 0; }
static int ls(MD_SPANTYPE t, void* d, void* u) { (void)t;(void)d;(void)u; return 0; }
static int tx(MD_TEXTTYPE t, const MD_CHAR* s, MD_SIZE sz, void* u) {
    char txt[96]; int i, j = 0;
    (void)t; (void)u;
    for(i = 0; i < (int)sz && j < (int)sizeof(txt)-3; i++) {
        if(s[i] == '\n') { txt[j++] = '\\'; txt[j++] = 'n'; }
        else             { txt[j++] = (char)s[i]; }
    }
    txt[j] = '\0';
    if(n < MAXE) snprintf(ev[n++], 128, "TEXT %s", txt);
    return 0;
}
int main(void) {
    MD_PARSER p; MD_PARSER_CTX* ctx;
    memset(&p, 0, sizeof(p));
    p.enter_block = eb; p.leave_block = lb; p.enter_span = es; p.leave_span = ls; p.text = tx;
    if(md_stream_init(&p, NULL, &ctx) != 0) { printf("FAIL init\n"); return 1; }
    /* Chunk boundaries exercise the holdback across feeds:
     * line, held blank, next content line (blank proven interior),
     * two trailing blanks, then a paragraph. */
    if(md_stream_feed(ctx, "    a\n", 6) != 0) { printf("FAIL f1\n"); return 1; }
    mark();
    if(md_stream_feed(ctx, "\n", 1) != 0) { printf("FAIL f2\n"); return 1; }
    if(md_stream_feed(ctx, "    b\n", 6) != 0) { printf("FAIL f3\n"); return 1; }
    mark();
    if(md_stream_feed(ctx, "\n\n", 2) != 0) { printf("FAIL f4\n"); return 1; }
    if(md_stream_feed(ctx, "x\n", 2) != 0) { printf("FAIL f5\n"); return 1; }
    if(md_stream_flush(ctx) != 0) { printf("FAIL fl\n"); return 1; }
    md_stream_finish(ctx);
    for(n = 0; n < 64 && ev[n][0]; n++) printf("%s\n", ev[n]);
    return 0;
}
'''

output_chunk = build_and_run("indchunk", indented_driver)

single_events = run_event("    a\n\n    b\n\n\nx\n")

lines = output_chunk.split("\n")
blocks_before_first_mark = []
between = []
after_second_mark = []
seen = 0
for l in lines:
    if l.strip() == "MARK":
        seen += 1
        continue
    if seen == 0:
        blocks_before_first_mark.append(l.strip())
    elif seen == 1:
        between.append(l.strip())
    else:
        after_second_mark.append(l.strip())

# enter_block fires on the first code line, before the block closes.
early_ok = ("+BLOCK CODE" in blocks_before_first_mark
            and "-BLOCK CODE" not in blocks_before_first_mark)
# "    a\n" = code enter + TEXT "a" + TEXT "\n".
line1_ok = blocks_before_first_mark == ['+BLOCK DOC', '+BLOCK CODE',
                                        'TEXT a', 'TEXT \\n']
# Interior blank flushed before the second code line, then "b" + newline.
interior_ok = between == ['TEXT \\n', 'TEXT b', 'TEXT \\n']
# Trailing blanks dropped at block close: nothing between MARK and close.
trailing_ok = after_second_mark[0] == "-BLOCK CODE"
# Cross-chunk feeding must produce the same callback sequence as one feed.
chunk_events = [l for l in lines if l.strip() != "MARK"]
chunk_ok = event_types("\n".join(chunk_events)) == event_types(single_events)

if early_ok and line1_ok and interior_ok and trailing_ok and chunk_ok:
    print("  PASS: indented code per-line emission across chunk boundaries")
    passed += 1
else:
    print(f"  FAIL: indented code per-line emission")
    print(f"    before first MARK: {blocks_before_first_mark}")
    print(f"    between marks:     {between}")
    print(f"    after second MARK: {after_second_mark}")
    print(f"    chunk == single:   {chunk_ok}")
    failed += 1


# ── Test 8: table conversion resets paragraph compaction state ─────────────

output = build_and_run("tablepara", r'''
#include <stdio.h>
#include <string.h>
#include "md4cs.h"
static char text[256];
static int text_len;
static int eb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)t; (void)d; (void)u; return 0;
}
static int lb(MD_BLOCKTYPE t, void* d, void* u) {
    (void)t; (void)d; (void)u; return 0;
}
static int es(MD_SPANTYPE t, void* d, void* u) {
    (void)t; (void)d; (void)u; return 0;
}
static int ls(MD_SPANTYPE t, void* d, void* u) {
    (void)t; (void)d; (void)u; return 0;
}
static int tx(MD_TEXTTYPE t, const MD_CHAR* s, MD_SIZE n, void* u) {
    MD_SIZE room;
    (void)t; (void)u;
    room = (MD_SIZE) sizeof(text) - 1 - (MD_SIZE) text_len;
    if(n > room) n = room;
    if(n > 0) {
        memcpy(text + text_len, s, n);
        text_len += (int)n;
    }
    text[text_len] = '\0';
    return 0;
}
int main(void) {
    const char* input = "| a | b |\n| --- | --- |\n| c | d |\n\nhello after\n";
    MD_PARSER p;
    MD_PARSER_CTX* ctx;
    int i;
    memset(&p, 0, sizeof(p));
    p.flags = MD_DIALECT_GITHUB;
    p.enter_block = eb; p.leave_block = lb;
    p.enter_span = es; p.leave_span = ls; p.text = tx;
    if(md_stream_init(&p, NULL, &ctx) != 0) { printf("FAIL init\n"); return 1; }
    for(i = 0; input[i] != '\0'; i++) {
        if(md_stream_feed(ctx, input + i, 1) != 0) {
            md_stream_finish(ctx);
            printf("FAIL feed\n");
            return 1;
        }
    }
    if(md_stream_flush(ctx) != 0) { md_stream_finish(ctx); printf("FAIL flush\n"); return 1; }
    md_stream_finish(ctx);
    printf("%s\n", strstr(text, "hello after") != NULL ? "PASS" : text);
    return strstr(text, "hello after") != NULL ? 0 : 1;
}
''')

if output == "PASS":
    print("  PASS: table followed by paragraph survives compaction")
    passed += 1
else:
    print(f"  FAIL: table/paragraph compaction — {output}")
    failed += 1


print(f"\n{passed} passed, {failed} failed")
sys.exit(0 if failed == 0 else 1)
