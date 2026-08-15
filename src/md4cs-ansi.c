/*
 * md4cs-ansi: ANSI/SGR terminal renderer for mdflow
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
 */

#include "md4cs-ansi.h"
#include "entity.h"
#include "highlight.h"
#include "html.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <limits.h>


#define MAX_BLOCK_DEPTH         64
#define MAX_ISOLATED_DEPTH     16

struct TableColumn {
    int natural_width;
    int rendered_width;
    int min_width;
    int align;
};

struct TableCell {
    int offset;
    int len;
};

struct TableRow {
    int first_cell;
    int n_cells;
};


/* Forward decls used by early helpers. */
static void emit_bq_prefix(MD_ANSI_RENDERER* r);
static int in_quote_context(const MD_ANSI_RENDERER* r);
static int block_has_style(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style);
static void code_block_flush(MD_ANSI_RENDERER* r);
static void reference_line_emit(MD_ANSI_RENDERER* r, const char* line, int len);


/* Renderer state — persists across callbacks. */
struct MD_ANSI_RENDERER_tag {
    const MD_ANSI_THEME* theme;
    void (*output)(const char* str, int size, void* userdata);
    void* userdata;
    int failed;                 /* allocation/rendering failure */

    /* Output spool: coalesce per-span writes into one callback call per
     * complete line (bounded; flushed on newline, on overflow, or at
     * destroy). */
    char* out_spool;
    int out_spool_len;

    /* Block style stack — prefixes of blocks that have non-empty styles. */
    const char* block_style_prefix[MAX_BLOCK_DEPTH];
    int block_style_depth;

    /* Isolated span style stack — link, code, highlight, image label,
     * footnote ref, superscript, subscript. Re-applied after every reset so
     * nested inline spans keep the enclosing span styling. */
    const MD_ANSI_STYLE* isolated_style_stack[MAX_ISOLATED_DEPTH];
    int isolated_style_depth;

    /* Emphasis nesting counters. */
    int n_bold;
    int n_italic;
    int n_strikethrough;
    int n_underline;

    /* Spacing: emit blank line before next block content. */
    int need_blank_line;

    /* Context flags. */
    int blockquote_depth;
    int blockquote_needs_prefix;
    int in_list_item;
    int list_item_just_opened;
    int in_list_ordered;
    int list_depth;
    int list_item_number;
    int saved_list_item_number[MAX_BLOCK_DEPTH];
    int saved_in_list_ordered[MAX_BLOCK_DEPTH];
    int code_had_content;
    int in_html_block;

    /* Code-block highlighting: active only when the fence label names a
     * recognized major language (see enter_code); the scanner state is
     * fed per MD_TEXT_CODE chunk and tokens are emitted as they
     * finalize (line by line). */
    int hl_active;
    MD_HL_STATE hl_state;

    /* OSC 8 hyperlink emission (on by default). */
    int osc8_enabled;

    /* Href saved from inline HTML <a> tag. */
    char html_link_href[512];
    int html_link_href_len;

    /* HTML construct scan state — persists across MD_TEXT_HTML callbacks
     * so a construct split by the parser (comment, PI, decl, CDATA) stays
     * in the same logical section until its end marker arrives. */
    HTML_SCAN_STATE html_state;

    int spoiler_active;
    int admonition_active;
    const MD_ANSI_STYLE* admonition_style;

    /* Track last output character (for deduplicating newlines). */
    char last_char;

    /* Link URL for rendering after link text. */
    char link_href[512];
    int link_href_len;

    /* Image src and title for [IMG: ...] rendering. */
    char img_src[512];
    int img_src_len;
    char img_title[256];
    int img_title_len;

    /* Reference link/image label for [text][label] rendering. */
    char ref_label[512];
    int ref_label_len;

    /* Reference-definition line accumulation (for OSC 8 URL wrapping). */
    int in_reference_section;
    char ref_line_buf[8192];
    int ref_line_len;

    /* Terminal width for table column limiting (0 = no limit). */
    int table_term_width;

    /* Table rendering state (buffer-and-render on TABLE close). */
#define MAX_REPAINT_COUNT 50
    int in_table;
    int in_table_cell;
    int table_col_count;
    int table_col;               /* current column being accumulated */
    int table_in_header;         /* currently inside THEAD block */
    int table_in_body;           /* currently inside TBODY block */
    int table_row_active;        /* current body row has a descriptor */
    int table_header_rendered;   /* top/header/separator already drawn */

    /* Dynamic column metadata. */
    struct TableColumn* table_columns;
    int table_col_capacity;

    /* Number of terminal lines the rendered table currently occupies
     * (for ANSI cursor-up redraw). */
    int table_lines;

    /* Redraw oscillation guard. */
    int table_repaint_count;

    /* Cell content accumulation (grows as needed). */
    char* table_cell_buf;
    int table_cell_cap;
    int table_cell_len;

    /* One arena owns all rendered cell bytes.  Rows and cells contain only
     * offsets and lengths, so late redraws do not require per-cell mallocs. */
    char* table_text;
    int table_text_len;
    int table_text_capacity;
    struct TableCell* table_cells;
    int table_cell_count;
    int table_cell_capacity;
    struct TableRow* table_rows;
    int table_row_capacity;       /* allocated rows */
    int table_n_rows;

    /* Header row is a range in table_cells. */
    int table_header_start;
    int table_header_cols;

};


/********************************
 ***  ANSI output helpers    ***
 ********************************/

/* When inside a table cell, all output redirects to the cell buffer
 * instead of stdout. This preserves ANSI formatting (bold, code, links)
 * within cell content for later table rendering. */
#define OUTPUT_SPOOL_CAP 8192

static void
write_output(MD_ANSI_RENDERER* r, const char* str, int len)
{
    int needed;

    if(len <= 0)
        return;
    if(r->failed)
        return;

    if(r->in_table_cell) {
        if(len > INT_MAX - r->table_cell_len - 1) {
            r->failed = 1;
            return;
        }
        needed = r->table_cell_len + len + 1;
        if(needed > r->table_cell_cap) {
            int new_cap;
            char* new_buf;

            if(r->table_cell_cap == 0)
                new_cap = 1024;
            else if(r->table_cell_cap > INT_MAX / 2)
                new_cap = needed;
            else
                new_cap = r->table_cell_cap * 2;
            while(new_cap < needed) {
                if(new_cap > INT_MAX / 2) {
                    new_cap = needed;
                    break;
                }
                new_cap *= 2;
            }
            new_buf = (char*) realloc(r->table_cell_buf, (size_t) new_cap);
            if(new_buf == NULL) {
                r->failed = 1;
                return;
            }
            r->table_cell_buf = new_buf;
            r->table_cell_cap = new_cap;
        }
        memcpy(r->table_cell_buf + r->table_cell_len, str, (size_t) len);
        r->table_cell_len += len;
    } else if(r->output != NULL) {
        /* Coalesce spans into the spool and emit once per complete line,
         * so each line costs one callback call instead of one per span.
         * A line is still flushed the instant its '\n' arrives. */
        if(len >= OUTPUT_SPOOL_CAP) {
            /* Oversized chunk: emit pending data, then pass it through. */
            if(r->out_spool_len > 0) {
                r->output(r->out_spool, r->out_spool_len, r->userdata);
                r->out_spool_len = 0;
            }
            r->output(str, len, r->userdata);
            r->last_char = str[len - 1];
        } else {
            if(r->out_spool == NULL) {
                r->out_spool = (char*) malloc(OUTPUT_SPOOL_CAP);
                if(r->out_spool == NULL) {
                    /* OOM: fall back to direct output. */
                    r->output(str, len, r->userdata);
                    r->last_char = str[len - 1];
                    return;
                }
            }
            if(r->out_spool_len + len > OUTPUT_SPOOL_CAP) {
                /* Spool full with no newline yet: emit what we have. */
                r->output(r->out_spool, r->out_spool_len, r->userdata);
                r->out_spool_len = 0;
            }
            memcpy(r->out_spool + r->out_spool_len, str, (size_t) len);
            r->out_spool_len += len;
            r->last_char = str[len - 1];
            if(r->out_spool[r->out_spool_len - 1] == '\n') {
                /* Spool ends at a line boundary: emit it in one call. */
                r->output(r->out_spool, r->out_spool_len, r->userdata);
                r->out_spool_len = 0;
            }
        }
    }
}

static void
write_str(MD_ANSI_RENDERER* r, const char* str)
{
    write_output(r, str, (int) strlen(str));
}

static void
write_sgr_reset(MD_ANSI_RENDERER* r)
{
    write_str(r, "\033[0m");
}

/* Write an OSC 8 hyperlink opener for href, sanitizing bytes that could
 * terminate or reshape the escape sequence (percent-encoded). */
static void
write_osc8_open(MD_ANSI_RENDERER* r, const char* href, int href_len)
{
    static const char hex[] = "0123456789ABCDEF";
    int i;

    write_str(r, "\033]8;;");
    for(i = 0; i < href_len; i++) {
        unsigned char c = (unsigned char) href[i];
        if(c == ';' || c == '\\' || c == ']' || c == '\033' || c == '\007'
                || c < 0x20 || c == 0x7f) {
            char tmp[4];
            tmp[0] = '%';
            tmp[1] = hex[c >> 4];
            tmp[2] = hex[c & 0x0f];
            tmp[3] = '\0';
            write_str(r, tmp);
        } else {
            write_output(r, href + i, 1);
        }
    }
    write_str(r, "\033\\");
}

/* Write the OSC 8 hyperlink terminator. */
static void
write_osc8_close(MD_ANSI_RENDERER* r)
{
    write_str(r, "\033]8;;\033\\");
}

/* Translate entity to its UTF-8 equivalent, or output the verbatim one
 * if such entity is unknown. */
static void
render_entity(MD_ANSI_RENDERER* r, const MD_CHAR* text, MD_SIZE size)
{
    char tmp[8];
    int n = entity_decode((const char*) text, (int) size, tmp);
    if(n > 0)
        write_output(r, tmp, n);
    else
        write_output(r, text, size);
}

/* Write an MD_ATTRIBUTE with entity decoding (null chars → U+FFFD). */
static void
render_attribute_to_output(MD_ANSI_RENDERER* r, const MD_ATTRIBUTE* attr)
{
    int i;
    for(i = 0; attr->substr_offsets[i] < attr->size; i++) {
        MD_TEXTTYPE type = attr->substr_types[i];
        MD_OFFSET off = attr->substr_offsets[i];
        MD_SIZE sz = attr->substr_offsets[i+1] - off;

        switch(type) {
            case MD_TEXT_NULLCHAR:
                write_output(r, "\xef\xbf\xbd", 3);
                break;
            case MD_TEXT_ENTITY:
                render_entity(r, attr->text + off, sz);
                break;
            default:
                write_output(r, attr->text + off, (int)sz);
                break;
        }
    }
}

/* Copy an MD_ATTRIBUTE into a fixed-size buffer with entity decoding.
 * Returns bytes written (not including trailing NUL). */
static int
copy_attribute_decoded(const MD_ATTRIBUTE* attr, char* buf, int buf_size)
{
    int off = 0;
    int i;
    for(i = 0; off < buf_size - 1  &&  attr->substr_offsets[i] < attr->size; i++) {
        MD_TEXTTYPE type = attr->substr_types[i];
        MD_OFFSET src_off = attr->substr_offsets[i];
        MD_SIZE sz = attr->substr_offsets[i+1] - src_off;
        const MD_CHAR* src = attr->text + src_off;
        int space = buf_size - 1 - off;

        switch(type) {
            case MD_TEXT_NULLCHAR:
                if(space >= 3) {
                    memcpy(buf + off, "\xef\xbf\xbd", 3);
                    off += 3;
                }
                break;

            case MD_TEXT_ENTITY: {
                char tmp[8];
                int n = entity_decode((const char*) src, (int) sz, tmp);
                if(n > 0) {
                    if(space >= n) {
                        memcpy(buf + off, tmp, n);
                        off += n;
                    }
                } else {
                    int copy = (int)sz;
                    if(copy > space) copy = space;
                    if(copy > 0) { memcpy(buf + off, src, copy); off += copy; }
                }
                break;
            }

            default:
                {
                    int copy = (int)sz;
                    if(copy > space) copy = space;
                    if(copy > 0) { memcpy(buf + off, src, copy); off += copy; }
                }
                break;
        }
    }
    buf[off] = '\0';
    return off;
}

/* A single wrapped slice — complete ANSI string, ready to output.
 * Allocated via malloc; free with free_slices(). */

struct WrappedSlice {
    char* text;     /* allocated NUL-terminated ANSI string */
    int len;        /* byte length (excluding NUL) */
    int visible;    /* visible display width (cached) */
};

/* Free all slices in an array. */
static void
free_slices(struct WrappedSlice* slices, int n)
{
    int i;
    for(i = 0; i < n; i++)
        free(slices[i].text);
}

/* Forward declarations for UTF-8 helpers used by wrap_content. */
static unsigned utf8_decode(const char* str, int len, int* p, int* p_len);
static int char_width(unsigned cp);
static int display_cluster(const char* str, int len, int start,
                           int* p_end, unsigned* p_cp, int* p_width);

#define WRAP_STYLE_CAP 4096

struct WrapStyleState {
    char data[WRAP_STYLE_CAP];
    int len;
};

struct WrapUnit {
    int start;
    int end;
    int width;
    unsigned cp;
    int is_space;
    int break_after;
    int forced_break;
    int no_break_before;
};

struct WrapLine {
    int first;
    int end;
    int visible;
};

/* Append bytes to a growable buffer. */
static int
wrap_buffer_append(char** p_buf, int* p_len, int* p_cap,
                   const char* text, int len)
{
    int needed;
    int new_cap;
    char* new_buf;

    if(len <= 0)
        return 0;
    if(*p_len > INT_MAX - len)
        return -1;

    needed = *p_len + len;
    if(needed >= *p_cap) {
        new_cap = (*p_cap > 0) ? *p_cap : 256;
        while(new_cap <= needed) {
            if(new_cap > INT_MAX / 2) {
                if(needed == INT_MAX)
                    return -1;
                new_cap = needed + 1;
                break;
            }
            new_cap *= 2;
        }
        new_buf = (char*) realloc(*p_buf, (size_t) new_cap);
        if(new_buf == NULL)
            return -1;
        *p_buf = new_buf;
        *p_cap = new_cap;
    }

    memcpy(*p_buf + *p_len, text, (size_t) len);
    *p_len += len;
    return 0;
}

/* Record one display unit in the growable wrapping input. */
static int
wrap_add_unit(struct WrapUnit** p_units, int* p_count, int* p_cap,
              int start, int end, int width, unsigned cp,
              int is_space, int forced_break, int no_break_before)
{
    struct WrapUnit* new_units;
    int new_cap;

    if(*p_count >= *p_cap) {
        if(*p_count == INT_MAX)
            return -1;
        if(*p_cap > INT_MAX / 2)
            new_cap = *p_count + 1;
        else
            new_cap = (*p_cap > 0) ? *p_cap * 2 : 64;
        new_units = (struct WrapUnit*) realloc(*p_units,
                                               (size_t)new_cap * sizeof(**p_units));
        if(new_units == NULL)
            return -1;
        *p_units = new_units;
        *p_cap = new_cap;
    }

    (*p_units)[*p_count].start = start;
    (*p_units)[*p_count].end = end;
    (*p_units)[*p_count].width = width;
    (*p_units)[*p_count].cp = cp;
    (*p_units)[*p_count].is_space = is_space;
    (*p_units)[*p_count].break_after = 0;
    (*p_units)[*p_count].forced_break = forced_break;
    (*p_units)[*p_count].no_break_before = no_break_before;
    (*p_count)++;
    return 0;
}

/* Return true for breakable whitespace, excluding non-breaking spaces. */
static int
wrap_is_space(unsigned cp)
{
    return (cp == 0x0009 || cp == 0x0020 || cp == 0x000B || cp == 0x000C
            || cp == 0x001D || cp == 0x001E || cp == 0x001F
            || cp == 0x1680 || (cp >= 0x2000 && cp <= 0x200A)
            || cp == 0x205F || cp == 0x3000);
}

/* Return true for punctuation where breaking after the character is useful
 * for terminal text, especially URLs and path-like values. */
static int
wrap_is_punctuation(unsigned cp)
{
    switch(cp) {
        case '-': case '/': case '\\': case '_': case '.': case ',':
        case ';': case ':': case '!': case '?': case ')': case ']':
        case '}': case '%': case '&': case '=': case '+':
        case 0x3001: case 0x3002: case 0xFF0C: case 0xFF0E:
            return 1;
        default:
            return 0;
    }
}

/* Return the end of an SGR sequence, or end when the sequence is malformed. */
static int
wrap_sgr_end(const char* text, int len, int start)
{
    int i = start + 2;

    while(i < len && text[i] != 'm')
        i++;
    if(i < len)
        i++;
    return i;
}

/* Return the end of an OSC sequence (BEL or ESC \ termination). */
static int
wrap_osc_end(const char* text, int len, int start)
{
    int i = start + 2;

    while(i < len) {
        if(text[i] == '\007') {
            i++;
            break;
        }
        if(text[i] == '\033' && i + 1 < len && text[i + 1] == '\\') {
            i += 2;
            break;
        }
        i++;
    }
    return i;
}

/* The renderer emits full resets between style regions.  Preserve every
 * active SGR sequence after the most recent reset so a continuation line can
 * restore the same state without guessing from individual style names. */
static void
wrap_style_consume(struct WrapStyleState* state, const char* text, int len)
{
    int i;
    int has_reset = 0;

    if(len < 3)
        return;

    for(i = 2; i < len - 1; i++) {
        if(text[i] == '0'
                && (i == 2 || text[i - 1] == ';')
                && (i + 1 == len - 1 || text[i + 1] == ';')) {
            has_reset = 1;
            break;
        }
    }

    if(has_reset || (len == 3 && text[2] == 'm')) {
        state->len = 0;
        return;
    }

    if(state->len + len < WRAP_STYLE_CAP) {
        memcpy(state->data + state->len, text, (size_t) len);
        state->len += len;
    }
}

/* Advance a style state over a source range without copying visible text. */
static void
wrap_style_advance(const char* text, int start, int end,
                   struct WrapStyleState* state)
{
    int i = start;
    int j;

    while(i < end) {
        if(text[i] == '\033' && i + 1 < end && text[i + 1] == '[') {
            j = wrap_sgr_end(text, end, i);
            wrap_style_consume(state, text + i, j - i);
            i = j;
        } else if(text[i] == '\033' && i + 1 < end && text[i + 1] == ']') {
            i = wrap_osc_end(text, end, i);
        } else {
            i++;
        }
    }
}

/* Copy a source range while updating the style state for its SGR tokens. */
static int
wrap_copy_range(const char* text, int start, int end,
                struct WrapStyleState* state,
                char** p_buf, int* p_len, int* p_cap)
{
    int i = start;
    int j;

    while(i < end) {
        if(text[i] == '\033' && i + 1 < end && text[i + 1] == '[') {
            j = wrap_sgr_end(text, end, i);
            if(wrap_buffer_append(p_buf, p_len, p_cap, text + i, j - i) != 0)
                return -1;
            wrap_style_consume(state, text + i, j - i);
            i = j;
        } else {
            j = i + 1;
            if(wrap_buffer_append(p_buf, p_len, p_cap, text + i, j - i) != 0)
                return -1;
            i = j;
        }
    }
    return 0;
}

/* Append a completed slice to a dynamically growing slice array. */
static int
wrap_save_slice(struct WrappedSlice** p_slices, int* p_count, int* p_cap,
                const char* buf, int len, int visible)
{
    struct WrappedSlice* new_slices;
    int new_cap;
    char* copy;

    if(*p_count >= *p_cap) {
        if(*p_count == INT_MAX)
            return -1;
        if(*p_cap > INT_MAX / 2)
            new_cap = *p_count + 1;
        else
            new_cap = (*p_cap > 0) ? *p_cap * 2 : 8;
        new_slices = (struct WrappedSlice*) realloc(*p_slices,
                         (size_t)new_cap * sizeof(**p_slices));
        if(new_slices == NULL)
            return -1;
        *p_slices = new_slices;
        *p_cap = new_cap;
    }

    copy = (char*) malloc((size_t) len + 1);
    if(copy == NULL)
        return -1;
    if(len > 0)
        memcpy(copy, buf, (size_t) len);
    copy[len] = '\0';
    (*p_slices)[*p_count].text = copy;
    (*p_slices)[*p_count].len = len;
    (*p_slices)[*p_count].visible = visible;
    (*p_count)++;
    return 0;
}

/* Word-aware wrapping of ANSI-formatted text.
 *
 * Preferred breaks are whitespace and selected punctuation.  If no preferred
 * break fits, the current display unit is used as an emergency break.  The
 * output slices are independent strings: continuation slices begin with the
 * active SGR state, and non-final slices end with a reset. */
static int
wrap_content(const char* text, int len, int display_width,
             struct WrappedSlice** p_out)
{
    struct WrapUnit* units = NULL;
    struct WrapLine* lines = NULL;
    struct WrappedSlice* slices = NULL;
    struct WrapStyleState state;
    char* buf = NULL;
    int unit_count = 0;
    int unit_cap = 0;
    int line_count = 0;
    int line_cap = 0;
    int slice_count = 0;
    int slice_cap = 0;
    int line_first = 0;
    int line_width = 0;
    int last_break = -1;
    int last_break_next = -1;
    int last_break_visible = 0;
    int last_was_forced = 0;
    int i = 0;
    int j;
    int state_pos = 0;
    int buf_len = 0;
    int buf_cap = 0;
    int line;

    memset(&state, 0, sizeof(state));
    if(display_width < 1)
        display_width = 1;

    /* Tokenize visible units.  SGR sequences are intentionally not units;
     * they are carried by the source ranges and style-state scanner. */
    while(i < len) {
        unsigned cp;
        int width;
        int is_space;
        int forced = 0;
        int no_break_before;

        if(text[i] == '\033' && i + 1 < len && text[i + 1] == '[') {
            i = wrap_sgr_end(text, len, i);
            continue;
        }

        if(text[i] == '\033' && i + 1 < len && text[i + 1] == ']') {
            i = wrap_osc_end(text, len, i);
            continue;
        }

        if(text[i] == '\n' || text[i] == '\r') {
            j = i + 1;
            if(text[i] == '\r' && j < len && text[j] == '\n')
                j++;
            if(wrap_add_unit(&units, &unit_count, &unit_cap,
                             i, j, 0, 0, 0, 1, 0) != 0)
                goto error;
            i = j;
            continue;
        }

        display_cluster(text, len, i, &j, &cp, &width);
        is_space = wrap_is_space(cp);
        no_break_before = 0;
        if(width == 0)
            no_break_before = 1;

        if(wrap_add_unit(&units, &unit_count, &unit_cap,
                         i, j, width, cp, is_space, forced,
                         no_break_before) != 0)
            goto error;

        i = j;
    }

    /* Mark break opportunities after whitespace, punctuation, and between
     * adjacent wide characters such as CJK ideographs. */
    for(i = 0; i < unit_count; i++) {
        if(units[i].forced_break)
            continue;
        if(units[i].is_space || wrap_is_punctuation(units[i].cp)) {
            units[i].break_after = 1;
        } else if(units[i].width == 2 && i + 1 < unit_count
                  && !units[i + 1].forced_break
                  && units[i + 1].width == 2
                  && !units[i + 1].no_break_before) {
            units[i].break_after = 1;
        }
    }

    /* Select line ranges using greedy wrapping with the latest preferred
     * break that still fits.  Whitespace at a selected boundary is omitted
     * rather than emitted at the start or end of a terminal line. */
    i = 0;
    while(i < unit_count) {
        struct WrapUnit* unit = &units[i];

        if(unit->forced_break) {
            if(line_count >= line_cap) {
                struct WrapLine* new_lines;
                int new_cap;
                if(line_count == INT_MAX)
                    goto error;
                new_cap = (line_cap > INT_MAX / 2)
                        ? line_count + 1
                        : ((line_cap > 0) ? line_cap * 2 : 8);
                new_lines = (struct WrapLine*) realloc(lines,
                                  (size_t)new_cap * sizeof(*lines));
                if(new_lines == NULL)
                    goto error;
                lines = new_lines;
                line_cap = new_cap;
            }
            lines[line_count].first = line_first;
            lines[line_count].end = i;
            lines[line_count].visible = line_width;
            line_count++;
            line_first = i + 1;
            line_width = 0;
            last_break = -1;
            last_break_next = -1;
            last_break_visible = 0;
            last_was_forced = 1;
            i++;
            continue;
        }

        /* Drop whitespace selected as the first content of a new line. */
        if(i == line_first && unit->is_space) {
            line_first++;
            i = line_first;
            continue;
        }

        if(unit->width > 0 && line_width > 0
                && line_width + unit->width > display_width
                && !unit->no_break_before) {
            int boundary;
            int next;
            int emit_visible;

            if(last_break >= line_first) {
                boundary = last_break;
                next = last_break_next;
                emit_visible = last_break_visible;
                if(boundary == line_first) {
                    boundary = i;
                    next = i;
                    emit_visible = line_width;
                }
            } else {
                boundary = i;
                next = i;
                emit_visible = line_width;
            }

            if(boundary <= line_first) {
                boundary = i + 1;
                next = i + 1;
                emit_visible = line_width + unit->width;
            }

            if(line_count >= line_cap) {
                struct WrapLine* new_lines;
                int new_cap;
                if(line_count == INT_MAX)
                    goto error;
                new_cap = (line_cap > INT_MAX / 2)
                        ? line_count + 1
                        : ((line_cap > 0) ? line_cap * 2 : 8);
                new_lines = (struct WrapLine*) realloc(lines,
                                  (size_t)new_cap * sizeof(*lines));
                if(new_lines == NULL)
                    goto error;
                lines = new_lines;
                line_cap = new_cap;
            }
            lines[line_count].first = line_first;
            lines[line_count].end = boundary;
            lines[line_count].visible = emit_visible;
            line_count++;

            line_first = next;
            while(line_first < unit_count && units[line_first].is_space)
                line_first++;
            i = line_first;
            line_width = 0;
            last_break = -1;
            last_break_next = -1;
            last_break_visible = 0;
            last_was_forced = 0;
            continue;
        }

        line_width += unit->width;
        if(unit->break_after) {
            if(unit->is_space) {
                if(last_break < line_first || last_break_next != i) {
                    last_break = i;
                    last_break_visible = line_width - unit->width;
                }
                last_break_next = i + 1;
            } else {
                last_break = i + 1;
                last_break_next = i + 1;
                last_break_visible = line_width;
            }
        }
        last_was_forced = 0;
        i++;
    }

    if(line_first < unit_count || line_count == 0 || last_was_forced) {
        if(line_count >= line_cap) {
            struct WrapLine* new_lines;
            int new_cap;
            if(line_count == INT_MAX)
                goto error;
            new_cap = (line_cap > INT_MAX / 2)
                    ? line_count + 1
                    : ((line_cap > 0) ? line_cap * 2 : 8);
            new_lines = (struct WrapLine*) realloc(lines,
                              (size_t)new_cap * sizeof(*lines));
            if(new_lines == NULL)
                goto error;
            lines = new_lines;
            line_cap = new_cap;
        }
        lines[line_count].first = line_first;
        lines[line_count].end = unit_count;
        lines[line_count].visible = line_width;
        line_count++;
    }

    /* Materialize ranges as self-contained ANSI slices. */
    for(line = 0; line < line_count; line++) {
        int is_first = (line == 0);
        int is_last = (line == line_count - 1);
        int source_start;
        int source_end;

        if(lines[line].first < lines[line].end) {
            source_start = units[lines[line].first].start;
            source_end = units[lines[line].end - 1].end;
        } else if(lines[line].first < unit_count) {
            source_start = units[lines[line].first].start;
            source_end = source_start;
        } else {
            source_start = len;
            source_end = len;
        }

        /* The first slice must retain any SGR prefix emitted before the
         * first visible unit.  A final slice must retain trailing resets and
         * other non-visible SGR bytes emitted when the cell closes. */
        if(is_first)
            source_start = 0;
        if(is_last)
            source_end = len;

        wrap_style_advance(text, state_pos, source_start, &state);
        state_pos = source_start;
        buf_len = 0;
        if(!is_first
                && wrap_buffer_append(&buf, &buf_len, &buf_cap,
                                       state.data, state.len) != 0)
            goto error;
        if(wrap_copy_range(text, source_start, source_end, &state,
                           &buf, &buf_len, &buf_cap) != 0)
            goto error;
        if(!is_last
                && wrap_buffer_append(&buf, &buf_len, &buf_cap,
                                      "\033[0m", 4) != 0)
            goto error;
        if(wrap_save_slice(&slices, &slice_count, &slice_cap,
                           buf, buf_len, lines[line].visible) != 0)
            goto error;
        state_pos = source_end;
    }

    free(buf);
    free(lines);
    free(units);
    *p_out = slices;
    return slice_count;

error:
    free(buf);
    free(lines);
    free(units);
    free_slices(slices, slice_count);
    free(slices);
    *p_out = NULL;
    return 0;
}

/* Decode a UTF-8 character from str at offset *p.
 * Advances *p past the encoded character and returns the codepoint
 * and (via p_len) the byte length.  p_len is 1 for invalid sequences. */
static unsigned
utf8_decode(const char* str, int len, int* p, int* p_len)
{
    unsigned ch;
    int i = *p;
    int n;

    if(i >= len) { *p_len = 0; return 0; }

    if((str[i] & 0x80) == 0) {
        ch = (unsigned char)str[i];
        n = 1;
    } else if((str[i] & 0xE0) == 0xC0 && i + 1 < len) {
        ch = ((unsigned char)str[i] & 0x1F) << 6
           | ((unsigned char)str[i+1] & 0x3F);
        n = 2;
    } else if((str[i] & 0xF0) == 0xE0 && i + 2 < len) {
        ch = ((unsigned char)str[i] & 0x0F) << 12
           | ((unsigned char)str[i+1] & 0x3F) << 6
           | ((unsigned char)str[i+2] & 0x3F);
        n = 3;
    } else if((str[i] & 0xF8) == 0xF0 && i + 3 < len) {
        ch = ((unsigned char)str[i] & 0x07) << 18
           | ((unsigned char)str[i+1] & 0x3F) << 12
           | ((unsigned char)str[i+2] & 0x3F) << 6
           | ((unsigned char)str[i+3] & 0x3F);
        n = 4;
    } else {
        ch = (unsigned char)str[i];
        n = 1;
    }

    *p = i + n;
    *p_len = n;
    return ch;
}

/* Return the display width of a Unicode codepoint:
 *   0 = combining/zero-width
 *   2 = CJK / emoji / wide
 *   1 = everything else
 *
 * Ranges are grouped by Unicode block and ordered by codepoint.
 */
static int
char_width(unsigned cp)
{
    /* ── Zero-width combining marks (General Category Mn, Mc, Me) ── */
    if(cp == 0x00AD) return 0;
    if(cp >= 0x0300 && cp <= 0x036F) return 0;   /* Combining Diacritical Marks */
    if(cp >= 0x0483 && cp <= 0x0489) return 0;
    if(cp >= 0x0591 && cp <= 0x05BD) return 0;
    if(cp == 0x05BF || cp == 0x05C7) return 0;
    if(cp >= 0x05C1 && cp <= 0x05C2) return 0;
    if(cp >= 0x05C4 && cp <= 0x05C5) return 0;
    if(cp >= 0x0610 && cp <= 0x061A) return 0;
    if(cp >= 0x064B && cp <= 0x065F) return 0;
    if(cp == 0x0670) return 0;
    if(cp >= 0x06D6 && cp <= 0x06DC) return 0;
    if(cp >= 0x06DF && cp <= 0x06E4) return 0;
    if(cp >= 0x06E7 && cp <= 0x06E8) return 0;
    if(cp >= 0x06EA && cp <= 0x06ED) return 0;
    if(cp == 0x0711) return 0;
    if(cp >= 0x0730 && cp <= 0x074A) return 0;
    if(cp >= 0x07A6 && cp <= 0x07B0) return 0;
    if(cp >= 0x0900 && cp <= 0x0902) return 0;
    if(cp == 0x093A) return 0;
    if(cp >= 0x0941 && cp <= 0x0948) return 0;
    if(cp >= 0x0962 && cp <= 0x0963) return 0;

    /* ── Zero-width: Variation Selectors ── */
    if(cp >= 0xFE00 && cp <= 0xFE0F) return 0;    /* Variation Selectors */
    if(cp >= 0xE0100 && cp <= 0xE01EF) return 0;  /* Variation Selectors Supplement */

    /* Zero-width joiners, format controls, and the enclosing keycap. */
    if(cp >= 0x200B && cp <= 0x200F) return 0;
    if(cp == 0x20E3) return 0;
    if(cp >= 0xE0020 && cp <= 0xE007F) return 0;

    /* ── Width 2: East Asian / CJK ── */
    /* Hangul */
    if(cp >= 0x1100 && cp <= 0x115F) return 2;    /* Hangul Jamo */
    if(cp >= 0xA960 && cp <= 0xA97C) return 2;    /* Hangul Jamo Extended-A */
    if(cp >= 0xAC00 && cp <= 0xD7A3) return 2;    /* Hangul Syllables */
    if(cp >= 0xD7B0 && cp <= 0xD7FF) return 2;    /* Hangul Jamo Extended-B */

    /* CJK Radicals, Kangxi, CJK Symbols */
    if(cp >= 0x2E80 && cp <= 0x303E) return 2;

    /* Hiragana, Katakana, Bopomofo, Enclosed CJK */
    if(cp >= 0x3040 && cp <= 0x33BF) return 2;

    /* CJK Unified Ideographs */
    if(cp >= 0x3400 && cp <= 0x4DBF) return 2;    /* Extension A */
    if(cp >= 0x4E00 && cp <= 0xA4CF) return 2;    /* Main block + Yi */
    if(cp >= 0xF900 && cp <= 0xFAFF) return 2;    /* Compatibility */
    if(cp >= 0x20000 && cp <= 0x3FFFF) return 2;   /* Extension B, C, D, E, F */

    /* CJK Compatibility Forms, Fullwidth Forms */
    if(cp >= 0xFE10 && cp <= 0xFE19) return 2;    /* Vertical Forms */
    if(cp >= 0xFE30 && cp <= 0xFE6F) return 2;    /* CJK Compatibility Forms */
    if(cp >= 0xFF01 && cp <= 0xFF60) return 2;    /* Fullwidth Forms */
    if(cp >= 0xFFE0 && cp <= 0xFFE6) return 2;    /* Fullwidth Signs */

    /* Kana Supplement */
    if(cp >= 0x1B000 && cp <= 0x1B0FF) return 2;

    /* ── Width 2: BMP emoji with default emoji presentation ──
     *
     * Older text-style symbols in U+2600..U+27BF (☀ ✂ ✓ …) stay width 1
     * unless a variation selector turns them into an emoji cluster.  The
     * cluster scanner below handles FE0F/FE0E and multi-codepoint emoji,
     * so only the default-presentation BMP ranges belong here. */
    if(cp == 0x231A || cp == 0x231B) return 2;         /* Watch, hourglass */
    if(cp == 0x2329 || cp == 0x232A) return 2;         /* Angle brackets */
    if(cp >= 0x23E9 && cp <= 0x23EC) return 2;         /* Media buttons */
    if(cp == 0x23F0 || cp == 0x23F3) return 2;         /* Alarm, hourglass */
    if(cp >= 0x25FD && cp <= 0x25FE) return 2;         /* Medium squares */
    if(cp >= 0x2614 && cp <= 0x2615) return 2;         /* Umbrella, coffee */
    if(cp >= 0x2630 && cp <= 0x2637) return 2;         /* Trigrams */
    if(cp >= 0x2648 && cp <= 0x2653) return 2;         /* Zodiac */
    if(cp == 0x267F) return 2;                         /* Wheelchair */
    if(cp >= 0x268A && cp <= 0x268F) return 2;         /* Misc symbols */
    if(cp == 0x2693) return 2;                         /* Anchor */
    if(cp == 0x26A1) return 2;                         /* High voltage */
    if(cp >= 0x26AA && cp <= 0x26AB) return 2;         /* Circles */
    if(cp >= 0x26BD && cp <= 0x26BE) return 2;         /* Sports */
    if(cp >= 0x26C4 && cp <= 0x26C5) return 2;         /* Snowman, cloud sun */
    if(cp == 0x26CE || cp == 0x26D4 || cp == 0x26EA) return 2;
    if(cp >= 0x26F2 && cp <= 0x26F3) return 2;         /* Fountain, flag hole */
    if(cp == 0x26F5 || cp == 0x26FA || cp == 0x26FD) return 2;
    if(cp == 0x2705) return 2;                         /* Check mark button */
    if(cp >= 0x270A && cp <= 0x270B) return 2;         /* Fist, raised hand */
    if(cp == 0x2728 || cp == 0x274C || cp == 0x274E) return 2;
    if(cp >= 0x2753 && cp <= 0x2755) return 2;         /* Question/exclaim */
    if(cp == 0x2757) return 2;                         /* Exclamation */
    if(cp >= 0x2795 && cp <= 0x2797) return 2;         /* Math signs */
    if(cp == 0x27B0 || cp == 0x27BF) return 2;         /* Loops */
    if(cp >= 0x2B1B && cp <= 0x2B1C) return 2;         /* Large squares */
    if(cp == 0x2B50 || cp == 0x2B55) return 2;         /* Star, circle */

    /* ── Width 2: SMP emoji (including enclosed alphanumerics and
     * regional indicators; multi-codepoint sequences are collapsed to
     * one 2-cell cluster by the scanner above) ── */
    if(cp >= 0x1F004 && cp <= 0x1F9FF) return 2;
    if(cp >= 0x1F170 && cp <= 0x1F1FF) return 2;
    if(cp >= 0x1FA00 && cp <= 0x1FAFF) return 2;

    return 1;
}

static int
is_regional_indicator(unsigned cp)
{
    return cp >= 0x1F1E6 && cp <= 0x1F1FF;
}

static int
is_emoji_modifier(unsigned cp)
{
    return cp >= 0x1F3FB && cp <= 0x1F3FF;
}

/* Codepoints that force an emoji-style 2-cell cluster. */
static int
is_emoji_cluster_cp(unsigned cp)
{
    return cp == 0xFE0F || cp == 0x200D || cp == 0x20E3
        || cp >= 0x1F000
        || (cp >= 0xE0020 && cp <= 0xE007F)
        || is_emoji_modifier(cp);
}

/* Consume one display cluster starting at byte `start` and report its
 * first codepoint, end byte, and terminal width.
 *
 * Common emoji graphemes are measured as one 2-cell unit: flag pairs,
 * ZWJ families, skin-tone modifiers, keycaps, variation-selector
 * presentation, and tag sequences.  Text-presentation BMP symbols
 * without FE0F remain single-width, which is what terminals actually
 * advance for old glyphs such as U+2600 and U+2702. */
static int
display_cluster(const char* text, int len, int start,
                int* p_end, unsigned* p_cp, int* p_width)
{
    int i = start;
    int n;
    unsigned cp;
    unsigned prev;
    int emoji = 0;
    int width = 0;

    cp = utf8_decode(text, len, &i, &n);
    prev = cp;
    if(is_emoji_cluster_cp(cp))
        emoji = 1;
    width += char_width(cp);

    while(i < len) {
        int q = i;
        int qn;
        unsigned nxt = utf8_decode(text, len, &q, &qn);

        if(nxt == 0x200D && q < len) {
            int r = q;
            int rn;
            unsigned after = utf8_decode(text, len, &r, &rn);
            if(is_emoji_cluster_cp(after))
                emoji = 1;
            width += char_width(nxt) + char_width(after);
            prev = after;
            i = r;
            continue;
        }
        if(char_width(nxt) == 0
           || is_emoji_modifier(nxt)
           || nxt == 0x20E3
           || (nxt >= 0xE0020 && nxt <= 0xE007F)
           || (is_regional_indicator(prev) && is_regional_indicator(nxt))) {
            if(is_emoji_cluster_cp(nxt))
                emoji = 1;
            width += char_width(nxt);
            prev = nxt;
            i = q;
            continue;
        }
        break;
    }

    *p_end = i;
    *p_cp = cp;
    *p_width = emoji ? 2 : width;
    return 0;
}

/* Measure one rendered cell in a single pass: total visible width and the
 * width of the longest run that the word-aware wrapper cannot break at a
 * preferred point (whitespace-delimited word, URL segment, or sequence of
 * wide characters).  ANSI SGR sequences and zero-width characters add no
 * visible width; newlines and carriage returns count one column, matching
 * the previous visible_len() so natural widths do not change. */
static void
cell_measure(const char* str, int len, int* p_visible, int* p_min_word)
{
    int visible = 0;
    int run = 0;
    int max_run = 0;
    int prev_was_wide = 0;
    int i = 0;

    while(i < len) {
        int end;
        unsigned cp;
        int width;
        int is_space;
        int no_break_before;

        if(str[i] == '\033' && i + 1 < len && str[i + 1] == '[') {
            i += 2;
            while(i < len && str[i] != 'm')
                i++;
            if(i < len)
                i++;
            continue;
        }

        if(str[i] == '\033' && i + 1 < len && str[i + 1] == ']') {
            i = wrap_osc_end(str, len, i);
            continue;
        }

        if(str[i] == '\n' || str[i] == '\r') {
            visible++;
            if(run > max_run)
                max_run = run;
            run = 0;
            prev_was_wide = 0;
            i++;
            continue;
        }

        display_cluster(str, len, i, &end, &cp, &width);
        i = end;
        is_space = wrap_is_space(cp);
        no_break_before = (width == 0);

        visible += width;

        if(is_space) {
            /* Whitespace is dropped at line edges, so it never forces a
             * run to be wider than the words around it. */
            if(run > max_run)
                max_run = run;
            run = 0;
        } else if(prev_was_wide && width == 2 && !no_break_before) {
            /* The wrapper can break between adjacent wide characters. */
            if(run > max_run)
                max_run = run;
            run = width;
        } else {
            run += width;
        }

        if(wrap_is_punctuation(cp)) {
            /* The wrapper can break after punctuation, which stays on the
             * same line as the preceding run. */
            if(run > max_run)
                max_run = run;
            run = 0;
        }

        prev_was_wide = (width == 2);
    }

    if(run > max_run)
        max_run = run;
    *p_visible = visible;
    *p_min_word = max_run;
}

/* Re-apply the innermost block style after a reset. */
static void
reapply_block_style(MD_ANSI_RENDERER* r)
{
    if(r->block_style_depth > 0) {
        const char* prefix = r->block_style_prefix[r->block_style_depth - 1];
        if(prefix != NULL  &&  *prefix != '\0')
            write_str(r, prefix);
    }
}

/* Build and write combined emphasis SGR codes. */
static void
write_emphasis_sgr(MD_ANSI_RENDERER* r)
{
    char buf[64];
    int pos = 0;
    int need_semi = 0;

    if(r->n_bold > 0) {
        buf[pos++] = '\033'; buf[pos++] = '[';
        buf[pos++] = '1';
        need_semi = 1;
    }
    if(r->n_italic > 0) {
        if(!need_semi) { buf[pos++] = '\033'; buf[pos++] = '['; need_semi = 1; }
        else buf[pos++] = ';';
        buf[pos++] = '3';
    }
    if(r->n_strikethrough > 0) {
        if(!need_semi) { buf[pos++] = '\033'; buf[pos++] = '['; need_semi = 1; }
        else buf[pos++] = ';';
        buf[pos++] = '9';
    }
    if(r->n_underline > 0) {
        if(!need_semi) { buf[pos++] = '\033'; buf[pos++] = '['; need_semi = 1; }
        else buf[pos++] = ';';
        buf[pos++] = '4';
    }
    if(need_semi) {
        buf[pos++] = 'm';
        write_output(r, buf, pos);
    }
}

/* Re-apply all active styles: block style + isolated spans + spoiler +
 * emphasis. Later sequences override earlier ones for shared attributes,
 * and spoiler is applied after isolated spans so nested content stays
 * hidden. */
static void
reapply_all(MD_ANSI_RENDERER* r)
{
    int i;

    reapply_block_style(r);
    for(i = 0; i < r->isolated_style_depth; i++)
        write_str(r, r->isolated_style_stack[i]->prefix);
    if(r->spoiler_active)
        write_str(r, r->theme->spoiler.prefix);
    write_emphasis_sgr(r);
}

static void
emit_blank_line(MD_ANSI_RENDERER* r)
{
    if(in_quote_context(r))
        emit_bq_prefix(r);
    write_str(r, "\n");
}

static void
signal_bl(MD_ANSI_RENDERER* r)
{
    r->need_blank_line = 1;
}

/* Cap emphasis nesting at 3 (prevents unbounded SGR sequences). */
static void
emph_inc(int* counter)
{
    if(*counter < 3)
        (*counter)++;
}

static void
emph_dec(int* counter)
{
    if(*counter > 0)
        (*counter)--;
}

static void
style_reset_reapply(MD_ANSI_RENDERER* r)
{
    write_sgr_reset(r);
    reapply_all(r);
}

static void
open_isolated_style_marker(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style,
                           const char* marker, int marker_len)
{
    write_sgr_reset(r);
    reapply_all(r);
    if(r->isolated_style_depth < MAX_ISOLATED_DEPTH)
        r->isolated_style_stack[r->isolated_style_depth++] = style;
    if(marker_len > 0)
        write_output(r, marker, marker_len);
    write_str(r, style->prefix);
    if(r->spoiler_active)
        write_str(r, r->theme->spoiler.prefix);
}

static void
open_isolated_style(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style)
{
    open_isolated_style_marker(r, style, NULL, 0);
}

static void
close_isolated_style(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style)
{
    write_str(r, style->suffix);
    if(r->isolated_style_depth > 0)
        r->isolated_style_depth--;
    reapply_all(r);
}

/* Emit " (url)" with link_url theme styling. */
static void
emit_link_url_paren(MD_ANSI_RENDERER* r, const char* href, int href_len)
{
    if(href_len <= 0)
        return;
    write_str(r, " (");
    write_str(r, r->theme->link_url.prefix);
    write_output(r, href, href_len);
    write_str(r, r->theme->link_url.suffix);
    write_str(r, ")");
}

static int
in_quote_context(const MD_ANSI_RENDERER* r)
{
    return (r->blockquote_depth > 0 || r->admonition_active);
}

static void
mark_bq_prefix_after_nl(MD_ANSI_RENDERER* r, const MD_CHAR* text, MD_SIZE size)
{
    if(size > 0 && text[size - 1] == '\n' && in_quote_context(r))
        r->blockquote_needs_prefix = 1;
}

static void
maybe_emit_bq_prefix(MD_ANSI_RENDERER* r, MD_SIZE size)
{
    if(r->blockquote_needs_prefix && size > 0)
        emit_bq_prefix(r);
}

/* Save parent list state when entering a nested list; emit leading newline.
 * The parent's state is saved per nesting depth (one slot per depth), so
 * nested lists deeper than two levels cannot overwrite it. */
static void
list_enter_nested(MD_ANSI_RENDERER* r)
{
    if(r->list_depth > 0) {
        r->saved_list_item_number[r->list_depth - 1] = r->list_item_number;
        r->saved_in_list_ordered[r->list_depth - 1] = r->in_list_ordered;
        write_output(r, "\n", 1);
    }
}

static void
list_leave_nested(MD_ANSI_RENDERER* r)
{
    if(r->list_depth > 0)
        r->list_depth--;
    if(r->list_depth > 0) {
        r->list_item_number = r->saved_list_item_number[r->list_depth - 1];
        r->in_list_ordered = r->saved_in_list_ordered[r->list_depth - 1];
    }
    if(r->list_depth == 0)
        signal_bl(r);
}

static void
push_block_style(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style)
{
    if(!block_has_style(r, style))
        return;
    if(r->block_style_depth < MAX_BLOCK_DEPTH)
        r->block_style_prefix[r->block_style_depth++] = style->prefix;
    write_str(r, style->prefix);
}

static void
pop_block_style(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style)
{
    if(!block_has_style(r, style))
        return;
    if(r->block_style_depth > 0)
        r->block_style_depth--;
    write_sgr_reset(r);
    reapply_all(r);
}

static int
table_ensure_columns(MD_ANSI_RENDERER* r, int n_cols)
{
    int new_cap;
    struct TableColumn* new_columns;

    if(n_cols <= r->table_col_capacity)
        return 0;

    if(r->table_col_capacity > INT_MAX / 2)
        new_cap = n_cols;
    else
        new_cap = (r->table_col_capacity > 0) ? r->table_col_capacity * 2 : 8;
    while(new_cap < n_cols)
        if(new_cap > INT_MAX / 2) {
            new_cap = n_cols;
            break;
        } else {
            new_cap *= 2;
        }
    new_columns = (struct TableColumn*) realloc(r->table_columns,
                         (size_t)new_cap * sizeof(*new_columns));
    if(new_columns == NULL)
        return -1;
    memset(new_columns + r->table_col_capacity, 0,
           (size_t)(new_cap - r->table_col_capacity) * sizeof(*new_columns));
    r->table_columns = new_columns;
    r->table_col_capacity = new_cap;
    return 0;
}

static int
table_ensure_rows(MD_ANSI_RENDERER* r, int n_rows)
{
    int new_cap;
    struct TableRow* new_rows;

    if(n_rows <= r->table_row_capacity)
        return 0;

    if(r->table_row_capacity > INT_MAX / 2)
        new_cap = n_rows;
    else
        new_cap = (r->table_row_capacity > 0) ? r->table_row_capacity * 2 : 16;
    while(new_cap < n_rows)
        if(new_cap > INT_MAX / 2) {
            new_cap = n_rows;
            break;
        } else {
            new_cap *= 2;
        }
    new_rows = (struct TableRow*) realloc(r->table_rows,
                                          (size_t)new_cap * sizeof(*new_rows));
    if(new_rows == NULL)
        return -1;
    r->table_rows = new_rows;
    r->table_row_capacity = new_cap;
    return 0;
}

static int
table_append_cell(MD_ANSI_RENDERER* r, const char* text, int len,
                  struct TableCell* out)
{
    int new_cap;
    char* new_text;

    if(r->table_cell_count >= r->table_cell_capacity) {
        int new_cell_cap;
        struct TableCell* new_cells;

        if(r->table_cell_count == INT_MAX)
            return -1;
        if(r->table_cell_capacity > INT_MAX / 2)
            new_cell_cap = r->table_cell_count + 1;
        else
            new_cell_cap = (r->table_cell_capacity > 0)
                         ? r->table_cell_capacity * 2 : 32;
        while(new_cell_cap <= r->table_cell_count) {
            if(new_cell_cap > INT_MAX / 2) {
                new_cell_cap = r->table_cell_count + 1;
                break;
            }
            new_cell_cap *= 2;
        }
        new_cells = (struct TableCell*) realloc(r->table_cells,
                         (size_t)new_cell_cap * sizeof(*new_cells));
        if(new_cells == NULL)
            return -1;
        r->table_cells = new_cells;
        r->table_cell_capacity = new_cell_cap;
    }

    if(len > 0) {
        int needed;

        if(len > INT_MAX - r->table_text_len)
            return -1;
        if(r->table_text_len + len > r->table_text_capacity) {
            needed = r->table_text_len + len;

            if(r->table_text_capacity > INT_MAX / 2)
                new_cap = needed;
            else
                new_cap = (r->table_text_capacity > 0)
                        ? r->table_text_capacity * 2 : 4096;
            while(new_cap < needed) {
                if(new_cap > INT_MAX / 2) {
                    new_cap = needed;
                    break;
                }
                new_cap *= 2;
            }
            new_text = (char*) realloc(r->table_text, (size_t)new_cap);
            if(new_text == NULL)
                return -1;
            r->table_text = new_text;
            r->table_text_capacity = new_cap;
        }
        memcpy(r->table_text + r->table_text_len, text, (size_t)len);
    }

    out->offset = r->table_text_len;
    out->len = len;
    r->table_cells[r->table_cell_count] = *out;
    r->table_text_len += len;
    r->table_cell_count++;
    return 0;
}

static const char*
table_cell_text(const MD_ANSI_RENDERER* r, const struct TableCell* cell)
{
    if(cell == NULL || cell->len <= 0 || r->table_text == NULL)
        return "";
    return r->table_text + cell->offset;
}

static void
table_free_all(MD_ANSI_RENDERER* r)
{
    free(r->table_columns);
    r->table_columns = NULL;
    r->table_col_capacity = 0;

    free(r->table_text);
    r->table_text = NULL;
    r->table_text_len = 0;
    r->table_text_capacity = 0;

    free(r->table_cells);
    r->table_cells = NULL;
    r->table_cell_count = 0;
    r->table_cell_capacity = 0;

    free(r->table_rows);
    r->table_rows = NULL;
    r->table_row_capacity = 0;
    r->table_n_rows = 0;

    r->table_header_start = 0;
    r->table_header_cols = 0;
    r->table_header_rendered = 0;
    r->table_row_active = 0;
    r->table_col_count = 0;
    r->table_lines = 0;
    r->table_repaint_count = 0;

    free(r->table_cell_buf);
    r->table_cell_buf = NULL;
    r->table_cell_len = 0;
    r->table_cell_cap = 0;
}


/********************************
 ***  Default theme           ***
 ********************************/

#define STYLE_SIMPLE(pre) \
    { (pre), (int)(sizeof(pre) - 1), "\033[0m", 4 }

#define STYLE_EMPTY \
    { "", 0, "", 0 }

static const MD_ANSI_THEME MD_ANSI_DEFAULT_THEME = {
    STYLE_SIMPLE("\033[1;48;5;63;38;5;228m"),   /* h1 */
    STYLE_SIMPLE("\033[1;34m"),                  /* h2 */
    STYLE_SIMPLE("\033[1;34m"),                  /* h3 */
    STYLE_SIMPLE("\033[1;34m"),                  /* h4 */
    STYLE_SIMPLE("\033[1;34m"),                  /* h5 */
    STYLE_SIMPLE("\033[1;34m"),                  /* h6 */

    STYLE_SIMPLE("\033[1m"),                     /* bold */
    STYLE_SIMPLE("\033[3m"),                     /* italic */
    STYLE_SIMPLE("\033[9m"),                     /* strikethrough */

    STYLE_SIMPLE("\033[38;5;215;48;5;236m"),    /* inline_code */
    STYLE_EMPTY,                                 /* code_block (no base style) */
    STYLE_SIMPLE("\033[3;2;33m"),                /* code_block_lang (italic dim yellow) */

    STYLE_SIMPLE("\033[38;5;252m"),              /* hl_normal (light gray) */
    STYLE_SIMPLE("\033[38;5;176m"),              /* hl_keyword */
    STYLE_EMPTY,                                 /* hl_punct (plain) */
    STYLE_SIMPLE("\033[38;5;114m"),              /* hl_string */
    STYLE_SIMPLE("\033[3;38;5;245m"),            /* hl_comment (italic) */

    STYLE_SIMPLE("\033[2m"),                     /* hr */

    STYLE_SIMPLE("\033[2m"),                     /* bullet_item */
    STYLE_SIMPLE("\033[2m"),                     /* ordered_item */
    STYLE_SIMPLE("\033[2m"),                     /* blockquote */

    STYLE_SIMPLE("\033[2m"),                     /* table_border */
    STYLE_SIMPLE("\033[1m"),                     /* table_header */
    STYLE_EMPTY,                                 /* table_cell */

    STYLE_SIMPLE("\033[1;4;38;5;75m"),           /* link_text (bold underline bright blue) */
    STYLE_SIMPLE("\033[2;34m"),                  /* link_url */

    STYLE_SIMPLE("\033[1;38;5;141m"),            /* image_label (bold bright purple) */

    STYLE_EMPTY,                                 /* text */

    STYLE_SIMPLE("\033[2;34m"),                  /* footnote_ref */

    STYLE_SIMPLE("\033[1;33m"),                  /* admonition (fallback) */
    STYLE_SIMPLE("\033[1;36m"),                  /* admonition_note (cyan) */
    STYLE_SIMPLE("\033[1;32m"),                  /* admonition_tip (green) */
    STYLE_SIMPLE("\033[1;35m"),                  /* admonition_important (purple) */
    STYLE_SIMPLE("\033[1;33m"),                  /* admonition_warning (yellow) */
    STYLE_SIMPLE("\033[1;31m"),                  /* admonition_caution (red) */

    STYLE_SIMPLE("\033[4m"),                     /* underline */
    STYLE_SIMPLE("\033[7m"),                     /* highlight */
    STYLE_SIMPLE("\033[2;33m"),                  /* spoiler_label (dim yellow) */
    STYLE_SIMPLE("\033[38;5;232;48;5;232m"),     /* spoiler (invisible: black on black) */
    STYLE_SIMPLE("\033[31m"),                    /* superscript */
    STYLE_SIMPLE("\033[31m"),                    /* subscript */
};

const MD_ANSI_THEME*
md_ansi_default_theme(void)
{
    return &MD_ANSI_DEFAULT_THEME;
}


/********************************
 ***  Create / destroy       ***
 ********************************/

MD_ANSI_RENDERER*
md_ansi_renderer_create(const MD_ANSI_THEME* theme,
                        void (*output)(const char* str, int size, void* userdata),
                        void* userdata)
{
    MD_ANSI_RENDERER* r;

    r = (MD_ANSI_RENDERER*) calloc(1, sizeof(MD_ANSI_RENDERER));
    if(r == NULL)
        return NULL;

    r->theme = (theme != NULL) ? theme : &MD_ANSI_DEFAULT_THEME;
    r->output = output;
    r->userdata = userdata;
    r->osc8_enabled = 1;
    return r;
}

void
md_ansi_renderer_destroy(MD_ANSI_RENDERER* renderer)
{
    if(renderer == NULL)
        return;
    if(renderer->out_spool_len > 0  &&  renderer->output != NULL) {
        /* Deliver a trailing partial line (no newline at EOF). */
        renderer->output(renderer->out_spool, renderer->out_spool_len,
                         renderer->userdata);
        renderer->out_spool_len = 0;
    }
    free(renderer->out_spool);
    hl_release(&renderer->hl_state);
    free(renderer->table_cell_buf);
    table_free_all(renderer);
    free(renderer);
}

void
md_ansi_set_term_width(MD_ANSI_RENDERER* renderer, int term_width)
{
    renderer->table_term_width = term_width;
}

void
md_ansi_renderer_set_osc8(MD_ANSI_RENDERER* renderer, int enable)
{
    if(renderer != NULL)
        renderer->osc8_enabled = (enable != 0);
}


/********************************
 ***  Style lookup helpers    ***
 ********************************/

static const MD_ANSI_STYLE*
get_block_style(MD_ANSI_RENDERER* r, MD_BLOCKTYPE type, void* detail)
{
    switch(type) {
        case MD_BLOCK_H: {
            unsigned level = (detail != NULL) ? ((MD_BLOCK_H_DETAIL*)detail)->level : 1;
            switch(level) {
                case 1: return &r->theme->h1;
                case 2: return &r->theme->h2;
                case 3: return &r->theme->h3;
                case 4: return &r->theme->h4;
                case 5: return &r->theme->h5;
                default: return &r->theme->h6;
            }
        }
        case MD_BLOCK_CODE:     return &r->theme->code_block;
        /* Blockquote style is handled via the text callback prefix,
         * not as a block-level style. */
        case MD_BLOCK_HR:       return &r->theme->hr;
        /* Table border style is written by draw_table_border() directly,
         * not via the block style stack. */
        case MD_BLOCK_TH:               return &r->theme->table_header;
        case MD_BLOCK_TD:               return &r->theme->table_cell;
        case MD_BLOCK_REFERENCE_SECTION: return &r->theme->link_url;
        default:                        return NULL;
    }
}

static int
block_has_style(MD_ANSI_RENDERER* r, const MD_ANSI_STYLE* style)
{
    (void)r;
    return (style != NULL  &&  style->prefix_len > 0);
}


/********************************
 ***  Block open/close before ***
 ***  content                  ***
 ********************************/

/* Emit a blank-line separator if needed, then clear the flag. */
static void
check_bl(MD_ANSI_RENDERER* r)
{
    if(r->need_blank_line) {
        emit_blank_line(r);
        r->need_blank_line = 0;
    }
}


/********************************
 ***  Table rendering         ***
 ********************************/

/* UTF-8 box-drawing characters (most are 3 bytes each). */
#define BDR_H  "\xe2\x94\x80"       /* ─ */
#define BDR_V  "\xe2\x94\x82"       /* │ */
#define BDR_TL "\xe2\x94\x8c"       /* ┌ */
#define BDR_TM "\xe2\x94\xac"       /* ┬ */
#define BDR_TR "\xe2\x94\x90"       /* ┐ */
#define BDR_ML "\xe2\x94\x9c"       /* ├ */
#define BDR_MM "\xe2\x94\xbc"       /* ┼ */
#define BDR_MR "\xe2\x94\xa4"       /* ┤ */
#define BDR_BL "\xe2\x94\x94"       /* └ */
#define BDR_BM "\xe2\x94\xb4"       /* ┴ */
#define BDR_BR "\xe2\x94\x98"       /* ┘ */

/* Forward declarations (used in leave_block callback). */
static void draw_table_border(MD_ANSI_RENDERER* r, const int* widths, int n_cols,
                              const char* left, const char* join, const char* right);
static int draw_table_row(MD_ANSI_RENDERER* r, const int* widths, int n_cols,
                           const struct TableCell* cells, int n_cells);
static void limit_table_widths(MD_ANSI_RENDERER* r, int* widths, int n_cols,
                               int term_width);
static void redraw_table(MD_ANSI_RENDERER* r);
static int wrap_content(const char* text, int len, int display_width,
                        struct WrappedSlice** p_out);
static void free_slices(struct WrappedSlice* slices, int n);
static void table_store_cell(MD_ANSI_RENDERER* r);
static void table_leave_tr(MD_ANSI_RENDERER* r);
static void table_leave_thead(MD_ANSI_RENDERER* r);
static void table_leave_table(MD_ANSI_RENDERER* r);


/********************************
 ***  Block callbacks         ***
 ********************************/

static void
enter_ul(MD_ANSI_RENDERER* r, void* detail)
{
    list_enter_nested(r);
    r->in_list_ordered = 0;
    r->list_depth++;
    (void)detail;
}

static void
enter_ol(MD_ANSI_RENDERER* r, void* detail)
{
    list_enter_nested(r);
    r->in_list_ordered = 1;
    r->list_depth++;
    if(detail != NULL)
        r->list_item_number = ((MD_BLOCK_OL_DETAIL*)detail)->start;
    else
        r->list_item_number = 1;
}

/* Emit the task-list marker: "[ ]" for unchecked, "[✓]" for checked
 * (GFM source uses "[x]"/"[X]"; the display uses a Unicode check mark). */
static void
write_task_marker(MD_ANSI_RENDERER* r, char task_mark)
{
    if(task_mark == ' ') {
        write_output(r, "[ ] ", 4);
    } else {
        write_output(r, "[\xE2\x9C\x93] ", 6);
    }
}

static void
enter_li(MD_ANSI_RENDERER* r, void* detail)
{
    int is_task = 0;
    char task_mark = ' ';
    int i;

    if(r->blockquote_depth > 0 || r->admonition_active)
        emit_bq_prefix(r);

    if(detail != NULL) {
        MD_BLOCK_LI_DETAIL* li = (MD_BLOCK_LI_DETAIL*)detail;
        is_task = li->is_task;
        task_mark = li->task_mark;
    }

    r->in_list_item = 1;

    for(i = 1; i < r->list_depth; i++)
        write_output(r, "  ", 2);

    if(r->in_list_ordered) {
        write_str(r, r->theme->ordered_item.prefix);
        if(is_task) {
            write_task_marker(r, task_mark);
        } else {
            char num[16];
            int n = snprintf(num, sizeof(num), "%d. ", r->list_item_number);
            if(r->list_item_number > 0)
                r->list_item_number++;
            if(n > 0)
                write_output(r, num, n);
        }
        write_str(r, r->theme->ordered_item.suffix);
    } else {
        write_str(r, r->theme->bullet_item.prefix);
        if(is_task) {
            write_task_marker(r, task_mark);
        } else {
            unsigned char bullet[] = { 0xE2, 0x80, 0xA2, ' ', '\0' };
            write_output(r, (const char*)bullet, 4);
        }
        write_str(r, r->theme->bullet_item.suffix);
    }

    /* Track that only the marker has been written so far: a paragraph
     * opened right after the marker must not get a spurious separator. */
    r->list_item_just_opened = 1;
}

static void
enter_code(MD_ANSI_RENDERER* r, void* detail)
{
    r->code_had_content = 0;
    r->hl_active = 0;
    hl_reset(&r->hl_state);
    if(in_quote_context(r)) {
        emit_bq_prefix(r);
        r->blockquote_needs_prefix = 1;
    }
    if(detail != NULL) {
        MD_BLOCK_CODE_DETAIL* cd = (MD_BLOCK_CODE_DETAIL*)detail;
        if(cd->lang.text != NULL && cd->lang.size > 0) {
            /* Highlight only code blocks whose fence label names a major
             * language; unknown and unlabeled blocks render plain. */
            r->hl_active = hl_lang_supported((const char*) cd->lang.text,
                                             (int) cd->lang.size);
            write_str(r, r->theme->code_block_lang.prefix);
            render_attribute_to_output(r, &cd->lang);
            write_str(r, r->theme->code_block_lang.suffix);
            write_str(r, "\n");
        }
    }
}

static void
enter_admonition(MD_ANSI_RENDERER* r, void* detail)
{
    r->admonition_active = 1;
    r->admonition_style = NULL;
    r->blockquote_needs_prefix = 1;
    if(detail == NULL)
        return;
    {
        MD_BLOCK_ADMONITION_DETAIL* ad = (MD_BLOCK_ADMONITION_DETAIL*)detail;
        if(ad->type.text == NULL  ||  ad->type.size == 0)
            return;

        if(ad->type.size == 4 && memcmp(ad->type.text, "note", 4) == 0)
            r->admonition_style = &r->theme->admonition_note;
        else if(ad->type.size == 3 && memcmp(ad->type.text, "tip", 3) == 0)
            r->admonition_style = &r->theme->admonition_tip;
        else if(ad->type.size == 9 && memcmp(ad->type.text, "important", 9) == 0)
            r->admonition_style = &r->theme->admonition_important;
        else if(ad->type.size == 7 && memcmp(ad->type.text, "warning", 7) == 0)
            r->admonition_style = &r->theme->admonition_warning;
        else if(ad->type.size == 7 && memcmp(ad->type.text, "caution", 7) == 0)
            r->admonition_style = &r->theme->admonition_caution;
        else
            r->admonition_style = &r->theme->admonition;

        if(r->admonition_style->prefix_len > 0)
            write_str(r, r->admonition_style->prefix);
        write_output(r, "[", 1);
        render_attribute_to_output(r, &ad->type);
        write_output(r, "]\n", 2);
        write_sgr_reset(r);
    }
}

static void
enter_heading(MD_ANSI_RENDERER* r, void* detail)
{
    int level;
    int i;
    if(detail == NULL)
        return;
    level = ((MD_BLOCK_H_DETAIL*)detail)->level;
    if(level > 1) {
        for(i = 0; i < level; i++)
            write_output(r, "#", 1);
        write_output(r, " ", 1);
    }
}

static void
enter_hr(MD_ANSI_RENDERER* r)
{
    int i;
    for(i = 0; i < 16; i++)
        write_output(r, "\xe2\x94\x80", 3);
}

static void
enter_table(MD_ANSI_RENDERER* r)
{
    r->in_table = 1;
    r->table_col_count = 0;
    r->table_col = 0;
    r->table_in_header = 0;
    r->table_in_body = 0;
    r->table_row_active = 0;
    r->table_n_rows = 0;
    r->table_header_start = 0;
    r->table_header_cols = 0;
}

static void
enter_tr(MD_ANSI_RENDERER* r)
{
    r->table_col = 0;
    r->table_row_active = 0;
    if(r->table_in_body) {
        if(r->table_n_rows == INT_MAX
                || table_ensure_rows(r, r->table_n_rows + 1) != 0) {
            r->failed = 1;
            return;
        }
        r->table_rows[r->table_n_rows].first_cell = r->table_cell_count;
        r->table_rows[r->table_n_rows].n_cells = 0;
        r->table_n_rows++;
        r->table_row_active = 1;
    }
}

int
md_ansi_enter_block(MD_BLOCKTYPE type, void* detail, void* userdata)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;
    const MD_ANSI_STYLE* style = get_block_style(r, type, detail);

    /* Streaming emits a loose list item's first paragraph without a P
     * wrapper (the tight/loose first-item tradeoff), so the renderer must
     * reproduce the line break and blank line that batch's -P + check_bl
     * would have produced before a later paragraph in the same item. */
    if(type == MD_BLOCK_P  &&  r->in_list_item
       &&  r->last_char != '\n'  &&  !r->list_item_just_opened) {
        write_output(r, "\n", 1);
        r->need_blank_line = 1;
    }

    check_bl(r);

    switch(type) {
        case MD_BLOCK_UL:   enter_ul(r, detail); break;
        case MD_BLOCK_OL:   enter_ol(r, detail); break;
        case MD_BLOCK_LI:   enter_li(r, detail); break;
        case MD_BLOCK_QUOTE:
            r->blockquote_depth++;
            r->blockquote_needs_prefix = 1;
            break;
        case MD_BLOCK_P:
            if(in_quote_context(r))
                r->blockquote_needs_prefix = 1;
            break;
        case MD_BLOCK_CODE: enter_code(r, detail); break;
        case MD_BLOCK_HTML:
            r->in_html_block = 1;
            memset(&r->html_state, 0, sizeof(r->html_state));
            if(in_quote_context(r))
                r->blockquote_needs_prefix = 1;
            break;
        case MD_BLOCK_FOOTNOTE_DEF:
            if(detail != NULL) {
                MD_BLOCK_FOOTNOTE_DEF_DETAIL* det = (MD_BLOCK_FOOTNOTE_DEF_DETAIL*)detail;
                write_str(r, r->theme->footnote_ref.prefix);
                write_output(r, "[", 1);
                write_output(r, det->label.text, det->label.size);
                write_output(r, "] ", 2);
                write_str(r, r->theme->footnote_ref.suffix);
            }
            break;
        case MD_BLOCK_TH:
        case MD_BLOCK_TD:
            /* Redirect style write into cell buffer (may be discarded below). */
            r->in_table_cell = 1;
            break;
        case MD_BLOCK_REFERENCE_SECTION:
            r->in_reference_section = 1;
            r->ref_line_len = 0;
            write_str(r, "\n");
            write_str(r, r->theme->h2.prefix);
            write_str(r, "References");
            write_str(r, r->theme->h2.suffix);
            write_str(r, "\n");
            break;
        case MD_BLOCK_FOOTNOTE_DEF_SECTION:
            write_str(r, "\n");
            write_str(r, r->theme->h2.prefix);
            write_str(r, "Footnotes");
            write_str(r, r->theme->h2.suffix);
            write_str(r, "\n");
            break;
        default:
            break;
    }

    push_block_style(r, style);

    switch(type) {
        case MD_BLOCK_ADMONITION: enter_admonition(r, detail); break;
        case MD_BLOCK_H:          enter_heading(r, detail); break;
        case MD_BLOCK_HR:         enter_hr(r); break;
        case MD_BLOCK_TABLE:      enter_table(r); break;
        case MD_BLOCK_THEAD:      r->table_in_header = 1; break;
        case MD_BLOCK_TBODY:      r->table_in_body = 1; break;
        case MD_BLOCK_TR:         enter_tr(r); break;
        case MD_BLOCK_TH:
        case MD_BLOCK_TD:
            /* Reset cell buffer after style push — matches prior behavior
             * where TH bold prefix was written then discarded by len=0. */
            r->in_table_cell = 1;
            r->table_cell_len = 0;
            if(r->table_col == INT_MAX
                    || table_ensure_columns(r, r->table_col + 1) != 0) {
                r->failed = 1;
            } else if(detail != NULL) {
                r->table_columns[r->table_col].align =
                    ((MD_BLOCK_TD_DETAIL*)detail)->align;
            }
            break;
        default:
            break;
    }

    return r->failed ? -1 : 0;
}

static void
table_store_cell(MD_ANSI_RENDERER* r)
{
    int col = r->table_col;
    int len = r->table_cell_len;
    int visible;
    int min_word;
    struct TableCell cell;

    r->in_table_cell = 0;

    /* Empty cells normally inherit the already allocated cell buffer from
     * an earlier cell.  Keep the callback safe even when the first cell is
     * empty and no buffer has been allocated yet. */
    if(r->table_cell_buf != NULL)
        r->table_cell_buf[len] = '\0';
    else
        len = 0;

    if(col == INT_MAX || table_ensure_columns(r, col + 1) != 0) {
        r->failed = 1;
    } else {
        if(r->table_cell_buf != NULL)
            cell_measure(r->table_cell_buf, len, &visible, &min_word);
        else {
            visible = 0;
            min_word = 0;
        }
        if(table_append_cell(r,
                (r->table_cell_buf != NULL) ? r->table_cell_buf : "",
                len, &cell) != 0) {
            r->failed = 1;
        } else {
            if(r->table_in_header) {
                if(r->table_header_cols == 0)
                    r->table_header_start = r->table_cell_count - 1;
                r->table_header_cols++;
            } else if(r->table_row_active) {
                r->table_rows[r->table_n_rows - 1].n_cells++;
            }
            if(visible > INT_MAX - 2
                    || visible + 2 > r->table_columns[col].natural_width)
                r->table_columns[col].natural_width =
                    (visible > INT_MAX - 2) ? INT_MAX : visible + 2;
            if(min_word > INT_MAX - 2)
                min_word = INT_MAX - 2;
            {
                int min_width = min_word + 2;
                if(min_width < 3)
                    min_width = 3;
                if(min_width > r->table_columns[col].min_width)
                    r->table_columns[col].min_width = min_width;
            }
        }
    }

    if(r->table_col < INT_MAX)
        r->table_col++;
    if(col < INT_MAX && col + 1 > r->table_col_count
            && col + 1 <= r->table_col_capacity)
        r->table_col_count = col + 1;
}

static void
table_leave_tr(MD_ANSI_RENDERER* r)
{
    if(!(r->table_in_body  &&  r->table_row_active
            &&  r->table_n_rows > 0))
        return;
    {
        int last = r->table_n_rows - 1;
        r->table_row_active = 0;
        if(r->table_header_rendered) {
            int n_cols_r = r->table_col_count;
            int* new_widths;
            int i, widths_changed = 0;

            new_widths = (int*) malloc((size_t)n_cols_r * sizeof(int));
            if(new_widths == NULL) {
                r->failed = 1;
                return;
            }

            for(i = 0; i < n_cols_r; i++) {
                new_widths[i] = r->table_columns[i].natural_width;
                if(new_widths[i] < 3) new_widths[i] = 3;
            }
            if(r->table_term_width > 0)
                limit_table_widths(r, new_widths, n_cols_r,
                                   r->table_term_width);
            if(r->failed) {
                free(new_widths);
                return;
            }

            for(i = 0; i < n_cols_r; i++) {
                if(new_widths[i] != r->table_columns[i].rendered_width) {
                    widths_changed = 1;
                    break;
                }
            }

            if(widths_changed) {
                r->table_repaint_count++;
                if(r->table_repaint_count > MAX_REPAINT_COUNT) {
                    int* rendered_widths = (int*) malloc(
                            (size_t)n_cols_r * sizeof(int));
                    if(rendered_widths == NULL) {
                        free(new_widths);
                        r->failed = 1;
                        return;
                    }
                    for(i = 0; i < n_cols_r; i++)
                        rendered_widths[i] =
                            r->table_columns[i].rendered_width;
                    r->table_lines += draw_table_row(r, rendered_widths,
                            n_cols_r, r->table_cells
                            + r->table_rows[last].first_cell,
                            r->table_rows[last].n_cells);
                    free(rendered_widths);
                } else {
                    for(i = 0; i < n_cols_r; i++)
                        r->table_columns[i].rendered_width = new_widths[i];
                    redraw_table(r);
                }
            } else {
                int* rendered_widths = (int*) malloc(
                        (size_t)n_cols_r * sizeof(int));
                if(rendered_widths == NULL) {
                    free(new_widths);
                    r->failed = 1;
                    return;
                }
                for(i = 0; i < n_cols_r; i++)
                    rendered_widths[i] = r->table_columns[i].rendered_width;
                r->table_lines += draw_table_row(r, rendered_widths,
                        n_cols_r, r->table_cells
                        + r->table_rows[last].first_cell,
                        r->table_rows[last].n_cells);
                free(rendered_widths);
            }
            free(new_widths);
        }
    }
}

static void
table_leave_thead(MD_ANSI_RENDERER* r)
{
    int n_cols = r->table_col_count;
    int* widths;
    int i;

    if(!(r->in_table  &&  !r->table_header_rendered))
        return;

    if(n_cols > 0) {
        widths = (int*) malloc((size_t)n_cols * sizeof(int));
        if(widths == NULL) {
            r->failed = 1;
            return;
        }
        for(i = 0; i < n_cols; i++) {
            widths[i] = r->table_columns[i].natural_width;
            if(widths[i] < 3)
                widths[i] = 3;
        }
        if(r->table_term_width > 0)
            limit_table_widths(r, widths, n_cols, r->table_term_width);
        if(r->failed) {
            free(widths);
            return;
        }

        for(i = 0; i < n_cols; i++) {
            r->table_columns[i].rendered_width = widths[i];
        }

        r->table_repaint_count = 0;

        check_bl(r);
        draw_table_border(r, widths, n_cols, BDR_TL, BDR_TM, BDR_TR);
        {
            const struct TableCell* header_cells = NULL;
            int hlines;
            if(r->table_header_cols > 0 && r->table_cells != NULL)
                header_cells = r->table_cells + r->table_header_start;
            hlines = draw_table_row(r, widths, n_cols,
                                    header_cells, r->table_header_cols);
            draw_table_border(r, widths, n_cols, BDR_ML, BDR_MM, BDR_MR);
            r->table_lines = 2 + hlines;
        }
        free(widths);
    }
    r->table_header_rendered = 1;
}

static void
table_leave_table(MD_ANSI_RENDERER* r)
{
    int n_cols = r->table_col_count;
    if(n_cols > 0  &&  r->table_header_rendered) {
        int* widths = (int*) malloc((size_t)n_cols * sizeof(int));
        int i;
        if(widths == NULL) {
            r->failed = 1;
        } else {
            for(i = 0; i < n_cols; i++)
                widths[i] = r->table_columns[i].rendered_width;
            draw_table_border(r, widths, n_cols, BDR_BL, BDR_BM, BDR_BR);
            free(widths);
        }
    }

    signal_bl(r);
    r->in_table = 0;
    table_free_all(r);
}

int
md_ansi_leave_block(MD_BLOCKTYPE type, void* detail, void* userdata)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;
    const MD_ANSI_STYLE* style = get_block_style(r, type, detail);
    (void)detail;

    /* Empty code blocks: emit a visible placeholder. */
    if(type == MD_BLOCK_CODE) {
        code_block_flush(r);
        if(!r->code_had_content) {
            if(in_quote_context(r))
                emit_bq_prefix(r);
            write_output(r, "\xe2\x90\xa3 ", 4);  /* U+2423 (␣) open box symbol */
        }
    }

    pop_block_style(r, style);

    switch(type) {
        case MD_BLOCK_H:
        case MD_BLOCK_P:
        case MD_BLOCK_CODE:
        case MD_BLOCK_HR:
            write_str(r, "\n");
            signal_bl(r);
            break;

        case MD_BLOCK_QUOTE:
            if(r->blockquote_needs_prefix)
                emit_bq_prefix(r);
            write_str(r, "\n");
            signal_bl(r);
            if(r->blockquote_depth > 0)
                r->blockquote_depth--;
            break;

        case MD_BLOCK_ADMONITION:
            r->admonition_active = 0;
            r->admonition_style = NULL;
            write_str(r, "\n");
            signal_bl(r);
            break;

        case MD_BLOCK_REFERENCE_SECTION:
            if(r->ref_line_len > 0) {
                reference_line_emit(r, r->ref_line_buf, r->ref_line_len);
                r->ref_line_len = 0;
            }
            r->in_reference_section = 0;
            write_str(r, "\n");
            signal_bl(r);
            break;

        case MD_BLOCK_FOOTNOTE_DEF:
            write_str(r, "\n");
            break;

        case MD_BLOCK_FOOTNOTE_DEF_SECTION:
            write_str(r, "\n");
            break;

        case MD_BLOCK_LI:
            if(r->last_char != '\n')
                write_str(r, "\n");
            r->in_list_item = 0;
            r->list_item_just_opened = 0;
            break;

        case MD_BLOCK_UL:
        case MD_BLOCK_OL:
            list_leave_nested(r);
            break;

        default:
            break;
    }

    if(type == MD_BLOCK_HTML)
        r->in_html_block = 0;
    if(type == MD_BLOCK_THEAD) {
        r->table_in_header = 0;
        table_leave_thead(r);
    }
    if(type == MD_BLOCK_TBODY)
        r->table_in_body = 0;
    if(type == MD_BLOCK_TH || type == MD_BLOCK_TD)
        table_store_cell(r);
    if(type == MD_BLOCK_TR)
        table_leave_tr(r);
    if(type == MD_BLOCK_TABLE)
        table_leave_table(r);

    return r->failed ? -1 : 0;
}


/********************************
 ***  Span callbacks          ***
 ********************************/

int
md_ansi_enter_span(MD_SPANTYPE type, void* detail, void* userdata)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;
    (void)detail;

    /* Streamed paragraph text (including span opens) can arrive before
     * enter_block(P) fires. The pending blank-line separator set by the
     * previous block's close must be flushed here, before the span's own
     * output/SGR, not deferred to the first text callback. */
    check_bl(r);

    /* A span at the start of a blockquote line must emit the pending │
     * prefix before its own output/SGR, otherwise the bar inherits the
     * span's styling and a full reset in emit_bq_prefix() drops it. */
    if(r->blockquote_needs_prefix == 0
       && in_quote_context(r)
       && r->last_char == '\n')
        r->blockquote_needs_prefix = 1;
    if(r->blockquote_needs_prefix)
        emit_bq_prefix(r);

    switch(type) {
        case MD_SPAN_EM:
            emph_inc(&r->n_italic);
            style_reset_reapply(r);
            break;
        case MD_SPAN_STRONG:
            emph_inc(&r->n_bold);
            style_reset_reapply(r);
            break;
        case MD_SPAN_DEL:
            emph_inc(&r->n_strikethrough);
            style_reset_reapply(r);
            break;
        case MD_SPAN_U:
            emph_inc(&r->n_underline);
            style_reset_reapply(r);
            break;

        case MD_SPAN_MARK:
            open_isolated_style(r, &r->theme->highlight);
            break;

        case MD_SPAN_CODE:
            open_isolated_style(r, &r->theme->inline_code);
            break;

        case MD_SPAN_A:
            if(detail != NULL) {
                MD_SPAN_A_DETAIL* ad = (MD_SPAN_A_DETAIL*)detail;
                r->link_href_len = copy_attribute_decoded(&ad->href,
                                        r->link_href, (int)sizeof(r->link_href));
            }
            if(r->osc8_enabled && r->link_href_len > 0)
                write_osc8_open(r, r->link_href, r->link_href_len);
            open_isolated_style(r, &r->theme->link_text);
            break;

        case MD_SPAN_IMG:
            write_str(r, "[IMG: ");
            r->img_src_len = 0;
            r->img_title_len = 0;
            if(detail != NULL) {
                MD_SPAN_IMG_DETAIL* id = (MD_SPAN_IMG_DETAIL*)detail;
                r->img_src_len = copy_attribute_decoded(&id->src,
                                    r->img_src, (int)sizeof(r->img_src));
                r->img_title_len = copy_attribute_decoded(&id->title,
                                    r->img_title, (int)sizeof(r->img_title));
            }
            if(r->osc8_enabled && r->img_src_len > 0)
                write_osc8_open(r, r->img_src, r->img_src_len);
            open_isolated_style(r, &r->theme->image_label);
            break;

        case MD_SPAN_LATEXMATH:
        case MD_SPAN_LATEXMATH_DISPLAY:
            open_isolated_style(r, &r->theme->inline_code);
            break;

        case MD_SPAN_WIKILINK:
            open_isolated_style(r, &r->theme->link_text);
            break;

        case MD_SPAN_FOOTNOTE_REF:
            open_isolated_style(r, &r->theme->footnote_ref);
            {
                MD_SPAN_FOOTNOTE_REF_DETAIL* det = (MD_SPAN_FOOTNOTE_REF_DETAIL*)detail;
                write_output(r, "[", 1);
                if(det != NULL)
                    write_output(r, det->label.text, det->label.size);
                write_output(r, "]", 1);
            }
            break;

        case MD_SPAN_SPOILER:
            write_str(r, r->theme->spoiler_label.prefix);
            write_str(r, "[spoiler] ");
            write_str(r, r->theme->spoiler_label.suffix);
            write_str(r, r->theme->spoiler.prefix);
            r->spoiler_active = 1;
            break;

        case MD_SPAN_SUPERSCRIPT:
            open_isolated_style_marker(r, &r->theme->superscript, "^", 1);
            break;

        case MD_SPAN_SUBSCRIPT:
            open_isolated_style_marker(r, &r->theme->subscript, "_", 1);
            break;

        case MD_SPAN_REFERENCE_LINK:
            open_isolated_style(r, &r->theme->link_text);
            r->ref_label_len = 0;
            if(detail != NULL) {
                MD_SPAN_REFERENCE_LINK_DETAIL* det = (MD_SPAN_REFERENCE_LINK_DETAIL*)detail;
                r->ref_label_len = copy_attribute_decoded(&det->label,
                                    r->ref_label, (int)sizeof(r->ref_label));
            }
            break;

        case MD_SPAN_REFERENCE_IMAGE:
            write_str(r, "[IMG: ");
            r->ref_label_len = 0;
            if(detail != NULL) {
                MD_SPAN_REFERENCE_LINK_DETAIL* det = (MD_SPAN_REFERENCE_LINK_DETAIL*)detail;
                r->ref_label_len = copy_attribute_decoded(&det->label,
                                    r->ref_label, (int)sizeof(r->ref_label));
            }
            open_isolated_style(r, &r->theme->image_label);
            break;

        default:
            break;
    }

    return r->failed ? -1 : 0;
}

int
md_ansi_leave_span(MD_SPANTYPE type, void* detail, void* userdata)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;
    (void)detail;

    switch(type) {
        case MD_SPAN_EM:
            emph_dec(&r->n_italic);
            style_reset_reapply(r);
            break;
        case MD_SPAN_STRONG:
            emph_dec(&r->n_bold);
            style_reset_reapply(r);
            break;
        case MD_SPAN_DEL:
            emph_dec(&r->n_strikethrough);
            style_reset_reapply(r);
            break;
        case MD_SPAN_U:
            emph_dec(&r->n_underline);
            style_reset_reapply(r);
            break;

        case MD_SPAN_MARK:
            close_isolated_style(r, &r->theme->highlight);
            break;

        case MD_SPAN_CODE:
            close_isolated_style(r, &r->theme->inline_code);
            break;

        case MD_SPAN_A:
            write_str(r, r->theme->link_text.suffix);
            if(r->isolated_style_depth > 0)
                r->isolated_style_depth--;
            if(r->osc8_enabled) {
                if(r->link_href_len > 0)
                    write_osc8_close(r);
            } else {
                emit_link_url_paren(r, r->link_href, r->link_href_len);
            }
            r->link_href_len = 0;
            reapply_all(r);
            break;

        case MD_SPAN_IMG:
            close_isolated_style(r, &r->theme->image_label);
            if(r->osc8_enabled) {
                if(r->img_src_len > 0)
                    write_osc8_close(r);
                write_output(r, "]", 1);
            } else {
                write_str(r, " (");
                write_str(r, r->theme->link_url.prefix);
                if(r->img_src_len > 0)
                    write_output(r, r->img_src, r->img_src_len);
                write_str(r, r->theme->link_url.suffix);
                if(r->img_title_len > 0) {
                    write_output(r, " \"", 2);
                    write_output(r, r->img_title, r->img_title_len);
                    write_output(r, "\"", 1);
                }
                write_output(r, ")]", 2);
            }
            r->img_src_len = 0;
            r->img_title_len = 0;
            break;

        case MD_SPAN_LATEXMATH:
        case MD_SPAN_LATEXMATH_DISPLAY:
            close_isolated_style(r, &r->theme->inline_code);
            break;

        case MD_SPAN_WIKILINK:
            close_isolated_style(r, &r->theme->link_text);
            break;

        case MD_SPAN_FOOTNOTE_REF:
            close_isolated_style(r, &r->theme->footnote_ref);
            break;

        case MD_SPAN_SPOILER:
            write_str(r, r->theme->spoiler.suffix);
            r->spoiler_active = 0;
            reapply_all(r);
            break;

        case MD_SPAN_SUPERSCRIPT:
            close_isolated_style(r, &r->theme->superscript);
            break;

        case MD_SPAN_SUBSCRIPT:
            close_isolated_style(r, &r->theme->subscript);
            break;

        case MD_SPAN_REFERENCE_LINK:
            close_isolated_style(r, &r->theme->link_text);
            write_output(r, " ", 1);
            write_str(r, r->theme->link_url.prefix);
            write_output(r, "ref:", 4);
            write_output(r, "[", 1);
            if(r->ref_label_len > 0)
                write_output(r, r->ref_label, r->ref_label_len);
            write_output(r, "]", 1);
            write_str(r, r->theme->link_url.suffix);
            r->ref_label_len = 0;
            break;

        case MD_SPAN_REFERENCE_IMAGE:
            close_isolated_style(r, &r->theme->image_label);
            if(r->ref_label_len > 0) {
                write_str(r, r->theme->link_url.prefix);
                write_output(r, " (ref:[", 7);
                write_output(r, r->ref_label, r->ref_label_len);
                write_output(r, "])]", 3);
                write_str(r, r->theme->link_url.suffix);
            } else {
                write_output(r, ")]", 2);
            }
            r->ref_label_len = 0;
            break;

        default:
            break;
    }

    return r->failed ? -1 : 0;
}



/* Apply or remove styling for a parsed HTML tag.
 * Returns 1 if handled, 0 if the tag should be output verbatim. */
static int
handle_html_tag(MD_ANSI_RENDERER* r, const HTML_TAG_INFO* tag)
{
    switch(tag->type) {
        case HTML_TAG_BR:
            write_output(r, "\n", 1);
            if(in_quote_context(r))
                r->blockquote_needs_prefix = 1;
            return 1;

        case HTML_TAG_IMG:
            if(r->blockquote_needs_prefix)
                emit_bq_prefix(r);
            if(r->osc8_enabled && tag->src_len > 0)
                write_osc8_open(r, tag->src, tag->src_len);
            write_str(r, r->theme->image_label.prefix);
            if(tag->alt_len > 0)
                write_output(r, tag->alt, tag->alt_len);
            write_str(r, r->theme->image_label.suffix);
            if(r->osc8_enabled) {
                if(tag->src_len > 0)
                    write_osc8_close(r);
            } else if(tag->src_len > 0) {
                write_str(r, " (");
                write_output(r, tag->src, tag->src_len);
                write_str(r, ")");
            }
            return 1;

        case HTML_TAG_A:
            if(tag->is_closer) {
                close_isolated_style(r, &r->theme->link_text);
                if(r->osc8_enabled) {
                    if(r->html_link_href_len > 0)
                        write_osc8_close(r);
                } else if(r->html_link_href_len > 0) {
                    write_str(r, " (");
                    write_str(r, r->theme->link_url.prefix);
                    write_output(r, r->html_link_href, r->html_link_href_len);
                    write_str(r, r->theme->link_url.suffix);
                    write_str(r, ")");
                }
                r->html_link_href_len = 0;
            } else {
                r->html_link_href_len = html_attr_decode(tag->href, tag->href_len,
                                         r->html_link_href,
                                         (int)sizeof(r->html_link_href));
                if(r->blockquote_needs_prefix)
                    emit_bq_prefix(r);
                if(r->osc8_enabled && r->html_link_href_len > 0)
                    write_osc8_open(r, r->html_link_href, r->html_link_href_len);
                open_isolated_style(r, &r->theme->link_text);
            }
            return 1;

        case HTML_TAG_STRONG:
        case HTML_TAG_B:
            if(tag->is_closer)
                emph_dec(&r->n_bold);
            else
                emph_inc(&r->n_bold);
            style_reset_reapply(r);
            return 1;

        case HTML_TAG_EM:
        case HTML_TAG_I:
            if(tag->is_closer)
                emph_dec(&r->n_italic);
            else
                emph_inc(&r->n_italic);
            style_reset_reapply(r);
            return 1;

        case HTML_TAG_DEL:
            if(tag->is_closer)
                emph_dec(&r->n_strikethrough);
            else
                emph_inc(&r->n_strikethrough);
            style_reset_reapply(r);
            return 1;

        case HTML_TAG_CODE:
        case HTML_TAG_KBD:
            if(tag->is_closer)
                close_isolated_style(r, &r->theme->inline_code);
            else {
                if(r->blockquote_needs_prefix)
                    emit_bq_prefix(r);
                open_isolated_style(r, &r->theme->inline_code);
            }
            return 1;

        default:
            return 0;
    }
}

/* html_scan() event callback: translate HTML constructs into styled
 * output. Unknown tags and unparsed content are output verbatim;
 * comments never reach here (the scanner drops them). */
static void
html_event_cb(void* userdata, int event, const char* text, int size,
              const HTML_TAG_INFO* tag)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;

    switch(event) {
        case HTML_EVENT_TEXT:
        case HTML_EVENT_CDATA:
        case HTML_EVENT_PI:
        case HTML_EVENT_DECL:
            write_output(r, text, size);
            break;

        case HTML_EVENT_ENTITY:
            render_entity(r, text, size);
            break;

        case HTML_EVENT_TAG:
            if(!handle_html_tag(r, tag))
                write_output(r, text, size);
            break;

        default:
            break;
    }
}

/* Emit blockquote │ prefix for each nesting depth, or admonition color
 * prefix if inside an admonition.  Clears blockquote_needs_prefix. */
static void
emit_bq_prefix(MD_ANSI_RENDERER* r)
{
    int d;
    int total = (r->blockquote_depth > 0) ? r->blockquote_depth : r->admonition_active;
    for(d = 0; d < total; d++) {
        unsigned char vbar[] = { 0xE2, 0x94, 0x82, ' ', '\0' };
        int use_admo = (r->admonition_active && d == total - 1);
        if(use_admo && r->admonition_style != NULL
                && r->admonition_style->prefix_len > 0)
            write_str(r, r->admonition_style->prefix);
        else
            write_str(r, r->theme->blockquote.prefix);
        write_output(r, (const char*)vbar, 4);
        write_sgr_reset(r);
    }
    r->blockquote_needs_prefix = 0;
    reapply_all(r);
}

/* Write body text, applying blockquote/admonition line prefixes as needed. */
static void
write_text_with_bq(MD_ANSI_RENDERER* r, const MD_CHAR* text, MD_SIZE size)
{
    /* Streaming can emit paragraph text before enter_block(P), so the
     * prefix armed by enter_block may not exist yet; a fresh line after a
     * block close (e.g. a heading inside a quote) must still get the bar.
     * Batch never hits this: its enter_block(P) arms the prefix. */
    if(r->blockquote_needs_prefix == 0
       && in_quote_context(r)
       && r->last_char == '\n')
        r->blockquote_needs_prefix = 1;
    maybe_emit_bq_prefix(r, size);
    write_output(r, text, size);
    mark_bq_prefix_after_nl(r, text, size);
}


/********************************
 ***  Code block highlighting ***
 ********************************/

/* Token emission callback (defined below). */
static void emit_code_token(void* userdata, int style,
                            const char* text, int len);

/* Feed one MD_TEXT_CODE chunk into the highlighter; tokens that finalize
 * are emitted immediately. On allocation failure, drop highlighting for
 * the rest of the block and write the chunk raw. */
static void
code_feed(MD_ANSI_RENDERER* r, const MD_CHAR* text, MD_SIZE size)
{
    if(hl_feed(&r->hl_state, (const char*) text, (int) size,
               emit_code_token, r) != 0) {
        r->hl_active = 0;
        hl_reset(&r->hl_state);
        maybe_emit_bq_prefix(r, size);
        write_output(r, text, size);
        mark_bq_prefix_after_nl(r, text, size);
    }
}

/* Token emission callback for the code highlighter: write the token with
 * its style, re-arming the blockquote │ bar at each newline (re-opening
 * the span style after the bar so both survive). */
static void
emit_code_token(void* userdata, int style, const char* text, int len)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;
    const MD_ANSI_STYLE* st = NULL;
    int i = 0;

    switch(style) {
        case HL_STYLE_PLAIN:   st = &r->theme->hl_normal; break;
        case HL_STYLE_KEYWORD: st = &r->theme->hl_keyword; break;
        case HL_STYLE_PUNCT:   st = &r->theme->hl_punct;   break;
        case HL_STYLE_STRING:  st = &r->theme->hl_string;  break;
        case HL_STYLE_COMMENT: st = &r->theme->hl_comment; break;
        default:               break;
    }

    /* Whitespace tokens are unformatted: never give them a
     * color, even when hl_normal has one. Keeps the output clean (no SGR
     * around every space/newline). */
    if(st == &r->theme->hl_normal) {
        int k;
        for(k = 0; k < len; k++) {
            char c = text[k];
            if(!(c == ' ' || c == '\t' || c == '\n'
                 || c == '\v' || c == '\f' || c == '\r'))
                break;
        }
        if(k == len)
            st = NULL;
    }

    if(st != NULL && st->prefix_len > 0)
        write_str(r, st->prefix);

    while(i < len) {
        int j = i;
        while(j < len && text[j] != '\n')
            j++;
        if(r->blockquote_needs_prefix) {
            emit_bq_prefix(r);
            if(st != NULL && st->prefix_len > 0)
                write_str(r, st->prefix);
        }
        write_output(r, text + i, j - i);
        if(j < len) {
            write_output(r, "\n", 1);
            if(in_quote_context(r))
                r->blockquote_needs_prefix = 1;
        }
        i = j + 1;
    }

    if(st != NULL && st->prefix_len > 0) {
        write_sgr_reset(r);
        reapply_all(r);
    }
}

/* Flush the highlighter at block close: process the held-back characters
 * and the final token, then release the scanner state. */
static void
code_block_flush(MD_ANSI_RENDERER* r)
{
    if(r->hl_active)
        hl_finish(&r->hl_state, emit_code_token, r);
    else
        hl_reset(&r->hl_state);
    r->hl_active = 0;
}

/* Emit one reference-definition line "[label]: dest ...".  With OSC 8
 * enabled the destination is wrapped in a clickable hyperlink; the block
 * style (link_url) is already active around the whole line. */
static void
reference_line_emit(MD_ANSI_RENDERER* r, const char* line, int len)
{
    int i = 0;
    int dest_beg = -1;
    int dest_end = -1;

    if(len > 0 && line[0] == '[') {
        for(i = 1; i < len; i++) {
            if(line[i] == ']')
                break;
        }
        if(i < len && line[i] == ']') {
            i++;
            if(i < len && line[i] == ':') {
                i++;
                while(i < len && (line[i] == ' ' || line[i] == '\t'))
                    i++;
                dest_beg = i;
                while(i < len && line[i] != ' ' && line[i] != '\t')
                    i++;
                dest_end = i;
            }
        }
    }

    if(dest_beg < 0 || dest_end <= dest_beg) {
        write_output(r, line, len);
        return;
    }

    write_output(r, line, dest_beg);
    if(r->osc8_enabled) {
        write_osc8_open(r, line + dest_beg, dest_end - dest_beg);
        write_output(r, line + dest_beg, dest_end - dest_beg);
        write_osc8_close(r);
    } else {
        write_output(r, line + dest_beg, dest_end - dest_beg);
    }
    write_output(r, line + dest_end, len - dest_end);
}

/* Accumulate reference-section text into per-line buffers so each
 * definition line can be emitted with a clickable destination. */
static void
reference_section_text(MD_ANSI_RENDERER* r, const char* text, int size)
{
    int i = 0;

    while(i < size) {
        if(text[i] == '\n') {
            reference_line_emit(r, r->ref_line_buf, r->ref_line_len);
            write_output(r, "\n", 1);
            r->ref_line_len = 0;
            i++;
            continue;
        }
        if(r->ref_line_len >= (int)sizeof(r->ref_line_buf) - 1) {
            /* Unusually long line: emit raw so memory stays bounded. */
            write_output(r, r->ref_line_buf, r->ref_line_len);
            r->ref_line_len = 0;
            while(i < size && text[i] != '\n') {
                write_output(r, text + i, 1);
                i++;
            }
            if(i < size) {
                write_output(r, "\n", 1);
                i++;
            }
            continue;
        }
        r->ref_line_buf[r->ref_line_len++] = text[i];
        i++;
    }
}

int
md_ansi_text(MD_TEXTTYPE type, const MD_CHAR* text, MD_SIZE size, void* userdata)
{
    MD_ANSI_RENDERER* r = (MD_ANSI_RENDERER*) userdata;

    /* Any visible text means the list item marker is no longer the last
     * thing written. */
    r->list_item_just_opened = 0;

    /* Streamed paragraph text can arrive before its enter_block(P) fires
     * (paragraph close). The pending blank-line separator set by the
     * previous block's leave must therefore be flushed here, when the text
     * actually starts, instead of only in enter_block(). A no-op whenever
     * enter_block() already ran (it clears the flag). */
    check_bl(r);

    switch(type) {
        case MD_TEXT_BR:
        case MD_TEXT_SOFTBR:
            write_output(r, "\n", 1);
            if(in_quote_context(r))
                r->blockquote_needs_prefix = 1;
            break;

        case MD_TEXT_NULLCHAR:
            write_output(r, "\xef\xbf\xbd", 3);
            break;

        case MD_TEXT_ENTITY:
            render_entity(r, text, size);
            break;

        case MD_TEXT_CODE:
            if(r->hl_active) {
                /* Highlight incrementally; tokens emit as they finalize. */
                if(size > 0 && !(size == 1 && text[0] == '\n'))
                    r->code_had_content = 1;
                code_feed(r, text, size);
            } else {
                maybe_emit_bq_prefix(r, size);
                if(size > 0 && !(size == 1 && text[0] == '\n'))
                    r->code_had_content = 1;
                write_output(r, text, size);
                mark_bq_prefix_after_nl(r, text, size);
            }
            break;

        case MD_TEXT_HTML:
            if(r->in_html_block) {
                maybe_emit_bq_prefix(r, size);
                html_scan((const char*) text, (int) size,
                          &r->html_state, html_event_cb, r);
                mark_bq_prefix_after_nl(r, text, size);
                break;
            }
            {
                HTML_TAG_INFO tag;
                html_parse_tag((const char*) text, (int) size, &tag);
                if(!handle_html_tag(r, &tag))
                    goto html_verbatim;
            }
            break;

        html_verbatim:
        default:
            if(r->in_reference_section) {
                reference_section_text(r, (const char*) text, (int) size);
                break;
            }
            write_text_with_bq(r, text, size);
            break;
    }

    return r->failed ? -1 : 0;
}


/********************************
 ***  Table rendering         ***
 ********************************/

/* Draw a horizontal table border.
 *   left/join/right: box-drawing connectors (e.g. ┌, ┬, ┐)
 *   fill: horizontal line char (─)
 */
static void
draw_table_border(MD_ANSI_RENDERER* r, const int* widths, int n_cols,
                  const char* left, const char* join, const char* right)
{
    int i;

    write_str(r, r->theme->table_border.prefix);
    write_str(r, left);
    for(i = 0; i < n_cols; i++) {
        int w;
        for(w = 0; w < widths[i]; w++)
            write_str(r, BDR_H);
        if(i < n_cols - 1)
            write_str(r, join);
    }
    write_str(r, right);
    write_str(r, r->theme->table_border.suffix);
    write_str(r, "\n");
}

/* Draw a single table row, following mdflow's drawRow exactly.
 * Wraps each cell, then draws the row line by line with alignment.
 * Returns the number of terminal lines drawn. */
static int
draw_table_row(MD_ANSI_RENDERER* r, const int* widths, int n_cols,
               const struct TableCell* cells, int n_cells)
{
    struct WrappedSlice** cell_slices;
    int* n_slices;
    int col, line;
    int max_lines = 1;

    if(n_cols <= 0)
        return 0;
    if(n_cells > n_cols)
        n_cells = n_cols;

    cell_slices = (struct WrappedSlice**) calloc((size_t)n_cols,
                                                  sizeof(*cell_slices));
    n_slices = (int*) calloc((size_t)n_cols, sizeof(*n_slices));
    if(cell_slices == NULL || n_slices == NULL) {
        free(cell_slices);
        free(n_slices);
        r->failed = 1;
        return 1;
    }

    /* For each column: wrap cell text into slices. */
    for(col = 0; col < n_cols; col++) {
        int content_w = widths[col] - 2;
        const char* cell_text;
        int cell_len;

        if(content_w < 1) content_w = 1;
        if(col < n_cells && cells != NULL) {
            cell_text = table_cell_text(r, &cells[col]);
            cell_len = cells[col].len;
        } else {
            cell_text = "";
            cell_len = 0;
        }
        cell_slices[col] = NULL;
        n_slices[col] = wrap_content(cell_text, cell_len, content_w,
                                      &cell_slices[col]);
        if(n_slices[col] <= 0) {
            r->failed = 1;
            goto cleanup;
        }
        if(n_slices[col] > max_lines)
            max_lines = n_slices[col];
    }

    /* Draw each line. */
    for(line = 0; line < max_lines; line++) {
        write_str(r, r->theme->table_border.prefix);
        write_str(r, BDR_V);
        write_str(r, r->theme->table_border.suffix);

        for(col = 0; col < n_cols; col++) {
            int content_w = widths[col] - 2;
            const struct WrappedSlice* sl = NULL;

            if(content_w < 1) content_w = 1;

            /* Get the slice for this line, or empty. */
            if(line < n_slices[col]) {
                sl = &cell_slices[col][line];
            }

            /* Compute alignment padding for this slice. */
            {
                int vis = sl ? sl->visible : 0;
                int align_val = (col < r->table_col_count)
                              ? r->table_columns[col].align : 0;
                int pad = content_w - vis;
                int left_pad = 0;
                int right_pad;
                if(pad < 0) pad = 0;
                right_pad = pad;
                if(align_val == MD_ALIGN_RIGHT) {
                    left_pad = pad; right_pad = 0;
                } else if(align_val == MD_ALIGN_CENTER) {
                    left_pad = pad / 2; right_pad = pad - left_pad;
                }

                write_output(r, " ", 1);
                while(left_pad > 0) { write_output(r, " ", 1); left_pad--; }
                if(sl && sl->len > 0)
                    write_output(r, sl->text, sl->len);
                while(right_pad > 0) { write_output(r, " ", 1); right_pad--; }
                write_output(r, " ", 1);
            }

            if(col < n_cols - 1) {
                write_str(r, r->theme->table_border.prefix);
                write_str(r, BDR_V);
                write_str(r, r->theme->table_border.suffix);
            }
        }

        write_str(r, r->theme->table_border.prefix);
        write_str(r, BDR_V);
        write_str(r, r->theme->table_border.suffix);
        write_str(r, "\n");
    }

cleanup:
    /* Free all slices. */
    for(col = 0; col < n_cols; col++) {
        free_slices(cell_slices[col], n_slices[col]);
        free(cell_slices[col]);
    }
    free(n_slices);
    free(cell_slices);

    return max_lines;
}

/* If the table is wider than the terminal, shrink columns with one cheap
 * proportional pass.  Each column starts at the width of its longest
 * preferred-break-free run (word, URL segment, or wide-character sequence);
 * the remaining space is distributed proportionally to natural width,
 * capped at natural width.  When even those floors do not fit, the floors
 * themselves are scaled proportionally.  This keeps the allocator O(columns)
 * per row instead of re-wrapping every cell for each candidate width. */
static void
limit_table_widths(MD_ANSI_RENDERER* r, int* widths, int n_cols, int term_width)
{
    int* caps;
    int* mins;
    int* weights;
    long total_min;
    long total_weight;
    int total;
    int available;
    int allocated;
    int floors_fit;
    int i;

    if(term_width <= 0 || n_cols == 0)
        return;

    total = n_cols + 1;
    for(i = 0; i < n_cols; i++)
        total += widths[i];
    if(total <= term_width)
        return;

    /* Very narrow terminals cannot fit even the renderer's minimum cells. */
    if(term_width < 10 || n_cols * 3 + n_cols + 1 > term_width)
        return;

    available = term_width - n_cols - 1;

    caps = (int*) malloc((size_t)n_cols * sizeof(int));
    mins = (int*) malloc((size_t)n_cols * sizeof(int));
    weights = (int*) malloc((size_t)n_cols * sizeof(int));
    if(caps == NULL || mins == NULL || weights == NULL) {
        free(caps);
        free(mins);
        free(weights);
        r->failed = 1;
        return;
    }

    total_min = 0;
    for(i = 0; i < n_cols; i++) {
        caps[i] = widths[i];
        if(caps[i] < 3)
            caps[i] = 3;
        mins[i] = (r->table_columns != NULL)
                ? r->table_columns[i].min_width : 3;
        if(mins[i] < 3)
            mins[i] = 3;
        if(mins[i] > caps[i])
            mins[i] = caps[i];
        total_min += mins[i];
    }

    floors_fit = (total_min <= available);

    allocated = 0;
    total_weight = 0;
    for(i = 0; i < n_cols; i++) {
        widths[i] = floors_fit ? mins[i] : 3;
        allocated += widths[i];
        weights[i] = floors_fit ? caps[i] : (mins[i] - 3);
        if(weights[i] < 0)
            weights[i] = 0;
        total_weight += weights[i];
    }

    if(allocated < available) {
        int remaining = available - allocated;
        for(i = 0; i < n_cols; i++) {
            int cap = floors_fit ? caps[i] : mins[i];
            int max_extra = cap - widths[i];
            int extra;
            if(max_extra <= 0 || total_weight == 0)
                continue;
            extra = (int)((weights[i] * (long)remaining) / total_weight);
            if(extra > max_extra)
                extra = max_extra;
            widths[i] += extra;
            allocated += extra;
        }
    }

    /* Fill rounding leftovers, favoring columns with the most room. */
    while(allocated < available) {
        int best_idx = -1;
        int best_room = 0;
        for(i = 0; i < n_cols; i++) {
            int cap = floors_fit ? caps[i] : mins[i];
            int room = cap - widths[i];
            if(room > best_room) {
                best_room = room;
                best_idx = i;
            }
        }
        if(best_idx < 0)
            break;
        widths[best_idx]++;
        allocated++;
    }

    free(caps);
    free(mins);
    free(weights);
}

/* Redraw the entire table from stored rows when widths change.
 * Uses ANSI cursor-up + clear to overwrite the previous rendering. */
static void
redraw_table(MD_ANSI_RENDERER* r)
{
    int n_cols = r->table_col_count;
    int* widths;
    int i;

    if(n_cols == 0)
        return;

    widths = (int*) malloc((size_t)n_cols * sizeof(int));
    if(widths == NULL) {
        r->failed = 1;
        return;
    }
    for(i = 0; i < n_cols; i++)
        widths[i] = r->table_columns[i].rendered_width;

    /* Move cursor up and clear to end of display. */
    if(r->table_lines > 0) {
        char buf[32];
        int n = snprintf(buf, sizeof(buf), "\033[%dA\r\033[J", r->table_lines);
        if(n > 0)
            write_output(r, buf, n);
    }

    /* Redraw everything (no check_bl — we're overwriting, not starting fresh). */
    draw_table_border(r, widths, n_cols, BDR_TL, BDR_TM, BDR_TR);
    {
        const struct TableCell* header_cells = NULL;
        int hlines;
        if(r->table_header_cols > 0 && r->table_cells != NULL)
            header_cells = r->table_cells + r->table_header_start;
        hlines = draw_table_row(r, widths, n_cols,
                                header_cells, r->table_header_cols);
        draw_table_border(r, widths, n_cols, BDR_ML, BDR_MM, BDR_MR);
        r->table_lines = 2 + hlines;
    }

    for(i = 0; i < r->table_n_rows; i++) {
        const struct TableRow* row = &r->table_rows[i];
        const struct TableCell* cells = NULL;
        if(r->table_cells != NULL)
            cells = r->table_cells + row->first_cell;
        r->table_lines += draw_table_row(r, widths, n_cols, cells,
                                         row->n_cells);
    }
    free(widths);
}


/********************************
 ***  Batch render API        ***
 ********************************/

int
md_ansi(const MD_CHAR* text, MD_SIZE size,
        const MD_ANSI_THEME* theme,
        void (*output)(const char* str, int size, void* userdata),
        void* userdata)
{
    MD_ANSI_RENDERER* renderer;
    MD_PARSER parser;
    int ret;

    renderer = md_ansi_renderer_create(theme, output, userdata);
    if(renderer == NULL)
        return -1;

    memset(&parser, 0, sizeof(MD_PARSER));
    parser.abi_version = 0;
    parser.flags = 0;
    parser.enter_block = md_ansi_enter_block;
    parser.leave_block = md_ansi_leave_block;
    parser.enter_span = md_ansi_enter_span;
    parser.leave_span = md_ansi_leave_span;
    parser.text = md_ansi_text;

    ret = md_parse(text, size, &parser, (void*) renderer);

    md_ansi_renderer_destroy(renderer);
    return ret;
}
