package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cjccjj/mdflow/pkg/markdown"

	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 || term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, `mdflow is a realtime streaming Markdown renderer.

Usage:
  llm "explain goroutines" | mdflow
  cat file.md | mdflow

mdflow reads from stdin only; it does not accept file arguments.
`)
		os.Exit(1)
	}

	r := markdown.NewRenderer(os.Stdout)
	br := bufio.NewReader(os.Stdin)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if _, werr := r.Write([]byte(line)); werr != nil {
				log.Fatal(werr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		log.Fatal(err)
	}
}
