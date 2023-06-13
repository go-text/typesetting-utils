package src

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

var arabicLigatures = []rune{
	0xF2EE, 0xFC08, 0xFC0E, 0xFC12, 0xFC32, 0xFC3F, 0xFC40, 0xFC41, 0xFC42,
	0xFC44, 0xFC4E, 0xFC5E, 0xFC60, 0xFC61, 0xFC62, 0xFC6A, 0xFC6D, 0xFC6F,
	0xFC70, 0xFC73, 0xFC75, 0xFC86, 0xFC8F, 0xFC91, 0xFC94, 0xFC9C, 0xFC9D,
	0xFC9E, 0xFC9F, 0xFCA1, 0xFCA2, 0xFCA3, 0xFCA4, 0xFCA8, 0xFCAA, 0xFCAC,
	0xFCB0, 0xFCC9, 0xFCCA, 0xFCCB, 0xFCCC, 0xFCCD, 0xFCCE, 0xFCCF, 0xFCD0,
	0xFCD1, 0xFCD2, 0xFCD3, 0xFCD5, 0xFCDA, 0xFCDB, 0xFCDC, 0xFCDD, 0xFD30,
	0xFD88, 0xFEF5, 0xFEF6, 0xFEF7, 0xFEF8, 0xFEF9, 0xFEFA, 0xFEFB, 0xFEFC,
	0xF201, 0xF211, 0xF2EE,
}

type (
	shapingTable struct {
		table    map[rune]shapeMap
		min, max rune
	}
	ligatures map[[3]rune]shapeMap
)

func reverseRunes(a []rune) {
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
}

func runesKey(rs []rune) (out [3]rune) {
	if L := len(rs); L != 2 && L != 3 {
		check(fmt.Errorf("unsupported number of items %d", L))
	}
	for i, r := range rs {
		out[i] = r
	}
	return out
}

// lightweigth version of map[shape]rune
type shapeMap [5]rune

func (db unicodeDatabase) arabicShaping() (shapingTable, ligatures) {
	// initialisation
	st := shapingTable{table: make(map[rune]shapeMap), min: maxUnicode, max: 0}
	lg := make(ligatures)

	// manually add PUA ligatures
	entries := append(db.chars,
		parseUnicodeEntry(strings.Split("F201;PUA ARABIC LIGATURE LELLAH ISOLATED FORM;Lo;0;AL;<isolated> 0644 0644 0647;;;;N;;;;;", ";")),
		parseUnicodeEntry(strings.Split("F211;PUA ARABIC LIGATURE LAM WITH MEEM WITH JEEM INITIAL FORM;Lo;0;AL;<initial> 0644 0645 062C;;;;N;;;;;", ";")),
		parseUnicodeEntry(strings.Split("F2EE;PUA ARABIC LIGATURE SHADDA WITH FATHATAN ISOLATED FORM;Lo;0;AL;<isolated> 0020 064B 0651;;;;N;;;;;", ";")),
	)

	for _, item := range entries {
		if item.shape == none {
			continue
		}

		c, shape, items := item.char, item.shape, item.shapingItems
		if len(items) != 1 {
			// Mark ligatures start with space and are in visual order, so we
			// remove the space and reverse the items.
			if items[0] == 0x0020 {
				items = items[1:]
				reverseRunes(items)
				shape = none
			}
			// We only care about a subset of ligatures
			if !inR(c, arabicLigatures...) {
				continue
			}

			// save ligature
			key := runesKey(items)
			v := lg[key]
			v[shape] = c
			lg[key] = v
		} else {
			// Save shape
			key := items[0]
			v := st.table[key]
			v[shape] = c
			st.table[key] = v

			if key > st.max {
				st.max = key
			}
			if key < st.min {
				st.min = key
			}
		}
	}

	return st, lg
}

func generateArabicShaping(db unicodeDatabase, joining map[rune]ArabicJoining, w io.Writer) {
	shapingTable, ligatures := db.arabicShaping()

	fmt.Fprintln(w, harfbuzzHeader)

	fmt.Fprintln(w, `
	
	import "github.com/go-text/typesetting/language"`)

	// Joining

	// sort for determinism
	var keys []rune
	for r := range joining {
		keys = append(keys, r)
	}
	sortRunes(keys)

	fmt.Fprintf(w, "var arabicJoinings = map[rune]arabicJoining{ // %d entries \n", len(keys))
	for _, r := range keys {
		fmt.Fprintf(w, "0x%04x: %q,\n", r, joining[r])
	}
	fmt.Fprintln(w, "}")

	// Shaping

	if shapingTable.min == 0 || shapingTable.max < shapingTable.min {
		check(errors.New("error: no shaping pair found, something wrong with reading input"))
	}

	fmt.Fprintf(w, "const firstArabicShape = 0x%04x\n", shapingTable.min)
	fmt.Fprintf(w, "const lastArabicShape = 0x%04x\n", shapingTable.max)

	fmt.Fprintln(w, `
	// arabicShaping defines the shaping for arabic runes. Each entry is indexed by
	// the shape, between 0 and 3:
	//   - 0: initial
	//   - 1: medial
	//   - 2: final
	//   - 3: isolated
	// See also the bounds given by [firstArabicShape] and [lastArabicShape].`)
	fmt.Fprintf(w, "var arabicShaping = [...][4]uint16{ // required memory: %d KB \n", (shapingTable.max-shapingTable.min+1)*4*4/1000)
	for c := shapingTable.min; c <= shapingTable.max; c++ {
		fmt.Fprintf(w, "{0x%04x,0x%04x,0x%04x,0x%04x},\n",
			shapingTable.table[c][initial], shapingTable.table[c][medial], shapingTable.table[c][final], shapingTable.table[c][isolated])
	}
	fmt.Fprintln(w, "}")

	// Ligatures

	ligas2 := map[rune][][2]rune{}
	ligas3 := map[rune][][3]rune{}
	ligasMark2 := map[rune][][2]rune{}
	shapes := shapingTable.table
	for key, shapes_ := range ligatures {
		for sha, c := range shapes_ {
			shape := shapeT(sha)
			if c == 0 { // shapes[sha] not defined
				continue
			}

			if key[2] != 0 { // len(key) == 3
				var liga [3]rune
				switch shape {
				case isolated:
					liga = [3]rune{shapes[key[0]][initial], shapes[key[1]][medial], shapes[key[2]][final]}
				case final:
					liga = [3]rune{shapes[key[0]][medial], shapes[key[1]][medial], shapes[key[2]][final]}
				case initial:
					liga = [3]rune{shapes[key[0]][initial], shapes[key[1]][medial], shapes[key[2]][medial]}
				default:
					check(fmt.Errorf("unexpected shape %d %x", sha, c))
				}
				ligas3[liga[0]] = append(ligas3[liga[0]], [3]rune{liga[1], liga[2], c})
			} else { // len(key) == 2
				var liga [2]rune
				switch shape {
				case none:
					liga := key[0:2]
					ligasMark2[liga[0]] = append(ligasMark2[liga[0]], [2]rune{liga[1], c})
					continue
				case isolated:
					liga = [2]rune{shapes[key[0]][initial], shapes[key[1]][final]}
				case final:
					liga = [2]rune{shapes[key[0]][medial], shapes[key[1]][final]}
				case initial:
					liga = [2]rune{shapes[key[0]][initial], shapes[key[1]][medial]}
				default:
					check(fmt.Errorf("unexpected shape %d", sha))
				}
				ligas2[liga[0]] = append(ligas2[liga[0]], [2]rune{liga[1], c})
			}
		}
	}
	var sortedLigas2, sortedLigas3, sortedLigasMark2 []rune
	for r := range ligas2 {
		sortedLigas2 = append(sortedLigas2, r)
	}
	for r := range ligas3 {
		sortedLigas3 = append(sortedLigas3, r)
	}
	for r := range ligasMark2 {
		sortedLigasMark2 = append(sortedLigasMark2, r)
	}
	sortRunes(sortedLigas2)
	sortRunes(sortedLigas3)
	sortRunes(sortedLigasMark2)

	fmt.Fprintln(w)
	fmt.Fprintf(w, `
	type arabicLig struct {
		components []rune // currently with length 1 or 2
		ligature   rune
	}
	
	type arabicTableEntry struct {
		First     rune
		Ligatures []arabicLig
	}
	
	// arabicLigatureTable exposes lam-alef ligatures
	var arabicLigatureTable = [...]arabicTableEntry{`)
	fmt.Fprintln(w)
	for _, first := range sortedLigas2 {
		fmt.Fprintf(w, "  { 0x%04x, []arabicLig{\n", first)
		ligas := ligas2[first]
		sort.Slice(ligas, func(i, j int) bool {
			return ligas[i][0] < ligas[j][0]
		})
		for _, liga := range ligas {
			fmt.Fprintf(w, "    { []rune{0x%04x}, 0x%04x },\n", liga[0], liga[1])
		}
		fmt.Fprintln(w, "  }},")
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, `
	var arabicLigatureMarkTable = [...]arabicTableEntry{`)
	fmt.Fprintln(w)
	for _, first := range sortedLigasMark2 {
		fmt.Fprintf(w, "  { 0x%04x, []arabicLig{\n", first)
		ligas := ligasMark2[first]
		sort.Slice(ligas, func(i, j int) bool {
			return ligas[i][0] < ligas[j][0]
		})
		for _, liga := range ligas {
			fmt.Fprintf(w, "    { []rune{0x%04x}, 0x%04x },\n", liga[0], liga[1])
		}
		fmt.Fprintln(w, "  }},")
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	fmt.Fprintf(w, `
	var arabicLigature3Table = [...]arabicTableEntry{`)
	fmt.Fprintln(w)
	for _, first := range sortedLigas3 {
		fmt.Fprintf(w, "  { 0x%04x, []arabicLig{\n", first)
		ligas := ligas3[first]
		sort.Slice(ligas, func(i, j int) bool {
			return ligas[i][0] < ligas[j][0]
		})
		for _, liga := range ligas {
			fmt.Fprintf(w, "    { []rune{0x%04x, 0x%04x }, 0x%04x },\n", liga[0], liga[1], liga[2])
		}
		fmt.Fprintln(w, "  }},")
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func generateHasArabicJoining(joining map[rune]ArabicJoining, scripts map[string][]rune, w io.Writer) {
	scriptsRev := map[rune]string{}
	for s, rs := range scripts {
		for _, r := range rs {
			scriptsRev[r] = s
		}
	}
	scriptsArabic := map[string]bool{}
	for r, j := range joining {
		if j != T && j != U {
			script := scriptsRev[r]
			if script != "Common" && script != "Inherited" {
				scriptsArabic[script] = true
			}
		}
	}
	var scriptList []string
	for s := range scriptsArabic {
		scriptList = append(scriptList, fmt.Sprintf("language.%s", s))
	}
	sort.Strings(scriptList) // determinism

	fmt.Fprintf(w, `

	// hasArabicJoining return 'true' if the given script has arabic joining.
	func hasArabicJoining(script language.Script) bool {
		switch script {
		case %s:
			return true
		default: 
			return false
		}
	}`, strings.Join(scriptList, ","))
}

// -------------------------------------- PUA remap --------------------------------------

// map [firstRune, firstRune + len(mapping) - 1] to mapping
type puaMapRange struct {
	firstRune rune
	mapping   []rune
}

func packPUAMap(m [][2]rune) (out []puaMapRange) {
	sort.Slice(m, func(i, j int) bool { return m[i][1] < m[j][1] })

	var (
		previousFrom rune = -1
		currentRange puaMapRange
	)
	for _, item := range m {
		to, from := item[0], item[1]
		if from == previousFrom+1 { // still in a range
			currentRange.mapping = append(currentRange.mapping, to)
		} else {
			// flush previous range (if needed) and start a new one
			if currentRange.firstRune != 0 {
				out = append(out, currentRange)
			}
			currentRange = puaMapRange{firstRune: from, mapping: []rune{to}}
		}
		previousFrom = from
	}

	// flush
	out = append(out, currentRange)

	return out
}

func generatePUASwitchCases(m []puaMapRange) string {
	var out strings.Builder
	const runeVar = "r"
	for _, r := range m {
		var code string
		if len(r.mapping) == 1 {
			code = fmt.Sprintf(`case 0x%x == %s:
				return 0x%x
				`, r.firstRune, runeVar, r.mapping[0],
			)
		} else {
			var chunks []string
			for _, to := range r.mapping {
				chunks = append(chunks, fmt.Sprintf("0x%x", to))
			}
			code = fmt.Sprintf(`case 0x%x <= %s && %s <= 0x%x:
				return [...]rune{%s}[%s - 0x%x]
				`, r.firstRune, runeVar, runeVar, r.firstRune+rune(len(r.mapping))-1,
				strings.Join(chunks, ","), runeVar, r.firstRune,
			)
		}
		out.WriteString(code)
	}
	return out.String()
}

func generateArabicPUAMapping(simp, trad [][2]rune, w io.Writer) {
	simpM, tradM := packPUAMap(simp), packPUAMap(trad)

	fmt.Fprintln(w, `// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

	package api
	
	// Code generated by typesettings-utils/generators/unicodedata/cmd/main.go DO NOT EDIT.
	`)

	fmt.Fprintf(w, `
	// Legacy Simplified Arabic encoding. Returns 0 if not found.
	func arabicPUASimpMap(r rune) rune {
		switch {
			%s}
		return 0
	}
	
	
	// Legacy Traditional Arabic encoding. Returns 0 if not found.
	func arabicPUATradMap(r rune) rune {
		switch {
			%s}
		return 0
	}
	`, generatePUASwitchCases(simpM), generatePUASwitchCases(tradM))
}
