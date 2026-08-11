/*
 * typewriter-test.c - deterministic tests for the CLI typewriter core.
 *
 * The pacer is driven with an explicit fake clock and a recording writer,
 * so no real time is spent sleeping.
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

#include "../mdflow/typewriter.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int failures = 0;
static int checks = 0;

#define CHECK(cond) \
    do { \
        checks++; \
        if(!(cond)) { \
            printf("FAIL %s:%d: %s\n", __FILE__, __LINE__, #cond); \
            failures++; \
        } \
    } while(0)


#define MAX_PARTS 8192

typedef struct {
    char esc[512];
    size_t esc_len;
    char text[256];
    size_t text_len;
    int visible;
    int pace;
} part_t;

static part_t parts[MAX_PARTS];
static int part_count;

static void
fake_writer(const tw_unit_t* unit, void* userdata)
{
    part_t* p;
    (void) userdata;

    if(part_count >= MAX_PARTS)
        return;
    p = &parts[part_count];
    p->esc_len = unit->esc_len < sizeof(p->esc)
                 ? unit->esc_len : sizeof(p->esc) - 1;
    if(p->esc_len > 0)
        memcpy(p->esc, unit->esc, p->esc_len);
    p->text_len = unit->text_len < sizeof(p->text)
                  ? unit->text_len : sizeof(p->text) - 1;
    if(p->text_len > 0)
        memcpy(p->text, unit->text, p->text_len);
    p->visible = unit->visible;
    p->pace = unit->pace;
    part_count++;
}

static void
expect_part(int i, const char* esc, const char* text)
{
    size_t esc_len = esc != NULL ? strlen(esc) : 0;
    size_t text_len = text != NULL ? strlen(text) : 0;

    CHECK(i < part_count);
    if(i >= part_count)
        return;
    CHECK(parts[i].esc_len == esc_len);
    CHECK(parts[i].text_len == text_len);
    if(esc_len > 0)
        CHECK(memcmp(parts[i].esc, esc, esc_len) == 0);
    if(text_len > 0)
        CHECK(memcmp(parts[i].text, text, text_len) == 0);
}


static tw_line_t*
make_line(const char* raw, double t)
{
    tw_line_t* line;
    tw_unit_t* units = NULL;
    size_t count = 0;
    size_t i;
    size_t len = strlen(raw);

    if(tw_tokenize_line(raw, len, &units, &count) != 0)
        return NULL;
    line = (tw_line_t*) calloc(1, sizeof(tw_line_t));
    if(line == NULL) {
        tw_units_free(units);
        return NULL;
    }
    line->data = (char*) malloc(len + 1);
    if(line->data == NULL) {
        tw_units_free(units);
        free(line);
        return NULL;
    }
    memcpy(line->data, raw, len + 1);
    line->data_len = len;
    line->units = units;
    line->unit_count = count;
    line->unit_pos = 0;
    line->arrived_at = t;
    for(i = 0; i < count; i++) {
        if(units[i].visible) {
            line->visible_count++;
            line->width += units[i].width;
        }
    }
    line->remaining_visible = line->visible_count;
    return line;
}


static void
test_tokenizer(void)
{
    tw_unit_t* units = NULL;
    size_t count = 0;
    size_t i;
    const char* raw;

    /* ANSI prefix is attached to the first visible character. */
    raw = "\x1b[1;34mHello\x1b[0m";
    CHECK(tw_tokenize_line(raw, strlen(raw), &units, &count) == 0);
    CHECK(count == 6);
    if(count >= 6) {
        CHECK(units[0].text_len == 1 && units[0].text[0] == 'H');
        CHECK(units[0].esc_len == strlen("\x1b[1;34m"));
        CHECK(memcmp(units[0].esc, "\x1b[1;34m",
                     strlen("\x1b[1;34m")) == 0);
        CHECK(units[4].visible == 1 && units[4].text[0] == 'o');
        CHECK(units[5].text_len == 0);
        CHECK(units[5].esc_len == strlen("\x1b[0m"));
        CHECK(memcmp(units[5].esc, "\x1b[0m", strlen("\x1b[0m")) == 0);
        CHECK(units[5].visible == 0 && units[5].pace == 0);
    }
    tw_units_free(units);
    units = NULL;
    count = 0;

    /* CJK, emoji ZWJ, flags and variation selectors group into one unit. */
    raw = "a\xe4\xbd\xa0\xe5\xa5\xbd"
          "\xf0\x9f\x91\xa8\xe2\x80\x8d\xf0\x9f\x91\xa9"
          "\xe2\x80\x8d\xf0\x9f\x91\xa7\xe2\x80\x8d\xf0\x9f\x91\xa6"
          "\xf0\x9f\x87\xa8\xf0\x9f\x87\xb3"
          "\xe2\x9d\xa4\xef\xb8\x8f";
    CHECK(tw_tokenize_line(raw, strlen(raw), &units, &count) == 0);
    CHECK(count == 6);
    if(count == 6) {
        const char* expect[] = {
            "a",
            "\xe4\xbd\xa0",
            "\xe5\xa5\xbd",
            ("\xf0\x9f\x91\xa8\xe2\x80\x8d\xf0\x9f\x91\xa9"
             "\xe2\x80\x8d\xf0\x9f\x91\xa7\xe2\x80\x8d\xf0\x9f\x91\xa6"),
            "\xf0\x9f\x87\xa8\xf0\x9f\x87\xb3",
            "\xe2\x9d\xa4\xef\xb8\x8f"
        };
        int expect_width[] = { 1, 2, 2, 2, 2, 2 };
        for(i = 0; i < count; i++) {
            CHECK(units[i].text_len == strlen(expect[i]));
            CHECK(memcmp(units[i].text, expect[i], strlen(expect[i])) == 0);
            CHECK(units[i].width == expect_width[i]);
            CHECK(units[i].visible == 1 && units[i].pace == 1);
        }
    }
    tw_units_free(units);
    units = NULL;
    count = 0;

    /* Text-presentation symbols stay narrow; FE0F and multi-codepoint
     * emoji promote the cluster to 2 cells. */
    raw = "\xe2\x98\x80 "                    /* ☀ */
          "\xe2\x98\x80\xef\xb8\x8f "        /* ☀️ */
          "\xf0\x9f\x91\x8d\xf0\x9f\x8f\xbd "  /* 👍🏽 */
          "1\xef\xb8\x8f\xe2\x83\xa3";       /* 1️⃣ */
    CHECK(tw_tokenize_line(raw, strlen(raw), &units, &count) == 0);
    CHECK(count == 7);
    if(count == 7) {
        int expect_width[] = { 1, 1, 2, 1, 2, 1, 2 };
        for(i = 0; i < count; i++) {
            CHECK(units[i].width == expect_width[i]);
            CHECK(units[i].visible == 1 && units[i].pace == 1);
        }
    }
    tw_units_free(units);
    units = NULL;
    count = 0;

    /* Combining mark stays with its base. */
    raw = "e\xcc\x81";
    CHECK(tw_tokenize_line(raw, strlen(raw), &units, &count) == 0);
    CHECK(count == 1);
    if(count == 1)
        CHECK(units[0].text_len == strlen(raw)
              && memcmp(units[0].text, raw, strlen(raw)) == 0);
    tw_units_free(units);
    units = NULL;
    count = 0;

    /* Newline is not paced. */
    raw = "hi\n";
    CHECK(tw_tokenize_line(raw, strlen(raw), &units, &count) == 0);
    CHECK(count == 3);
    if(count == 3) {
        CHECK(units[2].text_len == 1 && units[2].text[0] == '\n');
        CHECK(units[2].visible == 0 && units[2].pace == 0);
    }
    tw_units_free(units);
    units = NULL;
    count = 0;

    /* Trailing ANSI is emitted as a zero-width, non-pacing unit. */
    raw = "x\x1b[0m";
    CHECK(tw_tokenize_line(raw, strlen(raw), &units, &count) == 0);
    CHECK(count == 2);
    if(count == 2) {
        CHECK(units[1].text_len == 0);
        CHECK(units[1].esc_len == strlen("\x1b[0m"));
        CHECK(units[1].visible == 0 && units[1].pace == 0);
    }
    tw_units_free(units);
}


static void
test_pacer(void)
{
    tw_pacer_t p;
    tw_line_t* line;
    double t;
    int i;

    /* The first visible unit prints immediately. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 120, 1.0) == 0);
    line = make_line("Hello\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    CHECK(tw_pacer_step(&p, t) == 1);
    expect_part(0, "", "H");
    CHECK(tw_pacer_step(&p, t) == 0);  /* too early for the next character */
    tw_pacer_destroy(&p);

    /* Pacing interval: one tick per visible unit. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 120, 1.0) == 0);
    line = make_line("ab\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    CHECK(tw_pacer_step(&p, t) == 1);
    t += 1.0 / 60.0;
    CHECK(tw_pacer_step(&p, t) == 1);
    expect_part(0, "", "a");
    expect_part(1, "", "b");
    tw_pacer_destroy(&p);

    /* Burst keeps only the newest line once the backlog threshold is hit. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 10, 1.0) == 0);
    line = make_line("1234567890\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("abc\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    tw_pacer_maybe_burst(&p, 1, t);
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->visible_count == 3);
        CHECK(p.stats.burst_count == 1);
        CHECK(p.stats.burst_chars == 10);
    }
    expect_part(0, "", "1");
    tw_pacer_destroy(&p);

    /* A multi-line batch bursts even below the threshold. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 100, 1.0) == 0);
    line = make_line("123\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("abc\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    tw_pacer_maybe_burst(&p, 2, t);
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1)
        CHECK(p.queue[p.queue_head]->visible_count == 3);
    CHECK(p.stats.burst_count == 1);
    tw_pacer_destroy(&p);

    /* Trailing empty lines do not steal the newest-visible-line slot. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 100, 1.0) == 0);
    line = make_line("123\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("abc\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    tw_pacer_maybe_burst(&p, 3, t);
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->visible_count == 3);
        CHECK(p.queue[p.queue_head]->trailing.count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].len == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].data[0] == '\n');
    }
    CHECK(p.stats.burst_count == 1);
    CHECK(p.stats.burst_chars == 3);
    tw_pacer_destroy(&p);

    /* An empty-only batch does not burst older content. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 100, 1.0) == 0);
    line = make_line("123\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    tw_pacer_maybe_burst(&p, 2, t);
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->trailing.count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].count == 2);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].len == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].data[0] == '\n');
    }
    CHECK(p.stats.burst_count == 0);
    tw_pacer_destroy(&p);

    /* An empty line alone never forces a threshold burst, but a later
     * content line does. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 10, 1.0) == 0);
    line = make_line("1234567890\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    tw_pacer_maybe_burst(&p, 1, t);
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->trailing.count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].count == 1);
    }
    CHECK(p.stats.burst_count == 0);
    line = make_line("abc\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    tw_pacer_maybe_burst(&p, 1, t);
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1)
        CHECK(p.queue[p.queue_head]->visible_count == 3);
    CHECK(p.stats.burst_count == 1);
    tw_pacer_destroy(&p);

    /* Trailing empty lines are coalesced: thousands of blank lines never
     * grow the queue, and they print immediately after the visible line. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 1000000000.0,
                        100000, 1000000000.0) == 0);
    line = make_line("abc\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    for(i = 0; i < 1000; i++) {
        line = make_line("\n", t);
        CHECK(line != NULL);
        CHECK(tw_pacer_add_line(&p, line) == 0);
    }
    CHECK(p.queue_count == 1);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->trailing.count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].count == 1000);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].len == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].data[0] == '\n');
    }
    CHECK(tw_pacer_step(&p, t) == 1);   /* 'a' */
    t += 1e-6;
    CHECK(tw_pacer_step(&p, t) == 1);   /* 'b' */
    t += 1e-6;
    CHECK(tw_pacer_step(&p, t) == 1);   /* 'c' */
    CHECK(tw_pacer_step(&p, t) == 1);   /* '\n' + 1000 coalesced newlines */
    CHECK(part_count == 1004);
    CHECK(parts[3].text_len == 1 && parts[3].text[0] == '\n');
    CHECK(parts[1003].text_len == 1 && parts[1003].text[0] == '\n');
    CHECK(p.queue_count == 0);
    tw_pacer_destroy(&p);

    /* Distinct empty-line patterns become separate runs, in order. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 1000000000.0,
                        100000, 1000000000.0) == 0);
    line = make_line("x\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\x1b[0m\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    line = make_line("\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->trailing.count == 3);
        CHECK(p.queue[p.queue_head]->trailing.runs[0].count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[1].count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[2].count == 1);
        CHECK(p.queue[p.queue_head]->trailing.runs[1].len == 5);
    }
    tw_pacer_destroy(&p);

    /* Burst stats count only the remaining units of a partially paced line. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 10, 1.0) == 0);
    line = make_line("1234567890\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    CHECK(tw_pacer_step(&p, t) == 1);
    t += 1.0 / 60.0;
    CHECK(tw_pacer_step(&p, t) == 1);
    t += 1.0 / 60.0;
    CHECK(tw_pacer_step(&p, t) == 1);
    CHECK(p.pending_visible == 7);
    line = make_line("abcde\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    CHECK(p.pending_visible == 12);
    tw_pacer_maybe_burst(&p, 1, t);
    CHECK(p.stats.burst_chars == 7);
    CHECK(p.pending_visible == 5);
    CHECK(p.stats.output_chars == 10);
    if(p.queue_count == 1) {
        CHECK(p.queue[p.queue_head]->visible_count == 5);
        CHECK(p.queue[p.queue_head]->remaining_visible == 5);
    }
    tw_pacer_destroy(&p);

    /* An empty pacer queue waits and has no idle interval. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 120, 1.0) == 0);
    CHECK(tw_pacer_step(&p, t) == 0);
    CHECK(tw_pacer_idle_interval(&p, t) == -1.0);
    tw_pacer_destroy(&p);

    /* Newlines do not consume a pacing tick. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 120, 1.0) == 0);
    line = make_line("a\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    CHECK(tw_pacer_step(&p, t) == 1);
    t += 1.0 / 60.0;
    CHECK(tw_pacer_step(&p, t) == 1);   /* newline unit */
    CHECK(p.queue_count == 0);
    tw_pacer_destroy(&p);

    /* Queue growth after the head has advanced (slow streaming: lines are
     * paced one at a time, so head > 0 while older slots stay empty). */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 1000000000.0,
                        1000000, 1000000000.0) == 0);
    for(i = 0; i < 16; i++) {
        line = make_line("a\n", t);
        CHECK(line != NULL);
        CHECK(tw_pacer_add_line(&p, line) == 0);
    }
    for(i = 0; i < 15; i++) {
        CHECK(tw_pacer_step(&p, t) == 1);   /* 'a' */
        CHECK(tw_pacer_step(&p, t) == 1);   /* '\n' pops the line */
        t += 1e-6;
    }
    CHECK(p.queue_head == 15);
    CHECK(p.queue_count == 1);
    line = make_line("b\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    CHECK(p.queue_count == 2);
    tw_pacer_destroy(&p);

    /* The max-delay cap bursts an overdue line. */
    part_count = 0;
    t = 0.0;
    CHECK(tw_pacer_init(&p, fake_writer, NULL, 60.0, 120, 1.0) == 0);
    line = make_line("abcdefghij\n", t);
    CHECK(line != NULL);
    CHECK(tw_pacer_add_line(&p, line) == 0);
    t = 1.1;
    CHECK(tw_pacer_idle_interval(&p, t) == 0.0);
    CHECK(tw_pacer_step(&p, t) == 1);
    CHECK(p.queue_count == 0);
    CHECK(p.stats.timeout_burst_count == 1);
    CHECK(p.stats.timeout_burst_chars == 10);
    CHECK(p.stats.max_lag >= 1.0);
    tw_pacer_destroy(&p);
}


static void
test_input_state(void)
{
    tw_input_state_t s;
    double now;
    int i;

    /* Character and line counts. */
    tw_input_state_init(&s, 60.0, 0.5, 1000000.0);
    tw_input_state_observe_chars(&s, "ab", 2, 0.0);
    CHECK(s.chars_seen == 2);
    tw_input_state_observe_chars(&s, "c\n", 2, 0.2);
    CHECK(s.chars_seen == 4);
    CHECK(s.lines_completed == 1);

    /* Rate tracks the chunk interval. */
    tw_input_state_init(&s, 60.0, 0.5, 1000000.0);
    now = 100.0;
    tw_input_state_observe_chars(&s, "a", 1, now);
    for(i = 0; i < 150; i++) {
        now += 1.0 / 120.0;
        tw_input_state_observe_chars(&s, "a", 1, now);
    }
    CHECK(s.rate > 115.0 && s.rate < 125.0);

    /* The rate can exceed the display max so bypass detection works. */
    tw_input_state_init(&s, 60.0, 0.5, 1000000.0);
    now = 100.0;
    tw_input_state_observe_chars(&s, "a", 1, now);
    for(i = 0; i < 600; i++) {
        now += 1.0 / 10000.0;
        tw_input_state_observe_chars(&s, "a", 1, now);
    }
    CHECK(s.rate > 5000.0);

    /* A partial UTF-8 sequence is carried across chunks. */
    tw_input_state_init(&s, 60.0, 0.15, 1000000.0);
    tw_input_state_observe_chars(&s, "\xc3", 1, 1.0);
    CHECK(s.chars_seen == 0);
    tw_input_state_observe_chars(&s, "\xa9", 1, 1.1);
    CHECK(s.chars_seen == 1);

    /* Bypass needs a real arrival interval, not the fallback CPS. */
    tw_input_state_init(&s, 6000.0, 0.15, 1000000.0);
    CHECK(s.rate_measured == 0);
    tw_input_state_observe_chars(&s,
        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        150, 1.0);
    CHECK(s.rate_measured == 0);   /* one chunk: no span yet */
    tw_input_state_observe_chars(&s,
        "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        40, 1.1);
    CHECK(s.rate_measured == 1);
    CHECK(s.rate < 5000.0);        /* fallback CPS replaced by 400/s */

    /* A backpressure-induced burst (a big read right after a small one)
     * must not be mistaken for a fast stream. */
    tw_input_state_init(&s, 60.0, 0.15, 1000000.0);
    tw_input_state_observe_chars(&s,
        "aaaaaaaaaaaaaaaaaaaa", 20, 10.0);
    tw_input_state_observe_chars(&s,
        "bbbbbbbbbbbbbbbbbbbb", 20, 10.0005);
    tw_input_state_observe_chars(&s,
        "cccccccccccccccccccc", 20, 10.001);
    tw_input_state_observe_chars(&s,
        "dddddddddddddddddddd", 20, 10.0015);
    CHECK(s.rate_measured == 0);   /* span below the 50 ms minimum */
    CHECK(s.rate == 60.0);
    tw_input_state_observe_chars(&s,
        "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", 40, 11.0);
    CHECK(s.rate_measured == 1);
    CHECK(s.rate < 5000.0);        /* window average, not the spike */

    /* Sparse token-by-token streams are still measurable: 8 chars over 4
     * seconds classify as slow, with no lower clamp. */
    tw_input_state_init(&s, 60.0, 0.15, 1000000.0);
    tw_input_state_seed_start(&s, 0.0);
    for(i = 0; i < 8; i++) {
        tw_input_state_observe_chars(&s, "a", 1, 0.5 + 0.5 * (double) i);
    }
    CHECK(s.rate_measured == 1);
    CHECK(s.rate > 1.5 && s.rate < 2.5);   /* ~2 chars/sec, no 20 clamp */

    /* A long first-token wait counts as slow: 8 chars starting 10 seconds
     * after the seed sample classify as typewriter, not bypass. */
    tw_input_state_init(&s, 60.0, 0.15, 1000000.0);
    tw_input_state_seed_start(&s, 0.0);
    for(i = 0; i < 8; i++) {
        tw_input_state_observe_chars(&s, "b", 1, 10.0 + 0.1 * (double) i);
    }
    CHECK(s.rate_measured == 1);
    CHECK(s.rate < 5000.0);
    CHECK(s.rate > 0.0 && s.rate < 1.0);   /* ~0.75 chars/sec */
}


int
main(void)
{
    test_tokenizer();
    test_pacer();
    test_input_state();

    printf("%d checks, %d failures\n", checks, failures);
    return failures == 0 ? 0 : 1;
}
