package render

import (
	"io"
	"os"

	"golang.org/x/term"
)

func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
