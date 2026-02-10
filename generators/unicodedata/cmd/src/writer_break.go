package src

import (
	"fmt"
	"io"
	"sort"
	"unicode"

	"github.com/go-text/typesetting-utils/generators/unicodedata/cmd/src/packtab"
	"golang.org/x/text/unicode/rangetable"
)

func generateIndicConjunctBreak(derivedCore map[string][]rune, w io.Writer) {
	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)

	// these table are used for UAX29 (GB9c)
	linker := rangetable.New(derivedCore["Linker"]...)
	consonnant := rangetable.New(derivedCore["Consonant"]...)
	extend := rangetable.New(derivedCore["Extend"]...)

	totalSize := tableSize(linker) + tableSize(consonnant) + tableSize(extend)

	codeLinker := printTable(linker, false)
	codeConsonant := printTable(consonnant, false)
	codeExtend := printTable(extend, false)
	fmt.Fprintf(w, `
	// indicCBLinker matches runes with Indic_Conjunct_Break property of 
	// Linker and is used for UAX29, rule GB9c.
	var indicCBLinker = %s

	// indicCBConsonant matches runes with Indic_Conjunct_Break property of 
	// Consonant and is used for UAX29, rule GB9c.
	var indicCBConsonant = %s

	// indicCBExtend matches runes with Indic_Conjunct_Break property of 
	// Extend and is used for UAX29, rule GB9c.
	var indicCBExtend = %s

	// Total size %d B.`, codeLinker, codeConsonant, codeExtend, totalSize)
}

func generateIndicConjunctBreakPacktab(derivedCore map[string][]rune, w io.Writer) {
	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)

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

func generateGraphemeBreak(datas map[string][]rune, w io.Writer) {
	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)

	var sortedClasses []string
	for key := range datas {
		sortedClasses = append(sortedClasses, key)
	}
	sort.Strings(sortedClasses)

	totalSize := 0
	list := ""
	var allGraphemes []*unicode.RangeTable
	for _, className := range sortedClasses {
		runes := datas[className]
		table := rangetable.New(runes...)
		totalSize += tableSize(table)

		s := printTable(table, false)
		fmt.Fprintf(w, "// GraphemeBreakProperty: %s\n", className)
		fmt.Fprintf(w, "var GraphemeBreak%s = %s\n\n", className, s)

		list += fmt.Sprintf("GraphemeBreak%s, // %s \n", className, className)
		allGraphemes = append(allGraphemes, table)
	}

	// generate a union table to speed up lookup
	allTable := rangetable.Merge(allGraphemes...)
	fmt.Fprintf(w, "// contains all the runes having a non nil grapheme break property\n")
	fmt.Fprintf(w, "var graphemeBreakAll = %s\n\n", printTable(allTable, false))

	totalSize += tableSize(allTable)
	fmt.Fprintf(w, "// Total size %d B.\n\n", totalSize)

	fmt.Fprintf(w, `var graphemeBreaks = [...]*unicode.RangeTable{
	%s}
	`, list)
}

func generateGraphemeBreakPacktab(datas map[string][]rune, w io.Writer) {
	var (
		sortedClasses []string
		maxRune       rune
	)
	for key, runes := range datas {
		sortedClasses = append(sortedClasses, key)
		if m := maxRunes(runes); m > maxRune {
			maxRune = m
		}
	}
	sort.Strings(sortedClasses)

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
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)
	fmt.Fprintf(w, `
	const (
		%s
	)
	`, flags)
	fmt.Fprintln(w, code)
}

// Supported line breaking classes for Unicode 17.0.0.
// Table loading depends on this: classes not listed here aren't loaded.
var lineBreakClasses = [][2]string{
	{"BK", "Mandatory Break"},
	{"CR", "Carriage Return"},
	{"LF", "Line Feed"},
	{"NL", "Next Line"},
	{"SP", "Space"},
	{"NU", "Numeric"},
	{"AL", "Alphabetic"},
	{"IS", "Infix Numeric Separator"},
	{"PR", "Prefix Numeric"},
	{"PO", "Postfix Numeric"},
	{"OP", "Open Punctuation"},
	{"CL", "Close Punctuation"},
	{"CP", "Close Parenthesis"},
	{"QU", "Quotation"},
	{"HY", "Hyphen"},
	{"SG", "Surrogate"},
	{"GL", `Non-breaking ("Glue")`},
	{"NS", "Nonstarter"},
	{"EX", "Exclamation/Interrogation"},
	{"SY", "Symbols Allowing Break After"},
	{"VF", "Virama Final"},
	{"VI", "Virama"},
	{"HL", "Hebrew Letter"},
	{"ID", "Ideographic"},
	{"IN", "Inseparable"},
	{"BA", "Break After"},
	{"BB", "Break Before"},
	{"B2", "Break Opportunity Before and After"},
	{"ZW", "Zero Width Space"},
	{"CM", "Combining Mark"},
	{"EB", "Emoji Base"},
	{"EM", "Emoji Modifier"},
	{"WJ", "Word Joiner"},
	{"ZWJ", "Zero width joiner"},
	{"H2", "Hangul LV Syllable"},
	{"H3", "Hangul LVT Syllable"},
	{"HH", "Unambiguous Hyphen"},
	{"JL", "Hangul L Jamo"},
	{"JV", "Hangul V Jamo"},
	{"JT", "Hangul T Jamo"},
	{"RI", "Regional Indicator"},
	{"CB", "Contingent Break Opportunity"},
	{"AI", "Ambiguous (Alphabetic or Ideographic)"},
	{"AK", "Aksara"},
	{"AP", "Aksara Pre-Base"},
	{"AS", "Aksara Start"},
	{"CJ", "Conditional Japanese Starter"},
	{"SA", "Complex Context Dependent (South East Asian)"},
	{"XX", "Unknown"},
}

func generateLineBreak(datas map[string][]rune, w io.Writer) {
	dict := ""

	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)

	for i, class := range lineBreakClasses {
		className := class[0]
		table := rangetable.New(datas[className]...)
		s := printTable(table, false)
		fmt.Fprintf(w, "// %s\n", lineBreakClasses[i][1])
		fmt.Fprintf(w, "var Break%s = %s\n\n", className, s)

		dict += fmt.Sprintf("Break%s, // %s \n", className, className)
	}

	fmt.Fprintf(w, `var lineBreaks = [...]*unicode.RangeTable{
		%s}
	`, dict)
}
