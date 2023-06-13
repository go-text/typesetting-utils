package src

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func generateUSETable(generalCategory map[rune]string, indicS, indicP, blocks, indicSAdd, indicPAdd, derivedCoreProperties, scripts map[string][]rune,
	joining map[rune]ArabicJoining, w io.Writer,
) {
	data := aggregateUSETable(generalCategory, indicS, indicP, blocks, indicSAdd, indicPAdd, derivedCoreProperties, scripts, joining)

	fmt.Fprintln(w, harfbuzzHeader)

	var total int
	used := 0
	lastBlock := ""
	printBlock := func(block string, start, end rune) {
		if block != "" && block != lastBlock {
			fmt.Fprintln(w)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  /* %s */\n", block)
			if start%16 != 0 {
				fmt.Fprint(w, strings.Repeat(" ", int(20+(start%16*6))))
			}
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
			var ds string
			if in {
				num += 1
				ds = d[0]
			} else if _, has := generalCategory[u]; has {
				ds = "O"
			} else {
				ds = "WJ"
			}
			fmt.Fprintf(w, "%6s,", "_"+ds)
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
	// num := 0
	offset := 0
	var starts, ends []rune
	var useMappingKeys []string
	for k := range useMapping {
		useMappingKeys = append(useMappingKeys, k)
	}
	sort.Strings(useMappingKeys)

	fmt.Fprintln(w, "const (")
	for _, k := range useMappingKeys {
		if len(usePositions[k]) != 0 {
			continue
		}
		fmt.Fprintf(w, "_%s	 = useSM_ex_%s\n", k, k)
	}
	fmt.Fprintln(w, ")")

	var usePositionKeys []string
	for k := range usePositions {
		usePositionKeys = append(usePositionKeys, k)
	}
	sort.Strings(usePositionKeys)
	fmt.Fprintln(w, "const (")
	for _, k := range usePositionKeys {
		v := usePositions[k]
		if len(v) == 0 {
			continue
		}
		var sortedVKeys []string
		for k := range v {
			sortedVKeys = append(sortedVKeys, k)
		}
		sort.Strings(sortedVKeys)
		for _, suf := range sortedVKeys {
			tag := k + suf
			fmt.Fprintf(w, "_%s = useSM_ex_%s\n", tag, tag)
		}
	}
	fmt.Fprintln(w, ")")

	offsetsDef := ""
	fmt.Fprintln(w, "var  useTable = [...]uint8{")
	for _, u := range uu {
		if u <= last {
			continue
		}
		if data[u][0] == "O" {
			continue
		}
		block := data[u][1]

		start := u / 8 * 8
		end := start + 1
		for inR(end, uu...) && block == data[end][1] {
			end += 1
		}
		end = (end-1)/8*8 + 7

		if start != last+1 {
			if start-last <= 1+16*3 {
				printBlock("", last+1, start-1)
			} else {
				if last >= 0 {
					ends = append(ends, last+1)
					offset += int(ends[len(ends)-1] - starts[len(starts)-1])
				}
				fmt.Fprintln(w)
				fmt.Fprintln(w)
				offsetsDef += fmt.Sprintf("offsetUSE0x%04xu = %d \n", start, offset)
				starts = append(starts, start)
			}

			printBlock(block, start, end)
			last = end
		}
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

	fmt.Fprintln(w, "func getUSECategory (u rune) uint8 {")
	fmt.Fprintf(w, "  switch u >> %d {", pageBits)

	pagesSet := map[rune]bool{}
	for _, u := range append(starts, ends...) {
		pagesSet[u>>pageBits] = true
	}
	var pages []rune
	for p := range pagesSet {
		pages = append(pages, p)
	}
	sortRunes(pages)
	for _, p := range pages {
		fmt.Fprintf(w, "    case 0x%0X:\n", p)
		for i, start := range starts {
			end := ends[i]
			if p != start>>pageBits && p != end>>pageBits {
				continue
			}
			offset := fmt.Sprintf("offsetUSE0x%04xu", start)
			fmt.Fprintf(w, "      if  0x%04X <= u && u <= 0x%04X { return useTable[u - 0x%04X + %s]}; \n", start, end-1, start, offset)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "  }")
	fmt.Fprintln(w, "  return useSM_ex_O;")
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	// maintain at least 50% occupancy in the table */
	if occupancy < 50 {
		check(fmt.Errorf("table too sparse, please investigate: %d", occupancy))
	}
}

func aggregateUSETable(generalCategory map[rune]string, indicS, indicP, blocks, indicSAdd, indicPAdd, derivedCoreProperties, scripts map[string][]rune,
	joining map[rune]ArabicJoining,
) map[rune][2]string {
	// special cases: https://github.com/MicrosoftDocs/typography-issues/issues/336
	indicSAdd["Syllable_Modifier"] = indicSAdd["Consonant_Final_Modifier"]
	delete(indicSAdd, "Consonant_Final_Modifier")
	indicPAdd["Not_Applicable"] = indicPAdd["NA"]
	delete(indicPAdd, "NA")
	derivedCoreProperties = map[string][]rune{"Default_Ignorable_Code_Point": derivedCoreProperties["Default_Ignorable_Code_Point"]}

	// aggregate each file input

	// IndicSyllabic, IndicsPositional, ArabicShaping, DerivedCoreProperties, General, Blocks, Scripts
	data := [7]map[rune]string{{}, {}, {}, {}, {}, {}, {}}
	agg := func(d map[rune]string, runes map[string][]rune) {
		for s, rs := range runes {
			for _, r := range rs {
				d[r] = s
			}
		}
	}

	// IndicSyllabicCategory.txt
	agg(data[0], indicS)
	// IndicPositionalCategory.txt
	agg(data[1], indicP)
	// ArabicShaping.txt
	for r, j := range joining { // do not uses groups
		if j == Alaph || j == DalathRish {
			j = R
		}
		data[2][r] = "jt_" + string(j)
	}
	// DerivedCoreProperties.txt
	agg(data[3], derivedCoreProperties)
	// UnicodeData.txt
	data[4] = generalCategory
	// Blocks.txt
	agg(data[5], blocks)
	// Scripts.txt
	agg(data[6], scripts)
	// IndicSyllabicCategory-Additional.txt
	agg(data[0], indicSAdd)
	// IndicPositionalCategory-Additional.txt
	agg(data[1], indicPAdd)

	// number of occurences of each property
	values := [7]map[string]int{{}, {}, {}, {}, {}, {}, {}}
	for i, d := range data {
		for _, s := range d {
			values[i][s]++
		}
	}

	defaults := [...]string{"Other", "Not_Applicable", "jt_X", "", "Cn", "No_Block", "Unknown"}

	// Merge data into one dict:
	for i, v := range defaults {
		values[i][v]++
	}
	combined := map[rune][7]string{}
	for i, d := range data {
		for u, v := range d {
			vals, in := combined[u]
			if !in {
				if i >= 4 {
					continue
				}
				vals = defaults
			}
			vals[i] = v
			combined[u] = vals
		}
	}

	disabledScripts := map[string]bool{
		"Arabic":    true,
		"Lao":       true,
		"Samaritan": true,
		"Syriac":    true,
		"Thai":      true,
	}

	for k, v := range combined {
		if disabledScripts[v[6]] {
			delete(combined, k)
		}
	}

	return mapToUse(combined)
}

func in(v string, vs ...string) bool {
	for _, s := range vs {
		if v == s {
			return true
		}
	}
	return false
}

func inR(v rune, vs ...rune) bool {
	for _, s := range vs {
		if v == s {
			return true
		}
	}
	return false
}

func isBase(U rune, UISC, UDI, UGC, AJT string) bool {
	return in(UISC, "Number", "Consonant", "Consonant_Head_Letter",
		"Tone_Letter", "Vowel_Independent") ||
		// https://github.com/MicrosoftDocs/typography-issues/issues/484
		(in(AJT, "jt_C", "jt_D", "jt_L", "jt_R") && UISC != "Joiner") ||
		(UGC == "Lo" && in(UISC, "Avagraha", "Bindu", "Consonant_Final", "Consonant_Medial",
			"Consonant_Subjoined", "Vowel", "Vowel_Dependent"))
}

func isBaseNum(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Brahmi_Joining_Number"
}

func isBaseOther(U rune, UISC, _, _, _ string) bool {
	if UISC == "Consonant_Placeholder" {
		return true
	}
	return inR(U, 0x2015, 0x2022, 0x25FB, 0x25FC, 0x25FD, 0x25FE)
}

// Also includes VARIATION_SELECTOR, WJ, and ZWJ
func isCGJ(_ rune, UISC, UDI, UGC, _ string) bool {
	return UISC == "Joiner" || (UDI != "" && in(UGC, "Mc", "Me", "Mn"))
}

func isConsFinal(U rune, UISC, UDI, UGC, AJT string) bool {
	return (UISC == "Consonant_Final" && UGC != "Lo") ||
		UISC == "Consonant_Succeeding_Repha"
}

func isConsFinalMod(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Syllable_Modifier"
}

func isConsMed(U rune, UISC, UDI, UGC, AJT string) bool {
	// Consonant_Initial_Postfixed is new in Unicode 11; not in the spec.
	return (UISC == "Consonant_Medial" && UGC != "Lo" ||
		UISC == "Consonant_Initial_Postfixed")
}

func isConsMod(U rune, UISC, UDI, UGC, AJT string) bool {
	return in(UISC, "Nukta", "Gemination_Mark", "Consonant_Killer")
}

func isConsSub(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Consonant_Subjoined" && UGC != "Lo"
}

func isConsWithStacker(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Consonant_With_Stacker"
}

func isHalant(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Virama" && !isHalantOrVowelModifier(U, UISC, UDI, UGC, AJT)
}

func isHalantOrVowelModifier(U rune, UISC, UDI, UGC, AJT string) bool {
	// Split off of HALANT
	return U == 0x0DCA
}

func isHalantNum(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Number_Joiner"
}

func isHieroglyph(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Hieroglyph"
}

func isHieroglyphJoiner(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Hieroglyph_Joiner"
}

func isHieroglyphSegmentBegin(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Hieroglyph_Segment_Begin"
}

func isHieroglyphSegmentEnd(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Hieroglyph_Segment_End"
}

func isInvisibleStacker(U rune, UISC, UDI, UGC, AJT string) bool {
	// Split off of HALANT
	return UISC == "Invisible_Stacker" && !isSakot(U, UISC, UDI, UGC, AJT)
}

func isZwnj(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Non_Joiner"
}

func isOther(U rune, UISC, UDI, UGC, AJT string) bool {
	// Also includes BASE_IND, and SYM
	return (UGC == "Po" || in(UISC, "Consonant_Dead", "Joiner", "Modifying_Letter", "Other")) &&
		!isBase(U, UISC, UDI, UGC, AJT) &&
		!isBaseOther(U, UISC, UDI, UGC, AJT) &&
		!isCGJ(U, UISC, UDI, UGC, AJT) &&
		!isSymMod(U, UISC, UDI, UGC, AJT) &&
		!isWordJoiner(U, UISC, UDI, UGC, AJT)
}

func isRepha(U rune, UISC, UDI, UGC, AJT string) bool {
	return in(UISC, "Consonant_Preceding_Repha", "Consonant_Prefixed")
}

// Split off of HALANT
func isSakot(U rune, UISC, UDI, UGC, AJT string) bool {
	return U == 0x1A60
}

func isSymMod(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Symbol_Modifier"
}

func isVowel(U rune, UISC, UDI, UGC, AJT string) bool {
	return UISC == "Pure_Killer" ||
		(UGC != "Lo" && in(UISC, "Vowel", "Vowel_Dependent"))
}

func isVowelMod(U rune, UISC, UDI, UGC, AJT string) bool {
	return (in(UISC, "Tone_Mark", "Cantillation_Mark", "Register_Shifter", "Visarga") ||
		(UGC != "Lo" && UISC == "Bindu"))
}

func isWordJoiner(U rune, UISC, UDI, UGC, AJT string) bool {
	// Also includes Rsv
	return (UDI != "" && !inR(U, 0x115F, 0x1160, 0x3164, 0xFFA0, 0x1BCA0, 0x1BCA1, 0x1BCA2, 0x1BCA3) &&
		UISC == "Other" &&
		!isCGJ(U, UISC, UDI, UGC, AJT)) || UGC == "Cn"
}

var useMapping = map[string]func(U rune, UISC, UDI, UGC, AJT string) bool{
	"B":    isBase,
	"N":    isBaseNum,
	"GB":   isBaseOther,
	"CGJ":  isCGJ,
	"F":    isConsFinal,
	"FM":   isConsFinalMod,
	"M":    isConsMed,
	"CM":   isConsMod,
	"SUB":  isConsSub,
	"CS":   isConsWithStacker,
	"H":    isHalant,
	"HVM":  isHalantOrVowelModifier,
	"HN":   isHalantNum,
	"IS":   isInvisibleStacker,
	"G":    isHieroglyph,
	"J":    isHieroglyphJoiner,
	"SB":   isHieroglyphSegmentBegin,
	"SE":   isHieroglyphSegmentEnd,
	"ZWNJ": isZwnj,
	"O":    isOther,
	"R":    isRepha,
	"Sk":   isSakot,
	"SM":   isSymMod,
	"V":    isVowel,
	"VM":   isVowelMod,
	"WJ":   isWordJoiner,
}

var usePositions = map[string]map[string][]string{
	"F": {
		"Abv": {"Top"},
		"Blw": {"Bottom"},
		"Pst": {"Right"},
	},
	"M": {
		"Abv": {"Top"},
		"Blw": {"Bottom", "Bottom_And_Left", "Bottom_And_Right"},
		"Pst": {"Right"},
		"Pre": {"Left", "Top_And_Bottom_And_Left"},
	},
	"CM": {
		"Abv": {"Top"},
		"Blw": {"Bottom", "Overstruck"},
	},
	"V": {
		"Abv": {"Top", "Top_And_Bottom", "Top_And_Bottom_And_Right", "Top_And_Right"},
		"Blw": {"Bottom", "Overstruck", "Bottom_And_Right"},
		"Pst": {"Right"},
		"Pre": {"Left", "Top_And_Left", "Top_And_Left_And_Right", "Left_And_Right"},
	},
	"VM": {
		"Abv": {"Top"},
		"Blw": {"Bottom", "Overstruck"},
		"Pst": {"Right"},
		"Pre": {"Left"},
	},
	"SM": {
		"Abv": {"Top"},
		"Blw": {"Bottom"},
	},
	"H":   nil,
	"HVM": nil,
	"IS":  nil,
	"B":   nil,
	"FM": {
		"Abv": {"Top"},
		"Blw": {"Bottom"},
		"Pst": {"Not_Applicable"},
	},
	"R":   nil,
	"SUB": nil,
}

func mapToUse(data map[rune][7]string) map[rune][2]string {
	out := map[rune][2]string{}
	for U, vals := range data {
		UISC, UIPC, AJT, UDI, UGC, UBlock, _ := vals[0], vals[1], vals[2], vals[3], vals[4], vals[5], vals[6]
		// Resolve Indic_Syllabic_Category

		// These don't have UISC assigned in Unicode 13.0.0, but have UIPC
		if 0x1CE2 <= U && U <= 0x1CE8 {
			UISC = "Cantillation_Mark"
		}

		// Tibetan:
		// These don't have UISC assigned in Unicode 13.0.0, but have UIPC
		if 0x0F18 <= U && U <= 0x0F19 || 0x0F3E <= U && U <= 0x0F3F {
			UISC = "Vowel_Dependent"
		}

		// U+1CED should only be allowed after some of
		// the nasalization marks, maybe only for U+1CE9..U+1CF1.
		if U == 0x1CED {
			UISC = "Tone_Mark"
		}

		var values []string
		for k, v := range useMapping {
			if v(U, UISC, UDI, UGC, AJT) {
				values = append(values, k)
			}
		}
		if len(values) != 1 {
			check(fmt.Errorf("in mapToUSE, multiple mappings for 0x%x (%s %s %s %s): %v", U, UISC, UDI, UGC, AJT, values))
		}

		USE := values[0]

		// Resolve Indic_Positional_Category

		// https://github.com/harfbuzz/harfbuzz/pull/1037
		//  and https://github.com/harfbuzz/harfbuzz/issues/1631
		if inR(U, 0x11302, 0x11303, 0x114C1) {
			UIPC = "Top"
		}
		if 0x1CF8 <= U && U <= 0x1CF9 {
			UIPC = "Top"
		}

		// https://github.com/harfbuzz/harfbuzz/pull/982
		// also  https://github.com/harfbuzz/harfbuzz/issues/1012
		if 0x1112A <= U && U <= 0x1112B {
			UIPC = "Top"
		}
		if 0x11131 <= U && U <= 0x11132 {
			UIPC = "Top"
		}

		if _, inPos := usePositions[USE]; !in(UIPC, "Not_Applicable", "Visual_Order_Left") && U != 0x0F7F && !inPos {
			check(fmt.Errorf("in mapToUSE: %x %s %s %s %s %s %s", U, UIPC, USE, UISC, UDI, UGC, AJT))
		}

		posMapping := usePositions[USE]
		if len(posMapping) != 0 {
			var values []string
			for k, v := range posMapping {
				if in(UIPC, v...) {
					values = append(values, k)
				}
			}
			if len(values) != 1 {
				check(fmt.Errorf("in mapToUSE: %x %s %s %s %s %s %s %v", U, UIPC, USE, UISC, UDI, UGC, AJT, values))
			}
			USE = USE + values[0]
		}
		out[U] = [2]string{USE, UBlock}
	}
	return out
}
