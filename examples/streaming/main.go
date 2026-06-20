package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cjccjj/mdflow/pkg/markdown"
)

func main() {
	r := markdown.NewRenderer(os.Stdout)

	chunks := []string{
		"# Stream",
		"ing **wor",
		"ks**!\n",
		"- item 1\n",
		"* item 2\n",
	}

	for _, c := range chunks {
		r.Write([]byte(c))
		fmt.Fprintln(os.Stderr, "> sent chunk:", c)
		time.Sleep(200 * time.Millisecond)
	}

	r.Close()
}
