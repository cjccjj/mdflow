/*
 * mdflow — streaming Markdown-to-terminal renderer
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

#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/stat.h>
#include <pthread.h>

#if defined(__linux__) || defined(__APPLE__)
#include <sys/ioctl.h>
#endif

#include "mdflow.h"
#include "typewriter.h"

#define CHUNK_SIZE 65536

/* Bounded rendered-line queue between the reader thread and the pacer.
 * The reader blocks when the queue is full, so the pacer can never fall
 * arbitrarily far behind (memory stays flat even for fast local input). */
#define TYPEWRITER_LINE_QUEUE_CAP 512

/* Maximum rendered lines processed per pacer-loop iteration. Keeps the
 * pacer batch bounded so burst and bypass decisions stay timely even when
 * the reader is far ahead. */
#define TYPEWRITER_DRAIN_BATCH 64

/* Fixed typewriter behavior (no user settings): pacing is on by default,
 * always adaptive with raw-input metadata, with a hard 1-second delay cap
 * and a hard 5000 chars/sec bypass threshold. */
#define TYPEWRITER_DEFAULT_CPS 60.0
#define TYPEWRITER_BUFFER_SIZE 120
#define TYPEWRITER_MAX_DELAY 1.0
#define TYPEWRITER_MAX_STREAM_CPS 5000.0


/* Partial line carried across read() calls. */
typedef struct {
    char* data;      /* buffered bytes of the incomplete trailing line */
    size_t alloc;    /* allocated size of data */
    size_t len;      /* bytes in use */
} mdflow_feeder_t;


/* Append data to the feeder buffer, growing it as needed. */
static int
feeder_append(mdflow_feeder_t* f, const char* data, size_t len)
{
    if(f->len + len > f->alloc) {
        size_t new_alloc = (f->alloc > 0 ? f->alloc : CHUNK_SIZE);
        char* new_data;

        while(new_alloc < f->len + len)
            new_alloc *= 2;
        new_data = (char*) realloc(f->data, new_alloc);
        if(new_data == NULL)
            return -1;
        f->data = new_data;
        f->alloc = new_alloc;
    }

    memcpy(f->data + f->len, data, len);
    f->len += len;
    return 0;
}


/* Feed a byte range to the parser, splitting only if a single line
 * exceeds the int length limit of mdflow_write(). */
static int
feed_bytes(mdflow_t* mf, const char* data, size_t len)
{
    while(len > 0) {
        int n = (len > (size_t) 0x7FFFFFFF) ? 0x7FFFFFFF : (int) len;
        if(mdflow_write(mf, data, n) != 0)
            return -1;
        data += n;
        len -= (size_t) n;
    }
    return 0;
}


/* Feed Markdown to the parser one complete line at a time.
 *
 * The streaming parser reclaims its input buffer at the end of a feed
 * call only when no block is open. Feeding whole 64 KB read chunks meant
 * compaction almost never ran (block boundaries rarely fall on chunk
 * boundaries), so memory grew roughly one-to-one with input size. Feeding
 * line by line gives the parser a chance to compact after every block
 * close, keeping memory proportional to the largest open block.
 *
 * A line ends at '\n', or at '\r' (with '\r\n' counted as a single
 * ending). A trailing '\r' at the end of a feed is held back in case it
 * is the first half of a '\r\n' pair split across reads; at EOF it is
 * emitted. At EOF the final unterminated line is emitted too. */
static int
feed_line_by_line(mdflow_t* mf, mdflow_feeder_t* f,
                  const char* data, size_t len, int at_eof)
{
    size_t head = 0;
    size_t i;

    if(len > 0) {
        if(feeder_append(f, data, len) != 0)
            return -1;
    }

    i = 0;
    while(i < f->len) {
        if(f->data[i] == '\n') {
            if(feed_bytes(mf, f->data + head, i + 1 - head) != 0)
                return -1;
            head = i + 1;
        } else if(f->data[i] == '\r') {
            if(i + 1 < f->len) {
                int n = (f->data[i + 1] == '\n') ? 2 : 1;
                if(feed_bytes(mf, f->data + head, i + n - head) != 0)
                    return -1;
                head = i + n;
                i += n;
                continue;
            } else if(at_eof) {
                if(feed_bytes(mf, f->data + head, i + 1 - head) != 0)
                    return -1;
                head = i + 1;
            } else {
                /* Trailing '\r' may be the first half of a '\r\n' pair
                 * split across reads. Wait for more input. */
                break;
            }
        }
        i++;
    }

    if(head > 0) {
        memmove(f->data, f->data + head, f->len - head);
        f->len -= head;
    }

    if(at_eof && f->len > 0) {
        if(feed_bytes(mf, f->data, f->len) != 0)
            return -1;
        f->len = 0;
    }

    return 0;
}


static void
usage(void)
{
    fprintf(stderr,
        "mdflow -- streaming Markdown-to-terminal renderer\n"
        "Version %d.%d.%d\n"
        "\n"
        "Usage:  llm \"hello...\" | mdflow   pipe markdown to mdflow\n"
        "        mdflow < file.md          redirect a file to mdflow\n"
        "\n"
        "Options:\n"
        "  -h, --help           Display this help and exit\n"
        "  --typewriter-off     Disable typewriter effect\n"
        "  --osc8-off           Disable OSC 8 hyperlinks and show link URLs\n"
        "\n"
        "Note: mdflow reads only from stdin. It does not accept filenames as\n"
        "arguments. Use a shell pipe or input redirection as shown above.\n",
        MDFLOW_VERSION_MAJOR, MDFLOW_VERSION_MINOR, MDFLOW_VERSION_PATCH);
}


/* Output callback: write bytes to stdout. */
static void
output_cb(const char* str, int size, void* userdata)
{
    (void)userdata;
    fwrite(str, 1, (size_t)size, stdout);
}


/***********************************
 ***  Typewriter mode
 ***********************************/

typedef struct {
    int typewriter_off;
    int report;
    int osc8_off;
} typewriter_opts_t;


typedef pthread_t tw_thread_t;
typedef pthread_mutex_t tw_mutex_t;
typedef pthread_cond_t tw_cond_t;
typedef void* (*tw_thread_fn)(void*);
#define THREAD_RET void*


/* One complete rendered line waiting to be displayed. */
typedef struct {
    char* data;
    size_t len;
} raw_line_t;

/* Bounded FIFO of rendered lines shared between the reader thread
 * (producer) and the pacer loop (consumer). */
typedef struct {
    raw_line_t* items;
    size_t head;
    size_t count;
    size_t cap;
} line_queue_t;

typedef struct {
    line_queue_t q;
    int eof;
    int error;
} shared_state_t;

/* Accumulates output_cb chunks into complete lines. Only touched by the
 * reader thread. */
typedef struct {
    char* data;                 /* partial rendered line */
    size_t len;
    size_t cap;
    shared_state_t* shared;
    tw_mutex_t* lock;
    tw_cond_t* cond;
} line_sink_t;

typedef struct {
    mdflow_t* mf;
    mdflow_feeder_t* feeder;
    tw_input_state_t* input_state;
    line_sink_t* sink;
    shared_state_t* shared;
    tw_mutex_t* lock;
    tw_cond_t* cond;
} reader_args_t;


static int
mutex_init(tw_mutex_t* m)
{
    return pthread_mutex_init(m, NULL);
}

static void
mutex_destroy(tw_mutex_t* m)
{
    pthread_mutex_destroy(m);
}

static void
mutex_lock(tw_mutex_t* m)
{
    pthread_mutex_lock(m);
}

static void
mutex_unlock(tw_mutex_t* m)
{
    pthread_mutex_unlock(m);
}

static int
cond_init(tw_cond_t* c)
{
    return pthread_cond_init(c, NULL);
}

static void
cond_destroy(tw_cond_t* c)
{
    pthread_cond_destroy(c);
}

static void
cond_broadcast(tw_cond_t* c)
{
    pthread_cond_broadcast(c);
}

static int
cond_wait(tw_cond_t* c, tw_mutex_t* m)
{
    return pthread_cond_wait(c, m);
}

static int
cond_timedwait(tw_cond_t* c, tw_mutex_t* m, double seconds)
{
    struct timespec ts;
    int rc;

    clock_gettime(CLOCK_REALTIME, &ts);
    ts.tv_sec += (time_t) seconds;
    ts.tv_nsec += (long)((seconds - (double)(time_t) seconds)
                         * 1000000000.0);
    if(ts.tv_nsec >= 1000000000L) {
        ts.tv_sec++;
        ts.tv_nsec -= 1000000000L;
    }
    rc = pthread_cond_timedwait(c, m, &ts);
    return rc == ETIMEDOUT ? -1 : 0;
}

static int
thread_create(tw_thread_t* t, tw_thread_fn fn, void* arg)
{
    return pthread_create(t, NULL, fn, arg);
}

static int
thread_join(tw_thread_t* t)
{
    return pthread_join(*t, NULL);
}


static int
raw_line_push(line_queue_t* q, const char* data, size_t len)
{
    raw_line_t* slot;
    char* copy;

    if(q->head + q->count == q->cap) {
        size_t new_cap = q->cap > 0 ? q->cap * 2 : 32;
        raw_line_t* new_items =
            (raw_line_t*) realloc(q->items, new_cap * sizeof(raw_line_t));
        if(new_items == NULL)
            return -1;
        if(q->head > 0) {
            memmove(new_items, new_items + q->head,
                    q->count * sizeof(raw_line_t));
            q->head = 0;
        }
        q->items = new_items;
        q->cap = new_cap;
    }

    slot = &q->items[q->head + q->count];
    copy = (char*) malloc(len > 0 ? len : 1);
    if(copy == NULL)
        return -1;
    if(len > 0)
        memcpy(copy, data, len);
    slot->data = copy;
    slot->len = len;
    q->count++;
    return 0;
}

static void
raw_line_pop(line_queue_t* q, raw_line_t* out)
{
    *out = q->items[q->head];
    q->head++;
    q->count--;
    if(q->count == 0)
        q->head = 0;
}

static void
raw_line_free(raw_line_t* r)
{
    free(r->data);
    r->data = NULL;
    r->len = 0;
}

static void
line_queue_destroy(line_queue_t* q)
{
    size_t i;
    for(i = 0; i < q->count; i++)
        free(q->items[q->head + i].data);
    free(q->items);
    memset(q, 0, sizeof(*q));
}


/* Append a chunk to the partial-line buffer; complete lines (ending in
 * '\n') are queued for the pacer. */
static void
sink_append(line_sink_t* s, const char* data, size_t len)
{
    size_t start = 0;
    size_t i;

    if(len == 0)
        return;
    if(s->len + len > s->cap) {
        size_t new_cap = s->cap > 0 ? s->cap : 4096;
        char* new_data;
        while(new_cap < s->len + len)
            new_cap *= 2;
        new_data = (char*) realloc(s->data, new_cap);
        if(new_data == NULL) {
            mutex_lock(s->lock);
            s->shared->error = 1;
            mutex_unlock(s->lock);
            return;
        }
        s->data = new_data;
        s->cap = new_cap;
    }
    memcpy(s->data + s->len, data, len);
    s->len += len;

    for(i = 0; i < s->len; i++) {
        if(s->data[i] == '\n') {
            mutex_lock(s->lock);
            while(s->shared->q.count >= TYPEWRITER_LINE_QUEUE_CAP
                  && !s->shared->error)
                cond_wait(s->cond, s->lock);
            if(raw_line_push(&s->shared->q, s->data + start,
                             i + 1 - start) != 0)
                s->shared->error = 1;
            mutex_unlock(s->lock);
            cond_broadcast(s->cond);
            start = i + 1;
        }
    }
    if(start > 0) {
        memmove(s->data, s->data + start, s->len - start);
        s->len -= start;
    }
}

/* Queue a trailing rendered line that has no newline (EOF partial line). */
static void
sink_flush(line_sink_t* s)
{
    if(s->len > 0) {
        mutex_lock(s->lock);
        while(s->shared->q.count >= TYPEWRITER_LINE_QUEUE_CAP
              && !s->shared->error)
            cond_wait(s->cond, s->lock);
        if(raw_line_push(&s->shared->q, s->data, s->len) != 0)
            s->shared->error = 1;
        mutex_unlock(s->lock);
        cond_broadcast(s->cond);
        s->len = 0;
    }
}


/* Output callback for typewriter mode: assemble rendered lines. */
static void
typewriter_output_cb(const char* str, int size, void* userdata)
{
    line_sink_t* sink = (line_sink_t*) userdata;
    sink_append(sink, str, (size_t) size);
}


/* Pacer writer: write one unit's ANSI prefix and visible/control bytes. */
static void
pacer_writer(const tw_unit_t* unit, void* userdata)
{
    (void) userdata;
    if(unit->esc_len > 0)
        fwrite(unit->esc, 1, unit->esc_len, stdout);
    if(unit->text_len > 0)
        fwrite(unit->text, 1, unit->text_len, stdout);
}


/* Tokenize one rendered line into a pacer line. */
static tw_line_t*
build_line(const char* raw, size_t len, double arrived_at)
{
    tw_line_t* line = (tw_line_t*) calloc(1, sizeof(tw_line_t));
    tw_unit_t* units = NULL;
    size_t count = 0;
    size_t i;

    if(line == NULL)
        return NULL;
    line->data = (char*) malloc(len > 0 ? len : 1);
    if(line->data == NULL) {
        free(line);
        return NULL;
    }
    if(len > 0)
        memcpy(line->data, raw, len);
    line->data_len = len;
    if(tw_tokenize_line(line->data, len, &units, &count) != 0) {
        free(line->data);
        free(line);
        return NULL;
    }
    line->units = units;
    line->unit_count = count;
    line->unit_pos = 0;
    line->arrived_at = arrived_at;
    for(i = 0; i < count; i++) {
        if(units[i].visible) {
            line->visible_count++;
            line->width += units[i].width;
        }
    }
    line->remaining_visible = line->visible_count;
    return line;
}


/* Local-file bypass: stdin redirected from a regular file is not a live
 * stream, so smoothing is disabled from the start. */
static int
stdin_is_regular_file(void)
{
    struct stat st;
    if(fstat(STDIN_FILENO, &st) != 0)
        return 0;
    return S_ISREG(st.st_mode);
}


/* Reader thread: feed stdin into mdflow, observe raw-input metadata, and
 * queue rendered lines for the pacer. */
static THREAD_RET
reader_thread(void* arg)
{
    reader_args_t* a = (reader_args_t*) arg;
    char chunk[CHUNK_SIZE];
    int nread;
    int failed = 0;
    double now;

    tw_input_state_seed_start(a->input_state, tw_monotonic());

    for(;;) {
        nread = (int) read(STDIN_FILENO, chunk, CHUNK_SIZE);
        if(nread <= 0)
            break;
        now = tw_monotonic();
        tw_input_state_observe_chars(a->input_state, chunk,
                                     (size_t) nread, now);
        if(feed_line_by_line(a->mf, a->feeder, chunk, (size_t) nread, 0)
           != 0) {
            failed = 1;
            break;
        }
    }

    if(nread < 0)
        failed = 1;
    if(!failed) {
        if(feed_line_by_line(a->mf, a->feeder, NULL, 0, 1) != 0)
            failed = 1;
    }
    if(mdflow_close(a->mf) != 0)
        failed = 1;
    a->mf = NULL;
    sink_flush(a->sink);

    mutex_lock(a->lock);
    a->shared->eof = 1;
    if(failed)
        a->shared->error = 1;
    mutex_unlock(a->lock);
    cond_broadcast(a->cond);
    return 0;
}


/* Wait for a rendered line, EOF, or error. timeout < 0 waits forever. */
static void
wait_for_lines(tw_cond_t* cond, tw_mutex_t* lock, shared_state_t* shared,
               double timeout)
{
    mutex_lock(lock);
    while(shared->q.count == 0 && !shared->eof && !shared->error) {
        if(timeout < 0.0) {
            cond_wait(cond, lock);
        } else {
            if(cond_timedwait(cond, lock, timeout) != 0)
                break;
        }
    }
    mutex_unlock(lock);
}


static void
print_typewriter_report(const tw_input_state_t* state,
                        const tw_pacer_t* pacer,
                        int bypass, double start_time, double end_time)
{
    fprintf(stderr,
        "report: input_chars=%lld raw_lines=%lld output_chars=%lld "
        "burst_count=%lld burst_chars=%lld timeout_burst_count=%lld "
        "timeout_burst_chars=%lld max_backlog=%lld max_lag=%.3fs "
        "max_delay_cap=1.0s bypass=%s upstream_rate=%.1f/s "
        "input_meta=on wall_time=%.3fs\n",
        state->chars_seen,
        state->lines_completed,
        pacer->stats.output_chars,
        pacer->stats.burst_count,
        pacer->stats.burst_chars,
        pacer->stats.timeout_burst_count,
        pacer->stats.timeout_burst_chars,
        pacer->stats.max_backlog,
        pacer->stats.max_lag,
        bypass ? "on" : "off",
        state->rate,
        end_time - start_time);
}


static int
run_typewriter(int term_width, const typewriter_opts_t* opts)
{
    mdflow_t* mf = NULL;
    tw_input_state_t input_state;
    tw_pacer_t pacer;
    tw_mutex_t lock;
    tw_cond_t cond;
    shared_state_t shared;
    line_sink_t sink;
    mdflow_feeder_t feeder;
    tw_thread_t reader;
    reader_args_t rargs;
    int bypass = 1;     /* passthrough until the first measurement proves
                         * the stream is slow */
    int locked = 0;     /* one-shot decision made */
    int ret = 0;
    double start_time;
    double now;

    memset(&shared, 0, sizeof(shared));
    memset(&sink, 0, sizeof(sink));
    memset(&feeder, 0, sizeof(feeder));
    memset(&rargs, 0, sizeof(rargs));

    sink.shared = &shared;
    sink.lock = &lock;
    sink.cond = &cond;
    rargs.feeder = &feeder;

    tw_input_state_init(&input_state, TYPEWRITER_DEFAULT_CPS, 0.15,
                        1000000.0);
    tw_pacer_init(&pacer, pacer_writer, NULL, TYPEWRITER_DEFAULT_CPS,
                  TYPEWRITER_BUFFER_SIZE, TYPEWRITER_MAX_DELAY);

    if(mutex_init(&lock) != 0) {
        fprintf(stderr, "typewriter init failed.\n");
        return 1;
    }
    if(cond_init(&cond) != 0) {
        mutex_destroy(&lock);
        fprintf(stderr, "typewriter init failed.\n");
        return 1;
    }

    mf = mdflow_open(term_width, typewriter_output_cb, &sink);
    if(mf == NULL) {
        fprintf(stderr, "mdflow_open failed.\n");
        mutex_destroy(&lock);
        cond_destroy(&cond);
        return 1;
    }
    mdflow_set_osc8(mf, !opts->osc8_off);

    start_time = tw_monotonic();
    if(stdin_is_regular_file()) {
        locked = 1;     /* file redirect: passthrough, never re-evaluated */
        tw_input_state_mark_fast(&input_state);
    }

    rargs.mf = mf;
    rargs.input_state = &input_state;
    rargs.sink = &sink;
    rargs.shared = &shared;
    rargs.lock = &lock;
    rargs.cond = &cond;

    if(thread_create(&reader, reader_thread, &rargs) != 0) {
        fprintf(stderr, "failed to start reader thread.\n");
        mdflow_close(mf);
        free(feeder.data);
        mutex_destroy(&lock);
        cond_destroy(&cond);
        tw_pacer_destroy(&pacer);
        return 1;
    }

    for(;;) {
        int new_lines = 0;
        int new_visible = 0;
        int processed = 0;
        int eof = 0;
        int err = 0;
        int q_empty = 0;
        double interval;

        now = tw_monotonic();

        mutex_lock(&lock);
        while(shared.q.count > 0 && processed < TYPEWRITER_DRAIN_BATCH) {
            raw_line_t item;
            raw_line_pop(&shared.q, &item);
            cond_broadcast(&cond);
            mutex_unlock(&lock);

            if(bypass) {
                fwrite(item.data, 1, item.len, stdout);
            } else {
                tw_line_t* line = build_line(item.data, item.len, now);
                int visible_count;
                if(line == NULL) {
                    raw_line_free(&item);
                    mutex_lock(&lock);
                    shared.error = 1;
                    mutex_unlock(&lock);
                    ret = 1;
                    goto finish;
                }
                visible_count = line->visible_count;
                if(tw_pacer_add_line(&pacer, line) != 0) {
                    tw_line_free(line);
                    raw_line_free(&item);
                    mutex_lock(&lock);
                    shared.error = 1;
                    mutex_unlock(&lock);
                    ret = 1;
                    goto finish;
                }
                if(visible_count > 0)
                    new_visible++;
                new_lines++;
            }
            raw_line_free(&item);
            processed++;
            mutex_lock(&lock);
        }
        eof = shared.eof;
        err = shared.error;
        q_empty = (shared.q.count == 0);
        mutex_unlock(&lock);

        if(err != 0) {
            ret = 1;
            goto finish;
        }

        if(bypass && !locked) {
            /* Undecided: keep passing through until the first real rate
             * measurement exists. */
            double rate;
            int rate_measured;
            mutex_lock(&lock);
            rate = input_state.rate;
            rate_measured = input_state.rate_measured;
            mutex_unlock(&lock);
            if(rate_measured) {
                locked = 1;
                if(rate < TYPEWRITER_MAX_STREAM_CPS)
                    bypass = 0;   /* proven slow: start typing */
            } else {
                if(eof && q_empty)
                    break;
                /* Poll while undecided so the one-shot classification
                 * happens as soon as the rate is measurable, not when the
                 * first rendered line happens to arrive. */
                wait_for_lines(&cond, &lock, &shared, 0.05);
                continue;
            }
        }

        if(bypass) {
            /* Locked passthrough (fast stream, file redirect, or EOF
             * before any measurement). */
            if(eof && q_empty)
                break;
            wait_for_lines(&cond, &lock, &shared, -1.0);
            continue;
        }

        if(new_lines > 0 && new_visible > 0)
            tw_pacer_maybe_burst(&pacer, new_lines, now);
        if(eof && q_empty && pacer.queue_count == 0)
            break;

        {
            double rate;
            mutex_lock(&lock);
            rate = input_state.rate;
            mutex_unlock(&lock);
            pacer.cps = rate;
        }

        interval = tw_pacer_idle_interval(&pacer, now);
        if(interval < 0.0) {
            wait_for_lines(&cond, &lock, &shared, -1.0);
            continue;
        }
        if(interval > 0.0) {
            wait_for_lines(&cond, &lock, &shared, interval);
            continue;
        }
        tw_pacer_step(&pacer, now);
    }

finish:
    if(ret != 0) {
        /* The reader thread may still be blocked on stdin; the process is
         * about to exit, so skip joining and let the OS reclaim state. */
        return ret;
    }

    thread_join(&reader);

    /* CLI epilogue, identical to the non-typewriter path. */
    fputc('\n', stdout);
    output_cb("\033[0m", 4, NULL);

    now = tw_monotonic();
    if(opts->report)
        print_typewriter_report(&input_state, &pacer, bypass,
                                start_time, now);

    line_queue_destroy(&shared.q);
    free(sink.data);
    free(feeder.data);
    mutex_destroy(&lock);
    cond_destroy(&cond);
    tw_pacer_destroy(&pacer);
    return ret;
}


static int
parse_options(int argc, char* argv[], typewriter_opts_t* o, int* show_help)
{
    int i;

    memset(o, 0, sizeof(*o));

    for(i = 1; i < argc; i++) {
        const char* arg = argv[i];

        if(strcmp(arg, "-h") == 0 || strcmp(arg, "--help") == 0) {
            *show_help = 1;
            return 0;
        }
        if(strcmp(arg, "--typewriter-off") == 0) {
            o->typewriter_off = 1;
            continue;
        }
        if(strcmp(arg, "--report") == 0) {
            o->report = 1;
            continue;
        }
        if(strcmp(arg, "--osc8-off") == 0) {
            o->osc8_off = 1;
            continue;
        }

        fprintf(stderr, "mdflow: unknown option: %s\n", arg);
        return -1;
    }
    return 0;
}


int
main(int argc, char* argv[])
{
    typewriter_opts_t opts;
    int show_help = 0;
    mdflow_t* mf = NULL;
    mdflow_feeder_t feeder;
    char chunk[CHUNK_SIZE];
    int nread;
    int ret = 0;
    int term_width = 0;

    feeder.data = NULL;
    feeder.alloc = 0;
    feeder.len = 0;

    if(parse_options(argc, argv, &opts, &show_help) != 0) {
        usage();
        return 1;
    }
    if(show_help) {
        usage();
        return 0;
    }

    setbuf(stdout, NULL);

    /* If stdin is a terminal with no pipe/redirect, print usage and exit. */
    if(isatty(STDIN_FILENO))
    {
        usage();
        return 1;
    }

    /* Detect terminal width for table column limiting. */
#ifdef TIOCGWINSZ
    {
        struct winsize ws;
        if(ioctl(STDOUT_FILENO, TIOCGWINSZ, &ws) == 0  &&  ws.ws_col > 0)
            term_width = (int) ws.ws_col;
    }
#endif
    if(term_width == 0) {
        char* env = getenv("COLUMNS");
        if(env != NULL)
            term_width = atoi(env);
    }

    if(!opts.typewriter_off)
        return run_typewriter(term_width, &opts);

    mf = mdflow_open(term_width, output_cb, NULL);
    if(mf == NULL) {
        fprintf(stderr, "mdflow_open failed.\n");
        return 1;
    }
    mdflow_set_osc8(mf, !opts.osc8_off);

    while(1) {
        nread = (int) read(STDIN_FILENO, chunk, CHUNK_SIZE);
        if(nread <= 0)
            break;
        if(feed_line_by_line(mf, &feeder, chunk, (size_t) nread, 0) != 0) {
            fprintf(stderr, "mdflow_write failed.\n");
            ret = 1;
            goto out;
        }
    }

    if(nread < 0) {
        fprintf(stderr, "Error reading stdin: %s\n", strerror(errno));
        ret = 1;
        goto out;
    }

    if(feed_line_by_line(mf, &feeder, NULL, 0, 1) != 0) {
        fprintf(stderr, "mdflow_write failed.\n");
        ret = 1;
        goto out;
    }

    if(mdflow_close(mf) != 0) {
        fprintf(stderr, "mdflow_close failed.\n");
        ret = 1;
        goto out;
    }
    mf = NULL;

out:
    free(feeder.data);
    if(mf != NULL)
        mdflow_close(mf);

    fputc('\n', stdout);
    output_cb("\033[0m", 4, NULL);

    return ret;
}
