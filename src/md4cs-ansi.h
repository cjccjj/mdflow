/*
 * md4cs-ansi: ANSI/SGR terminal renderer for mdflow
 * (https://github.com/cjccjj/mdflow)
 *
 * Converts Markdown to ANSI/SGR terminal output using a configurable theme.
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
 */

#ifndef MD4C_ANSI_H
#define MD4C_ANSI_H

#ifdef __cplusplus
    extern "C" {
#endif

#include "md4cs.h"


/* A single ANSI style: prefix (SGR open) and suffix (SGR reset/close). */
typedef struct MD_ANSI_STYLE_tag {
    const char* prefix;         /* e.g. "\033[1;34m" */
    int         prefix_len;
    const char* suffix;         /* usually "\033[0m" */
    int         suffix_len;
} MD_ANSI_STYLE;

/* Theme: styles for every Markdown element type. */
typedef struct MD_ANSI_THEME_tag {
    MD_ANSI_STYLE h1, h2, h3, h4, h5, h6;
    MD_ANSI_STYLE bold, italic, strikethrough;
    MD_ANSI_STYLE inline_code, code_block, code_block_lang;
    MD_ANSI_STYLE hl_normal, hl_keyword, hl_punct, hl_string, hl_comment;
    MD_ANSI_STYLE hr;
    MD_ANSI_STYLE bullet_item, ordered_item;
    MD_ANSI_STYLE blockquote;
    MD_ANSI_STYLE table_border, table_header, table_cell;
    MD_ANSI_STYLE link_text, link_url;
    MD_ANSI_STYLE image_label;
    MD_ANSI_STYLE text;
    MD_ANSI_STYLE footnote_ref;
    MD_ANSI_STYLE admonition;
    MD_ANSI_STYLE admonition_note;
    MD_ANSI_STYLE admonition_tip;
    MD_ANSI_STYLE admonition_important;
    MD_ANSI_STYLE admonition_warning;
    MD_ANSI_STYLE admonition_caution;

    MD_ANSI_STYLE underline;
    MD_ANSI_STYLE highlight;
    MD_ANSI_STYLE spoiler_label;
    MD_ANSI_STYLE spoiler;
    MD_ANSI_STYLE superscript;
    MD_ANSI_STYLE subscript;
} MD_ANSI_THEME;


/* Returns the built-in default theme (statically allocated). */
const MD_ANSI_THEME* md_ansi_default_theme(void);


/* Batch render: convert Markdown to ANSI output.
 *
 * Parameters:
 *   text      — Markdown input (may contain null bytes)
 *   size      — input size
 *   theme     — ANSI theme to use (NULL = default)
 *   output    — callback invoked for each chunk of ANSI output
 *   userdata  — passed through to output callback
 *
 * Returns 0 on success, or non-zero if a callback returned non-zero.
 */
int md_ansi(const MD_CHAR* text, MD_SIZE size,
            const MD_ANSI_THEME* theme,
            void (*output)(const char* str, int size, void* userdata),
            void* userdata);


/* Callback implementations for use with md_parse() or md_stream_*().
 *
 * Usage:
 *     MD_PARSER parser;
 *     MD_ANSI_RENDERER renderer;
 *
 *     md_ansi_renderer_init(&renderer, theme, output_cb, userdata);
 *     parser.enter_block = md_ansi_enter_block;
 *     parser.leave_block = md_ansi_leave_block;
 *     parser.enter_span  = md_ansi_enter_span;
 *     parser.leave_span  = md_ansi_leave_span;
 *     parser.text        = md_ansi_text;
 *
 *     // batch:
 *     md_parse(text, size, &parser, &renderer);
 *     // or streaming:
 *     md_stream_init(&parser, &renderer, &stream_ctx);
 */

/* Opaque renderer state (defined in md4cs-ansi.c). */
typedef struct MD_ANSI_RENDERER_tag MD_ANSI_RENDERER;

/* Create and initialize a renderer.
 * Returns pointer to the new renderer, or NULL on error. */
MD_ANSI_RENDERER* md_ansi_renderer_create(
                           const MD_ANSI_THEME* theme,
                           void (*output)(const char* str, int size, void* userdata),
                           void* userdata);

/* Destroy a renderer created with md_ansi_renderer_create. */
void md_ansi_renderer_destroy(MD_ANSI_RENDERER* renderer);

/* Set the terminal width for table width limiting (0 = no limit). */
void md_ansi_set_term_width(MD_ANSI_RENDERER* renderer, int term_width);

/* Parser callbacks. */
int md_ansi_enter_block(MD_BLOCKTYPE type, void* detail, void* userdata);
int md_ansi_leave_block(MD_BLOCKTYPE type, void* detail, void* userdata);
int md_ansi_enter_span(MD_SPANTYPE type, void* detail, void* userdata);
int md_ansi_leave_span(MD_SPANTYPE type, void* detail, void* userdata);
int md_ansi_text(MD_TEXTTYPE type, const MD_CHAR* text, MD_SIZE size, void* userdata);


#ifdef __cplusplus
    }  /* extern "C" { */
#endif

#endif  /* MD4C_ANSI_H */
