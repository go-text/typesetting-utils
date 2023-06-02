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

	db := parseUnicodeDatabase(srcs.unicodeData)

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

	lineBreak, err := parseAnnexTables(srcs.lineBreak)
	check(err)

	eastAsianWidth, err := parseAnnexTables(srcs.eastAsianWidth)
	check(err)

	sentenceBreaks, err := parseAnnexTables(srcs.sentenceBreak)
	check(err)

	graphemeBreaks, err := parseAnnexTables(srcs.graphemeBreak)
	check(err)

	scriptsRanges, err := parseAnnexTablesAsRanges(srcs.scripts)
	check(err)

	b, err = data.Files.ReadFile("Scripts-iso15924.txt")
	check(err)
	scriptNames, err := parseScriptNames(b)
	check(err)

	derivedCore, err := parseAnnexTables(srcs.derivedCore)
	check(err)

	b, err = data.Files.ReadFile("ArabicPUASimplified.txt")
	check(err)
	puaSimp := parsePUAMapping(b)
	b, err = data.Files.ReadFile("ArabicPUATraditional.txt")
	check(err)
	puaTrad := parsePUAMapping(b)

	// generate
	join := func(path string) string { return filepath.Join(outputDir, path) }

	process(join("unicodedata/combining_classes.go"), func(w io.Writer) {
		generateCombiningClasses(db.combiningClasses, w)
	})
	process(join("unicodedata/emojis.go"), func(w io.Writer) {
		generateEmojis(emojis, w)
	})

	process(join("unicodedata/mirroring.go"), func(w io.Writer) {
		generateMirroring(mirrors, w)
	})
	process(join("unicodedata/decomposition.go"), func(w io.Writer) {
		generateDecomposition(db.combiningClasses, dms, compEx, w)
	})
	process(join("unicodedata/linebreak.go"), func(w io.Writer) {
		generateLineBreak(lineBreak, w)
	})
	process(join("unicodedata/east_asian_width.go"), func(w io.Writer) {
		generateEastAsianWidth(eastAsianWidth, w)
	})
	process(join("unicodedata/indic.go"), func(w io.Writer) {
		generateIndicCategories(indicS, w)
	})
	process(join("unicodedata/sentence_break.go"), func(w io.Writer) {
		generateSTermProperty(sentenceBreaks, w)
	})
	process(join("unicodedata/grapheme_break.go"), func(w io.Writer) {
		generateGraphemeBreakProperty(graphemeBreaks, w)
	})
	process(join("unicodedata/general_category.go"), func(w io.Writer) {
		generateGeneralCategories(db.generalCategory, w)
	})

	process(join("harfbuzz/emojis_list_test.go"), func(w io.Writer) {
		generateEmojisTest(emojisTests, w)
	})
	process(join("harfbuzz/ot_use_table.go"), func(w io.Writer) {
		generateUSETable(db.generalCategory, indicS, indicP, blocks, indicSAdd, indicPAdd, derivedCore, scripts, joiningTypes, w)
	})
	process(join("harfbuzz/ot_vowels_constraints.go"), func(w io.Writer) {
		generateVowelConstraints(scripts, vowelsConstraints, w)
	})
	process(join("harfbuzz/ot_indic_table.go"), func(w io.Writer) {
		generateIndicTable(indicS, indicP, blocks, w)
	})
	process(join("harfbuzz/ot_arabic_table.go"), func(w io.Writer) {
		generateArabicShaping(db, joiningTypes, w)
		generateHasArabicJoining(joiningTypes, scripts, w)
	})
	process(join("opentype/api/cmap_arabic_pua_table.go"), func(w io.Writer) {
		generateArabicPUAMapping(puaSimp, puaTrad, w)
	})

	process(join("language/scripts_table.go"), func(w io.Writer) {
		generateScriptLookupTable(scriptsRanges, scriptNames, w)
	})

	fmt.Println("Done.")
}

// write into filename
func process(filename string, generator func(w io.Writer)) {
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
}
