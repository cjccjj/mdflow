/*
 * highlight: single-pass, language-agnostic code highlighter.
 *
 * The scanner assigns one token type per character run by a fixed priority
 * chain: multiline strings and comments carry across line boundaries via
 * token state. No regex rules, no per-language modes, no ordering games.
 *
 * This file is part of mdflow (https://github.com/cjccjj/mdflow).
 * Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md).
 */

#ifndef MD_FLOW_HIGHLIGHT_H
#define MD_FLOW_HIGHLIGHT_H

/* Token style ids. */
#define HL_STYLE_PLAIN    0   /* whitespace, non-keyword words */
#define HL_STYLE_KEYWORD  1
#define HL_STYLE_PUNCT    2   /* operators and braces */
#define HL_STYLE_STRING   3   /* "..." strings and regex literals */
#define HL_STYLE_COMMENT  4   /* xml, block, // and # comments */

/* Called once per finalized token with the token's style and bytes. */
typedef void (*HL_TOKEN_FN)(void* userdata, int style, const char* text, int len);

/* Incremental scanner state for one code block. Zero-initialize with
 * hl_reset() before the first hl_feed(); feed the block's text chunk by
 * chunk and call hl_finish() at block end.
 *
 * The scanner carries token state across chunks: multiline strings and
 * comments span chunk boundaries, and up to 3 trailing characters are
 * held back until the next feed so lookahead decisions (e.g. `<!--`)
 * are identical to scanning the whole block at once. */
typedef struct MD_HL_STATE_tag {
    int token_type;
    int prev1;              /* escape-normalized previous character */
    int prev2;
    int last_token_type;
    int lag1;               /* raw chars 1/2/3 behind (for `-->` finalize) */
    int lag2;
    int lag3;
    char* token;            /* in-progress token (may span chunks) */
    int token_len;
    int token_cap;
    char carry[3];          /* unconsumed lookahead chars */
    int carry_len;
} MD_HL_STATE;

/* Initialize (or re-initialize) a scanner state. The state must be
 * zero-initialized (e.g. calloc) or already released. */
void hl_reset(MD_HL_STATE* state);

/* Release the state's buffers without scanning (for teardown). */
void hl_release(MD_HL_STATE* state);

/* Feed one chunk of code text. Finalized tokens are delivered to `emit`
 * immediately. Returns 0 on success, -1 on allocation failure (the
 * caller should drop highlighting for the rest of the block). */
int hl_feed(MD_HL_STATE* state, const char* text, int len,
            HL_TOKEN_FN emit, void* userdata);

/* End of input: process the held-back characters, deliver the final
 * tokens, and free the state's buffers. Returns 0 or -1. */
int hl_finish(MD_HL_STATE* state, HL_TOKEN_FN emit, void* userdata);

/* Convenience wrapper: scan one code block's full text in a single call.
 * Equivalent to reset + feed + finish. */
void hl_scan(const char* text, int len, HL_TOKEN_FN emit, void* userdata);

#endif  /* MD_FLOW_HIGHLIGHT_H */
