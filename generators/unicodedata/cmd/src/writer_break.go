package src

import (
	"fmt"
	"io"
	"sort"

	"github.com/go-text/typesetting-utils/generators/unicodedata/cmd/src/packtab"
)

func sortedKeys(classes map[string][]rune) (sortedClasses []string, maxRune rune) {
	for key, runes := range classes {
		sortedClasses = append(sortedClasses, key)
		if m := maxRunes(runes); m > maxRune {
			maxRune = m
		}
	}
	sort.Strings(sortedClasses)
	return
}

func generateIndicConjunctBreakPacktab(derivedCore map[string][]rune, w io.Writer) {
	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", unicodeVersion)

	fmt.Fprintf(w, `
	const (
		ICBLinker IndicConjunctBreak = 1 << iota
		ICBConsonant
		ICBExtend
	)
		`)

	// these table are used for UAX29 (GB9c)
	linker, consonant, extend := derivedCore["Linker"], derivedCore["Consonant"], derivedCore["Extend"]
	allRunes := append(append(linker, consonant...), extend...)
	table := make([]int, maxRunes(allRunes)+1)
	for _, r := range linker {
		table[r] = 1
	}
	for _, r := range consonant {
		table[r] = 2
	}
	for _, r := range extend {
		table[r] = 3
	}

	code := packtab.PackTable(table, 0, 9).Code("indicCB")
	fmt.Fprintln(w, code)
}

func generateGraphemeBreakPacktab(datas map[string][]rune, w io.Writer) {
	sortedClasses, maxRune := sortedKeys(datas)

	// map name to int (also generating flag constants)...
	flags := ""
	classToInt := map[string]int{}
	for i, c := range sortedClasses {
		// 0 is reserved for undefined,
		// but the first defined flag is still 1 << 0
		classToInt[c] = i + 1
		flags += fmt.Sprintf("GB_%s GraphemeBreak = 1 << %d\n", c, i)
	}
	// ... and build packab compatible table
	table := make([]int, maxRune+1)
	for className, runes := range datas {
		for _, r := range runes {
			table[r] = classToInt[className]
		}
	}

	code := packtab.PackTable(table, 0, 9).Code("gb")

	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", unicodeVersion)
	fmt.Fprintf(w, `
	const (
		%s
	)
	`, flags)
	fmt.Fprintln(w, code)
}

func generateWordBreakPropertyPacktab(db unicodeDatabase, datas map[string][]rune, derivedCore map[string][]rune, w io.Writer) {
	// some classes are always used together : merge them to simplify
	datas["ExtendFormat"] = append(append(append(datas["ExtendFormat"], datas["Extend"]...), datas["Format"]...), datas["ZWJ"]...)
	datas["NewlineCRLF"] = append(append(append(datas["NewlineCRLF"], datas["Newline"]...), datas["CR"]...), datas["LF"]...)

	delete(datas, "Extend")
	delete(datas, "Format")
	delete(datas, "Newline")
	delete(datas, "LF")
	delete(datas, "CR")
	delete(datas, "ZWJ")

	sortedClasses, maxRune := sortedKeys(datas)

	// map name to int (also generating flag constants)...
	flags := ""
	classToInt := map[string]int{}
	for i, c := range sortedClasses {
		// 0 is reserved for undefined,
		// but the first defined flag is still 1 << 0
		classToInt[c] = i + 1
		flags += fmt.Sprintf("WB_%s WordBreak = 1 << %d\n", c, i)
	}

	// ... and build packab compatible table1
	table1 := make([]int, maxRune+1)
	for className, runes := range datas {
		for _, r := range runes {
			table1[r] = classToInt[className]
		}
	}

	code := packtab.PackTable(table1, 0, 9).Code("wb")

	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", unicodeVersion)
	fmt.Fprintf(w, `
	const (
		%s
	)
	`, flags)
	fmt.Fprintln(w, code)
	fmt.Fprintln(w)

	// merge the tables for [Alphabetic] and Number
	alphabetic := derivedCore["Alphabetic"]
	categories, _ := db.generalCategories()
	nd, nl, no := categories["Nd"], categories["Nl"], categories["No"]

	all := append(alphabetic, nd...)
	all = append(all, nl...)
	all = append(all, no...)

	table2 := runesToTable(all)
	code2 := packtab.PackTable(table2, 0, 9).Code("word")
	fmt.Fprintln(w, code2)
}

// Supported line breaking classes for Unicode 17.0.0.
// Table loading depends on this: classes not listed here aren't loaded
// and are mapped to the zero value (XX, unknown)
func filterLineBreaks(m map[string][]rune) map[string][]rune {
	out := map[string][]rune{}
	for _, key := range []string{
		"BK", "CR", "LF", "NL", "SP", "NU", "AL", "IS", "PR", "PO", "OP", "CL",
		"CP", "QU", "HY", "SG", "GL", "NS", "EX", "SY", "VF", "VI",
		"HL", "ID", "IN", "BA", "BB", "B2", "ZW", "CM", "EB", "EM", "WJ", "ZWJ",
		"H2", "H3", "HH", "JL", "JV", "JT", "RI", "CB", "AI", "AK", "AP", "AS", "CJ", "SA",
	} {
		out[key] = m[key]
	}
	return out
}

func generateLineBreakPacktab(datas map[string][]rune, aliases map[string]string, w io.Writer) {
	datas = filterLineBreaks(datas)

	sortedClasses, maxRune := sortedKeys(datas)

	// map name to int (also generating flag constants)...
	flags := ""
	classToInt := map[string]int{}
	for i, c := range sortedClasses {
		// 0 is reserved for undefined,
		// but the first defined flag is still 1 << 0
		classToInt[c] = i + 1
		flags += fmt.Sprintf("LB_%s LineBreak = 1 << %d // %s\n", c, i, aliases[c])
	}

	// ... and build packab compatible table1
	table1 := make([]int, maxRune+1)
	for className, runes := range datas {
		for _, r := range runes {
			table1[r] = classToInt[className]
		}
	}

	code := packtab.PackTable(table1, 0, 9).Code("lb")

	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", unicodeVersion)
	fmt.Fprintf(w, `
	const (
		%s
	)
	`, flags)
	fmt.Fprintln(w, code)
}
