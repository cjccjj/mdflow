/*
 * MD4C: Markdown parser for C
 * (vendored from https://github.com/mity/md4c)
 *
 * Copyright (c) 2016-2026 Martin Mitáš
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

#ifndef MD4C_ENTITY_H
#define MD4C_ENTITY_H

#include <stdlib.h>


/* Most entities are formed by single Unicode codepoint, few by two codepoints.
 * Single-codepoint entities have codepoints[1] set to zero. */
typedef struct ENTITY_tag ENTITY;
struct ENTITY_tag {
    const char* name;
    unsigned codepoints[2];
};

const ENTITY* entity_lookup(const char* name, size_t name_size);

/* Encode a Unicode codepoint to UTF-8. Returns bytes written (up to 4),
 * or 0 for invalid codepoints (0, surrogates, > 0x10FFFF). */
int utf8_encode_codepoint(unsigned codepoint, char* out);

/* Decode an entity reference (`&name;`, `&#123;`, `&#xAB;`) to UTF-8.
 * Returns bytes written (up to 8), or 0 if unknown/malformed. */
int entity_decode(const char* text, int size, char* out);


#endif  /* MD4C_ENTITY_H */
