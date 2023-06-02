package src

import (
	"fmt"
	"io"
	"testing"

	"github.com/go-text/typesetting-utils/generators/unicodedata/data"
)

var srcs sources

func init() {
	// during dev, set fromCache to true to avoid fetching from the network
	srcs = fetchAll(true)
}

func TestParseUnicodeDatabase(t *testing.T) {
	db := parseUnicodeDatabase(srcs.unicodeData)
	if len(db.chars) != 33797 {
		t.Fatalf("got %d items", len(db.chars))
	}
}

func TestVowel(t *testing.T) {
	scripts, err := parseAnnexTables(srcs.scripts)
	check(err)

	b, err := data.Files.ReadFile("ms-use/IndicShapingInvalidCluster.txt")
	check(err)
	vowelsConstraints := parseUSEInvalidCluster(b)

	// generate
	constraints, _ := aggregateVowelData(scripts, vowelsConstraints)

	if len(constraints["Devanagari"].dict[0x0905].dict) != 12 {
		t.Errorf("expected 12 constraints for rune 0x0905")
	}
}

func TestIndicCombineCategories(t *testing.T) {
	if got := indicCombineCategories("M", "ABOVE_C"); got != 1543 {
		t.Fatalf("expected %d, got %d", 1543, got)
	}
}

func TestIndic(t *testing.T) {
	_, err := parseAnnexTables(srcs.blocks)
	check(err)

	_, err = parseAnnexTables(srcs.indicSyllabic)
	check(err)
	_, err = parseAnnexTables(srcs.indicPositional)
	check(err)
}

func TestScripts(t *testing.T) {
	scriptsRanges, err := parseAnnexTablesAsRanges(srcs.scripts)
	check(err)

	b, err := data.Files.ReadFile("Scripts-iso15924.txt")
	check(err)
	scriptNames, err := parseScriptNames(b)
	check(err)

	fmt.Println(len(compactScriptLookupTable(scriptsRanges, scriptNames)))
}

func TestArabic(t *testing.T) {
	db := parseUnicodeDatabase(srcs.unicodeData)
	joiningTypes := parseArabicShaping(srcs.arabic)
	generateArabicShaping(db, joiningTypes, io.Discard)
}
