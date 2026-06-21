package markdown

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func renderOneShot(input string) string {
	var buf bytes.Buffer
	r := NewRenderer(&buf)
	r.Write([]byte(input))
	r.Close()
	return buf.String()
}

func renderLineChunked(input string) string {
	var buf bytes.Buffer
	r := NewRenderer(&buf)
	for _, line := range splitAtNewlines(input) {
		r.Write([]byte(line))
	}
	r.Close()
	return buf.String()
}

func renderSplitAt(input string, positions ...int) string {
	var buf bytes.Buffer
	r := NewRenderer(&buf)
	prev := 0
	for _, pos := range positions {
		if pos > len(input) {
			pos = len(input)
		}
		if pos > prev {
			r.Write([]byte(input[prev:pos]))
		}
		prev = pos
	}
	if prev < len(input) {
		r.Write([]byte(input[prev:]))
	}
	r.Close()
	return buf.String()
}

func splitAtNewlines(input string) []string {
	if len(input) == 0 {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(input); i++ {
		if input[i] == '\n' {
			lines = append(lines, input[start:i+1])
			start = i + 1
		}
	}
	if start < len(input) {
		lines = append(lines, input[start:])
	}
	return lines
}

type goldenCase struct {
	name     string
	input    string
	wantText []string
	wantANSI []string
}

var goldenCases = []goldenCase{
	{
		name:     "bold",
		input:    "hello **world** today",
		wantText: []string{"hello world today"},
		wantANSI: []string{"\033[1m", "\033[0m"},
	},
	{
		name:     "italic_star",
		input:    "hello *world* today",
		wantText: []string{"hello world today"},
		wantANSI: []string{"\033[3m", "\033[0m"},
	},
	{
		name:     "italic_underscore",
		input:    "hello _world_ today",
		wantText: []string{"hello world today"},
		wantANSI: []string{"\033[3m", "\033[0m"},
	},
	{
		name:     "bold_underscore",
		input:    "hello __world__ today",
		wantText: []string{"hello world today"},
		wantANSI: []string{"\033[1m", "\033[0m"},
	},
	{
		name:     "strikethrough",
		input:    "hello ~~world~~ today",
		wantText: []string{"hello world today"},
		wantANSI: []string{"\033[9m", "\033[0m"},
	},
	{
		name:     "header_h1",
		input:    "# Title\n",
		wantText: []string{"Title"},
		wantANSI: []string{"\033[1;48;5;63;38;5;228m", "\033[0m"},
	},
	{
		name:     "header_h2",
		input:    "## Title\n",
		wantText: []string{"## Title"},
		wantANSI: []string{"\033[1;34m", "\033[0m"},
	},
	{
		name:     "header_h3",
		input:    "### Title\n",
		wantText: []string{"### Title"},
		wantANSI: []string{"\033[1;33m", "\033[0m"},
	},
	{
		name:     "header_h4",
		input:    "#### Title\n",
		wantText: []string{"#### Title"},
		wantANSI: []string{"\033[1;36m", "\033[0m"},
	},
	{
		name:     "header_h5",
		input:    "##### Title\n",
		wantText: []string{"##### Title"},
		wantANSI: []string{"\033[1;35m", "\033[0m"},
	},
	{
		name:     "header_h6",
		input:    "###### Title\n",
		wantText: []string{"###### Title"},
		wantANSI: []string{"\033[1;32m", "\033[0m"},
	},
	{
		name:     "inline_code",
		input:    "use `code` here",
		wantText: []string{"use code here"},
		wantANSI: []string{"\033[38;5;215;48;5;236m", "\033[0m"},
	},
	{
		name:     "inline_code_double_backtick",
		input:    "use ``co`de`` here",
		wantText: []string{"use co`de here"},
		wantANSI: []string{"\033[38;5;215;48;5;236m", "\033[0m"},
	},
	{
		name:     "code_block_fenced",
		input:    "```\ncode\n```\n",
		wantText: []string{"code"},
		wantANSI: []string{"\033[38;5;245m", "\033[0m"},
	},
	{
		name:     "code_block_tilde",
		input:    "~~~\ncode\n~~~\n",
		wantText: []string{"code"},
		wantANSI: []string{"\033[38;5;245m", "\033[0m"},
	},
	{
		name:     "code_block_with_lang",
		input:    "```go\nfunc main() {}\n```\n",
		wantText: []string{"go", "func main() {}"},
		wantANSI: []string{"\033[3;38;5;221m", "\033[38;5;245m", "\033[0m"},
	},
	{
		name:     "bullet_dash",
		input:    "- item\n",
		wantText: []string{"• item"},
		wantANSI: nil,
	},
	{
		name:     "bullet_star",
		input:    "* item\n",
		wantText: []string{"• item"},
		wantANSI: nil,
	},
	{
		name:     "ordered_list",
		input:    "1. item\n",
		wantText: []string{"1. item"},
		wantANSI: nil,
	},
	{
		name:     "ordered_list_paren",
		input:    "1) item\n",
		wantText: []string{"1) item"},
		wantANSI: nil,
	},
	{
		name:     "horizontal_rule_dash",
		input:    "---\n",
		wantText: []string{"────────────────"},
		wantANSI: []string{"\033[2m", "\033[0m"},
	},
	{
		name:     "horizontal_rule_star",
		input:    "***\n",
		wantText: []string{"────────────────"},
		wantANSI: []string{"\033[2m", "\033[0m"},
	},
	{
		name:     "horizontal_rule_underscore",
		input:    "___\n",
		wantText: []string{"────────────────"},
		wantANSI: []string{"\033[2m", "\033[0m"},
	},
	{
		name:     "blockquote",
		input:    "> quoted\n",
		wantText: []string{"│ quoted"},
		wantANSI: []string{"\033[2m", "\033[0m"},
	},
	{
		name:     "blockquote_multiline",
		input:    "> line1\n> line2\n",
		wantText: []string{"│ line1", "│ line2"},
		wantANSI: []string{"\033[2m", "\033[0m"},
	},
	{
		name:     "table_simple",
		input:    "| a | b |\n|---|---|\n| 1 | 2 |\n",
		wantText: []string{"a", "b", "1", "2", "┌", "┐", "└", "┘"},
		wantANSI: []string{"\033[2m", "\033[1m", "\033[0m"},
	},
	{
		name:     "hard_line_break",
		input:    "line1\\\nline2\n",
		wantText: []string{"line1", "line2"},
		wantANSI: nil,
	},
	{
		name:     "paragraph_spacing",
		input:    "para1\n\npara2\n",
		wantText: []string{"para1", "para2"},
		wantANSI: nil,
	},
	{
		name:     "mixed_bold_and_italic",
		input:    "**bold** and *italic*",
		wantText: []string{"bold and italic"},
		wantANSI: []string{"\033[1m", "\033[3m", "\033[0m"},
	},
	{
		name:     "backslash_escape_star",
		input:    "hello \\*world\\*",
		wantText: []string{"hello *world*"},
		wantANSI: nil,
	},
	{
		name:     "backslash_escape_hash",
		input:    "\\# not a header",
		wantText: []string{"# not a header"},
		wantANSI: nil,
	},
}

func TestGolden_OneShot(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			out := renderOneShot(tc.input)
			plain := stripANSI(out)

			for _, want := range tc.wantText {
				if !strings.Contains(plain, want) {
					t.Errorf("plain text missing %q\ninput: %q\nplain: %q", want, tc.input, plain)
				}
			}
			for _, want := range tc.wantANSI {
				if !strings.Contains(out, want) {
					t.Errorf("ANSI marker missing %q\ninput: %q\noutput: %q", want, tc.input, out)
				}
			}
		})
	}
}

func TestGolden_StreamingConsistency(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			oneShot := renderOneShot(tc.input)
			oneShotPlain := stripANSI(oneShot)

			lineChunked := renderLineChunked(tc.input)
			linePlain := stripANSI(lineChunked)

			if linePlain != oneShotPlain {
				t.Errorf("line-chunked: plain text differs from one-shot\ninput: %q\none-shot: %q\nchunked:  %q",
					tc.input, oneShotPlain, linePlain)
			}
			for _, want := range tc.wantANSI {
				if !strings.Contains(lineChunked, want) {
					t.Errorf("line-chunked: ANSI marker %q missing\ninput: %q\noutput: %q",
						want, tc.input, lineChunked)
				}
			}
		})
	}
}

var boundaryCases = []struct {
	name   string
	input  string
	splits []int
	desc   string
}{
	{"bold_mid_token", "**hello**", []int{1}, "split inside **"},
	{"bold_mid_word", "hello **world**", []int{8}, "split right after leading **"},
	{"italic_mid_word", "hello *world*", []int{7}, "split inside italic"},
	{"inline_code_mid", "use `code` here", []int{5}, "split inside inline code"},
	{"code_fence_open", "```\ncode\n```\n", []int{3}, "split at closing ```"},
	{"code_tilde_mid", "~~~\ntext\n~~~\n", []int{5}, "split inside tilde code block"},
	{"table_header_mid", "| Name |\n|------|\n| val  |\n", []int{3}, "split mid header cell"},
	{"table_row_mid", "| a | b |\n|---|---|\n| 1 | 2 |\n", []int{16}, "split mid body cell"},
	{"header_hash_space", "# Title\n", []int{2}, "split after '# '"},
	{"bullet_mid", "- item\n", []int{2}, "split after '- '"},
}

func TestGolden_CrossBoundary(t *testing.T) {
	for _, tc := range boundaryCases {
		t.Run(tc.name, func(t *testing.T) {
			oneShot := renderOneShot(tc.input)
			oneShotPlain := stripANSI(oneShot)

			chunked := renderSplitAt(tc.input, tc.splits...)
			chunkedPlain := stripANSI(chunked)

			if chunkedPlain != oneShotPlain {
				t.Errorf("%s: plain text differs\ninput: %q\nsplits: %v\none-shot: %q\nchunked:  %q",
					tc.desc, tc.input, tc.splits, oneShotPlain, chunkedPlain)
			}
		})
	}
}

func TestGolden_ANSIPairing(t *testing.T) {
	pairCases := []struct {
		input  string
		opens  []string
	}{
		{"**text**", []string{"\033[1m"}},
		{"*text*", []string{"\033[3m"}},
		{"~~text~~", []string{"\033[9m"}},
		{"`text`", []string{"\033[38;5;215;48;5;236m"}},
		{"```\ntext\n```\n", []string{"\033[38;5;245m"}},
		{"# text\n", []string{"\033[1;48;5;63;38;5;228m"}},
		{"---\n", []string{"\033[2m"}},
		{"> text\n", []string{"\033[2m"}},
	}

	for _, tc := range pairCases {
		t.Run(tc.input, func(t *testing.T) {
			out := renderOneShot(tc.input)

			for _, open := range tc.opens {
				opens := strings.Count(out, open)
				closes := strings.Count(out, "\033[0m")
				if opens == 0 || closes == 0 {
					continue
				}

				firstOpen := strings.Index(out, open)
				firstClose := strings.Index(out, "\033[0m")
				if firstOpen > firstClose {
					t.Errorf("reset before open:\n%s", out)
				}

				lastOpen := strings.LastIndex(out, open)
				lastClose := strings.LastIndex(out, "\033[0m")
				if lastOpen > lastClose {
					t.Errorf("unclosed style (last open after last close):\n%s", out)
				}
			}
		})
	}
}

func TestGolden_ResetCleanup(t *testing.T) {
	resetCases := []string{
		"**unclosed",
		"*unclosed",
		"`unclosed",
		"```\nunclosed",
		"~~unclosed",
	}

	for _, input := range resetCases {
		t.Run(input, func(t *testing.T) {
			out := renderOneShot(input)
			seqs := ansiRe.FindAllString(out, -1)

			depth := 0
			for _, s := range seqs {
				if s == "\033[0m" {
					depth--
				} else {
					depth++
				}
			}
			if depth > 0 {
				t.Errorf("unclosed ANSI styles: %d open remain\noutput: %q", depth, out)
			}
		})
	}
}

func TestGolden_UnderlineItalic(t *testing.T) {
	for _, input := range []string{"_italic_", "hello _world_"} {
		out := renderOneShot(input)
		if !strings.Contains(out, "\033[3m") {
			t.Errorf("italic underscore not styled: input=%q output=%q", input, out)
		}
		if !strings.Contains(out, "\033[0m") {
			t.Errorf("missing reset: input=%q output=%q", input, out)
		}
	}
}

func TestGolden_UnderlineBold(t *testing.T) {
	for _, input := range []string{"hello __world__"} {
		out := renderOneShot(input)
		if !strings.Contains(out, "\033[1m") {
			t.Errorf("bold underscore not styled: input=%q output=%q", input, out)
		}
		if !strings.Contains(out, "\033[0m") {
			t.Errorf("missing reset: input=%q output=%q", input, out)
		}
	}
}
