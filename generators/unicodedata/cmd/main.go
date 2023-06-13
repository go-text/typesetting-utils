// Generate lookup function for Unicode properties not
// covered by the standard package unicode.
package main

import (
	"flag"
	"log"

	"github.com/go-text/typesetting-utils/generators/unicodedata/cmd/src"
)

func main() {
	useCache := flag.Bool("cache", false, "use data from on disk cache")
	flag.Parse()
	outputDir := flag.Arg(0)
	if outputDir == "" {
		log.Fatal("missing output directory")
	}

	src.Generate(outputDir, *useCache)
}
