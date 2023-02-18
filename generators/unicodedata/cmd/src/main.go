// Generate lookup function for Unicode properties not
// covered by the standard package unicode.
package src

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-text/typesetting-utils/generators/unicodedata/data"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Generate generates Go files in [outputDir]
func Generate(outputDir string) {
	join := func(path string) string {
		return filepath.Join(outputDir, path)
	}

	sources := fetchAll()

	// parse
	fmt.Println("Parsing Unicode files...")

	err := parseUnicodeDatabase(sources.unicodeData)
	check(err)

	emojis, err := parseAnnexTables(sources.emoji)
	check(err)

	emojisTests := parseEmojisTest(sources.emojiTest)

	mirrors, err := parseMirroring(sources.bidiMirroring)
	check(err)

	dms, compEx, err := parseXML(sources.ucdXML)
	check(err)

	joiningTypes := parseArabicShaping(sources.arabic)

	scripts, err := parseAnnexTables(sources.scripts)
	check(err)

	blocks, err := parseAnnexTables(sources.blocks)
	check(err)

	indicS, err := parseAnnexTables(sources.indicSyllabic)
	check(err)

	indicP, err := parseAnnexTables(sources.indicPositional)
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

	lineBreak, err := parseAnnexTables(sources.lineBreak)
	check(err)

	eastAsianWidth, err := parseAnnexTables(sources.eastAsianWidth)
	check(err)

	sentenceBreaks, err := parseAnnexTables(sources.sentenceBreak)
	check(err)

	graphemeBreaks, err := parseAnnexTables(sources.graphemeBreak)
	check(err)

	scriptsRanges, err := parseAnnexTablesAsRanges(sources.scripts)
	check(err)

	b, err = data.Files.ReadFile("Scripts-iso15924.txt")
	check(err)
	scriptNames, err := parseScriptNames(b)
	check(err)

	derivedCore, err := parseAnnexTables(sources.derivedCore)
	check(err)

	// generate
	process(join("unicodedata/combining_classes.go"), func(w io.Writer) {
		generateCombiningClasses(combiningClasses, w)
	})
	process(join("unicodedata/emojis.go"), func(w io.Writer) {
		generateEmojis(emojis, w)
	})

	process(join("unicodedata/mirroring.go"), func(w io.Writer) {
		generateMirroring(mirrors, w)
	})
	process(join("unicodedata/decomposition.go"), func(w io.Writer) {
		generateDecomposition(dms, compEx, w)
	})
	process(join("unicodedata/arabic.go"), func(w io.Writer) {
		generateArabicShaping(joiningTypes, w)
		generateHasArabicJoining(joiningTypes, scripts, w)
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
	process(join("unicodedata/sentenceBreak.go"), func(w io.Writer) {
		generateSTermProperty(sentenceBreaks, w)
	})
	process(join("unicodedata/graphemeBreak.go"), func(w io.Writer) {
		generateGraphemeBreakProperty(graphemeBreaks, w)
	})

	process(join("harfbuzz/emojis_list_test.go"), func(w io.Writer) {
		generateEmojisTest(emojisTests, w)
	})
	process(join("harfbuzz/ot_use_table.go"), func(w io.Writer) {
		generateUSETable(indicS, indicP, blocks, indicSAdd, indicPAdd, derivedCore, scripts, joiningTypes, w)
	})
	process(join("harfbuzz/ot_vowels_constraints.go"), func(w io.Writer) {
		generateVowelConstraints(scripts, vowelsConstraints, w)
	})
	process(join("harfbuzz/ot_indic_table.go"), func(w io.Writer) {
		generateIndicTable(indicS, indicP, blocks, w)
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
