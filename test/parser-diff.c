/*
 * parser-diff — dump parser callback events for batch or streaming mode.
 * (https://github.com/cjccjj/mdflow)
 *
 * Copyright (c) 2026 Changjun Zhang
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
 * OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
 * IN THE SOFTWARE.
 *
 * Reads Markdown from stdin, outputs one line per callback event:
 *   +BLOCK   DOC
 *   +BLOCK   P
 *   TEXT     "content"
 *   >SPAN    MD_SPAN_A
 *   <SPAN    MD_SPAN_A
 *   -BLOCK   P
 *   -BLOCK   DOC
 *
 * Optional argv[1] is a numeric MD_PARSER::flags value (default 0).
 *
 * Intended to be driven by run-parser-diff.py which builds both binaries,
 * runs them on the same input, and compares their output.
 *
 * Compile without -DMD4C_STREAMING for batch md_parse():
 *   gcc -o test/parser-diff-batch test/parser-diff.c src/md4cs-batch.o
 *
 * Compile with -DMD4C_STREAMING for streaming:
 *   gcc -DMD4C_STREAMING -o test/parser-diff-stream test/parser-diff.c src/md4cs.o
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>

#include "md4cs.h"


#define MAX_EVENTS  (256 * 1024)
#define MAX_LINE    512

static char event_lines[MAX_EVENTS][MAX_LINE];
static int event_count;


static void
add_event(const char* fmt, ...)
{
    char buf[MAX_LINE];
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(buf, sizeof(buf), fmt, ap);
    va_end(ap);
    if(event_count < MAX_EVENTS)
        snprintf(event_lines[event_count++], MAX_LINE, "%s", buf);
}


/*
 * Block callbacks
 */

static int
enter_block(MD_BLOCKTYPE type, void* detail, void* userdata)
{
    (void)detail; (void)userdata;
    switch(type) {
        case MD_BLOCK_DOC:    add_event("+BLOCK   DOC"); break;
        case MD_BLOCK_P:      add_event("+BLOCK   P"); break;
        case MD_BLOCK_H:      add_event("+BLOCK   H  level=%d", ((MD_BLOCK_H_DETAIL*)detail)->level); break;
        case MD_BLOCK_CODE:   add_event("+BLOCK   CODE"); break;
        case MD_BLOCK_HR:     add_event("+BLOCK   HR"); break;
        case MD_BLOCK_UL:     add_event("+BLOCK   UL"); break;
        case MD_BLOCK_OL:     add_event("+BLOCK   OL"); break;
        case MD_BLOCK_LI:     add_event("+BLOCK   LI"); break;
        case MD_BLOCK_QUOTE:  add_event("+BLOCK   QUOTE"); break;
        case MD_BLOCK_HTML:   add_event("+BLOCK   HTML"); break;
        case MD_BLOCK_TABLE:   add_event("+BLOCK   TABLE"); break;
        case MD_BLOCK_THEAD:   add_event("+BLOCK   THEAD"); break;
        case MD_BLOCK_TBODY:   add_event("+BLOCK   TBODY"); break;
        case MD_BLOCK_TR:     add_event("+BLOCK   TR"); break;
        case MD_BLOCK_TH:     add_event("+BLOCK   TH"); break;
        case MD_BLOCK_TD:     add_event("+BLOCK   TD"); break;
        case MD_BLOCK_FOOTNOTE_DEF_SECTION: add_event("+BLOCK   FOOTNOTE_DEF_SECTION"); break;
        case MD_BLOCK_FOOTNOTE_DEF: {
            MD_BLOCK_FOOTNOTE_DEF_DETAIL* d = (MD_BLOCK_FOOTNOTE_DEF_DETAIL*)detail;
            add_event("+BLOCK   FOOTNOTE_DEF  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        case MD_BLOCK_ADMONITION:  add_event("+BLOCK   ADMONITION"); break;
#ifdef MD4C_STREAMING
        case MD_BLOCK_REFERENCE_SECTION: add_event("+BLOCK   REFERENCE_SECTION"); break;
#endif
        default: add_event("+BLOCK   type=%d", type); break;
    }
    return 0;
}

static int
leave_block(MD_BLOCKTYPE type, void* detail, void* userdata)
{
    (void)detail; (void)userdata;
    switch(type) {
        case MD_BLOCK_DOC:    add_event("-BLOCK   DOC"); break;
        case MD_BLOCK_P:      add_event("-BLOCK   P"); break;
        case MD_BLOCK_H:      add_event("-BLOCK   H"); break;
        case MD_BLOCK_CODE:   add_event("-BLOCK   CODE"); break;
        case MD_BLOCK_HR:     add_event("-BLOCK   HR"); break;
        case MD_BLOCK_UL:     add_event("-BLOCK   UL"); break;
        case MD_BLOCK_OL:     add_event("-BLOCK   OL"); break;
        case MD_BLOCK_LI:     add_event("-BLOCK   LI"); break;
        case MD_BLOCK_QUOTE:  add_event("-BLOCK   QUOTE"); break;
        case MD_BLOCK_HTML:   add_event("-BLOCK   HTML"); break;
        case MD_BLOCK_TABLE:   add_event("-BLOCK   TABLE"); break;
        case MD_BLOCK_THEAD:   add_event("-BLOCK   THEAD"); break;
        case MD_BLOCK_TBODY:   add_event("-BLOCK   TBODY"); break;
        case MD_BLOCK_TR:     add_event("-BLOCK   TR"); break;
        case MD_BLOCK_TH:     add_event("-BLOCK   TH"); break;
        case MD_BLOCK_TD:     add_event("-BLOCK   TD"); break;
        case MD_BLOCK_FOOTNOTE_DEF_SECTION: add_event("-BLOCK   FOOTNOTE_DEF_SECTION"); break;
        case MD_BLOCK_FOOTNOTE_DEF: add_event("-BLOCK   FOOTNOTE_DEF"); break;
        case MD_BLOCK_ADMONITION:  add_event("-BLOCK   ADMONITION"); break;
#ifdef MD4C_STREAMING
        case MD_BLOCK_REFERENCE_SECTION: add_event("-BLOCK   REFERENCE_SECTION"); break;
#endif
        default: add_event("-BLOCK   type=%d", type); break;
    }
    return 0;
}


/*
 * Span callbacks
 */

static int
enter_span(MD_SPANTYPE type, void* detail, void* userdata)
{
    (void)userdata;
    switch(type) {
        case MD_SPAN_EM:       add_event(">SPAN    MD_SPAN_EM"); break;
        case MD_SPAN_STRONG:   add_event(">SPAN    MD_SPAN_STRONG"); break;
        case MD_SPAN_DEL:      add_event(">SPAN    MD_SPAN_DEL"); break;
        case MD_SPAN_U:        add_event(">SPAN    MD_SPAN_U"); break;
        case MD_SPAN_MARK:     add_event(">SPAN    MD_SPAN_MARK"); break;
        case MD_SPAN_CODE:     add_event(">SPAN    MD_SPAN_CODE"); break;
        case MD_SPAN_A:        add_event(">SPAN    MD_SPAN_A"); break;
        case MD_SPAN_IMG:      add_event(">SPAN    MD_SPAN_IMG"); break;
        case MD_SPAN_LATEXMATH:        add_event(">SPAN    MD_SPAN_LATEXMATH"); break;
        case MD_SPAN_LATEXMATH_DISPLAY: add_event(">SPAN    MD_SPAN_LATEXMATH_DISPLAY"); break;
        case MD_SPAN_WIKILINK: add_event(">SPAN    MD_SPAN_WIKILINK"); break;
        case MD_SPAN_FOOTNOTE_REF: {
            MD_SPAN_FOOTNOTE_REF_DETAIL* d = (MD_SPAN_FOOTNOTE_REF_DETAIL*)detail;
            add_event(">SPAN    MD_SPAN_FOOTNOTE_REF  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        case MD_SPAN_SPOILER:  add_event(">SPAN    MD_SPAN_SPOILER"); break;
        case MD_SPAN_SUPERSCRIPT: add_event(">SPAN    MD_SPAN_SUPERSCRIPT"); break;
        case MD_SPAN_SUBSCRIPT:   add_event(">SPAN    MD_SPAN_SUBSCRIPT"); break;
#ifdef MD4C_STREAMING
        case MD_SPAN_REFERENCE_LINK: {
            MD_SPAN_REFERENCE_LINK_DETAIL* d = (MD_SPAN_REFERENCE_LINK_DETAIL*)detail;
            add_event(">SPAN    MD_SPAN_REFERENCE_LINK  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        case MD_SPAN_REFERENCE_IMAGE: {
            MD_SPAN_REFERENCE_LINK_DETAIL* d = (MD_SPAN_REFERENCE_LINK_DETAIL*)detail;
            add_event(">SPAN    MD_SPAN_REFERENCE_IMAGE  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
#endif
        default: add_event(">SPAN    type=%d", type); break;
    }
    return 0;
}

static int
leave_span(MD_SPANTYPE type, void* detail, void* userdata)
{
    (void)detail; (void)userdata;
    switch(type) {
        case MD_SPAN_EM:       add_event("<SPAN    MD_SPAN_EM"); break;
        case MD_SPAN_STRONG:   add_event("<SPAN    MD_SPAN_STRONG"); break;
        case MD_SPAN_DEL:      add_event("<SPAN    MD_SPAN_DEL"); break;
        case MD_SPAN_U:        add_event("<SPAN    MD_SPAN_U"); break;
        case MD_SPAN_MARK:     add_event("<SPAN    MD_SPAN_MARK"); break;
        case MD_SPAN_CODE:     add_event("<SPAN    MD_SPAN_CODE"); break;
        case MD_SPAN_A:        add_event("<SPAN    MD_SPAN_A"); break;
        case MD_SPAN_IMG:      add_event("<SPAN    MD_SPAN_IMG"); break;
        case MD_SPAN_LATEXMATH:        add_event("<SPAN    MD_SPAN_LATEXMATH"); break;
        case MD_SPAN_LATEXMATH_DISPLAY: add_event("<SPAN    MD_SPAN_LATEXMATH_DISPLAY"); break;
        case MD_SPAN_WIKILINK: add_event("<SPAN    MD_SPAN_WIKILINK"); break;
        case MD_SPAN_FOOTNOTE_REF: add_event("<SPAN    MD_SPAN_FOOTNOTE_REF"); break;
        case MD_SPAN_SPOILER:  add_event("<SPAN    MD_SPAN_SPOILER"); break;
        case MD_SPAN_SUPERSCRIPT: add_event("<SPAN    MD_SPAN_SUPERSCRIPT"); break;
        case MD_SPAN_SUBSCRIPT:   add_event("<SPAN    MD_SPAN_SUBSCRIPT"); break;
#ifdef MD4C_STREAMING
        case MD_SPAN_REFERENCE_LINK:  add_event("<SPAN    MD_SPAN_REFERENCE_LINK"); break;
        case MD_SPAN_REFERENCE_IMAGE: add_event("<SPAN    MD_SPAN_REFERENCE_IMAGE"); break;
#endif
        default: add_event("<SPAN    type=%d", type); break;
    }
    return 0;
}


/*
 * Text callback
 */

static int
text_cb(MD_TEXTTYPE type, const MD_CHAR* text, MD_SIZE size, void* userdata)
{
    const char* tn;
    (void)userdata;
    switch(type) {
        case MD_TEXT_NORMAL:    tn = "NORMAL"; break;
        case MD_TEXT_NULLCHAR:  tn = "NULLCHAR"; break;
        case MD_TEXT_BR:        tn = "BR"; break;
        case MD_TEXT_SOFTBR:    tn = "SOFTBR"; break;
        case MD_TEXT_ENTITY:    tn = "ENTITY"; break;
        case MD_TEXT_CODE:      tn = "CODE"; break;
        case MD_TEXT_HTML:      tn = "HTML"; break;
        case MD_TEXT_LATEXMATH: tn = "LATEXMATH"; break;
        default: tn = "?"; break;
    }
    add_event("TEXT  \"%.*s\"  (%s)", (int)size, text, tn);
    return 0;
}


static void
dump_events(void)
{
    int i;
    printf("%d events\n", event_count);
    for(i = 0; i < event_count; i++)
        printf("%s\n", event_lines[i]);
}


#ifdef MD4C_STREAMING

/* Streaming mode: use md_stream_init/feed/flush/finish */

int
main(int argc, char* argv[])
{
    MD_PARSER parser;
    MD_PARSER_CTX* ctx = NULL;
    char* input = NULL;
    size_t input_size = 0;
    size_t input_cap = 0;
    int ch;
    int ret;

    while((ch = fgetc(stdin)) != EOF) {
        if(input_size + 1 >= input_cap) {
            input_cap = input_cap ? input_cap * 2 : 65536;
            input = (char*) realloc(input, input_cap);
            if(!input) { fprintf(stderr, "oom\n"); return 1; }
        }
        input[input_size++] = (char)ch;
    }
    if(input) input[input_size] = '\0';

    memset(&parser, 0, sizeof(parser));
    parser.abi_version = 0;
    parser.flags = (argc > 1) ? (unsigned)strtoul(argv[1], NULL, 0) : 0;
    parser.enter_block = enter_block;
    parser.leave_block = leave_block;
    parser.enter_span  = enter_span;
    parser.leave_span  = leave_span;
    parser.text        = text_cb;

    ret = md_stream_init(&parser, NULL, &ctx);
    if(ret != 0) { fprintf(stderr, "md_stream_init error %d\n", ret); return 1; }

    ret = md_stream_feed(ctx, (const MD_CHAR*)input, (MD_SIZE)input_size);
    if(ret != 0) { fprintf(stderr, "md_stream_feed error %d\n", ret); return 1; }

    ret = md_stream_flush(ctx);
    if(ret != 0) { fprintf(stderr, "md_stream_flush error %d\n", ret); return 1; }

    md_stream_finish(ctx);
    dump_events();
    free(input);
    return 0;
}

#else

/* Batch mode: use md_parse */

int
main(int argc, char* argv[])
{
    MD_PARSER parser;
    char* input = NULL;
    size_t input_size = 0;
    size_t input_cap = 0;
    int ch;
    int ret;

    while((ch = fgetc(stdin)) != EOF) {
        if(input_size + 1 >= input_cap) {
            input_cap = input_cap ? input_cap * 2 : 65536;
            input = (char*) realloc(input, input_cap);
            if(!input) { fprintf(stderr, "oom\n"); return 1; }
        }
        input[input_size++] = (char)ch;
    }
    if(input) input[input_size] = '\0';

    memset(&parser, 0, sizeof(parser));
    parser.abi_version = 0;
    parser.flags = (argc > 1) ? (unsigned)strtoul(argv[1], NULL, 0) : 0;
    parser.enter_block = enter_block;
    parser.leave_block = leave_block;
    parser.enter_span  = enter_span;
    parser.leave_span  = leave_span;
    parser.text        = text_cb;

    ret = md_parse((const MD_CHAR*)input, (MD_SIZE)input_size, &parser, NULL);
    if(ret != 0) { fprintf(stderr, "md_parse error %d\n", ret); return 1; }

    dump_events();
    free(input);
    return 0;
}

#endif
