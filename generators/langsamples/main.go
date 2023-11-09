// Run
// go run main.go rune_coverage.go <OUTPUT_DIR>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-text/typesetting-utils/generators"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func openOrthFile(dir, filename string) (out []string) {
	filename = filepath.Join(dir, filename)
	content, err := os.ReadFile(filename)
	check(err)

	lines := bytes.Split(content, []byte{'\n'})
	for _, line := range lines {
		// ignore empty lines or comments
		if len(line) == 0 || bytes.HasPrefix(line, []byte{'#'}) {
			continue
		}

		line := string(line)
		// process include directives
		if _, toInclude, ok := strings.Cut(line, "include "); ok {
			out = append(out, openOrthFile(dir, toInclude)...)
		} else {
			// remove trailing spaces and comments
			line, _, _ = strings.Cut(line, "#")
			line, _, _ = strings.Cut(line, "\t")
			out = append(out, strings.TrimSpace(line))
		}
	}

	return out
}

func parseRune(s string) rune {
	s = strings.TrimPrefix(s, "0x")
	i, err := strconv.ParseUint(s, 16, 64)
	check(err)

	return rune(i)
}

func parseRunes(lines []string) map[rune]bool {
	// runes are written as one hex code 0x0E1A or a range 0E1A-0F56
	// a line may start by - to remove a rune from a previous include
	out := make(map[rune]bool)
	for _, line := range lines {
		if _, toRemove, ok := strings.Cut(line, "- "); ok {
			r := parseRune(toRemove)
			delete(out, r)
		} else if start, end, ok := strings.Cut(line, "-"); ok {
			s, e := parseRune(start), parseRune(end)
			for r := s; r <= e; r++ {
				out[r] = true
			}
		} else if start, end, ok := strings.Cut(line, ".."); ok {
			s, e := parseRune(start), parseRune(end)
			for r := s; r <= e; r++ {
				out[r] = true
			}
		} else {
			r := parseRune(line)
			out[r] = true
		}
	}

	return out
}

func newRuneSet(runes map[rune]bool) (out runeSet) {
	for k := range runes {
		out.Add(k)
	}
	return out
}

type languageRunes struct {
	lang  generators.Language
	runes runeSet
}

func scanOrthFiles() (out []languageRunes) {
	const dirName = "fc-lang"
	dir, err := os.ReadDir(dirName)
	check(err)

	for _, file := range dir {
		if !strings.HasSuffix(file.Name(), "orth") {
			continue
		}

		lines := openOrthFile(dirName, file.Name())
		rs := newRuneSet(parseRunes(lines))

		name, _, _ := strings.Cut(file.Name(), ".")
		lang := generators.NewLanguage(name)

		out = append(out, languageRunes{lang, rs})
	}

	// out is already sorted by lang

	return out
}

func printRuneSet(rs runeSet) string {
	chunks := make([]string, len(rs))
	for i, page := range rs {
		chunks[i] = fmt.Sprintf("{ref: 0x%04x, set: pageSet{0x%08x, 0x%08x, 0x%08x, 0x%08x, 0x%08x, 0x%08x, 0x%08x, 0x%08x}}",
			page.ref,
			page.set[0], page.set[1], page.set[2], page.set[3], page.set[4], page.set[5], page.set[6], page.set[7])
	}
	return "runeSet{\n" + strings.Join(chunks, ",\n") + ",\n}"
}

func printLangTables(langs []languageRunes) string {
	var s strings.Builder

	// print the table
	s.WriteString(`
	// languagesRunes stores the runes commonly used to write a language
	var languagesRunes = [...]struct{
		lang language.Language
		runes runeSet // sorted, inclusive ranges 
	}{
	`)
	for i, lang := range langs {
		s.WriteString(fmt.Sprintf(`{ /** index: %d */
				%q,
				%s,
			},
			`, i, lang.lang, printRuneSet(lang.runes)))
	}
	s.WriteString("}")

	return s.String()
}

func main() {
	flag.Parse()
	outDir := flag.Arg(0)

	fmt.Println("Scanning orth files...")
	langs := scanOrthFiles()
	fmt.Printf("Found %d languages. Generating table...\n", len(langs))

	outFile := filepath.Join(outDir, "langset_gen.go")
	err := os.WriteFile(outFile, []byte(`package fontscan

	// Copyright © 2002 Keith Packard
	//
	// Permission to use, copy, modify, distribute, and sell this software and its
	// documentation for any purpose is hereby granted without fee, provided that
	// the above copyright notice appear in all copies and that both that
	// copyright notice and this permission notice appear in supporting
	// documentation, and that the name of the author(s) not be used in
	// advertising or publicity pertaining to distribution of the software without
	// specific, written prior permission.  The authors make no
	// representations about the suitability of this software for any purpose.  It
	// is provided "as is" without express or implied warranty.
	//
	// THE AUTHOR(S) DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE,
	// INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS, IN NO
	// EVENT SHALL THE AUTHOR(S) BE LIABLE FOR ANY SPECIAL, INDIRECT OR
	// CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE,
	// DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
	// TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
	// PERFORMANCE OF THIS SOFTWARE.
	

	`+printLangTables(langs)), os.ModePerm)
	check(err)

	fmt.Println("Code written. Formatting...")
	cmd := exec.Command("goimports", "-w", outFile)
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	check(err)
	fmt.Println("Done.")
}
