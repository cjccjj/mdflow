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
	activeHeaderSuffix string
	tableHeader        []string
	tableRows          [][]string
	tableWidths        []int
	tableLines         int
}

func NewWriter(aw *AnsiWriter, theme Theme) *Writer {
	return &Writer{aw: aw, theme: theme}
}

func (w *Writer) SetLive(v bool) {
	w.live = v
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
		_, err := w.aw.WriteString(w.theme.Bold.Prefix)
		return err

	case parser.BoldEndEvent:
		_, err := w.aw.WriteString(w.theme.Bold.Suffix)
		return err

	case parser.ItalicStartEvent:
		_, err := w.aw.WriteString(w.theme.Italic.Prefix)
		return err

	case parser.ItalicEndEvent:
		_, err := w.aw.WriteString(w.theme.Italic.Suffix)
		return err

	case parser.StrikethroughStartEvent:
		_, err := w.aw.WriteString(w.theme.Strikethrough.Prefix)
		return err

	case parser.StrikethroughEndEvent:
		_, err := w.aw.WriteString(w.theme.Strikethrough.Suffix)
		return err

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

	default:
		return nil
	}
}

func (w *Writer) ResetStyles() error {
	w.inBlockquote = false
	w.activeHeaderSuffix = ""
	_, err := w.aw.WriteString("\033[0m")
	return err
}
