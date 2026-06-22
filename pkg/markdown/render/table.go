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

	widths = w.limitWidths(widths)

	w.tableActive = true
	w.tableHeader = headerCells
	w.tableRows = nil
	w.tableWidths = widths
	w.tableAligns = e.Aligns

	if w.live {
		if _, err := w.drawBorder(widths, "┌", "┬", "┐", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		headerLines, err := w.drawRow(widths, headerCells, w.theme.TableHeader, w.tableAligns)
		if err != nil {
			return err
		}
		if _, err := w.drawBorder(widths, "├", "┼", "┤", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		w.tableLines = 2 + headerLines
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

	newWidths = w.limitWidths(newWidths)

	if !w.live {
		w.tableWidths = newWidths
		return nil
	}

	if widthsEqual(newWidths, w.tableWidths) {
		n, err := w.drawRow(w.tableWidths, cells, w.theme.TableCell, w.tableAligns)
		if err != nil {
			return err
		}
		w.tableLines += n
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
		if _, err := w.drawRow(w.tableWidths, w.tableHeader, w.theme.TableHeader, w.tableAligns); err != nil {
			return err
		}
		if _, err := w.drawBorder(w.tableWidths, "├", "┼", "┤", "─"); err != nil {
			return err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return err
		}
		for _, row := range w.tableRows {
			if _, err := w.drawRow(w.tableWidths, row, w.theme.TableCell, w.tableAligns); err != nil {
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
	w.tableAligns = nil
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
	headerLines, err := w.drawRow(w.tableWidths, w.tableHeader, w.theme.TableHeader, w.tableAligns)
	if err != nil {
		return err
	}
	if _, err := w.drawBorder(w.tableWidths, "├", "┼", "┤", "─"); err != nil {
		return err
	}
	if _, err := w.aw.WriteString("\n"); err != nil {
		return err
	}
	totalLines := 2 + headerLines
	for _, row := range w.tableRows {
		n, err := w.drawRow(w.tableWidths, row, w.theme.TableCell, w.tableAligns)
		if err != nil {
			return err
		}
		totalLines += n
	}
	w.tableLines = totalLines
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

func (w *Writer) drawRow(widths []int, cells []string, cellStyle Style, aligns []int) (int, error) {
	nCols := len(widths)
	if nCols == 0 {
		return 0, nil
	}

	cellSlices := make([][]string, nCols)
	maxLines := 1
	for i := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		rendered := RenderInline(cell, w.theme)
		contentWidth := widths[i] - 2
		if contentWidth < 1 {
			contentWidth = 1
		}
		slices := WrapContent(rendered, contentWidth)
		cellSlices[i] = slices
		if len(slices) > maxLines {
			maxLines = len(slices)
		}
	}

	b := w.theme.TableBorder

	for lineNum := 0; lineNum < maxLines; lineNum++ {
		for col := 0; col < nCols; col++ {
			if _, err := w.aw.WriteString(b.Prefix); err != nil {
				return 0, err
			}
			if _, err := w.aw.WriteString("│"); err != nil {
				return 0, err
			}
			if _, err := w.aw.WriteString(b.Suffix); err != nil {
				return 0, err
			}

			contentWidth := widths[col] - 2
			if contentWidth < 1 {
				contentWidth = 1
			}

			var slice string
			if lineNum < len(cellSlices[col]) {
				slice = cellSlices[col][lineNum]
			}

			align := 0
			if aligns != nil && col < len(aligns) {
				align = aligns[col]
			}

			pad := contentWidth - VisibleLen(slice)
			if pad < 0 {
				pad = 0
			}
			leftPad := 0
			rightPad := pad
			if align == 1 {
				leftPad = pad / 2
				rightPad = pad - leftPad
			} else if align == 2 {
				leftPad = pad
				rightPad = 0
			}

			if _, err := w.aw.WriteString(" "); err != nil {
				return 0, err
			}
			if _, err := w.aw.WriteString(spaces(leftPad)); err != nil {
				return 0, err
			}
			if cellStyle.Prefix != "" {
				if _, err := w.aw.WriteString(cellStyle.Prefix); err != nil {
					return 0, err
				}
			}
			if _, err := w.aw.WriteString(slice); err != nil {
				return 0, err
			}
			if cellStyle.Suffix != "" {
				if _, err := w.aw.WriteString(cellStyle.Suffix); err != nil {
					return 0, err
				}
			}
			if _, err := w.aw.WriteString(spaces(rightPad)); err != nil {
				return 0, err
			}
			if _, err := w.aw.WriteString(" "); err != nil {
				return 0, err
			}
		}

		if _, err := w.aw.WriteString(b.Prefix); err != nil {
			return 0, err
		}
		if _, err := w.aw.WriteString("│"); err != nil {
			return 0, err
		}
		if _, err := w.aw.WriteString(b.Suffix); err != nil {
			return 0, err
		}
		if _, err := w.aw.WriteString("\n"); err != nil {
			return 0, err
		}
	}

	return maxLines, nil
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

func (w *Writer) limitWidths(widths []int) []int {
	if w.termWidth <= 0 || len(widths) == 0 {
		return widths
	}
	nCols := len(widths)
	total := nCols + 1
	for _, cw := range widths {
		total += cw
	}
	if total <= w.termWidth {
		return widths
	}
	if w.termWidth < 30 {
		return widths
	}

	available := w.termWidth - nCols - 1
	if available < nCols*3 {
		available = nCols * 3
	}

	sum := 0
	for _, cw := range widths {
		sum += cw
	}

	result := make([]int, nCols)
	allocated := 0
	for i, cw := range widths {
		result[i] = cw * available / sum
		if result[i] < 3 {
			result[i] = 3
		}
		allocated += result[i]
	}

	remainder := available - allocated
	for remainder > 0 {
		bestIdx := -1
		bestFrac := 0.0
		for i, cw := range widths {
			exact := float64(cw) * float64(available) / float64(sum)
			got := float64(result[i])
			frac := exact - got
			if frac > bestFrac {
				bestFrac = frac
				bestIdx = i
			}
		}
		if bestIdx < 0 || result[bestIdx] >= widths[bestIdx] {
			break
		}
		result[bestIdx]++
		remainder--
	}

	return result
}
