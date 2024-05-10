package main

import (
	"github.com/dackroyd/unindent"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(unindent.NewAnalyzer())
}
