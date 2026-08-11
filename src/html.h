/*
 * html: HTML construct scanner for the mdflow renderer.
 *
 * Two layers, mirroring the highlight module:
 *
 *   - html_parse_tag():  classify one <tag ...> (attributes captured).
 *   - html_scan():       walk raw HTML block text, splitting it into
 *                        constructs (tags, comments, PIs, CDATA sections,
 *                        declarations, entities, plain text) with a small
 *                        state machine that carries across parser text
 *                        callbacks. The scanner never styles anything; it
 *                        delivers events to a renderer-supplied callback,
 *                        which decides what to output.
 *
 * The scanner is renderer-independent: no styling, no terminal output, no
 * entity knowledge (entities are handed to the callback for decoding).
 *
 * This file is part of mdflow (https://github.com/cjccjj/mdflow).
 * Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md).
 */

#ifndef MD_FLOW_HTML_H
#define MD_FLOW_HTML_H

/* Tag types (renderer-independent classification). */
#define HTML_TAG_UNKNOWN   0
#define HTML_TAG_A         1
#define HTML_TAG_STRONG    2
#define HTML_TAG_B         3
#define HTML_TAG_EM        4
#define HTML_TAG_I         5
#define HTML_TAG_CODE      6
#define HTML_TAG_KBD       7
#define HTML_TAG_BR        8
#define HTML_TAG_IMG       9
#define HTML_TAG_DEL      10

/* One parsed tag: type, closer flag, and the href/src/alt attributes. */
typedef struct {
    int type;
    int is_closer;
    char href[512];
    int href_len;
    char src[512];
    int src_len;
    char alt[256];
    int alt_len;
} HTML_TAG_INFO;

/* html_scan() state — persists across MD_TEXT_HTML callbacks so a
 * construct split by the parser (comment, PI, declaration, CDATA) stays
 * in the same logical section until its end marker arrives. */
typedef struct {
    int in_cdata;       /* <![CDATA[ ... ]]> */
    int in_comment;     /* <!-- ... --> */
    int in_pi;          /* <? ... ?> */
    int in_decl;        /* <! ... > */
} HTML_SCAN_STATE;

/* Events delivered to the scan callback. */
#define HTML_EVENT_TEXT      1   /* plain text — output verbatim */
#define HTML_EVENT_ENTITY    2   /* &...; — decode or output verbatim */
#define HTML_EVENT_TAG       3   /* <...> — `tag` is valid; style or verbatim */
#define HTML_EVENT_CDATA     4   /* CDATA content only (markers stripped) */
#define HTML_EVENT_PI        5   /* <? ... ?> incl. markers — verbatim */
#define HTML_EVENT_DECL      6   /* <! ... > incl. markers — verbatim */

/* Comments produce no event: their content is invisible.
 * For HTML_EVENT_TAG, `text`/`size` carry the raw tag bytes. */
typedef void (*HTML_EVENT_FN)(void* userdata, int event,
                              const char* text, int size,
                              const HTML_TAG_INFO* tag);

/* Parse a single HTML tag from MD_TEXT_HTML content.
 * Fills info with type, closer flag, and href/src/alt attributes.
 * Silently ignores unsupported tags and non-tag HTML constructs. */
void html_parse_tag(const char* text, int size, HTML_TAG_INFO* info);

/* Decode entities in a raw HTML attribute value into a fixed buffer
 * (NUL-terminated). Unknown entities are copied verbatim. */
int html_attr_decode(const char* src, int len, char* dst, int dst_size);

/* Walk one chunk of HTML block text, delivering events to `emit`.
 * `state` carries construct boundaries across calls (zero it when the
 * HTML block starts). */
void html_scan(const char* text, int size, HTML_SCAN_STATE* state,
               HTML_EVENT_FN emit, void* userdata);

#endif  /* MD_FLOW_HTML_H */
