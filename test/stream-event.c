/*
 * stream-event — streaming parser test binary.
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
 * Reads markdown from stdin via the streaming API (md_stream_*),
 * prints callback events to stdout.  First line: "N events".
 *
 * Build:
 *   gcc -DMD4C_STREAMING -I src -o test/stream-event test/stream-event.c src/md4cs.c
 *
 * Usage:
 *   echo "markdown" | ./stream-event [--footnotes]
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


static int
enter_block(MD_BLOCKTYPE type, void* detail, void* userdata)
{
    (void)detail; (void)userdata;
    switch(type) {
        case MD_BLOCK_DOC:    add_event("+BLOCK   DOC"); break;
        case MD_BLOCK_P:      add_event("+BLOCK   P"); break;
        case MD_BLOCK_H:      add_event("+BLOCK   H"); break;
        case MD_BLOCK_CODE:   add_event("+BLOCK   CODE"); break;
        case MD_BLOCK_HR:     add_event("+BLOCK   HR"); break;
        case MD_BLOCK_UL:     add_event("+BLOCK   UL"); break;
        case MD_BLOCK_OL:     add_event("+BLOCK   OL"); break;
        case MD_BLOCK_LI:     add_event("+BLOCK   LI"); break;
        case MD_BLOCK_QUOTE:  add_event("+BLOCK   QUOTE"); break;
        case MD_BLOCK_HTML:   add_event("+BLOCK   HTML"); break;
        case MD_BLOCK_TABLE:  add_event("+BLOCK   TABLE"); break;
        case MD_BLOCK_THEAD:  add_event("+BLOCK   THEAD"); break;
        case MD_BLOCK_TBODY:  add_event("+BLOCK   TBODY"); break;
        case MD_BLOCK_TR:     add_event("+BLOCK   TR"); break;
        case MD_BLOCK_TH:     add_event("+BLOCK   TH"); break;
        case MD_BLOCK_TD:     add_event("+BLOCK   TD"); break;
        case MD_BLOCK_FOOTNOTE_DEF_SECTION: add_event("+BLOCK   FOOTNOTE_DEF_SECTION"); break;
        case MD_BLOCK_FOOTNOTE_DEF: {
            MD_BLOCK_FOOTNOTE_DEF_DETAIL* d = (MD_BLOCK_FOOTNOTE_DEF_DETAIL*)detail;
            add_event("+BLOCK   FOOTNOTE_DEF  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        case MD_BLOCK_ADMONITION:    add_event("+BLOCK   ADMONITION"); break;
        case MD_BLOCK_REFERENCE_SECTION: add_event("+BLOCK   REFERENCE_SECTION"); break;
        default: break;
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
        case MD_BLOCK_TABLE:  add_event("-BLOCK   TABLE"); break;
        case MD_BLOCK_THEAD:  add_event("-BLOCK   THEAD"); break;
        case MD_BLOCK_TBODY:  add_event("-BLOCK   TBODY"); break;
        case MD_BLOCK_TR:     add_event("-BLOCK   TR"); break;
        case MD_BLOCK_TH:     add_event("-BLOCK   TH"); break;
        case MD_BLOCK_TD:     add_event("-BLOCK   TD"); break;
        case MD_BLOCK_FOOTNOTE_DEF_SECTION: add_event("-BLOCK   FOOTNOTE_DEF_SECTION"); break;
        case MD_BLOCK_FOOTNOTE_DEF:         add_event("-BLOCK   FOOTNOTE_DEF"); break;
        case MD_BLOCK_ADMONITION:    add_event("-BLOCK   ADMONITION"); break;
        case MD_BLOCK_REFERENCE_SECTION: add_event("-BLOCK   REFERENCE_SECTION"); break;
        default: break;
    }
    return 0;
}

static int
enter_span(MD_SPANTYPE type, void* detail, void* userdata)
{
    (void)userdata;
    switch(type) {
        case MD_SPAN_EM:       add_event(">SPAN    EM"); break;
        case MD_SPAN_STRONG:   add_event(">SPAN    STRONG"); break;
        case MD_SPAN_DEL:      add_event(">SPAN    DEL"); break;
        case MD_SPAN_U:        add_event(">SPAN    U"); break;
        case MD_SPAN_MARK:     add_event(">SPAN    MARK"); break;
        case MD_SPAN_CODE:     add_event(">SPAN    CODE"); break;
        case MD_SPAN_A:        add_event(">SPAN    A"); break;
        case MD_SPAN_IMG:      add_event(">SPAN    IMG"); break;
        case MD_SPAN_LATEXMATH:        add_event(">SPAN    LATEXMATH"); break;
        case MD_SPAN_LATEXMATH_DISPLAY: add_event(">SPAN    LATEXMATH_DISPLAY"); break;
        case MD_SPAN_WIKILINK: add_event(">SPAN    WIKILINK"); break;
        case MD_SPAN_FOOTNOTE_REF: {
            MD_SPAN_FOOTNOTE_REF_DETAIL* d = (MD_SPAN_FOOTNOTE_REF_DETAIL*)detail;
            add_event(">SPAN    FOOTNOTE_REF  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        case MD_SPAN_SPOILER:  add_event(">SPAN    SPOILER"); break;
        case MD_SPAN_SUPERSCRIPT: add_event(">SPAN    SUPERSCRIPT"); break;
        case MD_SPAN_SUBSCRIPT:   add_event(">SPAN    SUBSCRIPT"); break;
        case MD_SPAN_REFERENCE_LINK: {
            MD_SPAN_REFERENCE_LINK_DETAIL* d = (MD_SPAN_REFERENCE_LINK_DETAIL*)detail;
            add_event(">SPAN    REF_LINK  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        case MD_SPAN_REFERENCE_IMAGE: {
            MD_SPAN_REFERENCE_LINK_DETAIL* d = (MD_SPAN_REFERENCE_LINK_DETAIL*)detail;
            add_event(">SPAN    REF_IMAGE  {label=\"%.*s\"}", (int)d->label.size, d->label.text);
            break;
        }
        default: break;
    }
    return 0;
}

static int
leave_span(MD_SPANTYPE type, void* detail, void* userdata)
{
    (void)detail; (void)userdata;
    switch(type) {
        case MD_SPAN_EM:       add_event("<SPAN    EM"); break;
        case MD_SPAN_STRONG:   add_event("<SPAN    STRONG"); break;
        case MD_SPAN_DEL:      add_event("<SPAN    DEL"); break;
        case MD_SPAN_U:        add_event("<SPAN    U"); break;
        case MD_SPAN_MARK:     add_event("<SPAN    MARK"); break;
        case MD_SPAN_CODE:     add_event("<SPAN    CODE"); break;
        case MD_SPAN_A:        add_event("<SPAN    A"); break;
        case MD_SPAN_IMG:      add_event("<SPAN    IMG"); break;
        case MD_SPAN_LATEXMATH:        add_event("<SPAN    LATEXMATH"); break;
        case MD_SPAN_LATEXMATH_DISPLAY: add_event("<SPAN    LATEXMATH_DISPLAY"); break;
        case MD_SPAN_WIKILINK: add_event("<SPAN    WIKILINK"); break;
        case MD_SPAN_FOOTNOTE_REF:      add_event("<SPAN    FOOTNOTE_REF"); break;
        case MD_SPAN_SPOILER:  add_event("<SPAN    SPOILER"); break;
        case MD_SPAN_SUPERSCRIPT: add_event("<SPAN    SUPERSCRIPT"); break;
        case MD_SPAN_SUBSCRIPT:   add_event("<SPAN    SUBSCRIPT"); break;
        case MD_SPAN_REFERENCE_LINK:  add_event("<SPAN    REF_LINK"); break;
        case MD_SPAN_REFERENCE_IMAGE: add_event("<SPAN    REF_IMAGE"); break;
        default: break;
    }
    return 0;
}

static int
text_cb(MD_TEXTTYPE type, const MD_CHAR* text, MD_SIZE size, void* userdata)
{
    const char* tn;
    char escaped[MAX_LINE];
    int i, j;
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
    j = 0;
    for(i = 0; i < (int)size && j < (int)sizeof(escaped) - 3; i++) {
        if(text[i] == '\n')       { escaped[j++] = '\\'; escaped[j++] = 'n'; }
        else if(text[i] == '\r')  { escaped[j++] = '\\'; escaped[j++] = 'r'; }
        else if(text[i] == '\\')  { escaped[j++] = '\\'; escaped[j++] = '\\'; }
        else if(text[i] == '"')   { escaped[j++] = '\\'; escaped[j++] = '"'; }
        else                         { escaped[j++] = (char)text[i]; }
    }
    escaped[j] = '\0';
    add_event("TEXT  \"%s\"  (%s)", escaped, tn);
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
    int use_footnotes = 0;
    size_t chunk_size = 0;

    if(argc > 1  &&  strcmp(argv[1], "--footnotes") == 0)
        use_footnotes = 1;

    if(argc > 1  &&  strcmp(argv[1], "--chunk") == 0) {
        if(argc > 2)
            chunk_size = (size_t) atol(argv[2]);
    }

    while((ch = fgetc(stdin)) != EOF) {
        if(input_size + 1 >= input_cap) {
            input_cap = input_cap ? input_cap * 2 : 65536;
            input = (char*) realloc(input, input_cap);
            if(!input) { fprintf(stderr, "oom\n"); return 1; }
        }
        input[input_size++] = (char)ch;
    }

    memset(&parser, 0, sizeof(parser));
    parser.abi_version = 0;
    parser.flags = use_footnotes ? MD_FLAG_FOOTNOTES : 0;
    parser.enter_block = enter_block;
    parser.leave_block = leave_block;
    parser.enter_span  = enter_span;
    parser.leave_span  = leave_span;
    parser.text        = text_cb;

    ret = md_stream_init(&parser, NULL, &ctx);
    if(ret != 0) { fprintf(stderr, "md_stream_init error %d\n", ret); return 1; }

    if(chunk_size > 0) {
        /* Feed in fixed-size chunks so compaction runs between feeds;
         * the event stream must be identical to a single-shot feed. */
        size_t off = 0;

        while(off < input_size) {
            size_t n = input_size - off;

            if(n > chunk_size)
                n = chunk_size;
            ret = md_stream_feed(ctx, (const MD_CHAR*) input + off,
                                 (MD_SIZE) n);
            if(ret != 0) { fprintf(stderr, "md_stream_feed error %d\n", ret); return 1; }
            off += n;
        }
    } else {
        ret = md_stream_feed(ctx, (const MD_CHAR*)input, (MD_SIZE)input_size);
        if(ret != 0) { fprintf(stderr, "md_stream_feed error %d\n", ret); return 1; }
    }

    ret = md_stream_flush(ctx);
    if(ret != 0) { fprintf(stderr, "md_stream_flush error %d\n", ret); return 1; }

    md_stream_finish(ctx);
    dump_events();
    free(input);
    return 0;
}
