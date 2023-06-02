package src

import (
	"fmt"
	"io"
	"strings"
)

// This file groups the constant extracted from the C++ Harfbuzz source code,
// and not coming from a Unicode data source.
// To reduce visual clutter, we also use their values in the generator, so
// that they are duplicated.

// Visual positions in a syllable from left to right.
// copied from ot_indic.go
var indicPositions = map[string]uint8{
	"START": 0,

	"RA_TO_BECOME_REPH": 1,
	"PRE_M":             2,
	"PRE_C":             3,

	"BASE_C":     4,
	"AFTER_MAIN": 5,

	"ABOVE_C": 6,

	"BEFORE_SUB": 7,
	"BELOW_C":    8,
	"AFTER_SUB":  9,

	"BEFORE_POST": 10,
	"POST_C":      11,
	"AFTER_POST":  12,

	"SMVD": 13,

	"END": 14,
}

// map unicode position names to internal Harfbuzz values
func getIndicPositionCategory(s string) uint8 {
	v, ok := indicPositions[s]
	if !ok {
		check(fmt.Errorf("unknown position category %s", s))
	}
	return v
}

var indicSyllabic = map[string]uint8{
	// copied from ot_indic_machine.rl
	"X":            0,
	"C":            1,
	"V":            2,
	"N":            3,
	"H":            4,
	"ZWNJ":         5,
	"ZWJ":          6,
	"M":            7,
	"SM":           8,
	"A":            9,
	"VD":           9,
	"PLACEHOLDER":  10,
	"DOTTEDCIRCLE": 11,
	"RS":           12,
	"MPst":         13,
	"Repha":        14,
	"Ra":           15,
	"CM":           16,
	"Symbol":       17,
	"CS":           18,

	// copied from ot_khmer_machine.rl
	"VAbv": 20,
	"VBlw": 21,
	"VPre": 22,
	"VPst": 23,

	"Robatic": 25,
	"Xgroup":  26,
	"Ygroup":  27,

	// copied from ot_myanmar_machine.rl
	"IV": 2,
	"DB": 3,  // Dot below	     = OT_N
	"GB": 10, // 		     = OT_PLACEHOLDER

	// 32+ are for Myanmar-specific values
	"As": 32, // Asat
	"MH": 35, // Medial Ha
	"MR": 36, // Medial Ra
	"MW": 37, // Medial Wa, Shan Wa
	"MY": 38, // Medial Ya, Mon Na, Mon Ma
	"PT": 39, // Pwo and other tones
	"VS": 40, // Variation selectors
	"ML": 41, // Medial Mon La
}

// map unicode category names to internal Harfbuzz values
func getIndicSyllabicCategory(s string) uint8 {
	v, ok := indicSyllabic[s]
	if !ok {
		check(fmt.Errorf("unknown syllabic category %s", s))
	}
	return v
}

// resolve the numerical value for syllabic and mantra and combine
// them in one uint16
func indicCombineCategories(Ss, Ms string) uint16 {
	S := getIndicSyllabicCategory(Ss)
	M := getIndicPositionCategory(Ms)
	return uint16(S) | uint16(M)<<8
}

var (
	allowedSingles = []rune{0x00A0, 0x25CC}
	allowedBlocks  = []string{
		"Basic Latin",
		"Latin-1 Supplement",
		"Devanagari",
		"Bengali",
		"Gurmukhi",
		"Gujarati",
		"Oriya",
		"Tamil",
		"Telugu",
		"Kannada",
		"Malayalam",
		"Myanmar",
		"Khmer",
		"Vedic Extensions",
		"General Punctuation",
		"Superscripts and Subscripts",
		"Devanagari Extended",
		"Myanmar Extended-B",
		"Myanmar Extended-A",
	}
)

// Convert categories & positions types
var (
	categoryMap = map[string]string{
		"Other":                       "X",
		"Avagraha":                    "Symbol",
		"Bindu":                       "SM",
		"Brahmi_Joining_Number":       "PLACEHOLDER", // Don't care.
		"Cantillation_Mark":           "A",
		"Consonant":                   "C",
		"Consonant_Dead":              "C",
		"Consonant_Final":             "CM",
		"Consonant_Head_Letter":       "C",
		"Consonant_Initial_Postfixed": "C", // TODO
		"Consonant_Killer":            "M", // U+17CD only.
		"Consonant_Medial":            "CM",
		"Consonant_Placeholder":       "PLACEHOLDER",
		"Consonant_Preceding_Repha":   "Repha",
		"Consonant_Prefixed":          "X", // Don't care.
		"Consonant_Subjoined":         "CM",
		"Consonant_Succeeding_Repha":  "CM",
		"Consonant_With_Stacker":      "CS",
		"Gemination_Mark":             "SM", // https://github.com/harfbuzz/harfbuzz/issues/552
		"Invisible_Stacker":           "H",
		"Joiner":                      "ZWJ",
		"Modifying_Letter":            "X",
		"Non_Joiner":                  "ZWNJ",
		"Nukta":                       "N",
		"Number":                      "PLACEHOLDER",
		"Number_Joiner":               "PLACEHOLDER", // Don't care.
		"Pure_Killer":                 "M",           // Is like a vowel matra.
		"Register_Shifter":            "RS",
		"Syllable_Modifier":           "SM",
		"Tone_Letter":                 "X",
		"Tone_Mark":                   "N",
		"Virama":                      "H",
		"Visarga":                     "SM",
		"Vowel":                       "V",
		"Vowel_Dependent":             "M",
		"Vowel_Independent":           "V",
	}
	positionMap = map[string]string{
		"Not_Applicable": "END",

		"Left":   "PRE_C",
		"Top":    "ABOVE_C",
		"Bottom": "BELOW_C",
		"Right":  "POST_C",

		// These should resolve to the position of the last part of the split sequence.
		"Bottom_And_Right":         "POST_C",
		"Left_And_Right":           "POST_C",
		"Top_And_Bottom":           "BELOW_C",
		"Top_And_Bottom_And_Left":  "BELOW_C",
		"Top_And_Bottom_And_Right": "POST_C",
		"Top_And_Left":             "ABOVE_C",
		"Top_And_Left_And_Right":   "POST_C",
		"Top_And_Right":            "POST_C",

		"Overstruck":        "AFTER_MAIN",
		"Visual_order_left": "PRE_M",
	}

	categoryOverrides = map[rune]string{
		// These are the variation-selectors. They only appear in the Myanmar grammar
		// but are not Myanmar-specific
		0xFE00: "VS",
		0xFE01: "VS",
		0xFE02: "VS",
		0xFE03: "VS",
		0xFE04: "VS",
		0xFE05: "VS",
		0xFE06: "VS",
		0xFE07: "VS",
		0xFE08: "VS",
		0xFE09: "VS",
		0xFE0A: "VS",
		0xFE0B: "VS",
		0xFE0C: "VS",
		0xFE0D: "VS",
		0xFE0E: "VS",
		0xFE0F: "VS",

		// These appear in the OT Myanmar spec, but are not Myanmar-specific
		0x2015: "PLACEHOLDER",
		0x2022: "PLACEHOLDER",
		0x25FB: "PLACEHOLDER",
		0x25FC: "PLACEHOLDER",
		0x25FD: "PLACEHOLDER",
		0x25FE: "PLACEHOLDER",

		// Indic

		0x0930: "Ra", // Devanagari
		0x09B0: "Ra", // Bengali
		0x09F0: "Ra", // Bengali
		0x0A30: "Ra", // Gurmukhi 	No Reph
		0x0AB0: "Ra", // Gujarati
		0x0B30: "Ra", // Oriya
		0x0BB0: "Ra", // Tamil 	No Reph
		0x0C30: "Ra", // Telugu 	Reph formed only with ZWJ
		0x0CB0: "Ra", // Kannada
		0x0D30: "Ra", // Malayalam 	No Reph, Logical Repha

		// The following act more like the Bindus.
		0x0953: "SM",
		0x0954: "SM",

		// U+0A40 GURMUKHI VOWEL SIGN II may be preceded by U+0A02 GURMUKHI SIGN BINDI.
		0x0A40: "MPst",

		// The following act like consonants.
		0x0A72: "C",
		0x0A73: "C",
		0x1CF5: "C",
		0x1CF6: "C",

		// TODO: The following should only be allowed after a Visarga.
		// For now, just treat them like regular tone marks.
		0x1CE2: "A",
		0x1CE3: "A",
		0x1CE4: "A",
		0x1CE5: "A",
		0x1CE6: "A",
		0x1CE7: "A",
		0x1CE8: "A",

		// TODO: The following should only be allowed after some of
		// the nasalization marks, maybe only for U+1CE9..U+1CF1.
		// For now, just treat them like tone marks.
		0x1CED: "A",

		// The following take marks in standalone clusters, similar to Avagraha.
		0xA8F2: "Symbol",
		0xA8F3: "Symbol",
		0xA8F4: "Symbol",
		0xA8F5: "Symbol",
		0xA8F6: "Symbol",
		0xA8F7: "Symbol",
		0x1CE9: "Symbol",
		0x1CEA: "Symbol",
		0x1CEB: "Symbol",
		0x1CEC: "Symbol",
		0x1CEE: "Symbol",
		0x1CEF: "Symbol",
		0x1CF0: "Symbol",
		0x1CF1: "Symbol",

		0x0A51: "M", // https://github.com/harfbuzz/harfbuzz/issues/524

		// According to ScriptExtensions.txt, these Grantha marks may also be used in Tamil,
		// so the Indic shaper needs to know their categories.
		0x11301: "SM",
		0x11302: "SM",
		0x11303: "SM",
		0x1133B: "N",
		0x1133C: "N",

		0x0AFB: "N", // https://github.com/harfbuzz/harfbuzz/issues/552
		0x0B55: "N", // https://github.com/harfbuzz/harfbuzz/issues/2849

		0x09FC: "PLACEHOLDER", // https://github.com/harfbuzz/harfbuzz/pull/1613
		0x0C80: "PLACEHOLDER", // https://github.com/harfbuzz/harfbuzz/pull/623
		0x0D04: "PLACEHOLDER", // https://github.com/harfbuzz/harfbuzz/pull/3511

		0x25CC: "DOTTEDCIRCLE",

		// Khmer

		0x179A: "Ra",

		0x17CC: "Robatic",
		0x17C9: "Robatic",
		0x17CA: "Robatic",

		0x17C6: "Xgroup",
		0x17CB: "Xgroup",
		0x17CD: "Xgroup",
		0x17CE: "Xgroup",
		0x17CF: "Xgroup",
		0x17D0: "Xgroup",
		0x17D1: "Xgroup",

		0x17C7: "Ygroup",
		0x17C8: "Ygroup",
		0x17DD: "Ygroup",
		0x17D3: "Ygroup", // Just guessing. Uniscribe doesn"t categorize it.

		0x17D9: "PLACEHOLDER", // https://github.com/harfbuzz/harfbuzz/issues/2384

		// Myanmar

		// https://docs.microsoft.com/en-us/typography/script-development/myanmar#analyze

		0x104E: "C", // The spec says C, IndicSyllableCategory says Consonant_Placeholder

		0x1004: "Ra",
		0x101B: "Ra",
		0x105A: "Ra",

		0x1032: "A",
		0x1036: "A",

		0x103A: "As",

		// 0x1040: "D0", // XXX The spec says D0, but Uniscribe doesn"t seem to do.

		0x103E: "MH",
		0x1060: "ML",
		0x103C: "MR",
		0x103D: "MW",
		0x1082: "MW",
		0x103B: "MY",
		0x105E: "MY",
		0x105F: "MY",

		0x1063: "PT",
		0x1064: "PT",
		0x1069: "PT",
		0x106A: "PT",
		0x106B: "PT",
		0x106C: "PT",
		0x106D: "PT",
		0xAA7B: "PT",

		0x1038: "SM",
		0x1087: "SM",
		0x1088: "SM",
		0x1089: "SM",
		0x108A: "SM",
		0x108B: "SM",
		0x108C: "SM",
		0x108D: "SM",
		0x108F: "SM",
		0x109A: "SM",
		0x109B: "SM",
		0x109C: "SM",

		0x104A: "PLACEHOLDER",
	}
	positionOverrides = map[rune]string{
		0x0A51: "BELOW_C", // https://github.com/harfbuzz/harfbuzz/issues/524

		0x0B01: "BEFORE_SUB", // Oriya Bindu is BeforeSub in the spec.
	}
)

func matraPosLeft(u rune, block string) string {
	return "PRE_M"
}

func matraPosRight(u rune, block string) string {
	switch block {
	case "Devanagari":
		return "AFTER_SUB"
	case "Bengali":
		return "AFTER_POST"
	case "Gurmukhi":
		return "AFTER_POST"
	case "Gujarati":
		return "AFTER_POST"
	case "Oriya":
		return "AFTER_POST"
	case "Tamil":
		return "AFTER_POST"
	case "Telugu":
		if u <= 0x0C42 {
			return "BEFORE_SUB"
		} else {
			return "AFTER_SUB"
		}
	case "Kannada":
		if u < 0x0CC3 || u > 0x0CD6 {
			return "BEFORE_SUB"
		} else {
			return "AFTER_SUB"
		}
	case "Malayalam":
		return "AFTER_POST"
	}
	return "AFTER_SUB"
}

func matraPosTop(u rune, block string) string {
	// BENG and MLYM don't have top matras.
	switch block {
	case "Devanagari":
		return "AFTER_SUB"
	case "Gurmukhi":
		return "AFTER_POST" // Deviate from spec
	case "Gujarati":
		return "AFTER_SUB"
	case "Oriya":
		return "AFTER_MAIN"
	case "Tamil":
		return "AFTER_SUB"
	case "Telugu":
		return "BEFORE_SUB"
	case "Kannada":
		return "BEFORE_SUB"
	}
	return "AFTER_SUB"
}

func matraPosBottom(u rune, block string) string {
	switch block {
	case "Devanagari":
		return "AFTER_SUB"
	case "Bengali":
		return "AFTER_SUB"
	case "Gurmukhi":
		return "AFTER_POST"
	case "Gujarati":
		return "AFTER_POST"
	case "Oriya":
		return "AFTER_SUB"
	case "Tamil":
		return "AFTER_POST"
	case "Telugu":
		return "BEFORE_SUB"
	case "Kannada":
		return "BEFORE_SUB"
	case "Malayalam":
		return "AFTER_POST"
	}
	return "AFTER_SUB"
}

func indicMatraPosition(u rune, pos, block string) string { // Reposition matra
	switch pos {
	case "PRE_C":
		return matraPosLeft(u, block)
	case "POST_C":
		return matraPosRight(u, block)
	case "ABOVE_C":
		return matraPosTop(u, block)
	case "BELOW_C":
		return matraPosBottom(u, block)
	}
	panic("")
}

func positionToCategory(pos string) string {
	switch pos {
	case "PRE_C":
		return "VPre"
	case "ABOVE_C":
		return "VAbv"
	case "BELOW_C":
		return "VBlw"
	case "POST_C":
		return "VPst"
	}
	panic("")
}

type indicInfo [3]string

func (ii indicInfo) unpack() (a, b, c string) { return ii[0], ii[1], ii[2] }

func aggregateIndicTable(indicS, indicP, blocks map[string][]rune) (map[rune]indicInfo, map[rune]indicInfo, indicInfo) {
	defaultsIndic := [3]string{"Other", "Not_Applicable", "No_Block"}

	// Merge unicodeData into one dict:
	unicodeData := [3]map[rune]string{{}, {}, {}}

	for t, rs := range indicS {
		for _, r := range rs {
			unicodeData[0][r] = t
		}
	}
	for t, rs := range indicP {
		for _, r := range rs {
			unicodeData[1][r] = t
		}
	}
	for t, rs := range blocks {
		for _, r := range rs {
			unicodeData[2][r] = t
		}
	}

	combined := map[rune]indicInfo{}
	for i, d := range unicodeData {
		for u, v := range d {
			vals, ok := combined[u]
			if i == 2 && !ok {
				continue
			}
			if !ok {
				vals = defaultsIndic
			}
			vals[i] = v
			combined[u] = vals
		}
	}

	// filter by ALLOWED_SINGLES and ALLOWED_BLOCKS
	for k, v := range combined {
		if !(inR(k, allowedSingles...) || in(v[2], allowedBlocks...)) {
			delete(combined, k)
		}
	}

	defaults := indicInfo{categoryMap[defaultsIndic[0]], positionMap[defaultsIndic[1]], defaultsIndic[2]}

	indicData := map[rune]indicInfo{}
	for k, v := range combined {
		cat, pos, block := v.unpack()
		cat = categoryMap[cat]
		pos = positionMap[pos]
		indicData[k] = indicInfo{cat, pos, block}
	}

	for k, newCat := range categoryOverrides {
		v, ok := indicData[k]
		if !ok {
			v = defaults
		}
		_, pos, _ := v.unpack()
		indicData[k] = indicInfo{newCat, pos, unicodeData[2][k]}
	}

	// We only expect position for certain types
	positionedCategories := []string{"CM", "SM", "RS", "H", "M", "MPst"}
	for k, v := range indicData {
		cat, _, block := v.unpack()
		if !in(cat, positionedCategories...) {
			indicData[k] = indicInfo{cat, "END", block}
		}
	}

	// Position overrides are more complicated

	// Keep in sync with CONSONANT_FLAGS in the shaper
	consonantCategories := []string{"C", "CS", "Ra", "CM", "V", "PLACEHOLDER", "DOTTEDCIRCLE"}
	matraCategories := []string{"M", "MPst"}
	smvdCategories := []string{"SM", "VD", "A", "Symbol"}
	for k, v := range indicData {
		cat, pos, block := v.unpack()
		if in(cat, consonantCategories...) {
			pos = "BASE_C"
		} else if in(cat, matraCategories...) {
			if strings.HasPrefix(block, "Khmer") || strings.HasPrefix(block, "Myanmar") {
				cat = positionToCategory(pos)
			} else {
				pos = indicMatraPosition(k, pos, block)
			}
		} else if in(cat, smvdCategories...) {
			pos = "SMVD"
		}
		indicData[k] = indicInfo{cat, pos, block}
	}

	for k, newPos := range positionOverrides {
		v, ok := indicData[k]
		if !ok {
			v = defaults
		}
		cat, _, _ := v.unpack()
		indicData[k] = indicInfo{cat, newPos, unicodeData[2][k]}
	}

	values := [3]map[string]int{
		{defaults[0]: 1},
		{defaults[1]: 1},
		{defaults[2]: 1},
	}
	for _, vv := range indicData {
		for i, v := range vv {
			values[i][v] = values[i][v] + 1
		}
	}

	// Move the outliers NO-BREAK SPACE and DOTTED CIRCLE out
	singles := map[rune]indicInfo{}
	for _, u := range allowedSingles {
		singles[u] = indicData[u]
		delete(indicData, u)
	}

	return indicData, singles, defaults
}

const harfbuzzHeader = `// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

// Code generated by typesettings-utils/generators/unicodedata/cmd/main.go DO NOT EDIT.`

func generateIndicTable(indicS, indicP, blocks map[string][]rune, w io.Writer) (starts, ends []rune) {
	data, singles, defaults := aggregateIndicTable(indicS, indicP, blocks)

	fmt.Fprintln(w, harfbuzzHeader)

	total := 0
	used := 0
	lastBlock := ""
	printBlock := func(block string, start, end rune) {
		if block != "" && block != lastBlock {
			fmt.Fprintln(w)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  /* %s */\n", block)
		}
		num := 0
		if start%8 != 0 {
			check(fmt.Errorf("in printBlock, expected start%%8 == 0, got %d", start))
		}
		if (end+1)%8 != 0 {
			check(fmt.Errorf("in printBlock, expected (end+1)%%8 == 0, got %d", end+1))
		}
		for u := start; u <= end; u++ {
			if u%16 == 0 {
				fmt.Fprintln(w)
				fmt.Fprintf(w, "  /* %04X */", u)
			}
			d, in := data[u]
			if in {
				num += 1
			} else {
				d = defaults
			}
			fmt.Fprintf(w, "0x%x,", indicCombineCategories(d[0], d[1]))
		}
		total += int(end - start + 1)
		used += num
		if block != "" {
			lastBlock = block
		}
	}
	var uu []rune
	for u := range data {
		uu = append(uu, u)
	}
	sortRunes(uu)

	last := rune(-100000)
	offset := 0
	fmt.Fprintln(w, "var indicTable = [...]uint16{")
	var offsetsDef string
	for _, u := range uu {
		if u <= last {
			continue
		}

		block := data[u][2]

		start := u / 8 * 8
		end := start + 1
		for inR(end, uu...) && block == data[end][2] {
			end += 1
		}
		end = (end-1)/8*8 + 7

		if start != last+1 {
			if start-last <= 1+16*2 {
				printBlock("", last+1, start-1)
			} else {
				if last >= 0 {
					ends = append(ends, last+1)
					offset += int(ends[len(ends)-1] - starts[len(starts)-1])
				}
				fmt.Fprintln(w)
				fmt.Fprintln(w)
				offsetsDef += fmt.Sprintf("offsetIndic0x%04xu = %d \n", start, offset)
				starts = append(starts, start)
			}
		}

		printBlock(block, start, end)
		last = end
	}

	ends = append(ends, last+1)
	offset += int(ends[len(ends)-1] - starts[len(starts)-1])
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	occupancy := used * 100. / total
	pageBits := 12
	fmt.Fprintf(w, "}; /* Table items: %d; occupancy: %d%% */\n", offset, occupancy)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "const (")
	fmt.Fprintln(w, offsetsDef)
	fmt.Fprintln(w, ")")

	fmt.Fprintln(w, "func indicGetCategories (u rune) uint16 {")
	fmt.Fprintf(w, "  switch u >> %d { \n", pageBits)

	pagesSet := map[rune]bool{}
	for _, u := range append(starts, ends...) {
		pagesSet[u>>pageBits] = true
	}
	for k := range singles {
		pagesSet[k>>pageBits] = true
	}
	var pages []rune
	for p := range pagesSet {
		pages = append(pages, p)
	}
	sortRunes(pages)
	for _, p := range pages {
		fmt.Fprintf(w, "    case 0x%0X:\n", p)
		for u, d := range singles {
			if p != u>>pageBits {
				continue
			}
			fmt.Fprintf(w, "      if u == 0x%04X {return 0x%x};\n", u, indicCombineCategories(d[0], d[1]))
		}
		for i, start := range starts {
			end := ends[i]
			if p != start>>pageBits && p != end>>pageBits {
				continue
			}
			offset := fmt.Sprintf("offsetIndic0x%04xu", start)
			fmt.Fprintf(w, "      if  0x%04X <= u && u <= 0x%04X {return indicTable[u - 0x%04X + %s]};\n", start, end-1, start, offset)
		}

		fmt.Fprintln(w, "")
	}
	fmt.Fprintln(w, "  }")
	fmt.Fprintf(w, "  return 0x%x\n", indicCombineCategories("X", "END"))
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	// Maintain at least 50% occupancy in the table */
	if occupancy < 50 {
		check(fmt.Errorf("table too sparse, please investigate: %d", occupancy))
	}

	return starts, ends // to do some basic tests
}
