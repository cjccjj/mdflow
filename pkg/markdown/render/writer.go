package render

import (
	"fmt"
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/parser"
)

type Writer struct {
	aw          *AnsiWriter
	theme       Theme
	live        bool
	tableActive bool
	inBlockquote bool
	tableHeader []string
	tableRows   [][]string
	tableWidths []int
	tableLines  int
}

func NewWriter(aw *AnsiWriter, theme Theme) *Writer {
	return &Writer{aw: aw, theme: theme}
}

func (w *Writer) SetLive(v bool) {
	w.live = v
}

func (w *Writer) Handle(e parser.Event) {
	switch e.Type {
	case parser.TextEvent:
		w.aw.WriteString(e.Value)

	case parser.NewlineEvent:
		w.aw.WriteString("\n")
		if w.inBlockquote {
			w.aw.WriteStyled("│ ", w.theme.Blockquote)
		}

	case parser.BlockquoteStartEvent:
		w.inBlockquote = true
		w.aw.WriteStyled("│ ", w.theme.Blockquote)

	case parser.BlockquoteEndEvent:
		w.inBlockquote = false

	case parser.HeaderStartEvent:
		style := w.theme.H1
		prefix := ""
		switch e.Value {
		case "h2":
			style = w.theme.H2
			prefix = "## "
		case "h3":
			style = w.theme.H3
			prefix = "### "
		case "h4":
			style = w.theme.H4
			prefix = "#### "
		case "h5":
			style = w.theme.H5
			prefix = "##### "
		case "h6":
			style = w.theme.H6
			prefix = "###### "
		}
		w.aw.WriteString(style.Prefix + prefix)

	case parser.HeaderEndEvent:
		w.aw.WriteString(w.theme.H1.Suffix)

	case parser.BoldStartEvent:
		w.aw.WriteString(w.theme.Bold.Prefix)

	case parser.BoldEndEvent:
		w.aw.WriteString(w.theme.Bold.Suffix)

	case parser.ItalicStartEvent:
		w.aw.WriteString(w.theme.Italic.Prefix)

	case parser.ItalicEndEvent:
		w.aw.WriteString(w.theme.Italic.Suffix)

	case parser.StrikethroughStartEvent:
		w.aw.WriteString(w.theme.Strikethrough.Prefix)

	case parser.StrikethroughEndEvent:
		w.aw.WriteString(w.theme.Strikethrough.Suffix)

	case parser.InlineCodeStartEvent:
		w.aw.WriteString(w.theme.InlineCode.Prefix)

	case parser.InlineCodeEndEvent:
		w.aw.WriteString(w.theme.InlineCode.Suffix)

	case parser.CodeBlockStartEvent:
		w.aw.WriteString(w.theme.CodeBlock.Prefix)

	case parser.CodeBlockEndEvent:
		w.aw.WriteString(w.theme.CodeBlock.Suffix)

	case parser.CodeBlockLangEvent:
		w.aw.WriteString(w.theme.CodeBlockLang.Prefix + e.Value + w.theme.CodeBlockLang.Suffix)

	case parser.HorizontalRuleEvent:
		w.aw.WriteString(w.theme.HorizontalRule.Prefix)
		w.aw.WriteString("────────────────")
		w.aw.WriteString(w.theme.HorizontalRule.Suffix)
		w.aw.WriteString("\n")

	case parser.BulletItemEvent:
		if e.Value != "" {
			w.aw.WriteString(e.Value)
		} else {
			w.aw.WriteString("• ")
		}

	case parser.TableStartEvent:
		w.handleTableStart(e)

	case parser.TableRowEvent:
		w.handleTableRow(e)

	case parser.TableEndEvent:
		w.handleTableEnd()
	}
}

func (w *Writer) handleTableStart(e parser.Event) {
	headerCells := strings.Split(e.Value, "\x00")
	sepWidths, _ := e.Extra.([]int)
	if sepWidths == nil {
		return
	}

	widths := make([]int, len(sepWidths))
	copy(widths, sepWidths)
	for i, c := range headerCells {
		rendered := RenderInline(c, w.theme)
		needed := VisibleLen(rendered) + 2
		if needed > widths[i] {
			widths[i] = needed
		}
	}
	for i := range widths {
		if widths[i] < 3 {
			widths[i] = 3
		}
	}

	w.tableActive = true
	w.tableHeader = headerCells
	w.tableRows = nil
	w.tableWidths = widths

	if w.live {
		w.drawBorder(widths, "┌", "┬", "┐", "─")
		w.aw.WriteString("\n")
		w.drawRow(widths, headerCells, w.theme.TableHeader)
		w.aw.WriteString("\n")
		w.drawBorder(widths, "├", "┼", "┤", "─")
		w.aw.WriteString("\n")
		w.tableLines = 3
	}
}

func (w *Writer) handleTableRow(e parser.Event) {
	cells := strings.Split(e.Value, "\x00")
	w.tableRows = append(w.tableRows, cells)

	newWidths := make([]int, len(w.tableWidths))
	copy(newWidths, w.tableWidths)
	for i, c := range cells {
		if i >= len(newWidths) {
			break
		}
		rendered := RenderInline(c, w.theme)
		needed := VisibleLen(rendered) + 2
		if needed > newWidths[i] {
			newWidths[i] = needed
		}
	}

	if !w.live {
		w.tableWidths = newWidths
		return
	}

	if widthsEqual(newWidths, w.tableWidths) {
		w.drawRow(w.tableWidths, cells, w.theme.TableCell)
		w.aw.WriteString("\n")
		w.tableLines++
	} else {
		w.tableWidths = newWidths
		w.redrawTable()
	}
}

func (w *Writer) handleTableEnd() {
	if len(w.tableWidths) == 0 {
		return
	}
	if !w.live {
		w.drawBorder(w.tableWidths, "┌", "┬", "┐", "─")
		w.aw.WriteString("\n")
		w.drawRow(w.tableWidths, w.tableHeader, w.theme.TableHeader)
		w.aw.WriteString("\n")
		w.drawBorder(w.tableWidths, "├", "┼", "┤", "─")
		w.aw.WriteString("\n")
		for _, row := range w.tableRows {
			w.drawRow(w.tableWidths, row, w.theme.TableCell)
			w.aw.WriteString("\n")
		}
	}
	w.drawBorder(w.tableWidths, "└", "┴", "┘", "─")
	w.aw.WriteString("\n")
	w.tableActive = false
	w.tableHeader = nil
	w.tableRows = nil
	w.tableWidths = nil
	w.tableLines = 0
}

func (w *Writer) redrawTable() {
	if w.live && w.tableLines > 0 {
		fmt.Fprintf(w.aw.w, "\033[%dA\r\033[J", w.tableLines)
	}
	w.drawBorder(w.tableWidths, "┌", "┬", "┐", "─")
	w.aw.WriteString("\n")
	w.drawRow(w.tableWidths, w.tableHeader, w.theme.TableHeader)
	w.aw.WriteString("\n")
	w.drawBorder(w.tableWidths, "├", "┼", "┤", "─")
	w.aw.WriteString("\n")
	for _, row := range w.tableRows {
		w.drawRow(w.tableWidths, row, w.theme.TableCell)
		w.aw.WriteString("\n")
	}
	w.tableLines = 3 + len(w.tableRows)
}

func (w *Writer) drawBorder(widths []int, left, mid, right, fill string) {
	if len(widths) == 0 {
		return
	}
	b := w.theme.TableBorder
	w.aw.WriteString(b.Prefix)
	w.aw.WriteString(left)
	for i, width := range widths {
		w.aw.WriteString(strings.Repeat(fill, width))
		if i < len(widths)-1 {
			w.aw.WriteString(mid)
		}
	}
	w.aw.WriteString(right)
	w.aw.WriteString(b.Suffix)
}

func (w *Writer) drawRow(widths []int, cells []string, cellStyle Style) {
	if len(widths) == 0 {
		return
	}
	b := w.theme.TableBorder

	for i := range widths {
		w.aw.WriteString(b.Prefix)
		w.aw.WriteString("│")
		w.aw.WriteString(b.Suffix)

		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		rendered := RenderInline(cell, w.theme)
		contentWidth := widths[i] - 2
		pad := contentWidth - VisibleLen(rendered)
		if pad < 0 {
			pad = 0
		}

		w.aw.WriteString(" ")
		if cellStyle.Prefix != "" {
			w.aw.WriteString(cellStyle.Prefix)
		}
		w.aw.WriteString(rendered)
		w.aw.WriteString(spaces(pad))
		if cellStyle.Suffix != "" {
			w.aw.WriteString(cellStyle.Suffix)
		}
		w.aw.WriteString(" ")
	}

	w.aw.WriteString(b.Prefix)
	w.aw.WriteString("│")
	w.aw.WriteString(b.Suffix)
}

func widthsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
