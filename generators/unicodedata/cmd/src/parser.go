package src

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// we do not support surrogates yet
const maxUnicode = 0x110000

func parseRune(s string) rune {
	s = strings.TrimPrefix(s, "0x")
	i, err := strconv.ParseUint(s, 16, 64)
	check(err)

	if i > maxUnicode {
		check(fmt.Errorf("invalid rune value: 0x%x", i))
	}
	return rune(i)
}

// parse a space separated list of runes
func parseRunes(s string) []rune {
	var out []rune
	for _, s := range strings.Fields(s) {
		out = append(out, parseRune(s))
	}
	return out
}

type runeRange struct {
	Start, End rune
}

func (r runeRange) runes() []rune {
	var out []rune
	for ru := r.Start; ru <= r.End; ru++ {
		out = append(out, ru)
	}
	return out
}

// split the file by line, ignore comments #
func getLines(b []byte) (out []string) {
	for _, l := range bytes.Split(b, []byte{'\n'}) {
		line := string(bytes.TrimSpace(l))
		if line == "" || line[0] == '#' { // reading header or comment
			continue
		}
		out = append(out, line)
	}
	return out
}

// split the file by line, ignore comments, and split each line by ';'
func splitLines(b []byte) (out [][]string) {
	for _, line := range getLines(b) {
		cs := strings.Split(line, ";")
		for i, s := range cs {
			cs[i] = strings.TrimSpace(s)
		}
		out = append(out, cs)
	}
	return
}

type unicodeDatabase struct {
	chars []unicodeEntry

	// deduced from [chars]
	generalCategory  map[rune]string
	combiningClasses map[uint8][]rune // class -> runes
}

func (db *unicodeDatabase) inferMaps() {
	// initialisation
	db.generalCategory = make(map[rune]string)
	db.combiningClasses = make(map[uint8][]rune)

	for _, item := range db.chars {
		c := item.char
		// general category
		db.generalCategory[c] = item.generalCategory
		// Combining class
		db.combiningClasses[item.combiningClass] = append(db.combiningClasses[item.combiningClass], c)
	}
}

type unicodeEntry struct {
	char            rune
	generalCategory string
	combiningClass  uint8

	// optional
	shape        shapeT
	shapingItems []rune // remaining after shape
}

// assume at least 6 fields
func parseUnicodeEntry(chunks []string) unicodeEntry {
	var item unicodeEntry

	// Rune
	item.char = parseRune(chunks[0])

	// General category
	item.generalCategory = strings.TrimSpace(chunks[2])

	// Combining class
	cc, err := strconv.Atoi(chunks[3])
	check(err)
	if cc > 0xFF {
		check(fmt.Errorf("combining class too high %d", cc))
	}
	item.combiningClass = uint8(cc)

	// we are now looking for <...> XXXX
	if chunks[5] == "" {
		return item
	}

	if chunks[5][0] != '<' {
		return item
	}

	items := strings.Split(chunks[5], " ")
	if len(items) < 2 {
		check(fmt.Errorf("invalid line %v", chunks))
	}

	item.shape = isShape(items[0])
	for _, r := range items[1:] {
		item.shapingItems = append(item.shapingItems, parseRune(r))
	}
	return item
}

// rune;comment;General_Category;Canonical_Combining_Class;Bidi_Class;Decomposition_Mapping;...;Bidi_Mirrored
func parseUnicodeDatabase(b []byte) unicodeDatabase {
	chars := make([]unicodeEntry, 0, 10_000)
	for _, chunks := range splitLines(b) {
		if len(chunks) < 6 {
			continue
		}
		chars = append(chars, parseUnicodeEntry(chunks))
	}

	out := unicodeDatabase{chars: chars}
	out.inferMaps()

	return out
}

// -1 for no shapeT
type shapeT int8

const (
	none shapeT = iota
	initial
	medial
	final
	isolated
)

func isShape(s string) shapeT {
	for i, tag := range [...]string{
		"<initial>",
		"<medial>",
		"<final>",
		"<isolated>",
	} {
		if tag == s {
			return shapeT(i + 1)
		}
	}
	return 0
}

func parseAnnexTablesAsRanges(b []byte) (map[string][]runeRange, error) {
	outRanges := map[string][]runeRange{}
	for _, parts := range splitLines(b) {
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid line: %s", parts)
		}
		rang, typ := strings.TrimSpace(parts[0]), strings.TrimSpace(strings.Split(parts[1], "#")[0])
		rangS := strings.Split(rang, "..")
		start := parseRune(rangS[0])
		end := start
		if len(rangS) > 1 {
			end = parseRune(rangS[1])
		}
		outRanges[typ] = append(outRanges[typ], runeRange{Start: start, End: end})
	}
	return outRanges, nil
}

func parseAnnexTables(b []byte) (map[string][]rune, error) {
	tmp, err := parseAnnexTablesAsRanges(b)
	if err != nil {
		return nil, err
	}
	outRanges := map[string][]rune{}
	for k, v := range tmp {
		for _, r := range v {
			outRanges[k] = append(outRanges[k], r.runes()...)
		}
	}
	return outRanges, nil
}

func parseMirroring(b []byte) (map[uint16]uint16, error) {
	out := make(map[uint16]uint16)
	for _, parts := range splitLines(b) {
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid line: %s", parts)
		}
		start, end := strings.TrimSpace(parts[0]), strings.TrimSpace(strings.Split(parts[1], "#")[0])
		startRune, endRune := parseRune(start), parseRune(end)
		if startRune > 0xFFFF {
			return nil, fmt.Errorf("rune %d overflows implementation limit", startRune)
		}
		if endRune > 0xFFFF {
			return nil, fmt.Errorf("rune %d overflows implementation limit", endRune)
		}
		out[uint16(startRune)] = uint16(endRune)
	}
	return out, nil
}

type ucdXML struct {
	XMLName xml.Name `xml:"ucd"`
	Reps    []group  `xml:"repertoire>group"`
}

type group struct {
	Dm        string `xml:"dm,attr"`
	Dt        string `xml:"dt,attr"`
	CompEx    string `xml:"Comp_Ex,attr"`
	Chars     []char `xml:"char"`
	Reserved  []char `xml:"reserved"`
	NonChar   []char `xml:"noncharacter"`
	Surrogate []char `xml:"surrogate"`
}

type char struct {
	Cp      string `xml:"cp,attr"`
	FirstCp string `xml:"first-cp,attr"`
	LastCp  string `xml:"last-cp,attr"`
	Dm      string `xml:"dm,attr"`
	Dt      string `xml:"dt,attr"`
	CompEx  string `xml:"Comp_Ex,attr"`
}

func parseXML(input []byte) (map[rune][]rune, map[rune]bool, error) {
	f, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
	if err != nil {
		return nil, nil, err
	}

	if len(f.File) != 1 {
		if err != nil {
			return nil, nil, errors.New("invalid zip file")
		}
	}
	content, err := f.File[0].Open()
	if err != nil {
		return nil, nil, err
	}

	var out ucdXML
	dec := xml.NewDecoder(content)
	err = dec.Decode(&out)
	if err != nil {
		return nil, nil, err
	}

	parseDm := func(dm string) (runes []rune) {
		if dm == "#" {
			return nil
		}
		return parseRunes(dm)
	}

	dms := map[rune][]rune{}
	compEx := map[rune]bool{}
	handleRunes := func(l []char, gr group) error {
		for _, ch := range l {
			if ch.Dm == "" {
				ch.Dm = gr.Dm
			}
			if ch.Dt == "" {
				ch.Dt = gr.Dt
			}
			if ch.CompEx == "" {
				ch.CompEx = gr.CompEx
			}
			if ch.Dt != "can" {
				continue
			}

			runes := parseDm(ch.Dm)

			if ch.Cp != "" {
				ru, err := strconv.ParseInt(ch.Cp, 16, 32)
				check(err)
				dms[rune(ru)] = runes
				if ch.CompEx == "Y" {
					compEx[rune(ru)] = true
				}
			} else {
				firstRune, err := strconv.ParseInt(ch.FirstCp, 16, 32)
				if err != nil {
					return err
				}
				lastRune, err := strconv.ParseInt(ch.LastCp, 16, 32)
				if err != nil {
					return err
				}
				for ru := firstRune; ru <= lastRune; ru++ {
					dms[rune(ru)] = runes
					if ch.CompEx == "Y" {
						compEx[rune(ru)] = true
					}
				}
			}
		}
		return nil
	}

	for _, group := range out.Reps {
		if err := handleRunes(group.Chars, group); err != nil {
			return nil, nil, err
		}
		if err := handleRunes(group.Reserved, group); err != nil {
			return nil, nil, err
		}
		if err := handleRunes(group.NonChar, group); err != nil {
			return nil, nil, err
		}
		if err := handleRunes(group.Surrogate, group); err != nil {
			return nil, nil, err
		}
	}

	// remove unused runes
	for i := 0xAC00; i < 0xAC00+11172; i++ {
		delete(dms, rune(i))
	}

	return dms, compEx, nil
}

// return the joining type and joining group
func parseArabicShaping(b []byte) map[rune]ArabicJoining {
	out := make(map[rune]ArabicJoining)
	for _, fields := range splitLines(b) {
		if len(fields) < 2 {
			check(fmt.Errorf("invalid line %v", fields))
		}

		var c rune
		_, err := fmt.Sscanf(fields[0], "%x", &c)
		if err != nil {
			check(fmt.Errorf("invalid line %v: %s", fields, err))
		}

		if c >= maxUnicode {
			check(fmt.Errorf("to high rune value: %d", c))
		}

		if fields[2] == "" {
			check(fmt.Errorf("invalid line %v", fields))
		}

		joiningType := ArabicJoining(fields[2][0])
		if len(fields) >= 4 {
			switch fields[3] {
			case "ALAPH":
				joiningType = 'a'
			case "DALATH RISH":
				joiningType = 'd'
			}
		}

		switch joiningType {
		case U, R, Alaph, DalathRish, D, C, L, T, G:
		default:
			check(fmt.Errorf("invalid joining type %s", string(joiningType)))
		}

		out[c] = joiningType
	}

	return out
}

func parseUSEInvalidCluster(b []byte) [][]rune {
	var constraints [][]rune
	for _, parts := range splitLines(b) {
		if len(parts) < 1 {
			check(fmt.Errorf("invalid line: %s", parts))
		}

		constraint := parseRunes(parts[0])
		if len(constraint) == 0 {
			continue
		}
		if len(constraint) == 1 {
			check(fmt.Errorf("prohibited sequence is too short: %v", constraint))
		}
		constraints = append(constraints, constraint)
	}
	return constraints
}

func parseEmojisTest(b []byte) (sequences [][]rune) {
	for _, line := range splitLines(b) {
		if len(line) == 0 {
			continue
		}
		runes := parseRunes(line[0])
		sequences = append(sequences, runes)
	}
	return sequences
}

// replace spaces by _
func parseScriptNames(b []byte) (map[string]uint32, error) {
	m := map[string]uint32{}
	for _, chunks := range splitLines(b) {
		code := chunks[0]
		if len(code) != 4 {
			return nil, fmt.Errorf("invalid code %s ", code)
		}

		if code == "Geok" {
			continue // special case: duplicate tag
		}
		tag := binary.BigEndian.Uint32([]byte(strings.ToLower(code)))

		name := chunks[4]
		if name == "" {
			// use English name as default
			name = strings.ReplaceAll(chunks[2], " ", "_")
		}
		if strings.ContainsAny(name, "(-") {
			continue
		}
		m[name] = tag
	}
	return m, nil
}

// copy of harfbuzz.arabicJoining
type ArabicJoining byte

const (
	U          ArabicJoining = 'U' // Un-joining, e.g. Full Stop
	R          ArabicJoining = 'R' // Right-joining, e.g. Arabic Letter Dal
	Alaph      ArabicJoining = 'a' // Alaph group (included in kind R)
	DalathRish ArabicJoining = 'd' // Dalat Rish group (included in kind R)
	D          ArabicJoining = 'D' // Dual-joining, e.g. Arabic Letter Ain
	C          ArabicJoining = 'C' // Join-Causing, e.g. Tatweel, ZWJ
	L          ArabicJoining = 'L' // Left-joining, i.e. fictional
	T          ArabicJoining = 'T' // Transparent, e.g. Arabic Fatha
	G          ArabicJoining = 'G' // Ignored, e.g. LRE, RLE, ZWNBSP
)

// ----------------------------- PUA remap -----------------------------

func parsePUAMapping(b []byte) (out [][2]rune) {
	for _, line := range getLines(b) {
		words := strings.Fields(line)
		r1, r2 := parseRune(words[0]), parseRune(words[1])
		out = append(out, [2]rune{r1, r2})
	}
	return out
}
