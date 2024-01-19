package generators

// copied from typesetting/language

var canonMap = [256]byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, '-', 0, 0,
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 0, 0, 0, 0, 0, 0,
	'-', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 0, 0, 0, 0, '-',
	0, 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 0, 0, 0, 0, 0,
}

// Language store the canonicalized BCP 47 tag,
// which has the generic form <lang>-<country>-<other tags>...
type Language string

// NewLanguage canonicalizes the language input (as a BCP 47 language tag), by converting it to
// lowercase, mapping '_' to '-', and stripping all characters other
// than letters, numbers and '-'.
func NewLanguage(language string) Language {
	out := make([]byte, 0, len(language))
	for _, r := range language {
		if r >= 0xFF {
			continue
		}
		can := canonMap[r]
		if can != 0 {
			out = append(out, can)
		}
	}
	return Language(out)
}
