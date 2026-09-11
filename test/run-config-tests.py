#!/usr/bin/env python3
# Copyright (c) 2026 Changjun Zhang  MIT License (see LICENSE)
"""Regression tests for configuration and plain terminal rendering."""

from pathlib import Path
import os
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
MDFLOW = ROOT / "build" / "mdflow" / "mdflow"
LIB = ROOT / "build" / "src" / "libmdflow.a"


API_DRIVER = r'''
#include <stdio.h>
#include <string.h>

#include "mdflow.h"

static char output[65536];
static int output_len;

static void
collect(const char* str, int size, void* userdata)
{
    (void)userdata;
    if(size < 0 || output_len > (int)sizeof(output) - size)
        return;
    memcpy(output + output_len, str, (size_t)size);
    output_len += size;
    if(output_len < (int)sizeof(output))
        output[output_len] = '\0';
}

static int
count_sequence(const char* sequence, int sequence_len)
{
    int count;
    int i;

    count = 0;
    for(i = 0; i + sequence_len <= output_len; i++) {
        if(memcmp(output + i, sequence, (size_t)sequence_len) == 0)
            count++;
    }
    return count;
}

static int
render(const char* input, const mdflow_config_t* config)
{
    mdflow_t* mf;
    int ret;

    output_len = 0;
    mf = mdflow_open(24, config, collect, NULL);
    if(mf == NULL)
        return -1;
    ret = mdflow_write(mf, input, (int)strlen(input));
    if(ret == 0)
        ret = mdflow_close(mf);
    else
        mdflow_close(mf);
    return ret;
}

static int
render_then_change_config(const char* input, mdflow_config_t* config)
{
    mdflow_t* mf;
    int ret;

    output_len = 0;
    mf = mdflow_open(24, config, collect, NULL);
    if(mf == NULL)
        return -1;
    config->theme = MDFLOW_THEME_DEFAULT;
    config->plain = 0;
    config->osc8 = 1;
    ret = mdflow_write(mf, input, (int)strlen(input));
    if(ret == 0)
        ret = mdflow_close(mf);
    else
        mdflow_close(mf);
    return ret;
}

static int
has_escape(void)
{
    int i;
    for(i = 0; i < output_len; i++) {
        if(output[i] == '\033')
            return 1;
    }
    return 0;
}

int
main(void)
{
    const char* input =
        "## Heading\n\n"
        "> quote **bold**\n\n"
        "[link](https://example.test) ![image](image.png)\n\n"
        "Unicode 你好 😀\n";
    const char* table =
        "| heading | body |\n"
        "|---|---|\n"
        "| one two three four | alpha beta gamma |\n"
        "| last | row |\n";
    mdflow_config_t plain;
    mdflow_config_t copied;
    mdflow_config_t invalid;

    plain.theme = MDFLOW_THEME_DEFAULT;
    plain.plain = 1;
    plain.osc8 = 1;
    copied.theme = MDFLOW_THEME_DEFAULT;
    copied.plain = 1;
    copied.osc8 = 1;
    invalid.theme = (mdflow_theme_t)99;
    invalid.plain = 1;
    invalid.osc8 = 1;

    if(render(input, &plain) != 0 || has_escape())
        return 1;
    if(strstr(output, "link (https://example.test)") == NULL
            || strstr(output, "IMG: image (image.png)") == NULL
            || strstr(output, "Unicode 你好 😀") == NULL)
        return 2;

    if(render(input, NULL) != 0 || !has_escape())
        return 7;

    if(render(table, &plain) != 0 || has_escape()
            || count_sequence("┌", 3) != 1
            || count_sequence("\033[", 2) != 0
            || strstr(output, "heading") == NULL
            || strstr(output, "alpha") == NULL)
        return 3;

    if(mdflow_open(0, &invalid, collect, NULL) != NULL)
        return 4;

    if(render_then_change_config("# copied config\n", &copied) != 0
            || has_escape())
        return 5;

    if(render("literal \033[31mred\033[0m\n", &plain) != 0
            || output_len < 8
            || memcmp(output + 8, "\033[31mred\033[0m", 12) != 0)
        return 6;

    puts("PASS: public config API");
    return 0;
}
'''


RENDERER_DRIVER = r'''
#include <stdio.h>
#include <string.h>

#include "md4cs-ansi.h"

static char output[131072];
static int output_len;
static int output_failed;

static void
collect(const char* str, int size, void* userdata)
{
    (void)userdata;
    if(size < 0 || output_len > (int)sizeof(output) - size - 1) {
        output_failed = 1;
        return;
    }
    memcpy(output + output_len, str, (size_t)size);
    output_len += size;
    output[output_len] = '\0';
}

static int
has_escape(void)
{
    int i;
    for(i = 0; i < output_len; i++) {
        if(output[i] == '\033')
            return 1;
    }
    return 0;
}

static int
render(const MD_ANSI_THEME* theme, int plain)
{
    const char* input =
        "# Heading\n\n"
        "> **outer `code`** ==marked==\n\n"
        "```c\n"
        "int value = \"text\";\n"
        "```\n\n"
        "| heading | body |\n"
        "|---|---|\n"
        "| **cell** | `code` |\n"
        "| longer | tail |\n";
    MD_ANSI_RENDERER* renderer;
    MD_PARSER parser;
    MD_PARSER_CTX* stream_ctx;
    int ret;

    output_len = 0;
    output_failed = 0;
    renderer = md_ansi_renderer_create(theme, collect, NULL);
    if(renderer == NULL)
        return -1;
    md_ansi_renderer_set_plain_mode(renderer, plain);

    memset(&parser, 0, sizeof(parser));
    parser.abi_version = 0;
    parser.flags = MD_DIALECT_GITHUB | MD_FLAG_HIGHLIGHT;
    parser.enter_block = md_ansi_enter_block;
    parser.leave_block = md_ansi_leave_block;
    parser.enter_span = md_ansi_enter_span;
    parser.leave_span = md_ansi_leave_span;
    parser.text = md_ansi_text;

    stream_ctx = NULL;
    ret = md_stream_init(&parser, renderer, &stream_ctx);
    if(ret == 0)
        ret = md_stream_feed(stream_ctx, (const MD_CHAR*)input,
                             (MD_SIZE)strlen(input));
    if(ret == 0)
        ret = md_stream_flush(stream_ctx);
    if(stream_ctx != NULL)
        md_stream_finish(stream_ctx);
    md_ansi_renderer_destroy(renderer);
    if(output_failed)
        return -2;
    return ret;
}

int
main(void)
{
    if(render(md_ansi_default_theme(), 1) != 0 || has_escape()
            || strstr(output, "outer code") == NULL
            || strstr(output, "┌") == NULL)
        return 1;

    puts("PASS: direct renderer plain mode");
    return 0;
}
'''


def run_cli(args, data=b"", env=None):
    merged = dict(os.environ)
    if env is not None:
        merged.update(env)
    return subprocess.run(
        [str(MDFLOW), *args], input=data, stdout=subprocess.PIPE,
        stderr=subprocess.PIPE, env=merged)


def check(condition, label, detail=b""):
    if condition:
        print("PASS: " + label)
        return True
    suffix = (": " + repr(detail)) if detail else ""
    print("FAIL: " + label + suffix)
    return False


def test_api():
    if not LIB.is_file():
        print("FAIL: public config API library not found: " + str(LIB))
        return False
    with tempfile.TemporaryDirectory() as temp_dir:
        source = Path(temp_dir) / "config-api.c"
        binary = Path(temp_dir) / "config-api"
        source.write_text(API_DRIVER, encoding="utf-8")
        compile_proc = subprocess.run([
            os.environ.get("CC", "gcc"), "-std=c99", "-Wall", "-Wextra",
            "-Wshadow", "-Wdeclaration-after-statement", "-DMD4C_STREAMING", "-I",
            str(ROOT / "src"), str(source), str(LIB), "-o", str(binary),
        ], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if compile_proc.returncode != 0:
            print("FAIL: public config API compile: "
                  + compile_proc.stderr.decode(errors="replace"))
            return False
        proc = subprocess.run([str(binary)], stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE)
        detail = proc.stderr or proc.stdout
        if not detail:
            detail = ("exit " + str(proc.returncode)).encode()
        return check(proc.returncode == 0, "public config API", detail)


def test_renderer():
    if not LIB.is_file():
        print("FAIL: direct renderer library not found: " + str(LIB))
        return False
    with tempfile.TemporaryDirectory() as temp_dir:
        source = Path(temp_dir) / "renderer-plain.c"
        binary = Path(temp_dir) / "renderer-plain"
        source.write_text(RENDERER_DRIVER, encoding="utf-8")
        compile_proc = subprocess.run([
            os.environ.get("CC", "gcc"), "-std=c99", "-Wall", "-Wextra",
            "-Wshadow", "-Wdeclaration-after-statement", "-DMD4C_STREAMING", "-I",
            str(ROOT / "src"), str(source), str(LIB), "-o", str(binary),
        ], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        if compile_proc.returncode != 0:
            print("FAIL: direct renderer compile: "
                  + compile_proc.stderr.decode(errors="replace"))
            return False
        proc = subprocess.run([str(binary)], stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE)
        detail = proc.stderr or proc.stdout
        if not detail:
            detail = ("exit " + str(proc.returncode)).encode()
        return check(proc.returncode == 0, "direct renderer plain mode", detail)


def test_cli():
    sample = (
        "## Heading\n\n"
        "> quote **bold**\n\n"
        "- item\n\n"
        "[link](https://example.test) ![image](image.png)\n\n"
        "Unicode 你好 😀\n"
    ).encode("utf-8")
    default_output = run_cli(["--typewriter-off"], sample)
    term256_output = run_cli(["--theme-term-256", "--typewriter-off"], sample)
    plain = run_cli(["--typewriter-off", "--plain"], sample)

    results = []
    help_proc = run_cli(["--help"])
    results.append(check(help_proc.returncode == 0
                         and all(flag in help_proc.stderr for flag in
                                  (b"--theme-term-256", b"--plain"))
                         and help_proc.stderr.count(b"--theme-") == 1,
                         "help lists the alternate theme and plain mode"))
    results.append(check(default_output.returncode == 0
                         and b"\033" in default_output.stdout,
                         "default theme is selected without a theme option",
                         default_output.stdout))
    results.append(check(term256_output.returncode == 0
                         and b"\033" in term256_output.stdout,
                         "TERM_256 theme option is accepted",
                         term256_output.stdout))
    results.append(check(plain.returncode == 0 and b"\033" not in plain.stdout,
                         "plain sample has no renderer-generated ESC bytes",
                         plain.stdout))
    results.append(check(b"## Heading" in plain.stdout
                         and b"|" not in plain.stdout
                         and b"quote bold" in plain.stdout
                         and b"link (https://example.test)" in plain.stdout
                         and b"IMG: image (image.png)" in plain.stdout
                         and "你好 😀".encode("utf-8") in plain.stdout,
                         "plain preserves structure, links, images, and Unicode",
                         plain.stdout))

    table = (
        "| heading | body |\n"
        "|---|---|\n"
        "| one two three four | alpha beta gamma |\n"
        "| last | row |\n"
    ).encode("utf-8")
    table_proc = run_cli(["--typewriter-off", "--plain"], table,
                         {"COLUMNS": "24"})
    table_text = table_proc.stdout.decode("utf-8", errors="replace")
    table_start = table_text.find("┌")
    table_lines = table_text[table_start:].splitlines() if table_start >= 0 else []
    width = len(table_lines[0]) if table_lines else 0
    results.append(check(table_proc.returncode == 0 and table_start >= 0
                         and table_text.count("┌") == 1
                         and "\033[" not in table_text
                         and all(len(line) == width for line in table_lines
                                 if line),
                         "plain table is rendered once without cursor redraw",
                         table_proc.stdout))

    tiny = b"tiny\n"
    paced = run_cli(["--plain", "--report"], tiny)
    unpaced = run_cli(["--plain", "--typewriter-off"], tiny)
    results.append(check(paced.returncode == 0 and unpaced.returncode == 0
                         and paced.stdout == unpaced.stdout
                         and b"bypass=on" in paced.stderr,
                         "plain leaves typewriter selection unchanged"))

    literal = b"literal \033[31mred\033[0m\n"
    literal_proc = run_cli(["--plain", "--typewriter-off"], literal)
    results.append(check(literal_proc.returncode == 0
                         and literal_proc.stdout.startswith(literal),
                         "plain preserves literal input ESC bytes",
                         literal_proc.stdout))

    redundant = run_cli(["--plain", "--osc8-off", "--typewriter-off"],
                        b"[x](https://e.test)\n")
    results.append(check(redundant.returncode == 0
                         and b"(https://e.test)" in redundant.stdout,
                         "--plain --osc8-off remains accepted"))

    for args in (("--theme-term-256", "--plain"),
                 ("--plain", "--theme-term-256")):
        proc = run_cli([*args, "--typewriter-off"], b"")
        results.append(check(proc.returncode != 0
                             and b"conflicting theme options" in proc.stderr,
                             "conflicting rendering selectors are rejected"))

    unknown = run_cli(["--safety-filter", "--typewriter-off"], b"")
    results.append(check(unknown.returncode != 0
                         and b"unknown option" in unknown.stderr,
                         "no safety-filter option is exposed"))
    return all(results)


def main():
    if not MDFLOW.is_file():
        print("error: mdflow binary not found: " + str(MDFLOW))
        return 1
    return 0 if test_api() and test_renderer() and test_cli() else 1


if __name__ == "__main__":
    sys.exit(main())
