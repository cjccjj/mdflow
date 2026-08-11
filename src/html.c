/*
 * html: HTML construct scanner for the mdflow renderer.
 *
 * The scanner is a single pass over HTML block text (and inline HTML
 * tags). It classifies constructs — tags, comments, processing
 * instructions, CDATA sections, declarations, entities, plain text —
 * and delivers them to a renderer-supplied callback. No styling, no
 * output, no entity knowledge lives here.
 *
 * Construct boundaries survive parser callback splits: html_scan()
 * carries its state in HTML_SCAN_STATE, so a <!-- opened at the end of
 * one MD_TEXT_HTML callback stays invisible until --> arrives in a later
 * callback (same for PIs, declarations and CDATA).
 *
 * This file is part of mdflow (https://github.com/cjccjj/mdflow).
 * Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE.md).
 */

#include "html.h"

#include <string.h>

#include "entity.h"


/* Case-insensitive ASCII name comparison against a literal. */
static int
html_tag_name_cmp(const char* name, int name_len, const char* expected)
{
    int i;
    for(i = 0; i < name_len; i++) {
        unsigned char c1 = (unsigned char)name[i];
        unsigned char c2 = (unsigned char)expected[i];
        if(c1 >= 'A' && c1 <= 'Z') c1 += 32;
        if(c2 >= 'A' && c2 <= 'Z') c2 += 32;
        if(c1 != c2) return 0;
    }
    return expected[name_len] == '\0';
}

void
html_parse_tag(const char* text, int size, HTML_TAG_INFO* info)
{
    int i = 1;
    int name_start, name_len;
    char attr_name[64];
    int attr_name_len;
    int an_start, j;
    char delim;
    int val_start, val_len;

    memset(info, 0, sizeof(*info));

    if(size == 0 || text[0] != '<')
        return;

    /* Check for closer. */
    if(i < size && text[i] == '/') {
        info->is_closer = 1;
        i++;
    }

    /* Read tag name. */
    name_start = i;
    while(i < size && ((text[i] >= 'a' && text[i] <= 'z')
                     || (text[i] >= 'A' && text[i] <= 'Z')
                     || (text[i] >= '0' && text[i] <= '9')
                     || text[i] == '-' || text[i] == ':'))
        i++;
    name_len = i - name_start;
    if(name_len == 0)
        return;

    /* Match tag name. */
    if(name_len == 1 && html_tag_name_cmp(text + name_start, name_len, "a"))
        info->type = HTML_TAG_A;
    else if(name_len == 6 && html_tag_name_cmp(text + name_start, name_len, "strong"))
        info->type = HTML_TAG_STRONG;
    else if(name_len == 1 && html_tag_name_cmp(text + name_start, name_len, "b"))
        info->type = HTML_TAG_B;
    else if(name_len == 2 && html_tag_name_cmp(text + name_start, name_len, "em"))
        info->type = HTML_TAG_EM;
    else if(name_len == 1 && html_tag_name_cmp(text + name_start, name_len, "i"))
        info->type = HTML_TAG_I;
    else if(name_len == 4 && html_tag_name_cmp(text + name_start, name_len, "code"))
        info->type = HTML_TAG_CODE;
    else if(name_len == 3 && html_tag_name_cmp(text + name_start, name_len, "kbd"))
        info->type = HTML_TAG_KBD;
    else if(name_len == 2 && html_tag_name_cmp(text + name_start, name_len, "br"))
        info->type = HTML_TAG_BR;
    else if(name_len == 3 && html_tag_name_cmp(text + name_start, name_len, "img"))
        info->type = HTML_TAG_IMG;
    else if(name_len == 3 && html_tag_name_cmp(text + name_start, name_len, "del"))
        info->type = HTML_TAG_DEL;
    else if(name_len == 1 && html_tag_name_cmp(text + name_start, name_len, "s"))
        info->type = HTML_TAG_DEL;

    if(info->type == HTML_TAG_UNKNOWN)
        return;

    /* Scan attributes. */
    while(i < size) {
        while(i < size && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n'))
            i++;
        if(i >= size || text[i] == '>' || (text[i] == '/' && i+1 < size && text[i+1] == '>'))
            break;

        /* Read attribute name. */
        an_start = i;
        while(i < size && ((text[i] >= 'a' && text[i] <= 'z')
                         || (text[i] >= 'A' && text[i] <= 'Z')
                         || text[i] == '-' || text[i] == '_' || text[i] == ':'))
            i++;
        attr_name_len = i - an_start;
        if(attr_name_len == 0 || attr_name_len >= (int)sizeof(attr_name))
            break;
        for(j = 0; j < attr_name_len; j++)
            attr_name[j] = text[an_start + j];
        attr_name[attr_name_len] = '\0';

        while(i < size && text[i] == ' ') i++;

        if(i < size && text[i] == '=') {
            i++;
            while(i < size && text[i] == ' ') i++;

            delim = 0;
            if(i < size && (text[i] == '"' || text[i] == '\'')) {
                delim = text[i];
                i++;
            }

            val_start = i;
            if(delim) {
                while(i < size && text[i] != delim) i++;
            } else {
                while(i < size && text[i] != ' ' && text[i] != '\t'
                        && text[i] != '\n' && text[i] != '>' && text[i] != '/')
                    i++;
            }
            val_len = i - val_start;
            if(delim && i < size) i++;

            /* Save known attributes. */
            if(html_tag_name_cmp(attr_name, attr_name_len, "href")
                    && val_len < (int)sizeof(info->href)) {
                memcpy(info->href, text + val_start, (size_t) val_len);
                info->href[val_len] = 0;
                info->href_len = val_len;
            }
            if(html_tag_name_cmp(attr_name, attr_name_len, "src")
                    && val_len < (int)sizeof(info->src)) {
                memcpy(info->src, text + val_start, (size_t) val_len);
                info->src[val_len] = 0;
                info->src_len = val_len;
            }
            if(html_tag_name_cmp(attr_name, attr_name_len, "alt")
                    && val_len < (int)sizeof(info->alt)) {
                memcpy(info->alt, text + val_start, (size_t) val_len);
                info->alt[val_len] = 0;
                info->alt_len = val_len;
            }
        }
    }

    /* Tag is incomplete if no closing > in this callback. */
    if(i >= size) {
        info->type = HTML_TAG_UNKNOWN;
        return;
    }
}

/* Decode entities in a raw HTML attribute value into a fixed buffer
 * (NUL-terminated). Unknown entities are copied verbatim. */
int
html_attr_decode(const char* src, int len, char* dst, int dst_size)
{
    int i = 0;
    int off = 0;

    while(i < len && off < dst_size - 1) {
        if(src[i] == '&') {
            int j = i + 1;
            while(j < len && src[j] != ';' && (j - i) < 32)
                j++;
            if(j < len && src[j] == ';') {
                char tmp[8];
                int n = entity_decode(src + i, j - i + 1, tmp);
                if(n > 0 && off + n <= dst_size - 1) {
                    memcpy(dst + off, tmp, (size_t) n);
                    off += n;
                    i = j + 1;
                    continue;
                }
            }
        }
        dst[off] = src[i];
        off++;
        i++;
    }
    dst[off] = '\0';
    return off;
}

/* Walk HTML block text, splitting it into constructs.
 * Comments produce no event (invisible). PIs, declarations and CDATA
 * content are delivered verbatim (CDATA markers stripped). Entities are
 * delivered as HTML_EVENT_ENTITY for the callback to decode.
 * Incomplete constructs (end marker not in this callback) carry their
 * state into the next call. Incomplete tags output the < literally and
 * rescan, to avoid consuming content. */
void
html_scan(const char* text, int size, HTML_SCAN_STATE* state,
          HTML_EVENT_FN emit, void* userdata)
{
    int i = 0;

    while(i < size) {
        /* Inside <![CDATA[ ... ]]>: content verbatim, markers stripped. */
        if(state->in_cdata) {
            int content_start = i;
            while(i+2 < size) {
                if(text[i] == ']' && text[i+1] == ']'
                        && text[i+2] == '>')
                    break;
                i++;
            }
            if(i+2 < size) {
                emit(userdata, HTML_EVENT_CDATA, text + content_start,
                     i - content_start, NULL);
                i += 3;
                state->in_cdata = 0;
            } else {
                /* No ]]> in this callback — deliver remaining text. */
                if(size > content_start)
                    emit(userdata, HTML_EVENT_CDATA, text + content_start,
                         size - content_start, NULL);
                i = size;
            }
            continue;
        }

        /* Inside a <!-- comment: content stays invisible until -->. */
        if(state->in_comment) {
            while(i+2 < size) {
                if(text[i] == '-' && text[i+1] == '-'
                        && text[i+2] == '>')
                    break;
                i++;
            }
            if(i+2 < size) {
                i += 3;
                state->in_comment = 0;
            } else {
                i = size;   /* still inside the comment */
            }
            continue;
        }

        /* Inside a <? ... ?> processing instruction: verbatim. */
        if(state->in_pi) {
            int content_start = i;
            while(i+1 < size) {
                if(text[i] == '?' && text[i+1] == '>')
                    break;
                i++;
            }
            if(i+1 < size) {
                i += 2;
                emit(userdata, HTML_EVENT_PI, text + content_start,
                     i - content_start, NULL);
                state->in_pi = 0;
            } else {
                if(size > content_start)
                    emit(userdata, HTML_EVENT_PI, text + content_start,
                         size - content_start, NULL);
                i = size;
            }
            continue;
        }

        /* Inside a <! ... > declaration: verbatim. */
        if(state->in_decl) {
            int content_start = i;
            while(i < size && text[i] != '>')
                i++;
            if(i < size) {
                i++;
                emit(userdata, HTML_EVENT_DECL, text + content_start,
                     i - content_start, NULL);
                state->in_decl = 0;
            } else {
                if(size > content_start)
                    emit(userdata, HTML_EVENT_DECL, text + content_start,
                         size - content_start, NULL);
                i = size;
            }
            continue;
        }

        if(text[i] == '<') {
            int tag_start = i;
            i++;
            if(i >= size) {
                emit(userdata, HTML_EVENT_TEXT, "<", 1, NULL);
                break;
            }

            /* Determine construct type from the character after <. */
            if(text[i] == '!' && i+1 < size && text[i+1] == '-'
                    && i+2 < size && text[i+2] == '-') {
                /* <!-- comment: scan for -->, content invisible. */
                i += 3;
                while(i+2 < size) {
                    if(text[i] == '-' && text[i+1] == '-'
                            && text[i+2] == '>')
                        break;
                    i++;
                }
                if(i+2 < size) {
                    i += 3;
                } else {
                    /* --> not in this callback: stay invisible until it
                     * arrives in a later callback. */
                    state->in_comment = 1;
                    i = size;
                }
            } else if(text[i] == '?') {
                /* <? processing instruction: output verbatim. */
                int pi_start = tag_start;
                i++;
                while(i+1 < size) {
                    if(text[i] == '?' && text[i+1] == '>')
                        break;
                    i++;
                }
                if(i+1 < size) {
                    i += 2;
                    emit(userdata, HTML_EVENT_PI, text + pi_start,
                         i - pi_start, NULL);
                } else {
                    /* ?> not in this callback: keep the opener and
                     * continue verbatim in later callbacks. */
                    state->in_pi = 1;
                    emit(userdata, HTML_EVENT_PI, text + pi_start,
                         size - pi_start, NULL);
                    i = size;
                }
            } else if(text[i] == '!' && i+7 < size
                    && text[i+1] == '[' && text[i+2] == 'C'
                    && text[i+3] == 'D' && text[i+4] == 'A'
                    && text[i+5] == 'T' && text[i+6] == 'A'
                    && text[i+7] == '[') {
                /* <![CDATA[...]]>: output content between markers. */
                state->in_cdata = 1;
                i += 8;
                {
                    int content_start = i;
                    while(i+2 < size) {
                        if(text[i] == ']' && text[i+1] == ']'
                                && text[i+2] == '>')
                            break;
                        i++;
                    }
                    if(i+2 < size) {
                        emit(userdata, HTML_EVENT_CDATA, text + content_start,
                             i - content_start, NULL);
                        i += 3;
                        state->in_cdata = 0;
                    } else {
                        if(size > content_start)
                            emit(userdata, HTML_EVENT_CDATA, text + content_start,
                                 size - content_start, NULL);
                    }
                }
            } else if(text[i] == '/' || (text[i] >= 'a' && text[i] <= 'z')
                    || (text[i] >= 'A' && text[i] <= 'Z')) {
                /* HTML tag: scan for > tracking quotes. */
                char delim = 0;

                while(i < size) {
                    if(delim) {
                        if(text[i] == delim)
                            delim = 0;
                        i++;
                    } else if(text[i] == '"' || text[i] == '\'') {
                        delim = text[i];
                        i++;
                    } else if(text[i] == '>') {
                        i++;
                        break;
                    } else if(text[i] == '<') {
                        break;
                    } else {
                        i++;
                    }
                }

                if(i <= tag_start + 1 || text[i-1] != '>') {
                    emit(userdata, HTML_EVENT_TEXT, "<", 1, NULL);
                    i = tag_start + 1;
                } else {
                    HTML_TAG_INFO tag_info;
                    html_parse_tag(text + tag_start, i - tag_start, &tag_info);
                    emit(userdata, HTML_EVENT_TAG, text + tag_start,
                         i - tag_start, &tag_info);
                }
            } else if(text[i] == '!') {
                /* <! ... > declaration: output verbatim (it's content). */
                int decl_start = tag_start;
                i++;
                while(i < size && text[i] != '>')
                    i++;
                if(i < size) {
                    i++;
                    emit(userdata, HTML_EVENT_DECL, text + decl_start,
                         i - decl_start, NULL);
                } else {
                    /* > not in this callback: keep the opener and
                     * continue verbatim in later callbacks. */
                    state->in_decl = 1;
                    emit(userdata, HTML_EVENT_DECL, text + decl_start,
                         size - decl_start, NULL);
                    i = size;
                }
            } else {
                emit(userdata, HTML_EVENT_TEXT, "<", 1, NULL);
            }
        } else if(text[i] == '&') {
            /* &...; entity candidate. */
            int ent_start = i;
            i++;
            while(i < size && text[i] != ';' && text[i] != '<'
                    && text[i] != '>' && text[i] != '&'
                    && (i - ent_start) < 32)
                i++;
            if(i < size && text[i] == ';') {
                i++;
                emit(userdata, HTML_EVENT_ENTITY, text + ent_start,
                     i - ent_start, NULL);
            } else {
                emit(userdata, HTML_EVENT_TEXT, text + ent_start,
                     i - ent_start, NULL);
            }
        } else {
            int chunk_start = i;
            while(i < size && text[i] != '<' && text[i] != '&')
                i++;
            emit(userdata, HTML_EVENT_TEXT, text + chunk_start,
                 i - chunk_start, NULL);
        }
    }
}
