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

#include "mdflow.h"
#include "md4cs-ansi.h"

#include <stdlib.h>
#include <string.h>


struct mdflow_tag {
    MD_ANSI_RENDERER* renderer;
    MD_PARSER_CTX*    stream_ctx;
    MD_PARSER         parser;
};


mdflow_t*
mdflow_open(int term_width,
            void (*output)(const char* str, int size, void* userdata),
            void* userdata)
{
    mdflow_t* mf;

    mf = (mdflow_t*) calloc(1, sizeof(mdflow_t));
    if(mf == NULL)
        return NULL;

    mf->renderer = md_ansi_renderer_create(md_ansi_default_theme(),
                                            output, userdata);
    if(mf->renderer == NULL) {
        free(mf);
        return NULL;
    }

    if(term_width > 0)
        md_ansi_set_term_width(mf->renderer, term_width);

    memset(&mf->parser, 0, sizeof(MD_PARSER));
    mf->parser.abi_version = 0;
    mf->parser.flags = MD_DIALECT_GITHUB
                      | MD_FLAG_HIGHLIGHT
                      | MD_FLAG_SPOILERS
                      | MD_FLAG_SUPERSCRIPTS
                      | MD_FLAG_SUBSCRIPTS
                      | MD_FLAG_UNDERLINE;
    mf->parser.enter_block = md_ansi_enter_block;
    mf->parser.leave_block = md_ansi_leave_block;
    mf->parser.enter_span  = md_ansi_enter_span;
    mf->parser.leave_span  = md_ansi_leave_span;
    mf->parser.text        = md_ansi_text;

    if(md_stream_init(&mf->parser, (void*) mf->renderer,
                      &mf->stream_ctx) != 0) {
        md_ansi_renderer_destroy(mf->renderer);
        free(mf);
        return NULL;
    }

    return mf;
}


int
mdflow_write(mdflow_t* mf, const char* data, int len)
{
    if(mf == NULL || len < 0 || (len > 0 && data == NULL))
        return -1;
    return md_stream_feed(mf->stream_ctx,
                          (const MD_CHAR*) data, (MD_SIZE) len);
}


int
mdflow_close(mdflow_t* mf)
{
    int ret;

    if(mf == NULL)
        return 0;

    ret = md_stream_flush(mf->stream_ctx);
    md_stream_finish(mf->stream_ctx);
    md_ansi_renderer_destroy(mf->renderer);
    free(mf);

    return ret;
}
