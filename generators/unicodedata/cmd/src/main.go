// Generate lookup function for Unicode properties not
// covered by the standard package unicode.
package src

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-text/typesetting-utils/generators/unicodedata/data"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// Generate generates Go files in [outputDir]
func Generate(outputDir string, dataFromCache bool) {
	srcs := fetchAll(dataFromCache)

	// parse
	fmt.Println("Parsing Unicode files...")

	db, err := parseUnicodeDatabase(srcs.unicodeData)
	check(err)

	emojis, err := parseAnnexTables(srcs.emoji)
	check(err)

	emojisTests := parseEmojisTest(srcs.emojiTest)

	mirrors, err := parseMirroring(srcs.bidiMirroring)
	check(err)

	dms, compEx, err := parseXML(srcs.ucdXML)
	check(err)

	joiningTypes := parseArabicShaping(srcs.arabic)

	scripts, err := parseAnnexTables(srcs.scripts)
	check(err)

	blocks, err := parseAnnexTables(srcs.blocks)
	check(err)

	indicS, err := parseAnnexTables(srcs.indicSyllabic)
	check(err)

	indicP, err := parseAnnexTables(srcs.indicPositional)
	check(err)

	b, err := data.Files.ReadFile("ms-use/IndicSyllabicCategory-Additional.txt")
	check(err)
	indicSAdd, err := parseAnnexTables(b)
	check(err)

	b, err = data.Files.ReadFile("ms-use/IndicPositionalCategory-Additional.txt")
	check(err)
	indicPAdd, err := parseAnnexTables(b)
	check(err)

	b, err = data.Files.ReadFile("ms-use/IndicShapingInvalidCluster.txt")
	check(err)
	vowelsConstraints := parseUSEInvalidCluster(b)

	lineBreaks, err := parseAnnexTables(srcs.lineBreak)
	check(err)

	eastAsianWidth, err := parseAnnexTables(srcs.eastAsianWidth)
	check(err)

	_, err = parseAnnexTables(srcs.sentenceBreak)
	check(err)

	graphemeBreaks, err := parseAnnexTables(srcs.graphemeBreak)
	check(err)

	scriptsRanges, err := parseAnnexTablesAsRanges(srcs.scripts)
	check(err)

	scriptNames, err := parseScriptNames(srcs.scriptNames)
	check(err)

	derivedCore, err := parseAnnexTables(srcs.derivedCore)
	check(err)

	valueAliases, err := parseValueAliases(srcs.propertyValueAliases)
	check(err)

	indicConjunctBreaks, err := parseDerivedCoreIndicCB(srcs.derivedCore)
	check(err)

	b, err = data.Files.ReadFile("ArabicPUASimplified.txt")
	check(err)
	puaSimp := parsePUAMapping(b)
	b, err = data.Files.ReadFile("ArabicPUATraditional.txt")
	check(err)
	puaTrad := parsePUAMapping(b)

	// generate
	join := func(path string) string { return filepath.Join(outputDir, path) }

	process(join("internal/unicodedata/combining_classes_test.go"), false, func(w io.Writer) {
		generateCombiningClasses(db.combiningClasses, w)
	})
	process(join("internal/unicodedata/combining_classes.go"), true, func(w io.Writer) {
		generateCombiningClassesPacktab(db, w)
	})
	process(join("internal/unicodedata/emojis.go"), false, func(w io.Writer) {
		generateEmojisPacktab(emojis, w)
	})
	process(join("internal/unicodedata/emojis_test.go"), false, func(w io.Writer) {
		generateEmojis(emojis, w)
	})

	process(join("internal/unicodedata/mirroring_test.go"), false, func(w io.Writer) {
		generateMirroring(mirrors, w)
	})
	process(join("internal/unicodedata/mirroring.go"), true, func(w io.Writer) {
		generateMirroringPacktab(db, mirrors, w)
	})
	process(join("internal/unicodedata/decomposition.go"), true, func(w io.Writer) {
		generateDecompositionPacktab(db.combiningClasses, dms, compEx, w)
	})
	process(join("internal/unicodedata/decomposition_test.go"), false, func(w io.Writer) {
		generateDecomposition(db.combiningClasses, dms, compEx, w)
	})
	process(join("internal/unicodedata/east_asian_width.go"), false, func(w io.Writer) {
		generateEastAsianWidthPacktab(eastAsianWidth, w)
	})
	process(join("internal/unicodedata/east_asian_width_test.go"), false, func(w io.Writer) {
		generateEastAsianWidth(eastAsianWidth, w)
	})

	process(join("internal/unicodedata/general_category.go"), true, func(w io.Writer) {
		generateGeneralCategoriesPacktab(db, valueAliases, w)
	})

	process(join("harfbuzz/emojis_list_test.go"), false, func(w io.Writer) {
		generateEmojisTest(emojisTests, w)
	})
	process(join("harfbuzz/ot_shape_use_table.go"), false, func(w io.Writer) {
		generateUSETable(db.generalCategory, indicS, indicP, blocks, indicSAdd, indicPAdd, derivedCore, scripts, joiningTypes, w)
	})
	process(join("harfbuzz/ot_shape_vowels_constraints.go"), false, func(w io.Writer) {
		generateVowelConstraints(scripts, vowelsConstraints, w)
	})
	process(join("harfbuzz/ot_shape_indic_table.go"), false, func(w io.Writer) {
		generateIndicTable(indicS, indicP, blocks, w)
	})
	process(join("harfbuzz/ot_shape_arabic_table.go"), false, func(w io.Writer) {
		generateArabicShaping(db, joiningTypes, w)
		generateHasArabicJoining(joiningTypes, scripts, w)
	})
	process(join("font/cmap_arabic_pua_table.go"), false, func(w io.Writer) {
		generateArabicPUAMapping(puaSimp, puaTrad, w)
	})

	process(join("language/scripts_table.go"), false, func(w io.Writer) {
		generateScriptLookupTable(scriptsRanges, scriptNames, w)
	})

	process(join("internal/unicodedata/line_break.go"), false, func(w io.Writer) {
		generateLineBreak(lineBreaks, w)
	})
	process(join("internal/unicodedata/grapheme_break.go"), false, func(w io.Writer) {
		generateGraphemeBreakPacktab(graphemeBreaks, w)
	})
	process(join("internal/unicodedata/grapheme_break_test.go"), false, func(w io.Writer) {
		generateGraphemeBreak(graphemeBreaks, w)
	})
	process(join("internal/unicodedata/indic_conjunct_break_test.go"), true, func(w io.Writer) {
		generateIndicConjunctBreak(indicConjunctBreaks, w)
	})
	process(join("internal/unicodedata/indic_conjunct_break.go"), true, func(w io.Writer) {
		generateIndicConjunctBreakPacktab(indicConjunctBreaks, w)
	})

	// just copy test files
	fmt.Println("Copying tests into segmenter/test/ ...")
	err = os.WriteFile(join("segmenter/test/LineBreakTest.txt"), srcs.lineBreakTest, os.ModePerm)
	check(err)
	err = os.WriteFile(join("segmenter/test/WordBreakTest.txt"), srcs.wordBreakTest, os.ModePerm)
	check(err)
	err = os.WriteFile(join("segmenter/test/GraphemeBreakTest.txt"), srcs.graphemeBreakTest, os.ModePerm)
	check(err)

	fmt.Println("Done.")
}

// write into filename
func process(filename string, unconvert bool, generator func(w io.Writer)) {
	fmt.Println("Generating", filename, "...")
	file, err := os.Create(filename)
	check(err)

	generator(file)

	err = file.Close()
	check(err)

	cmd := exec.Command("goimports", "-w", filename)
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	check(err)

	if !unconvert {
		return
	}
	cmd = exec.Command("unconvert", "-apply", filename)
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	check(err)
}
