/*
 * typewriter.c - paced terminal output layer for the mdflow CLI
 * (https://github.com/cjccjj/mdflow)
 *
 * The pacer is clock/writer agnostic: callers pass the current monotonic
 * time and a writer callback, which makes the burst and timing rules
 * unit-testable without sleeping.
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

#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif

#include "typewriter.h"

#include <stdlib.h>
#include <string.h>
#include <time.h>


double
tw_monotonic(void)
{
    struct timespec ts;

    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (double) ts.tv_sec + (double) ts.tv_nsec / 1000000000.0;
}


/***********************************
 ***  ANSI / visible-unit tokenizer
 ***********************************/

/* Decode a UTF-8 character from text at offset *pos. Advances *pos past the
 * encoded character and returns the codepoint and (via p_len) the byte
 * length. p_len is 1 for invalid sequences. */
static unsigned
utf8_decode(const char* text, size_t len, size_t* pos, size_t* p_len)
{
    unsigned ch;
    size_t i = *pos;
    size_t n;

    if(i >= len) {
        *p_len = 0;
        return 0;
    }

    if(((unsigned char) text[i] & 0x80) == 0) {
        ch = (unsigned char) text[i];
        n = 1;
    } else if(((unsigned char) text[i] & 0xE0) == 0xC0 && i + 1 < len) {
        ch = ((unsigned char) text[i] & 0x1F) << 6
           | ((unsigned char) text[i + 1] & 0x3F);
        n = 2;
    } else if(((unsigned char) text[i] & 0xF0) == 0xE0 && i + 2 < len) {
        ch = ((unsigned char) text[i] & 0x0F) << 12
           | ((unsigned char) text[i + 1] & 0x3F) << 6
           | ((unsigned char) text[i + 2] & 0x3F);
        n = 3;
    } else if(((unsigned char) text[i] & 0xF8) == 0xF0 && i + 3 < len) {
        ch = ((unsigned char) text[i] & 0x07) << 18
           | ((unsigned char) text[i + 1] & 0x3F) << 12
           | ((unsigned char) text[i + 2] & 0x3F) << 6
           | ((unsigned char) text[i + 3] & 0x3F);
        n = 4;
    } else {
        ch = (unsigned char) text[i];
        n = 1;
    }

    *pos = i + n;
    *p_len = n;
    return ch;
}

/* Return the index after a CSI sequence (ESC [ ... final byte). */
static size_t
csi_end_index(const char* text, size_t len, size_t start)
{
    size_t i = start + 2;
    while(i < len) {
        unsigned char c = (unsigned char) text[i];
        if(c >= 0x40 && c <= 0x7E)
            return i + 1;
        i++;
    }
    return len;
}

/* Return the index after an OSC sequence (BEL or ST termination). */
static size_t
osc_end_index(const char* text, size_t len, size_t start)
{
    size_t i = start + 2;
    while(i < len) {
        if(text[i] == '\x07')
            return i + 1;
        if(text[i] == '\x1b' && i + 1 < len && text[i + 1] == '\\')
            return i + 2;
        i++;
    }
    return len;
}

/* Return the index after the escape sequence starting at start. */
static size_t
escape_end_index(const char* text, size_t len, size_t start)
{
    char nxt;

    if(start + 1 >= len)
        return len;
    nxt = text[start + 1];
    if(nxt == '[')
        return csi_end_index(text, len, start);
    if(nxt == ']')
        return osc_end_index(text, len, start);
    if(nxt == '(' || nxt == ')' || nxt == '#' || nxt == '%') {
        size_t end = start + 3;
        return end < len ? end : len;
    }
    return start + 2;
}

/* Zero-width combining marks and format controls (General Category
 * Mn/Me/Cf, practical subset; width is report-only). */
static int
is_combining(unsigned cp)
{
    if(cp == 0x00AD) return 1;
    if(cp >= 0x0300 && cp <= 0x036F) return 1;
    if(cp >= 0x0483 && cp <= 0x0489) return 1;
    if(cp >= 0x0591 && cp <= 0x05BD) return 1;
    if(cp == 0x05BF || cp == 0x05C7) return 1;
    if(cp >= 0x05C1 && cp <= 0x05C2) return 1;
    if(cp >= 0x05C4 && cp <= 0x05C5) return 1;
    if(cp >= 0x0610 && cp <= 0x061A) return 1;
    if(cp >= 0x064B && cp <= 0x065F) return 1;
    if(cp == 0x0670) return 1;
    if(cp >= 0x06D6 && cp <= 0x06DC) return 1;
    if(cp >= 0x06DF && cp <= 0x06E4) return 1;
    if(cp >= 0x06E7 && cp <= 0x06E8) return 1;
    if(cp >= 0x06EA && cp <= 0x06ED) return 1;
    if(cp == 0x0711) return 1;
    if(cp >= 0x0730 && cp <= 0x074A) return 1;
    if(cp >= 0x07A6 && cp <= 0x07B0) return 1;
    if(cp >= 0x0900 && cp <= 0x0902) return 1;
    if(cp == 0x093A) return 1;
    if(cp >= 0x0941 && cp <= 0x0948) return 1;
    if(cp >= 0x0962 && cp <= 0x0963) return 1;
    if(cp >= 0x1AB0 && cp <= 0x1AFF) return 1;
    if(cp >= 0x1DC0 && cp <= 0x1DFF) return 1;
    if(cp >= 0x20D0 && cp <= 0x20FF) return 1;
    if(cp >= 0xFE20 && cp <= 0xFE2F) return 1;
    if(cp >= 0x0600 && cp <= 0x0605) return 1;
    if(cp == 0x061C || cp == 0x06DD || cp == 0x070F) return 1;
    if(cp >= 0x0890 && cp <= 0x0891) return 1;
    if(cp == 0x08E2 || cp == 0x180E) return 1;
    if(cp >= 0x200B && cp <= 0x200F) return 1;
    if(cp >= 0x202A && cp <= 0x202E) return 1;
    if(cp >= 0x2060 && cp <= 0x2064) return 1;
    if(cp >= 0x2066 && cp <= 0x206F) return 1;
    if(cp == 0xFEFF) return 1;
    if(cp >= 0xFFF9 && cp <= 0xFFFB) return 1;
    if(cp == 0x110BD || cp == 0x110CD) return 1;
    if(cp >= 0x13430 && cp <= 0x1343F) return 1;
    if(cp >= 0x1BCA0 && cp <= 0x1BCA3) return 1;
    if(cp >= 0x1D173 && cp <= 0x1D17A) return 1;
    if(cp == 0xE0001) return 1;
    if(cp >= 0xE0020 && cp <= 0xE007F) return 1;
    return 0;
}

static int
is_variation_selector(unsigned cp)
{
    return (cp >= 0xFE00 && cp <= 0xFE0F) || (cp >= 0xE0100 && cp <= 0xE01EF);
}

static int
is_emoji_modifier(unsigned cp)
{
    return cp >= 0x1F3FB && cp <= 0x1F3FF;
}

static int
is_regional_indicator(unsigned cp)
{
    return cp >= 0x1F1E6 && cp <= 0x1F1FF;
}

static int
is_keycap(unsigned cp)
{
    return cp == 0x20E3;
}

static int
is_zwj(unsigned cp)
{
    return cp == 0x200D;
}

static int
is_zero_width(unsigned cp)
{
    return is_combining(cp) || is_variation_selector(cp) || is_zwj(cp);
}

/* East Asian wide/fullwidth and emoji presentation ranges (practical
 * subset; report-only). */
static int
is_wide(unsigned cp)
{
    if(cp >= 0x1100 && cp <= 0x115F) return 1;
    if(cp >= 0xA960 && cp <= 0xA97C) return 1;
    if(cp >= 0xAC00 && cp <= 0xD7A3) return 1;
    if(cp >= 0xD7B0 && cp <= 0xD7FF) return 1;
    if(cp >= 0x2E80 && cp <= 0x303E) return 1;
    if(cp >= 0x3040 && cp <= 0x33BF) return 1;
    if(cp >= 0x3400 && cp <= 0x4DBF) return 1;
    if(cp >= 0x4E00 && cp <= 0xA4CF) return 1;
    if(cp >= 0xF900 && cp <= 0xFAFF) return 1;
    if(cp >= 0x20000 && cp <= 0x3FFFF) return 1;
    if(cp >= 0xFE10 && cp <= 0xFE19) return 1;
    if(cp >= 0xFE30 && cp <= 0xFE6F) return 1;
    if(cp >= 0xFF01 && cp <= 0xFF60) return 1;
    if(cp >= 0xFFE0 && cp <= 0xFFE6) return 1;
    if(cp >= 0x1B000 && cp <= 0x1B0FF) return 1;
    /* BMP emoji with default emoji presentation; text-style symbols
     * (☀ ✂ ✓ …) stay single-width unless FE0F makes the cluster emoji. */
    if(cp == 0x231A || cp == 0x231B) return 1;
    if(cp == 0x2329 || cp == 0x232A) return 1;
    if(cp >= 0x23E9 && cp <= 0x23EC) return 1;
    if(cp == 0x23F0 || cp == 0x23F3) return 1;
    if(cp >= 0x25FD && cp <= 0x25FE) return 1;
    if(cp >= 0x2614 && cp <= 0x2615) return 1;
    if(cp >= 0x2630 && cp <= 0x2637) return 1;
    if(cp >= 0x2648 && cp <= 0x2653) return 1;
    if(cp == 0x267F) return 1;
    if(cp >= 0x268A && cp <= 0x268F) return 1;
    if(cp == 0x2693) return 1;
    if(cp == 0x26A1) return 1;
    if(cp >= 0x26AA && cp <= 0x26AB) return 1;
    if(cp >= 0x26BD && cp <= 0x26BE) return 1;
    if(cp >= 0x26C4 && cp <= 0x26C5) return 1;
    if(cp == 0x26CE || cp == 0x26D4 || cp == 0x26EA) return 1;
    if(cp >= 0x26F2 && cp <= 0x26F3) return 1;
    if(cp == 0x26F5 || cp == 0x26FA || cp == 0x26FD) return 1;
    if(cp == 0x2705) return 1;
    if(cp >= 0x270A && cp <= 0x270B) return 1;
    if(cp == 0x2728 || cp == 0x274C || cp == 0x274E) return 1;
    if(cp >= 0x2753 && cp <= 0x2755) return 1;
    if(cp == 0x2757) return 1;
    if(cp >= 0x2795 && cp <= 0x2797) return 1;
    if(cp == 0x27B0 || cp == 0x27BF) return 1;
    if(cp >= 0x2B1B && cp <= 0x2B1C) return 1;
    if(cp == 0x2B50 || cp == 0x2B55) return 1;
    if(cp >= 0x1F004 && cp <= 0x1F9FF) return 1;
    if(cp >= 0x1F170 && cp <= 0x1F1FF) return 1;
    if(cp >= 0x1FA00 && cp <= 0x1FAFF) return 1;
    return 0;
}

/* Approximate terminal display columns for one grouped visible unit. */
static int
cluster_width(const char* text, size_t len)
{
    size_t i = 0;
    int emoji = 0;
    int width = 0;

    while(i < len) {
        size_t n;
        unsigned cp = utf8_decode(text, len, &i, &n);
        if(cp == 0xFE0F || cp == 0x200D || cp >= 0x1F000)
            emoji = 1;
        if(!is_zero_width(cp))
            width += is_wide(cp) ? 2 : 1;
    }
    if(emoji)
        return 2;
    return width > 0 ? width : 1;
}

static int
push_unit(tw_unit_t** units, size_t* count, size_t* cap,
          const char* text, size_t text_len,
          const char* esc, size_t esc_len,
          int width, int visible, int pace)
{
    tw_unit_t* u;

    if(*count == *cap) {
        size_t new_cap = *cap > 0 ? *cap * 2 : 16;
        tw_unit_t* new_units =
            (tw_unit_t*) realloc(*units, new_cap * sizeof(tw_unit_t));
        if(new_units == NULL)
            return -1;
        *units = new_units;
        *cap = new_cap;
    }
    u = &(*units)[*count];
    u->text = text;
    u->text_len = text_len;
    u->esc = esc;
    u->esc_len = esc_len;
    u->width = width;
    u->visible = visible;
    u->pace = pace;
    (*count)++;
    return 0;
}

int
tw_tokenize_line(const char* text, size_t len,
                 tw_unit_t** out_units, size_t* out_count)
{
    tw_unit_t* units = NULL;
    size_t count = 0;
    size_t cap = 0;
    size_t i = 0;
    size_t esc_start = 0;
    size_t esc_len = 0;

    *out_units = NULL;
    *out_count = 0;

    while(i < len) {
        unsigned char c = (unsigned char) text[i];

        if(c == 0x1B) {
            size_t end = escape_end_index(text, len, i);
            if(esc_len == 0)
                esc_start = i;
            esc_len += end - i;
            i = end;
            continue;
        }

        if(c == '\n' || c == '\r') {
            if(push_unit(&units, &count, &cap,
                         text + i, 1, text + esc_start, esc_len,
                         0, 0, 0) != 0)
                goto error;
            esc_len = 0;
            i++;
            continue;
        }

        if(c == '\t') {
            if(push_unit(&units, &count, &cap,
                         text + i, 1, text + esc_start, esc_len,
                         1, 1, 1) != 0)
                goto error;
            esc_len = 0;
            i++;
            continue;
        }

        if(c < 0x20 || c == 0x7F) {
            if(push_unit(&units, &count, &cap,
                         text + i, 1, text + esc_start, esc_len,
                         0, 0, 0) != 0)
                goto error;
            esc_len = 0;
            i++;
            continue;
        }

        /* Grapheme-like grouping for the current visible cluster. */
        {
            size_t start = i;
            size_t pos = i;
            size_t n;
            unsigned prev = utf8_decode(text, len, &pos, &n);

            i = pos;
            while(i < len) {
                size_t q = i;
                size_t q_len;
                unsigned nxt = utf8_decode(text, len, &q, &q_len);

                if(is_zwj(nxt) && q < len) {
                    size_t r = q;
                    size_t r_len;
                    unsigned after = utf8_decode(text, len, &r, &r_len);
                    i = r;
                    prev = after;
                    continue;
                }
                if(is_combining(nxt) || is_variation_selector(nxt)
                   || is_emoji_modifier(nxt) || is_keycap(nxt)) {
                    i = q;
                    prev = nxt;
                    continue;
                }
                if(is_regional_indicator(prev)
                   && is_regional_indicator(nxt)) {
                    i = q;
                    prev = nxt;
                    continue;
                }
                break;
            }

            if(push_unit(&units, &count, &cap,
                         text + start, i - start, text + esc_start, esc_len,
                         cluster_width(text + start, i - start),
                         1, 1) != 0)
                goto error;
            esc_len = 0;
        }
    }

    if(esc_len > 0) {
        /* Trailing ANSI with no following visible char: emit a zero-width,
         * non-pacing unit so the terminal state stays correct. */
        if(push_unit(&units, &count, &cap,
                     text + len, 0, text + esc_start, esc_len,
                     0, 0, 0) != 0)
            goto error;
    }

    *out_units = units;
    *out_count = count;
    return 0;

error:
    free(units);
    return -1;
}

void
tw_units_free(tw_unit_t* units)
{
    free(units);
}


/***********************************
 ***  Raw-input metadata
 ***********************************/

static int
utf8_seq_len(unsigned char c)
{
    if((c & 0x80) == 0)
        return 1;
    if((c & 0xE0) == 0xC0)
        return 2;
    if((c & 0xF0) == 0xE0)
        return 3;
    if((c & 0xF8) == 0xF0)
        return 4;
    return 1;
}

static int
utf8_valid(const unsigned char* s, size_t len)
{
    size_t i;
    for(i = 1; i < len; i++) {
        if((s[i] & 0xC0) != 0x80)
            return 0;
    }
    return 1;
}

void
tw_input_state_init(tw_input_state_t* s, double initial_rate,
                    double alpha, double rate_cap)
{
    memset(s, 0, sizeof(*s));
    s->alpha = alpha;
    s->rate_cap = rate_cap;
    s->rate = initial_rate;
}

void
tw_input_state_seed_start(tw_input_state_t* s, double now)
{
    if(s->sample_count == 0) {
        s->sample_time[0] = now;
        s->sample_chars[0] = 0;
        s->sample_head = 0;
        s->sample_count = 1;
    }
}

void
tw_input_state_observe_chars(tw_input_state_t* s,
                             const char* data, size_t len, double now)
{
    size_t i = 0;
    long long count = 0;

    /* Finish a partial UTF-8 sequence carried from a previous read. */
    if(s->carry_len > 0) {
        int need = utf8_seq_len(s->carry[0]) - s->carry_len;
        if(need > 0 && need <= (int) len) {
            unsigned char buf[4];
            size_t n;

            memcpy(buf, s->carry, (size_t) s->carry_len);
            memcpy(buf + s->carry_len, data, (size_t) need);
            n = (size_t)(s->carry_len + need);
            s->carry_len = 0;
            if(utf8_valid(buf, n)) {
                count++;
                s->chars_seen++;
            } else {
                /* Invalid lead byte: count it as one character. */
                count++;
                s->chars_seen++;
            }
            i = (size_t) need;
        } else if(need > 0) {
            size_t take = len;
            size_t space = (size_t)(4 - s->carry_len);
            if(take > space)
                take = space;
            memcpy(s->carry + s->carry_len, data, take);
            s->carry_len += (int) take;
            return;
        } else {
            /* Unexpected: carry is already complete; drop it. */
            s->carry_len = 0;
        }
    }

    while(i < len) {
        unsigned char c = (unsigned char) data[i];
        int seq = utf8_seq_len(c);

        if(seq == 1) {
            count++;
            s->chars_seen++;
            if(c == '\n')
                s->lines_completed++;
            i++;
            continue;
        }

        if((int) len - (int) i < seq) {
            s->carry_len = (int)(len - i);
            memcpy(s->carry, data + i, s->carry_len);
            i = len;
            break;
        }

        if(utf8_valid((const unsigned char*) data + i, (size_t) seq)) {
            count++;
            s->chars_seen++;
            i += (size_t) seq;
        } else {
            count++;
            s->chars_seen++;
            i++;
        }
    }

    if(count > 0) {
        size_t idx;
        size_t oldest;
        size_t newest;
        double span;
        long long chars;

        /* Append a (time, cumulative chars) sample. */
        if(s->sample_count == TW_RATE_SAMPLE_COUNT) {
            s->sample_head = (s->sample_head + 1) % TW_RATE_SAMPLE_COUNT;
            s->sample_count--;
        }
        idx = (size_t)((s->sample_head + s->sample_count)
                       % TW_RATE_SAMPLE_COUNT);
        s->sample_time[idx] = now;
        s->sample_chars[idx] = s->chars_seen;
        s->sample_count++;

        /* The rate is chars per second across the whole covered span, not
         * per-read gaps. The span starts at the first byte (seed sample),
         * so a slow first-token wait counts as slow, and a short burst
         * caused by our own backpressure (a big read right after a small
         * one) cannot look like a fast stream. */
        if(s->sample_count >= 2) {
            oldest = (size_t) s->sample_head;
            newest = (size_t)((s->sample_head + s->sample_count - 1)
                              % TW_RATE_SAMPLE_COUNT);
            span = s->sample_time[newest] - s->sample_time[oldest];
            chars = s->sample_chars[newest] - s->sample_chars[oldest];
            if(span >= 0.05 && chars >= 8) {
                double instant = (double) chars / span;
                if(instant > s->rate_cap * 2.0)
                    instant = s->rate_cap * 2.0;
                if(s->rate_measured)
                    s->rate += s->alpha * (instant - s->rate);
                else
                    s->rate = instant;  /* replace the fallback CPS */
                if(s->rate > s->rate_cap)
                    s->rate = s->rate_cap;
                s->rate_measured = 1;
            }
        }
    }
}

void
tw_input_state_mark_fast(tw_input_state_t* s)
{
    s->rate = s->rate_cap;
}


/***********************************
 ***  Pacer
 ***********************************/

static int
queue_push(tw_pacer_t* p, tw_line_t* line)
{
    if(p->queue_head + p->queue_count == p->queue_cap) {
        size_t new_cap = p->queue_cap > 0 ? p->queue_cap * 2 : 16;
        tw_line_t** new_queue =
            (tw_line_t**) realloc(p->queue, new_cap * sizeof(tw_line_t*));
        if(new_queue == NULL)
            return -1;
        if(p->queue_head > 0) {
            memmove(new_queue, new_queue + p->queue_head,
                    p->queue_count * sizeof(tw_line_t*));
            p->queue_head = 0;
        }
        p->queue = new_queue;
        p->queue_cap = new_cap;
    }
    p->queue[p->queue_head + p->queue_count] = line;
    p->queue_count++;
    return 0;
}

static tw_line_t*
queue_pop(tw_pacer_t* p)
{
    tw_line_t* line = p->queue[p->queue_head];
    p->queue_head++;
    p->queue_count--;
    if(p->queue_count == 0)
        p->queue_head = 0;
    return line;
}

/* Append an empty line's bytes to a trailing payload, coalescing runs of
 * identical byte patterns so memory stays bounded. */
static int
trailing_append(tw_trailing_t* t, const char* data, size_t len)
{
    tw_empty_run_t* run;
    char* copy;

    if(len == 0)
        return 0;
    if(t->count > 0) {
        run = &t->runs[t->count - 1];
        if(run->len == len && memcmp(run->data, data, len) == 0) {
            run->count++;
            return 0;
        }
    }
    if(t->count == t->cap) {
        size_t new_cap = t->cap > 0 ? t->cap * 2 : 4;
        tw_empty_run_t* new_runs = (tw_empty_run_t*)
            realloc(t->runs, new_cap * sizeof(tw_empty_run_t));
        if(new_runs == NULL)
            return -1;
        t->runs = new_runs;
        t->cap = new_cap;
    }
    copy = (char*) malloc(len);
    if(copy == NULL)
        return -1;
    memcpy(copy, data, len);
    run = &t->runs[t->count];
    run->data = copy;
    run->len = len;
    run->count = 1;
    t->count++;
    return 0;
}

/* Write a trailing payload through the pacer writer. Empty lines never
 * consume a pacing tick, so this is called right after the owning line's
 * last unit. */
static void
trailing_write(tw_pacer_t* p, const tw_trailing_t* t)
{
    size_t i;
    for(i = 0; i < t->count; i++) {
        const tw_empty_run_t* run = &t->runs[i];
        long long k;
        for(k = 0; k < run->count; k++) {
            tw_unit_t u;
            memset(&u, 0, sizeof(u));
            u.text = run->data;
            u.text_len = run->len;
            p->writer(&u, p->writer_ud);
        }
    }
}

static void
trailing_free(tw_trailing_t* t)
{
    size_t i;
    for(i = 0; i < t->count; i++)
        free(t->runs[i].data);
    free(t->runs);
    memset(t, 0, sizeof(*t));
}


void
tw_line_free(tw_line_t* line)
{
    if(line == NULL)
        return;
    trailing_free(&line->trailing);
    free(line->data);
    tw_units_free(line->units);
    free(line);
}

int
tw_pacer_init(tw_pacer_t* p, tw_writer_fn writer, void* writer_ud,
              double cps, int buffer_size, double max_delay)
{
    memset(p, 0, sizeof(*p));
    p->writer = writer;
    p->writer_ud = writer_ud;
    p->cps = cps;
    p->buffer_size = buffer_size > 0 ? buffer_size : 1;
    p->max_delay = max_delay;
    p->next_tick = -1.0;
    p->need_first_char = 1;
    return 0;
}

void
tw_pacer_destroy(tw_pacer_t* p)
{
    size_t i;
    for(i = 0; i < p->queue_count; i++)
        tw_line_free(p->queue[p->queue_head + i]);
    free(p->queue);
    memset(p, 0, sizeof(*p));
}

static void
stats_observe_arrival(tw_stats_t* st, const tw_line_t* line)
{
    st->input_chars += line->visible_count;
    if(!st->have_first_arrival) {
        st->first_arrival = line->arrived_at;
        st->have_first_arrival = 1;
    }
    st->last_arrival = line->arrived_at;
}

static void
stats_observe_output(tw_stats_t* st, long long n, double now)
{
    st->output_chars += n;
    if(!st->have_first_output) {
        st->first_output = now;
        st->have_first_output = 1;
    }
    st->last_output = now;
}

static void
stats_observe_line_complete(tw_stats_t* st, const tw_line_t* line, double now)
{
    double lag = now - line->arrived_at;
    if(lag > st->max_lag)
        st->max_lag = lag;
}

int
tw_pacer_add_line(tw_pacer_t* p, tw_line_t* line)
{
    if(line->visible_count == 0) {
        int rc = 0;
        /* Empty line: never queued as a full line object. If nothing is
         * queued it prints immediately; otherwise it is coalesced into the
         * trailing payload of the newest visible line. */
        if(line->data_len > 0) {
            if(p->queue_count == 0) {
                tw_unit_t u;
                memset(&u, 0, sizeof(u));
                u.text = line->data;
                u.text_len = line->data_len;
                p->writer(&u, p->writer_ud);
            } else {
                tw_line_t* last =
                    p->queue[p->queue_head + p->queue_count - 1];
                if(trailing_append(&last->trailing,
                                   line->data, line->data_len) != 0)
                    rc = -1;
            }
        }
        if(rc == 0)
            tw_line_free(line);   /* success: the pacer consumes the line */
        return rc;
    }

    if(queue_push(p, line) != 0)
        return -1;
    p->pending_visible += line->visible_count;
    if(p->pending_visible > p->stats.max_backlog)
        p->stats.max_backlog = p->pending_visible;
    stats_observe_arrival(&p->stats, line);
    return 0;
}

void
tw_pacer_maybe_burst(tw_pacer_t* p, int batch_size, double now)
{
    size_t anchor = (size_t) -1;
    size_t keep;
    int burst_chars = 0;
    int flushed = 0;
    size_t i;

    if(p->queue_count < 2)
        return;

    /* Find the newest line that still has visible units. */
    for(i = p->queue_count; i > 0; i--) {
        if(p->queue[p->queue_head + i - 1]->remaining_visible > 0) {
            anchor = i - 1;
            break;
        }
    }
    if(anchor == (size_t) -1)
        return;

    if(batch_size <= 1) {
        if(p->pending_visible < p->buffer_size)
            return;
        if(p->queue[p->queue_head + p->queue_count - 1]
               ->remaining_visible == 0)
            return;
    }

    keep = p->queue_count - anchor;
    while(p->queue_count > keep) {
        tw_line_t* old = queue_pop(p);
        size_t j;
        flushed = 1;
        burst_chars += old->remaining_visible;
        p->pending_visible -= old->remaining_visible;
        for(j = old->unit_pos; j < old->unit_count; j++)
            p->writer(&old->units[j], p->writer_ud);
        trailing_write(p, &old->trailing);
        stats_observe_line_complete(&p->stats, old, now);
        tw_line_free(old);
    }

    if(!flushed || burst_chars == 0)
        return;
    p->stats.burst_chars += burst_chars;
    p->stats.burst_count++;
    stats_observe_output(&p->stats, burst_chars, now);
    /* Pacing resumes from the newest line; its first unit is immediate. */
    p->need_first_char = 1;
}

static int
flush_front_timeout(tw_pacer_t* p, double now)
{
    tw_line_t* front;
    int burst_chars;
    size_t j;

    if(p->queue_count == 0)
        return 0;
    front = p->queue[p->queue_head];
    if(front->remaining_visible <= 0)
        return 0;
    if(now - front->arrived_at < p->max_delay)
        return 0;

    /* Hard delay cap: burst the front line's remaining units. */
    burst_chars = front->remaining_visible;
    p->pending_visible -= burst_chars;
    for(j = front->unit_pos; j < front->unit_count; j++)
        p->writer(&front->units[j], p->writer_ud);
    trailing_write(p, &front->trailing);
    stats_observe_line_complete(&p->stats, front, now);
    tw_line_free(queue_pop(p));

    p->stats.burst_chars += burst_chars;
    p->stats.burst_count++;
    p->stats.timeout_burst_chars += burst_chars;
    p->stats.timeout_burst_count++;
    stats_observe_output(&p->stats, burst_chars, now);
    p->need_first_char = 1;
    return 1;
}

int
tw_pacer_step(tw_pacer_t* p, double now)
{
    tw_line_t* line;
    tw_unit_t* unit;
    int visible;
    int pace;

    if(flush_front_timeout(p, now))
        return 1;
    if(p->queue_count == 0) {
        p->need_first_char = 1;
        return 0;
    }

    line = p->queue[p->queue_head];
    unit = &line->units[line->unit_pos];
    visible = unit->visible;
    pace = unit->pace;
    if(unit->pace && !p->need_first_char && p->next_tick >= 0.0
       && now < p->next_tick)
        return 0;

    p->writer(unit, p->writer_ud);
    line->unit_pos++;
    if(visible) {
        p->pending_visible--;
        line->remaining_visible--;
    }
    if(line->unit_pos == line->unit_count) {
        trailing_write(p, &line->trailing);
        stats_observe_line_complete(&p->stats, line, now);
        tw_line_free(queue_pop(p));
    }

    stats_observe_output(&p->stats, visible ? 1 : 0, now);
    if(pace) {
        p->next_tick = now + 1.0 / (p->cps > 0.001 ? p->cps : 0.001);
        p->need_first_char = 0;
    } else {
        /* Non-pacing units (newlines, controls) do not shift the tick. */
        p->need_first_char = 1;
    }
    return 1;
}

double
tw_pacer_idle_interval(tw_pacer_t* p, double now)
{
    tw_line_t* line;
    tw_unit_t* unit;

    if(p->queue_count == 0)
        return -1.0;
    line = p->queue[p->queue_head];
    if(now - line->arrived_at >= p->max_delay)
        return 0.0;
    unit = &line->units[line->unit_pos];
    if(!unit->pace || p->need_first_char || p->next_tick < 0.0)
        return 0.0;
    return p->next_tick - now > 0.0 ? p->next_tick - now : 0.0;
}
