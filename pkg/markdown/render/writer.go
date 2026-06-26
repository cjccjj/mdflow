package render

import (
	"github.com/cjccjj/mdflow/pkg/markdown/parser"
)

type Writer struct {
	aw                 *AnsiWriter
	theme              Theme
	live               bool
	tableActive        bool
	inBlockquote       bool
	inBold             bool
	inItalic           bool
	inStrikethrough    bool
	activeHeaderSuffix string
	tableHeader        []string
	tableRows          [][]string
	tableWidths        []int
	tableAligns        []int
	tableLines         int
	termWidth          int
	tableRepaintCount  int
}

func NewWriter(aw *AnsiWriter, theme Theme) *Writer {
	return &Writer{aw: aw, theme: theme}
}

func (w *Writer) SetLive(v bool) {
	w.live = v
}

func (w *Writer) SetTermWidth(width int) {
	w.termWidth = width
}

func (w *Writer) Handle(e parser.Event) error {
	switch e.Type {
	case parser.TextEvent:
		_, err := w.aw.WriteString(e.Value)
		return err

	case parser.NewlineEvent:
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		if w.inBlockquote {
			_, err := w.aw.WriteStyled("│ ", w.theme.Blockquote)
			return err
		}
		return nil

	case parser.BlockquoteStartEvent:
		w.inBlockquote = true
		_, err := w.aw.WriteStyled("│ ", w.theme.Blockquote)
		return err

	case parser.BlockquoteEndEvent:
		w.inBlockquote = false
		return nil

	case parser.HeaderStartEvent:
		style := w.theme.H1
		prefix := ""
		switch e.Level {
		case 2:
			style = w.theme.H2
			prefix = "## "
		case 3:
			style = w.theme.H3
			prefix = "### "
		case 4:
			style = w.theme.H4
			prefix = "#### "
		case 5:
			style = w.theme.H5
			prefix = "##### "
		case 6:
			style = w.theme.H6
			prefix = "###### "
		}
		w.activeHeaderSuffix = style.Suffix
		_, err := w.aw.WriteString(style.Prefix + prefix)
		return err

	case parser.HeaderEndEvent:
		if w.activeHeaderSuffix != "" {
			_, err := w.aw.WriteString(w.activeHeaderSuffix)
			w.activeHeaderSuffix = ""
			return err
		}
		_, err := w.aw.WriteString(w.theme.H1.Suffix)
		return err

	case parser.BoldStartEvent:
		return w.enterEmphasis("bold")

	case parser.BoldEndEvent:
		return w.exitEmphasis("bold")

	case parser.ItalicStartEvent:
		return w.enterEmphasis("italic")

	case parser.ItalicEndEvent:
		return w.exitEmphasis("italic")

	case parser.StrikethroughStartEvent:
		return w.enterEmphasis("strikethrough")

	case parser.StrikethroughEndEvent:
		return w.exitEmphasis("strikethrough")

	case parser.InlineCodeStartEvent:
		_, err := w.aw.WriteString(w.theme.InlineCode.Prefix)
		return err

	case parser.InlineCodeEndEvent:
		_, err := w.aw.WriteString(w.theme.InlineCode.Suffix)
		return err

	case parser.CodeBlockStartEvent:
		_, err := w.aw.WriteString(w.theme.CodeBlock.Prefix)
		return err

	case parser.CodeBlockEndEvent:
		_, err := w.aw.WriteString(w.theme.CodeBlock.Suffix)
		return err

	case parser.CodeBlockLangEvent:
		_, err := w.aw.WriteString(w.theme.CodeBlockLang.Prefix + e.Value + w.theme.CodeBlockLang.Suffix)
		return err

	case parser.HorizontalRuleEvent:
		if _, err := w.aw.WriteString(w.theme.HorizontalRule.Prefix); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("────────────────"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(w.theme.HorizontalRule.Suffix); err != nil {
			return err
		}
		_, err := w.aw.WriteString("\n")
		return err

	case parser.BulletItemEvent:
		if e.Value != "" {
			_, err := w.aw.WriteString(e.Value)
			return err
		}
		_, err := w.aw.WriteString("• ")
		return err

	case parser.TableStartEvent:
		return w.handleTableStart(e)

	case parser.TableRowEvent:
		return w.handleTableRow(e)

	case parser.TableEndEvent:
		return w.handleTableEnd()

	case parser.LinkEvent:
		if _, err := w.aw.WriteString(w.theme.LinkText.Prefix); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(e.Value); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(w.theme.LinkText.Suffix); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(" ("); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(w.theme.LinkURL.Prefix); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(e.URL); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(w.theme.LinkURL.Suffix); err != nil {
			return err
		}
		_, err := w.aw.WriteString(")")
		return err

	case parser.HTMLBlockStartEvent:
		_, err := w.aw.WriteString("\033[2m")
		return err

	case parser.HTMLBlockEndEvent:
		_, err := w.aw.WriteString("\033[0m")
		return err

	case parser.LinkRefDefEvent:
		if _, err := w.aw.WriteString("\033[2m[Link Def: "); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(e.Value); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(" \u2192 "); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(e.URL); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("]\033[0m"); err != nil {
			return err
		}
		return nil

	case parser.LinkRefEvent:
		if _, err := w.aw.WriteString(w.theme.LinkText.Prefix); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(e.Value); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(w.theme.LinkText.Suffix); err != nil {
			return err
		}
		if _, err := w.aw.WriteString(" [\u2192 "); err != nil {
			return err
		}
		label := e.URL
		if label == "" {
			label = "ref"
		}
		if _, err := w.aw.WriteString(label); err != nil {
			return err
		}
		_, err := w.aw.WriteString("]")
		return err

	default:
		return nil
	}
}

func (w *Writer) emphasisActive() bool {
	return w.inBold || w.inItalic || w.inStrikethrough
}

func (w *Writer) emphasisSGR() string {
	codes := make([]string, 0, 3)
	if w.inBold {
		codes = append(codes, "1")
	}
	if w.inItalic {
		codes = append(codes, "3")
	}
	if w.inStrikethrough {
		codes = append(codes, "9")
	}
	if len(codes) == 0 {
		return ""
	}
	sgr := "\033["
	for i, c := range codes {
		if i > 0 {
			sgr += ";"
		}
		sgr += c
	}
	return sgr + "m"
}

func (w *Writer) enterEmphasis(kind string) error {
	hadActive := w.emphasisActive()

	switch kind {
	case "bold":
		w.inBold = true
	case "italic":
		w.inItalic = true
	case "strikethrough":
		w.inStrikethrough = true
	}

	if hadActive {
		if _, err := w.aw.WriteString("\033[0m"); err != nil {
			return err
		}
		sgr := w.emphasisSGR()
		if sgr != "" {
			_, err := w.aw.WriteString(sgr)
			return err
		}
		return nil
	}

	themePrefix := ""
	switch kind {
	case "bold":
		themePrefix = w.theme.Bold.Prefix
	case "italic":
		themePrefix = w.theme.Italic.Prefix
	case "strikethrough":
		themePrefix = w.theme.Strikethrough.Prefix
	}
	_, err := w.aw.WriteString(themePrefix)
	return err
}

func (w *Writer) exitEmphasis(kind string) error {
	switch kind {
	case "bold":
		w.inBold = false
	case "italic":
		w.inItalic = false
	case "strikethrough":
		w.inStrikethrough = false
	}

	stillActive := w.emphasisActive()

	if stillActive {
		if _, err := w.aw.WriteString("\033[0m"); err != nil {
			return err
		}
		sgr := w.emphasisSGR()
		if sgr != "" {
			_, err := w.aw.WriteString(sgr)
			return err
		}
		return nil
	}

	themeSuffix := ""
	switch kind {
	case "bold":
		themeSuffix = w.theme.Bold.Suffix
	case "italic":
		themeSuffix = w.theme.Italic.Suffix
	case "strikethrough":
		themeSuffix = w.theme.Strikethrough.Suffix
	}
	_, err := w.aw.WriteString(themeSuffix)
	return err
}

func (w *Writer) ResetStyles() error {
	w.inBlockquote = false
	w.inBold = false
	w.inItalic = false
	w.inStrikethrough = false
	w.activeHeaderSuffix = ""
	_, err := w.aw.WriteString("\033[0m")
	return err
}
