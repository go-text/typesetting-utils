package src

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/go-text/typesetting-utils/generators/unicodedata/data"
)

var srcs sources

func init() {
	srcs = fetchAll()
}

func TestVowel(t *testing.T) {
	err := parseUnicodeDatabase(srcs.unicodeData)
	check(err)

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
	if got := indicCombineCategories("Pure_Killer", "Top"); got != 1543 {
		t.Fatalf("expected %d, got %d", 1543, got)
	}
}

func TestIndic(t *testing.T) {
	err := parseUnicodeDatabase(srcs.unicodeData)
	check(err)

	blocks, err := parseAnnexTables(srcs.blocks)
	check(err)

	indicS, err := parseAnnexTables(srcs.indicSyllabic)
	check(err)
	indicP, err := parseAnnexTables(srcs.indicPositional)
	check(err)

	startsExp := []rune{0x0028, 0x00B0, 0x0900, 0x1000, 0x1780, 0x1CD0, 0x2008, 0x2070, 0xA8E0, 0xA9E0, 0xAA60}
	endsExp := []rune{0x003F + 1, 0x00D7 + 1, 0x0DF7 + 1, 0x109F + 1, 0x17EF + 1, 0x1CFF + 1, 0x2017 + 1, 0x2087 + 1, 0xA8FF + 1, 0xA9FF + 1, 0xAA7F + 1}
	starts, ends := generateIndicTable(indicS, indicP, blocks, io.Discard)

	if !reflect.DeepEqual(starts, startsExp) {
		t.Fatalf("wrong starts; expected %v, got %v", startsExp, starts)
	}
	if !reflect.DeepEqual(ends, endsExp) {
		t.Fatalf("wrong ends; expected %v, got %v", endsExp, ends)
	}
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
