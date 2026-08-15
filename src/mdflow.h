/*
 * mdflow — streaming Markdown-to-ANSI terminal renderer
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

#ifndef MDFLOW_H
#define MDFLOW_H

#ifdef __cplusplus
    extern "C" {
#endif

/* Opaque streaming state. */
typedef struct mdflow_tag mdflow_t;

/* Open a streaming Markdown-to-ANSI renderer.
 *
 *   term_width  — terminal columns for table width limiting (0 = no limit)
 *   output      — called for each chunk of ANSI output
 *   userdata    — passed through to output
 *
 * Returns NULL on failure. */
mdflow_t* mdflow_open(int term_width,
                      void (*output)(const char* str, int size, void* userdata),
                      void* userdata);

/* Enable or disable OSC 8 clickable hyperlinks (enabled by default).
 * Call after mdflow_open() and before mdflow_close(). */
int mdflow_set_osc8(mdflow_t* mf, int enable);

/* Feed a chunk of Markdown input.
 * len must be non-negative; data may be NULL only when len is zero.
 * Returns 0 on success, or non-zero on invalid input, allocation failure,
 * or an output callback/parser error. */
int mdflow_write(mdflow_t* mf, const char* data, int len);

/* Finish rendering and free all resources.
 * Returns 0 on success; handle is invalid after call. */
int mdflow_close(mdflow_t* mf);


#ifdef __cplusplus
    }  /* extern "C" { */
#endif

#endif  /* MDFLOW_H */
