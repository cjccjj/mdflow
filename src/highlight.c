/*
 * highlight: single-pass, language-agnostic code highlighter (C89).
 *
 * The scanner decides one token type per character run with a fixed
 * priority chain; multiline strings and comments carry across line
 * boundaries via token state.
 *
 * Token types and their finalize rules:
 *
 *   - One character lookahead; `prev1`/`prev2` are the last two consumed
 *     characters, with `prev1` normalized to 1 when the previous character
 *     was a backslash inside a token of type < 7 (escape: cannot finalize).
 *   - Token finalization is decided by a table indexed on the current
 *     token type; multiline strings (5/6) and multiline comments (8)
 *     span newlines, single-line comments (9/10) end at a newline.
 *   - The next token's type is decided by a priority chain checked in
 *     descending order: '#' (10), '//' (9), slash-star (8), '<!--' (7),
 *     '\'' (6), '"' (5), regex heuristic (4), word (3), closing brace (2),
 *     operator (1), whitespace (0).
 *   - A '/' opens a regex literal only when the previous non-whitespace,
 *     non-comment token was an operator (type 1, since type 0 is never
 *     stored).
 *   - Words are styled as keywords iff they match one anchored alternation
 *     regex covering the keyword lists of many languages.
 *   - Well-formed UTF-8 sequences are consumed as single characters, so
 *     SGR styling can never split a multi-byte character.
 *
 * Character classes are ASCII (word chars: [A-Za-z0-9_], plus $).
 *
 * This file is part of mdflow (https://github.com/cjccjj/mdflow).
 * Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md).
 *
 * ------------------------------------------------------------------
 * Portions of this file are derived from microlight.js
 * (https://github.com/asvd/microlight), which is MIT licensed:
 *
 * Copyright (c) 2016 asvd <heliosframework@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
 * OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 * ------------------------------------------------------------------
 */

#include "highlight.h"

#include <stdlib.h>
#include <string.h>


/* ------------------------------------------------------------------ */
/*  Keyword matching: the keyword pattern is an anchored alternation   */
/*  of literals using only | ( ) ? — no classes, no repetition. The    */
/*  language it defines is finite, so it is expanded once into a       */
/*  sorted string table (binary search lookup) instead of using POSIX  */
/*  regex (regex.h is not portable enough to rely on).                 */
/* ------------------------------------------------------------------ */

static const char* keyword_regex =
    "^(a(bstract|lias|nd|rguments|rray|s(m|sert)?|uto)|"
    "b(ase|egin|ool(ean)?|reak|yte)|"
    "c(ase|atch|har|hecked|lass|lone|ompl|onst|ontinue)|"
    "de(bugger|cimal|clare|f(ault|er)?|init|l(egate|ete)?)|"
    "do|double|"
    "e(cho|ls?if|lse(if)?|nd|nsure|num|vent|x(cept|ec|p(licit|ort)|te(nds|nsion|rn)))|"
    "f(allthrough|alse|inal(ly)?|ixed|loat|or(each)?|riend|rom|unc(tion)?)|"
    "global|goto|guard|"
    "i(f|mp(lements|licit|ort)|n(it|clude(_once)?|line|out|stanceof|t(erface|ernal)?)?|s)|"
    "l(ambda|et|ock|ong)|"
    "m(icrolight|odule|utable)|"
    "NaN|"
    "n(amespace|ative|ext|ew|il|ot|ull)|"
    "o(bject|perator|r|ut|verride)|"
    "p(ackage|arams|rivate|rotected|rotocol|ublic)|"
    "r(aise|e(adonly|do|f|gister|peat|quire(_once)?|scue|strict|try|turn))|"
    "s(byte|ealed|elf|hort|igned|izeof|tatic|tring|truct|ubscript|uper|ynchronized|witch)|"
    "t(emplate|hen|his|hrows?|ransient|rue|ry|ype(alias|def|id|name|of))|"
    "u(n(checked|def(ined)?|ion|less|signed|til)|se|sing)|"
    "v(ar|irtual|oid|olatile)|"
    "w(char_t|hen|here|hile|ith)|"
    "xor|yield)$";


/* ------------------------------------------------------------------ */
/*  Pattern expansion to a sorted keyword table.                       */
/* ------------------------------------------------------------------ */

#define KW_MAX_STR  64
#define KW_MAX_LIST 256

/* Find the closing ')' matching the group opening at pat[open]. */
static int
hl_kw_group_end(const char* pat, int open, int end)
{
    int depth = 0;
    int p = open;
    while(p < end) {
        if(pat[p] == '(')
            depth++;
        else if(pat[p] == ')' && --depth == 0)
            return p;
        p++;
    }
    return end;   /* unterminated (defensive): treat rest as the group */
}

static int hl_kw_expand_seq(const char* pat, int beg, int end,
                            char (*out)[KW_MAX_STR], int max);

/* Expand the alternation pattern[beg..end) into every string it
 * matches (written to out[], which holds at most max strings). */
static int
hl_kw_expand_alt(const char* pat, int beg, int end,
                 char (*out)[KW_MAX_STR], int max)
{
    int n = 0;
    int start = beg;

    while(start <= end) {
        int q = start;
        int depth = 0;
        while(q < end) {
            if(pat[q] == '(')
                depth++;
            else if(pat[q] == ')')
                depth--;
            else if(pat[q] == '|' && depth == 0)
                break;
            q++;
        }
        n += hl_kw_expand_seq(pat, start, q, out + n, max - n);
        if(q >= end || n >= max)
            break;
        start = q + 1;
    }
    return n;
}

/* Expand the sequence pattern[beg..end) into every string it matches.
 * Sequence atoms: literal chars, optional chars, and (alternation)
 * groups with optional '?'. */
static int
hl_kw_expand_seq(const char* pat, int beg, int end,
                 char (*out)[KW_MAX_STR], int max)
{
    char list[KW_MAX_LIST][KW_MAX_STR];
    int n = 1;
    int p = beg;

    strcpy(list[0], "");

    while(p < end && n > 0) {
        if(pat[p] == '(') {
            /* (alt) with optional ? */
            int close = hl_kw_group_end(pat, p, end);
            int opt = (close + 1 < end && pat[close + 1] == '?');
            int next = close + 1 + (opt ? 1 : 0);
            char branches[KW_MAX_LIST][KW_MAX_STR];
            char newlist[KW_MAX_LIST][KW_MAX_STR];
            int nb = hl_kw_expand_alt(pat, p + 1, close, branches, KW_MAX_LIST);
            int m = 0;
            int i, j;

            /* Concatenate each current string with each group branch.
             * Results go to newlist: list[] is still being read. */
            for(i = 0; i < n && m < max; i++) {
                for(j = 0; j < nb && m < max; j++) {
                    size_t l1 = strlen(list[i]);
                    size_t l2 = strlen(branches[j]);
                    if(l1 + l2 < KW_MAX_STR) {
                        memcpy(newlist[m], list[i], l1);
                        memcpy(newlist[m] + l1, branches[j], l2 + 1);
                        m++;
                    }
                }
            }
            if(opt) {
                for(i = 0; i < n && m < max; i++) {
                    strcpy(newlist[m], list[i]);
                    m++;
                }
            }
            for(i = 0; i < m; i++)
                strcpy(list[i], newlist[i]);
            n = m;
            p = next;
        } else if(p + 1 < end && pat[p + 1] == '?') {
            /* optional single char */
            int m = 0;
            int i;
            for(i = 0; i < n && m < max; i++) {
                if(m != i)
                    strcpy(list[m], list[i]);
                m++;
                {
                    size_t l = strlen(list[i]);
                    if(l + 1 < KW_MAX_STR && m < max) {
                        strcpy(list[m], list[i]);
                        list[m][l] = pat[p];
                        list[m][l + 1] = '\0';
                        m++;
                    }
                }
            }
            n = m;
            p += 2;
        } else {
            /* literal char */
            int i;
            for(i = 0; i < n; i++) {
                size_t l = strlen(list[i]);
                if(l + 1 < KW_MAX_STR) {
                    list[i][l] = pat[p];
                    list[i][l + 1] = '\0';
                } else {
                    n = 0;   /* overflow (defensive): drop this expansion */
                    break;
                }
            }
            p++;
        }
    }

    {
        int i;
        for(i = 0; i < n; i++)
            strcpy(out[i], list[i]);
    }
    return n;
}

static char kw_table[KW_MAX_LIST][KW_MAX_STR];
static const char* kw_sorted[KW_MAX_LIST];
static int kw_count;
static int kw_ready;

static int
hl_kw_cmp(const void* a, const void* b)
{
    return strcmp(*(const char* const*) a, *(const char* const*) b);
}

/* Expand the keyword pattern into a sorted, deduplicated string table
 * (called once, lazily). */
static void
hl_kw_init(void)
{
    int i, j;
    int plen = (int) strlen(keyword_regex);
    int n = hl_kw_expand_alt(keyword_regex, 1, plen - 1, kw_table, KW_MAX_LIST);

    for(i = 0; i < n; i++)
        kw_sorted[i] = kw_table[i];
    /* Explicit void* cast: qsort takes void*, and the table is const. */
    qsort((void*) kw_sorted, n, sizeof(const char*), hl_kw_cmp);

    j = 0;
    for(i = 0; i < n; i++) {
        if(j == 0 || strcmp(kw_sorted[i], kw_sorted[j - 1]) != 0)
            kw_sorted[j++] = kw_sorted[i];
    }
    kw_count = j;
    kw_ready = 1;
}

/* Whole-word keyword test: true iff token matches the anchored
 * alternation exactly. */
static int
hl_is_keyword(const char* tok, int len)
{
    const char* key = tok;
    (void) len;

    if(!kw_ready)
        hl_kw_init();
    return (bsearch(&key, kw_sorted, (size_t) kw_count,
                    sizeof(const char*), hl_kw_cmp) != NULL);
}


/* ------------------------------------------------------------------ */
/*  Character class helpers (ASCII, as in the original JS regexes).   */
/* ------------------------------------------------------------------ */

/* JS [$\w] — a word character, $ included. */
static int
is_word_char(int chr)
{
    return (chr == '$'
            || (chr >= '0' && chr <= '9')
            || (chr >= 'A' && chr <= 'Z')
            || (chr >= 'a' && chr <= 'z')
            || chr == '_');
}

/* JS \S — non-whitespace. Whitespace = the ASCII portion of JS \s. */
static int
is_non_whitespace(int chr)
{
    return !(chr == ' ' || chr == '\t' || chr == '\n'
             || chr == '\v' || chr == '\f' || chr == '\r');
}

/* JS [\/{}[(\-+*=<>:;|\\.,?!&@~] — operators and braces. */
static int
is_operator(int chr)
{
    switch(chr) {
        case '/': case '{': case '}': case '[': case '(':
        case '-': case '+': case '*': case '=': case '<': case '>':
        case ':': case ';': case '|': case '\\': case '.':
        case ',': case '?': case '!': case '&': case '@': case '~':
            return 1;
        default:
            return 0;
    }
}

/* JS [\])] — closing braces. */
static int
is_closing_brace(int chr)
{
    return (chr == ')' || chr == ']' || chr == '}');
}


/* ------------------------------------------------------------------ */
/*  Scanner.                                                          */
/* ------------------------------------------------------------------ */

/* A token type value of "none" (JS undefined): never < 2 in the
 * regex-heuristic test. */
#define LAST_TYPE_NONE  (-1)

/* Sentinel for "no character" (JS undefined). */
#define NO_CHAR         (-1)

/* Read a byte from the virtual buffer a[0..a_len) + b[0..b_len).
 * Callers must only pass indices < a_len + b_len. */
static int
hl_get_c(const char* a, int a_len, const char* b, int i)
{
    if(i < a_len)
        return (unsigned char) a[i];
    return (unsigned char) b[i - a_len];
}

/* Scan up to `consume` chars of the virtual buffer a+b. State flows
 * through s; tokens are delivered to `emit` as they finalize. If
 * `final` is set, an end-of-input pass flushes the pending token.
 * On a chunk boundary, *p_consumed (if non-NULL) receives the actual
 * number of chars consumed — the multibyte skip may consume past
 * `consume`, and the caller must not re-feed those. Returns 0, or -1
 * on allocation failure. */
static int
hl_scan_internal(MD_HL_STATE* s, const char* a, int a_len,
                 const char* b, int b_len, int consume, int final,
                 int* p_consumed,
                 HL_TOKEN_FN emit, void* userdata)
{
    int len = a_len + b_len;
    int pos = 0;               /* index of next1 (JS: current position) */
    int next1 = (len > 0) ? hl_get_c(a, a_len, b, 0) : NO_CHAR;
    int chr = 1;               /* sentinel, as in the JS (chr = 1) */
    int prev1 = s->prev1;
    int prev2 = s->prev2;
    int lag1 = s->lag1;
    int lag2 = s->lag2;
    int lag3 = s->lag3;
    int token_type = s->token_type;
    int last_token_type = s->last_token_type;
    int multichar;
    int token_len = s->token_len;
    int token_cap = s->token_cap;
    char* token = s->token;
    int ret = 0;

    if(p_consumed != NULL)
        *p_consumed = 0;

    for(;;) {
        /* --- JS while-condition: prev2=prev1; prev1 = escape? 1 : chr --- */
        prev2 = prev1;
        if(token_type < 7 && prev1 == '\\') {
            prev1 = 1;          /* escaped character: cannot finalize */
        } else if(chr == NO_CHAR) {
            break;              /* JS: chr undefined -> loop exits */
        } else {
            prev1 = chr;
        }

        /* --- JS loop body: chr=next1; next1=text[++pos] --- */
        if(pos < consume) {
            chr = next1;
            next1 = (pos + 1 < len) ? hl_get_c(a, a_len, b, pos + 1) : NO_CHAR;
            pos++;
        } else if(final) {
            chr = NO_CHAR;      /* end-of-input flush pass */
        } else {
            break;              /* chunk boundary: hold the rest */
        }

        multichar = (token_len > 1);

        /* --- Finalize the current token? --- */
        if(chr == NO_CHAR                       /* end of content */
                || (token_type > 8 && chr == '\n')  /* //, # end at newline */
                || (token_type == 0 && is_non_whitespace(chr))
                || token_type == 1              /* operators: single char */
                || token_type == 2              /* braces: single char */
                || (token_type == 3 && !is_word_char(chr))
                || (token_type == 4 && (prev1 == '/' || prev1 == '\n')
                                        && multichar)
                || (token_type == 5 && prev1 == '"' && multichar)
                || (token_type == 6 && prev1 == '\'' && multichar)
                || (token_type == 7 && pos - 4 >= 0
                        && lag3 == '-' && prev2 == '-' && prev1 == '>')
                || (token_type == 8 && prev2 == '*' && prev1 == '/')) {

            /* Emit the finished token. */
            if(token_len > 0) {
                int style;

                if(token_type == 0)
                    style = HL_STYLE_PLAIN;
                else if(token_type < 3)
                    style = HL_STYLE_PUNCT;
                else if(token_type > 6)
                    style = HL_STYLE_COMMENT;
                else if(token_type > 3)
                    style = HL_STYLE_STRING;
                else {
                    token[token_len] = '\0';
                    style = hl_is_keyword(token, token_len)
                                ? HL_STYLE_KEYWORD : HL_STYLE_PLAIN;
                }

                emit(userdata, style, token, token_len);
            }

            /* Save the token type (skipping whitespace and comments). */
            if(token_type != 0 && token_type < 7)
                last_token_type = token_type;

            /* Start a new token; decide its type by the priority chain,
             * checked in descending order (10 .. 0). */
            token_len = 0;

            if(chr == '#')
                token_type = 10;
            else if(chr == '/' && next1 == '/')
                token_type = 9;
            else if(chr == '/' && next1 == '*')
                token_type = 8;
            else if(chr == '<' && next1 == '!'
                        && pos + 1 < len
                        && hl_get_c(a, a_len, b, pos + 1) == '-'
                        && pos + 2 < len
                        && hl_get_c(a, a_len, b, pos + 2) == '-')
                token_type = 7;
            else if(chr == '\'')
                token_type = 6;
            else if(chr == '"')
                token_type = 5;
            else if(chr == '/' && last_token_type == 1 && prev1 != '<')
                token_type = 4;
            else if(is_word_char(chr))
                token_type = 3;
            else if(is_closing_brace(chr))
                token_type = 2;
            else if(is_operator(chr))
                token_type = 1;
            else
                token_type = 0;
        }

        /* --- JS: token += chr --- */
        if(chr != NO_CHAR) {
            /* A UTF-8 sequence is appended whole: the scanner works on
             * characters, not bytes, so SGR styling can never split a
             * multi-byte character. */
            int seq = 1;
            int k;

            {
                unsigned char c0 = (unsigned char) hl_get_c(a, a_len, b,
                                                            pos - 1);
                if(c0 >= 0xC2 && c0 < 0xF5) {
                    int n = (c0 < 0xE0) ? 2 : (c0 < 0xF0) ? 3 : 4;
                    for(k = 1; k < n; k++) {
                        if(pos - 1 + k >= len
                                || ((unsigned char) hl_get_c(a, a_len, b,
                                    pos - 1 + k) & 0xC0) != 0x80)
                            break;
                        seq++;
                    }
                }
            }

            if(token_len + seq + 1 > token_cap) {
                int new_cap = (token_cap == 0) ? 64 : token_cap * 2;
                char* new_token = (char*) realloc(token, (size_t) new_cap);
                if(new_token == NULL) {
                    free(token);
                    s->token = NULL;
                    s->token_len = 0;
                    s->token_cap = 0;
                    return -1;
                }
                token = new_token;
                token_cap = new_cap;
            }
            for(k = 0; k < seq; k++) {
                token[token_len++] = (char) hl_get_c(a, a_len, b,
                                                     pos - 1 + k);
            }

            /* Keep the raw-character lags in step (one per consumed
             * character, including the sequence's continuation bytes). */
            for(k = 0; k < seq; k++) {
                lag3 = lag2;
                lag2 = lag1;
                lag1 = (int) (unsigned char) hl_get_c(a, a_len, b,
                                                      pos - 1 + k);
            }

            /* Skip the rest of the sequence in the scan: the invariant
             * next1 == text[pos] must hold for the next iteration. */
            if(seq > 1) {
                pos += seq - 1;
                next1 = (pos < len) ? hl_get_c(a, a_len, b, pos) : NO_CHAR;
            }
        }

        /* Report the number of chars consumed so far (may be past
         * `consume` when a UTF-8 sequence was skipped). */
        if(p_consumed != NULL)
            *p_consumed = pos;
    }

    /* Save the scanner state. */
    s->token_type = token_type;
    s->prev1 = prev1;
    s->prev2 = prev2;
    s->lag1 = lag1;
    s->lag2 = lag2;
    s->lag3 = lag3;
    s->last_token_type = last_token_type;
    s->token = token;
    s->token_len = token_len;
    s->token_cap = token_cap;

    return ret;
}

void
hl_reset(MD_HL_STATE* s)
{
    /* The state must be zero-initialized (e.g. calloc) or already
     * finished (hl_finish/hl_feed release the token buffer). */
    memset(s, 0, sizeof(MD_HL_STATE));
    s->last_token_type = LAST_TYPE_NONE;
}

void
hl_release(MD_HL_STATE* s)
{
    free(s->token);
    s->token = NULL;
    s->token_len = 0;
    s->token_cap = 0;
    s->carry_len = 0;
}

int
hl_feed(MD_HL_STATE* s, const char* text, int len,
        HL_TOKEN_FN emit, void* userdata)
{
    char a0[3] = { 0 };
    int a0_len;
    int total;
    int consume;
    int consumed = 0;
    int ret;

    if(s->carry_len > (int) sizeof(a0))
        s->carry_len = (int) sizeof(a0);
    a0_len = s->carry_len;
    memcpy(a0, s->carry, (size_t) a0_len);

    total = a0_len + len;
    consume = (total >= 4) ? total - 3 : 0;

    ret = hl_scan_internal(s, a0, a0_len, text, len, consume, 0,
                           &consumed, emit, userdata);
    if(ret != 0)
        return ret;

    /* New carry: the chars after the actually consumed prefix (a UTF-8
     * sequence may have been skipped past `consume`).  At most three
     * characters survive, one for each lookahead slot. */
    s->carry_len = 0;
    if(consumed < 0)
        consumed = 0;
    if(consumed < a0_len) {
        int n = a0_len - consumed;
        memcpy(s->carry + s->carry_len, a0 + consumed, (size_t) n);
        s->carry_len += n;
        if(len > 0)
            memcpy(s->carry + s->carry_len, text, (size_t) len);
        s->carry_len += len;
    } else {
        int off = consumed - a0_len;
        if(off < len)
            memcpy(s->carry, text + off, (size_t) (len - off));
        s->carry_len = len - off;
    }
    return 0;
}

int
hl_finish(MD_HL_STATE* s, HL_TOKEN_FN emit, void* userdata)
{
    char a0[3] = { 0 };
    int a0_len = s->carry_len;
    int ret;

    if(a0_len > (int) sizeof(a0))
        a0_len = (int) sizeof(a0);
    memcpy(a0, s->carry, (size_t) a0_len);

    ret = hl_scan_internal(s, a0, a0_len, NULL, 0, a0_len, 1, NULL,
                           emit, userdata);
    s->carry_len = 0;
    free(s->token);
    s->token = NULL;
    s->token_len = 0;
    s->token_cap = 0;
    return ret;
}

void
hl_scan(const char* text, int len, HL_TOKEN_FN emit, void* userdata)
{
    MD_HL_STATE s;

    hl_reset(&s);
    if(hl_feed(&s, text, len, emit, userdata) == 0)
        hl_finish(&s, emit, userdata);
}
