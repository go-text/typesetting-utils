package src

import (
	"fmt"
	"io"
	"sort"
	"unicode"

	"github.com/go-text/typesetting-utils/generators/unicodedata/cmd/src/packtab"
	"golang.org/x/text/unicode/rangetable"
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

	sortedClasses, _ := sortedKeys(datas)

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
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)
	fmt.Fprintf(w, `
	const (
		%s
	)
	`, flags)
	fmt.Fprintln(w, code)
}

func generateWordBreakProperty(db unicodeDatabase, wbClasses map[string][]rune, derivedCore map[string][]rune, w io.Writer) {
	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)

	// some classes are always used together : merge them to simplify
	wbClasses["ExtendFormat"] = append(append(append(wbClasses["ExtendFormat"], wbClasses["Extend"]...), wbClasses["Format"]...), wbClasses["ZWJ"]...)
	wbClasses["NewlineCRLF"] = append(append(append(wbClasses["NewlineCRLF"], wbClasses["Newline"]...), wbClasses["CR"]...), wbClasses["LF"]...)

	delete(wbClasses, "Extend")
	delete(wbClasses, "Format")
	delete(wbClasses, "Newline")
	delete(wbClasses, "LF")
	delete(wbClasses, "CR")
	delete(wbClasses, "ZWJ")

	var sortedClasses []string
	for key := range wbClasses {
		sortedClasses = append(sortedClasses, key)
	}
	sort.Strings(sortedClasses)

	totalSize := 0
	list := ""
	var allWords []*unicode.RangeTable
	for _, className := range sortedClasses {
		runes := wbClasses[className]
		table := rangetable.New(runes...)
		s := printTable(table, false)
		totalSize += tableSize(table)
		fmt.Fprintf(w, "// WordBreakProperty: %s\n", className)
		fmt.Fprintf(w, "var WordBreak%s = %s\n\n", className, s)

		list += fmt.Sprintf("WordBreak%s, // %s \n", className, className)
		allWords = append(allWords, table)
	}

	// generate a union table to speed up lookup
	allTable := rangetable.Merge(allWords...)
	fmt.Fprintf(w, "// contains all the runes having a non nil word break property\n")
	fmt.Fprintf(w, "var wordBreakAll = %s\n\n", printTable(allTable, false))

	totalSize += tableSize(allTable)
	fmt.Fprintf(w, "// Total size %d B.\n\n", totalSize)

	fmt.Fprintf(w, `var wordBreaks = [...]*unicode.RangeTable{
	%s}
	`, list)

	// merge the tables for [Alphabetic] and Number
	alphabetic := derivedCore["Alphabetic"]
	categories, _ := db.generalCategories()
	nd, nl, no := categories["Nd"], categories["Nl"], categories["No"]

	all := append(alphabetic, nd...)
	all = append(all, nl...)
	all = append(all, no...)
	table := rangetable.New(all...)
	fmt.Fprintf(w, `
	// Word contains all the runes we may found in a word,
	// that is either a Number (Nd, Nl, No) or a rune with the Alphabetic property
	var Word = %s
	
	// Size %d B.
	`, printTable(table, false), tableSize(table))
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
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)
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

func generateLineBreak(datas map[string][]rune, aliases map[string]string, w io.Writer) {
	datas = filterLineBreaks(datas)

	sortedClasses, _ := sortedKeys(datas)

	fmt.Fprint(w, unicodedataheader)
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)

	totalSize := 0
	dict := ""
	for _, class := range sortedClasses {
		table := rangetable.New(datas[class]...)
		totalSize += tableSize(table)
		s := printTable(table, false)
		fmt.Fprintf(w, "// %s\n", aliases[class])
		fmt.Fprintf(w, "var Break%s = %s\n\n", class, s)

		dict += fmt.Sprintf("Break%s,\n", class)
	}

	fmt.Fprintf(w, "// Total size %d B.\n\n", totalSize)

	fmt.Fprintf(w, `var lineBreaks = [...]*unicode.RangeTable{
		%s}
	`, dict)
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
	fmt.Fprintf(w, "// Unicode version: %s\n\n", version)
	fmt.Fprintf(w, `
	const (
		%s
	)
	`, flags)
	fmt.Fprintln(w, code)
}
