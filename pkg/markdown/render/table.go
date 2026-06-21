package render

import (
	"fmt"
	"strings"

	"github.com/cjccjj/mdflow/pkg/markdown/parser"
)

func (w *Writer) handleTableStart(e parser.Event) error {
	headerCells := e.Cells
	sepWidths := e.Widths
	if sepWidths == nil {
		return nil
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
		if _, err := w.drawBorder(widths, "┌", "┬", "┐", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		if _, err := w.drawRow(widths, headerCells, w.theme.TableHeader); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		if _, err := w.drawBorder(widths, "├", "┼", "┤", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		w.tableLines = 3
	}
	return nil
}

func (w *Writer) handleTableRow(e parser.Event) error {
	cells := e.Cells
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
		return nil
	}

	if widthsEqual(newWidths, w.tableWidths) {
		if _, err := w.drawRow(w.tableWidths, cells, w.theme.TableCell); err != nil {
			return err
		}
		_, err := w.aw.WriteString("\n")
		if err != nil {
			return err
		}
		w.tableLines++
		return nil
	}
	w.tableWidths = newWidths
	return w.redrawTable()
}

func (w *Writer) handleTableEnd() error {
	if len(w.tableWidths) == 0 {
		return nil
	}
	if !w.live {
		if _, err := w.drawBorder(w.tableWidths, "┌", "┬", "┐", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		if _, err := w.drawRow(w.tableWidths, w.tableHeader, w.theme.TableHeader); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		if _, err := w.drawBorder(w.tableWidths, "├", "┼", "┤", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		for _, row := range w.tableRows {
			if _, err := w.drawRow(w.tableWidths, row, w.theme.TableCell); err != nil {
				return err
			}
			if _, err := w.aw.WriteString("\n"); err != nil {
				return err
			}
		}
	}
	if _, err := w.drawBorder(w.tableWidths, "└", "┴", "┘", "─"); err != nil {
		return err
	}
	_, err := w.aw.WriteString("\n")
	if err != nil {
		return err
	}
	w.tableActive = false
	w.tableHeader = nil
	w.tableRows = nil
	w.tableWidths = nil
	w.tableLines = 0
	return nil
}

func (w *Writer) redrawTable() error {
	if w.live && w.tableLines > 0 {
		_, err := fmt.Fprintf(w.aw.w, "\033[%dA\r\033[J", w.tableLines)
		if err != nil {
			return err
		}
	}
	if _, err := w.drawBorder(w.tableWidths, "┌", "┬", "┐", "─"); err != nil {
		return err
	}
	if _, err := w.aw.WriteString("\n"); err != nil {
		return err
	}
	if _, err := w.drawRow(w.tableWidths, w.tableHeader, w.theme.TableHeader); err != nil {
		return err
	}
	if _, err := w.aw.WriteString("\n"); err != nil {
		return err
	}
	if _, err := w.drawBorder(w.tableWidths, "├", "┼", "┤", "─"); err != nil {
		return err
	}
	if _, err := w.aw.WriteString("\n"); err != nil {
		return err
	}
	for _, row := range w.tableRows {
		if _, err := w.drawRow(w.tableWidths, row, w.theme.TableCell); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
	}
	w.tableLines = 3 + len(w.tableRows)
	return nil
}

func (w *Writer) drawBorder(widths []int, left, mid, right, fill string) (int, error) {
	if len(widths) == 0 {
		return 0, nil
	}
	b := w.theme.TableBorder
	n := 0
	m, err := w.aw.WriteString(b.Prefix)
	n += m
	if err != nil {
		return n, err
	}
	m, err = w.aw.WriteString(left)
	n += m
	if err != nil {
		return n, err
	}
	for i, width := range widths {
		m, err = w.aw.WriteString(strings.Repeat(fill, width))
		n += m
		if err != nil {
			return n, err
		}
		if i < len(widths)-1 {
			m, err = w.aw.WriteString(mid)
			n += m
			if err != nil {
				return n, err
			}
		}
	}
	m, err = w.aw.WriteString(right)
	n += m
	if err != nil {
		return n, err
	}
	m, err = w.aw.WriteString(b.Suffix)
	n += m
	return n, err
}

func (w *Writer) drawRow(widths []int, cells []string, cellStyle Style) (int, error) {
	if len(widths) == 0 {
		return 0, nil
	}
	b := w.theme.TableBorder
	n := 0

	for i := range widths {
		m, err := w.aw.WriteString(b.Prefix)
		n += m
		if err != nil {
			return n, err
		}
		m, err = w.aw.WriteString("│")
		n += m
		if err != nil {
			return n, err
		}
		m, err = w.aw.WriteString(b.Suffix)
		n += m
		if err != nil {
			return n, err
		}

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

		m, err = w.aw.WriteString(" ")
		n += m
		if err != nil {
			return n, err
		}
		if cellStyle.Prefix != "" {
			m, err = w.aw.WriteString(cellStyle.Prefix)
			n += m
			if err != nil {
				return n, err
			}
		}
		m, err = w.aw.WriteString(rendered)
		n += m
		if err != nil {
			return n, err
		}
		m, err = w.aw.WriteString(spaces(pad))
		n += m
		if err != nil {
			return n, err
		}
		if cellStyle.Suffix != "" {
			m, err = w.aw.WriteString(cellStyle.Suffix)
			n += m
			if err != nil {
				return n, err
			}
		}
		m, err = w.aw.WriteString(" ")
		n += m
		if err != nil {
			return n, err
		}
	}

	m, err := w.aw.WriteString(b.Prefix)
	n += m
	if err != nil {
		return n, err
	}
	m, err = w.aw.WriteString("│")
	n += m
	if err != nil {
		return n, err
	}
	m, err = w.aw.WriteString(b.Suffix)
	n += m
	return n, err
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
