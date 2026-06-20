package main

import (
	"io"
	"log"
	"os"

	"github.com/cjccjj/mdflow/pkg/markdown"
)

func main() {
	r := markdown.NewRenderer(os.Stdout)
	if _, err := io.Copy(r, os.Stdin); err != nil {
		log.Fatal(err)
	}
	if err := r.Close(); err != nil {
		log.Fatal(err)
	}
}
