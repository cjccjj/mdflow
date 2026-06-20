package main

import (
	"os"

	"github.com/cjccjj/mdflow/pkg/markdown"
)

func main() {
	input := "# Hello mdflow\n\nThis is **bold** and `inline code`.\n\n```\ncode block\n```\n\n- bullet one\n* bullet two\n"

	r := markdown.NewRenderer(os.Stdout)
	r.Write([]byte(input))
	r.Close()
}
