/*
 * typewriter.h - paced terminal output layer for the mdflow CLI
 *
 * The pacer is deliberately free of terminal I/O and real time
 * dependencies: callers inject a writer and pass the current monotonic
 * time, which makes the burst and timing rules testable.
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

#ifndef MDFLOW_TYPEWRITER_H
#define MDFLOW_TYPEWRITER_H

#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Sliding-window rate samples. 1024 entries keep the measurement span from
 * the first byte even for sparse streams and fast read bursts. */
#define TW_RATE_SAMPLE_COUNT 1024

/* Monotonic clock in seconds. */
double tw_monotonic(void);


/***********************************
 ***  ANSI / visible-unit tokenizer
 ***********************************/

/* One output step: an optional ANSI prefix plus one visible/control token.
 * text and esc point into the line buffer passed to tw_tokenize_line();
 * the buffer must outlive the units. */
typedef struct tw_unit_tag {
    const char* text;       /* visible/control bytes; may be empty */
    size_t      text_len;
    const char* esc;        /* ANSI bytes preceding this unit */
    size_t      esc_len;
    int         width;      /* terminal display width (report only) */
    int         visible;
    int         pace;
} tw_unit_t;

/* Tokenize one complete rendered line (trailing newline included).
 * Returns 0 on success and sets *out_units / *out_count, or -1 on
 * allocation failure. Caller frees the array with tw_units_free(). */
int tw_tokenize_line(const char* text, size_t len,
                     tw_unit_t** out_units, size_t* out_count);
void tw_units_free(tw_unit_t* units);


/***********************************
 ***  Raw-input metadata
 ***********************************/

/* Upstream rate estimator fed by the stdin read loop. It is updated once
 * per chunk with the chunk's code-point count. */
typedef struct tw_input_state_tag {
    double alpha;
    double rate_cap;
    double rate;
    long long chars_seen;
    long long lines_completed;
    int rate_measured;          /* sliding-window rate has been computed */
    double sample_time[TW_RATE_SAMPLE_COUNT];
    long long sample_chars[TW_RATE_SAMPLE_COUNT];
    int sample_head;
    int sample_count;
    unsigned char carry[3];     /* partial UTF-8 sequence across reads */
    int carry_len;
} tw_input_state_t;

void tw_input_state_init(tw_input_state_t* s, double initial_rate,
                         double alpha, double rate_cap);

/* Seed the rate window with a (start time, 0 chars) sample so the first
 * measurement includes the wait for the first byte (AI first-token
 * latency counts as slow, not as a burst). */
void tw_input_state_seed_start(tw_input_state_t* s, double now);

/* Observe one stdin chunk: counts UTF-8 code points and line endings.
 * A trailing partial UTF-8 sequence is carried to the next call. */
void tw_input_state_observe_chars(tw_input_state_t* s,
                                  const char* data, size_t len, double now);

void tw_input_state_mark_fast(tw_input_state_t* s);


/***********************************
 ***  Pacer
 ***********************************/

/* One complete rendered line queued for display. Owns its raw bytes and
 * token units. Empty lines that arrive after it are stored compactly in
 * `trailing` instead of as separate queue entries. */

/* A run of identical empty-line byte patterns (usually just "\n"). */
typedef struct tw_empty_run_tag {
    char* data;
    size_t len;
    long long count;
} tw_empty_run_t;

typedef struct {
    tw_empty_run_t* runs;
    size_t count;
    size_t cap;
} tw_trailing_t;

typedef struct tw_line_tag {
    char*       data;           /* owned copy of the rendered line */
    size_t      data_len;
    tw_unit_t*  units;
    size_t      unit_count;
    size_t      unit_pos;       /* next unit to emit */
    int         visible_count;
    int         remaining_visible;
    int         width;
    double      arrived_at;
    tw_trailing_t trailing;     /* coalesced empty lines after this line */
} tw_line_t;

typedef void (*tw_writer_fn)(const tw_unit_t* unit, void* userdata);

typedef struct tw_stats_tag {
    long long input_chars;
    long long output_chars;
    long long burst_chars;
    long long burst_count;
    long long timeout_burst_count;
    long long timeout_burst_chars;
    long long max_backlog;
    double max_lag;
    double first_arrival;
    int have_first_arrival;
    double last_arrival;
    double first_output;
    int have_first_output;
    double last_output;
} tw_stats_t;

typedef struct tw_pacer_tag {
    tw_writer_fn writer;
    void* writer_ud;
    double cps;
    int buffer_size;
    double max_delay;
    tw_line_t** queue;
    size_t queue_head;
    size_t queue_count;
    size_t queue_cap;
    long long pending_visible;
    double next_tick;           /* < 0 = no tick scheduled */
    int need_first_char;
    tw_stats_t stats;
} tw_pacer_t;

int tw_pacer_init(tw_pacer_t* p, tw_writer_fn writer, void* writer_ud,
                  double cps, int buffer_size, double max_delay);
void tw_pacer_destroy(tw_pacer_t* p);
void tw_line_free(tw_line_t* line);

/* Pacer takes ownership of line on success. */
int tw_pacer_add_line(tw_pacer_t* p, tw_line_t* line);
void tw_pacer_maybe_burst(tw_pacer_t* p, int batch_size, double now);

/* Emit one unit if it is time. Returns 1 if a unit was emitted. */
int tw_pacer_step(tw_pacer_t* p, double now);

/* Seconds until the next paced unit may be emitted; 0 = immediate;
 * -1 = nothing queued. */
double tw_pacer_idle_interval(tw_pacer_t* p, double now);

#ifdef __cplusplus
}  /* extern "C" { */
#endif

#endif  /* MDFLOW_TYPEWRITER_H */
