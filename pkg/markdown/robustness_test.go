package markdown

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cjccjj/mdflow/pkg/markdown/render"
)

// ---- helper ----

func renderOutput(input string) string {
	var buf bytes.Buffer
	r := NewRenderer(&buf)
	r.Write([]byte(input))
	r.Close()
	return buf.String()
}

func renderStreaming(chunks ...string) string {
	var buf bytes.Buffer
	r := NewRenderer(&buf)
	for _, c := range chunks {
		r.Write([]byte(c))
	}
	r.Close()
	return buf.String()
}

// ---- supported tags render with ANSI ----

func TestRobustness_Bold(t *testing.T) {
	out := renderOutput("aaa **bold** bbb")
	if !strings.Contains(out, "\033[1mbold\033[0m") {
		t.Errorf("bold not styled: %q", out)
	}
	if !strings.Contains(out, "aaa") && !strings.Contains(out, "bbb") {
		t.Errorf("surrounding text lost: %q", out)
	}
}

func TestRobustness_Headers(t *testing.T) {
	tests := []string{
		"# H1\n",
		"## H2\n",
		"### H3\n",
	}
	for _, input := range tests {
		out := renderOutput(input)
		if !strings.Contains(out, "\033[") {
			t.Errorf("header not styled: input=%q output=%q", input, out)
		}
	}
}

func TestRobustness_InlineCode(t *testing.T) {
	out := renderOutput("use `code` here\n")
	if !strings.Contains(out, "\033[38;5;215;48;5;236mcode\033[0m") {
		t.Errorf("inline code not styled: %q", out)
	}
}

func TestRobustness_CodeBlock(t *testing.T) {
	out := renderOutput("```\ncode\n```\n")
	if !strings.Contains(out, "\033[38;5;245m") {
		t.Errorf("code block not styled: %q", out)
	}
}

func TestRobustness_Bullets(t *testing.T) {
	out := renderOutput("- item\n* another\n")
	if !strings.Contains(out, "item") || !strings.Contains(out, "another") {
		t.Errorf("bullet text missing: %q", out)
	}
}

// ---- unsupported tags print raw text, no crash ----

func TestRobustness_TableRenders(t *testing.T) {
	out := renderOutput("| a | b |\n|---|---|\n| 1 | 2 |\n")
	if !strings.Contains(out, "┌") {
		t.Errorf("table top border missing: %q", out)
	}
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Errorf("header cells missing: %q", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") {
		t.Errorf("body cells missing: %q", out)
	}
	if !strings.Contains(out, "└") {
		t.Errorf("table bottom border missing: %q", out)
	}
}

func TestRobustness_TableFalseAlarm(t *testing.T) {
	out := renderOutput("| not a table\n")
	if !strings.Contains(out, "| not a table") {
		t.Errorf("false alarm table should print raw: %q", out)
	}
}

func TestRobustness_TableStreaming(t *testing.T) {
	out := renderStreaming("| a | b |\n", "|---|---|\n", "| 1 | 2 |\n")
	if !strings.Contains(out, "┌") {
		t.Errorf("streaming table top border missing: %q", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") {
		t.Errorf("streaming body cells missing: %q", out)
	}
}

func TestRobustness_TableWithInlineStyles(t *testing.T) {
	out := renderOutput("| **bold** | *italic* |\n|------|--------|\n| x | y |\n")
	if !strings.Contains(out, "bold") {
		t.Errorf("bold cell missing: %q", out)
	}
	if !strings.Contains(out, "italic") {
		t.Errorf("italic cell missing: %q", out)
	}
}

func TestRobustness_TableAlignLeft(t *testing.T) {
	out := renderOutput("| **bold** | *italic* |\n|:-------|:--------|\n| x | y |\n")
	if !strings.Contains(out, "bold") {
		t.Errorf("bold cell missing: %q", out)
	}
	if !strings.Contains(out, "italic") {
		t.Errorf("italic cell missing: %q", out)
	}
}

func TestRobustness_TableAlignCenter(t *testing.T) {
	out := renderOutput("| a |\n|:---:|\n| b |\n")
	if !strings.Contains(out, "a") && !strings.Contains(out, "b") {
		t.Errorf("table with center alignment: %q", out)
	}
	if !strings.Contains(out, "┌") {
		t.Errorf("center align table top border missing: %q", out)
	}
}

func TestRobustness_TableAlignRight(t *testing.T) {
	out := renderOutput("| a |\n|---:|\n| b |\n")
	if !strings.Contains(out, "a") && !strings.Contains(out, "b") {
		t.Errorf("table with right alignment: %q", out)
	}
	if !strings.Contains(out, "┌") {
		t.Errorf("right align table top border missing: %q", out)
	}
}

func TestRobustness_TableAlignMixed(t *testing.T) {
	out := renderOutput("| a | b | c |\n|:---|---:|:---:|\n| 1 | 2 | 3 |\n")
	if !strings.Contains(out, "┌") {
		t.Errorf("mixed align table top border missing: %q", out)
	}
	if !strings.Contains(out, "1") || !strings.Contains(out, "2") || !strings.Contains(out, "3") {
		t.Errorf("mixed align body cells missing: %q", out)
	}
}

func TestRobustness_TableWrappedContent(t *testing.T) {
	out := renderOutput("| Name | Description |\n|------|-------------|\n| A    | a long description that wraps |\n")
	if !strings.Contains(out, "┌") {
		t.Errorf("wrapped table top border missing: %q", out)
	}
	if !strings.Contains(out, "long") {
		t.Errorf("wrapped content missing: %q", out)
	}
}

func TestRobustness_TableWrappedBold(t *testing.T) {
	out := renderOutput("| a |\n|---|\n| **bold text that wraps** |\n")
	if !strings.Contains(out, "bold") {
		t.Errorf("bold wrapped content missing: %q", out)
	}
	if !strings.Contains(out, "\033[1m") {
		t.Errorf("bold style missing: %q", out)
	}
}

func TestRobustness_TableWrappedEmoji(t *testing.T) {
	out := renderOutput("| a |\n|---|\n| hello 🟢 world |\n")
	if !strings.Contains(out, "hello") {
		t.Errorf("emoji wrapped content missing: %q", out)
	}
	if !strings.Contains(out, "┌") {
		t.Errorf("emoji table border missing: %q", out)
	}
}

func TestRobustness_TableWrappedStreaming(t *testing.T) {
	out := renderStreaming(
		"| a | b |\n",
		"|---|---|\n",
		"| short | a very long cell that wraps |\n",
		"| x | y |\n",
	)
	if !strings.Contains(out, "wraps") {
		t.Errorf("streaming wrapped content missing: %q", out)
	}
	if !strings.Contains(out, "┌") {
		t.Errorf("streaming wrapped table border missing: %q", out)
	}
	if !strings.Contains(out, "short") {
		t.Errorf("streaming short cell missing: %q", out)
	}
}

func TestRobustness_TableWrapAlign(t *testing.T) {
	out := renderOutput("| left | center | right |\n|:-----|:------:|------:|\n| a long text | b long text | c long text |\n")
	if !strings.Contains(out, "┌") {
		t.Errorf("wrap+align table border missing: %q", out)
	}
}

func TestRobustness_BlockquoteStyled(t *testing.T) {
	out := renderOutput("> quoted text\n")
	if !strings.Contains(out, "quoted text") {
		t.Errorf("blockquote text missing: %q", out)
	}
	if !strings.Contains(out, "│") {
		t.Errorf("blockquote prefix missing: %q", out)
	}
}

func TestRobustness_LinkPrintsRaw(t *testing.T) {
	out := renderOutput("see [link](http://example.com) here\n")
	if !strings.Contains(out, "\033[4;34m") {
		t.Errorf("link style not applied: %q", out)
	}
	if !strings.Contains(out, "link") {
		t.Errorf("link text missing: %q", out)
	}
	if !strings.Contains(out, "http://example.com") {
		t.Errorf("link url missing: %q", out)
	}
}

func TestRobustness_ImagePrintsRaw(t *testing.T) {
	out := renderOutput("![alt](img.png)\n")
	if !strings.Contains(out, "![alt](img.png)") {
		t.Errorf("image syntax lost: %q", out)
	}
}

func TestRobustness_HTMLPrintsRaw(t *testing.T) {
	out := renderOutput("<b>html bold</b>\n")
	if !strings.Contains(out, "<b>html bold</b>") {
		t.Errorf("html text lost: %q", out)
	}
}

func TestRobustness_ItalicStyled(t *testing.T) {
	out := renderOutput("this is *italic* text\n")
	if !strings.Contains(out, "\033[3mitalic\033[0m") {
		t.Errorf("italic not styled: %q", out)
	}
}

func TestRobustness_HorizontalRuleStyled(t *testing.T) {
	out := renderOutput("text\n\n---\nmore text\n")
	if !strings.Contains(out, "\033[2m────────────────\033[0m") {
		t.Errorf("horizontal rule not styled: %q", out)
	}
}

// ---- mixed content: supported + unsupported together ----

func TestRobustness_MixedSupportedAndUnsupported(t *testing.T) {
	input := "# Header\n\nThis is **bold** and [a link](http://x.com) and `code`.\n\n- item\n\n> blockquote\n"
	out := renderOutput(input)

	// Supported should be styled
	if !strings.Contains(out, "\033[1mbold\033[0m") {
		t.Errorf("bold not styled: %q", out)
	}

	// Link should now be styled
	if !strings.Contains(out, "\033[4;34m") {
		t.Errorf("link not styled: %q", out)
	}
	if !strings.Contains(out, "http://x.com") {
		t.Errorf("link url missing: %q", out)
	}
	if !strings.Contains(out, "blockquote") {
		t.Errorf("blockquote not preserved: %q", out)
	}
}

// ---- edge cases: no crash, clean output ----

func TestRobustness_Empty(t *testing.T) {
	out := renderOutput("")
	expected := "\033[0m"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestRobustness_LoneHash(t *testing.T) {
	out := renderOutput("#")
	expected := "#\033[0m"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestRobustness_LoneStar(t *testing.T) {
	out := renderOutput("*")
	expected := "*\033[0m"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestRobustness_LoneDash(t *testing.T) {
	out := renderOutput("-")
	expected := "-\033[0m"
	if out != expected {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestRobustness_LoneBacktick(t *testing.T) {
	out := renderOutput("`")
	// Single backtick starts inline code, Close() ends it — should have inline code style
	if !strings.Contains(out, "\033[38;5;215;48;5;236m") {
		t.Errorf("expected inline code style: %q", out)
	}
}

func TestRobustness_NestedBoldInsideBold(t *testing.T) {
	out := renderOutput("****text****")
	if !strings.Contains(out, "text") {
		t.Errorf("text missing: %q", out)
	}
}

func TestRobustness_BrokenBold(t *testing.T) {
	out := renderOutput("**unclosed")
	if !strings.Contains(out, "unclosed") {
		t.Errorf("text missing: %q", out)
	}
}

func TestRobustness_BoldInsideBullet(t *testing.T) {
	out := renderOutput("- **bold item**\n")
	if !strings.Contains(out, "bold item") {
		t.Errorf("text missing: %q", out)
	}
}

func TestRobustness_CodeInBullet(t *testing.T) {
	out := renderOutput("- `code item`\n")
	if !strings.Contains(out, "code item") {
		t.Errorf("text missing: %q", out)
	}
}

func TestRobustness_NewlinesOnly(t *testing.T) {
	out := renderOutput("\n\n\n")
	if strings.Count(out, "\n") != 3 {
		t.Errorf("expected 3 newlines, got %q", out)
	}
}

func TestRobustness_UnclosedCodeBlock(t *testing.T) {
	out := renderOutput("```\ncode without closing")
	if !strings.Contains(out, "code without closing") {
		t.Errorf("text missing: %q", out)
	}
}

// ---- streaming edge cases ----

func TestRobustness_StreamingLoneChars(t *testing.T) {
	out := renderStreaming("#", " ", "hello\n", "**", "bold", "**", "\n")
	if !strings.Contains(out, "hello") {
		t.Errorf("hello missing: %q", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("bold missing: %q", out)
	}
}

func TestRobustness_StreamingMixed(t *testing.T) {
	out := renderStreaming(
		"# He",
		"ader\n\n",
		"this is **bo",
		"ld** and `in",
		"line` code\n",
		"- bullet\n",
		"> quote\n",
		"| table |\n",
	)
	if !strings.Contains(out, "Header") {
		t.Errorf("header missing: %q", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("bold missing: %q", out)
	}
	if !strings.Contains(out, "quote") {
		t.Errorf("quote missing: %q", out)
	}
	if !strings.Contains(out, "| table |") {
		t.Errorf("table missing: %q", out)
	}
}

func TestRobustness_StreamingUTF8Split(t *testing.T) {
	// The snowman rune ☃ is 3 bytes: e2 98 83
	// We stream it byte by byte
	out := renderStreaming("\xe2", "\x98", "\x83")
	if !strings.Contains(out, "☃") {
		t.Errorf("expected snowman '☃', got %q", out)
	}
	if strings.Contains(out, "\ufffd") {
		t.Errorf("found replacement character in output: %q", out)
	}
}

// ---- large input doesn't crash or hang ----

func TestRobustness_LargeInput(t *testing.T) {
	var input strings.Builder
	for i := 0; i < 10000; i++ {
		input.WriteString("line ")
		input.WriteString(string(rune('0' + (i % 10))))
		if i%10 == 0 {
			input.WriteString("\n")
		}
	}
	out := renderOutput(input.String())
	if len(out) == 0 {
		t.Error("no output for large input")
	}
}

// ---- custom theme still works ----

func TestRobustness_CustomTheme(t *testing.T) {
	custom := render.DefaultTheme
	custom.Bold = render.Style{Prefix: "[[[", Suffix: "]]]"}

	var buf bytes.Buffer
	r := NewRenderer(&buf, WithTheme(custom))
	r.Write([]byte("**test**"))
	r.Close()

	output := buf.String()
	if !strings.Contains(output, "[[[test]]]") {
		t.Errorf("custom bold not applied: %q", output)
	}
}

// ---- panic recovery: invalid states shouldn't crash ----

func TestRobustness_ManyCloseBold(t *testing.T) {
	// Lots of ** pairs — just verify no crash
	out := renderOutput("******")
	if len(out) == 0 {
		t.Error("empty output")
	}
}

func TestRobustness_BacktickConfusion(t *testing.T) {
	// Mixed single and triple backticks
	out := renderOutput("`inline` and ```\nblock\n``` and `more`")
	if !strings.Contains(out, "inline") {
		t.Errorf("inline missing: %q", out)
	}
	if !strings.Contains(out, "block") {
		t.Errorf("block missing: %q", out)
	}
	if !strings.Contains(out, "more") {
		t.Errorf("more missing: %q", out)
	}
}

func TestRobustness_OnlySpecialChars(t *testing.T) {
	out := renderOutput("* ` # - | > [ ] ( ) ! ~")
	if len(out) == 0 {
		t.Error("empty output for special chars")
	}
}

func TestRobustness_SingleChunkManyPatterns(t *testing.T) {
	// All patterns in one chunk - nothing should hang
	out := renderOutput("# h1\n## h2\n**b**\n`c`\n```\nblock\n```\n- bullet\n")
	if len(out) == 0 {
		t.Error("empty output")
	}
}
