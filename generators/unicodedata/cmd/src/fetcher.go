package src

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// download the database files from the Unicode source

const (
	version            = "17.0.0"
	urlUCDXML          = "https://unicode.org/Public/" + version + "/ucdxml/ucd.nounihan.grouped.zip"
	urlUnicodeData     = "https://unicode.org/Public/" + version + "/ucd/UnicodeData.txt"
	urlEmoji           = "https://unicode.org/Public/" + version + "/ucd/emoji/emoji-data.txt"
	urlEmojiTest       = "https://unicode.org/Public/" + version + "/emoji/emoji-test.txt"
	urlBidiMirroring   = "https://unicode.org/Public/" + version + "/ucd/BidiMirroring.txt"
	urlArabic          = "https://unicode.org/Public/" + version + "/ucd/ArabicShaping.txt"
	urlScripts         = "https://unicode.org/Public/" + version + "/ucd/Scripts.txt"
	urlScriptNames     = "https://www.unicode.org/iso15924/iso15924.txt"
	urlIndicSyllabic   = "https://unicode.org/Public/" + version + "/ucd/IndicSyllabicCategory.txt"
	urlIndicPositional = "https://unicode.org/Public/" + version + "/ucd/IndicPositionalCategory.txt"
	urlBlocks          = "https://unicode.org/Public/" + version + "/ucd/Blocks.txt"
	urlEastAsianWidth  = "https://unicode.org/Public/" + version + "/ucd/EastAsianWidth.txt"
	urlDerivedCore     = "https://unicode.org/Public/" + version + "/ucd/DerivedCoreProperties.txt"

	urlLineBreak     = "https://unicode.org/Public/" + version + "/ucd/LineBreak.txt"
	urlSentenceBreak = "https://unicode.org/Public/" + version + "/ucd/auxiliary/SentenceBreakProperty.txt"
	urlGraphemeBreak = "https://unicode.org/Public/" + version + "/ucd/auxiliary/GraphemeBreakProperty.txt"

	urlLineBreakTest     = "https://www.unicode.org/Public/" + version + "/ucd/auxiliary/LineBreakTest.txt"
	urlGraphemeBreakTest = "https://www.unicode.org/Public/" + version + "/ucd/auxiliary/GraphemeBreakTest.txt"

	urlWordBreakTest = "https://www.unicode.org/Public/" + version + "/ucd/auxiliary/WordBreakTest.txt"
)

func fetchData(url string, fromCache bool) []byte {
	fileName := filepath.Join(os.TempDir(), "unicode_generator_"+path.Base(url))
	if fromCache {
		fmt.Println("Loading from cache", fileName, "...")
		data, err := os.ReadFile(fileName)
		check(err)
		return data
	}

	fmt.Println("Downloading", url, "...")
	resp, err := http.Get(url)
	check(err)

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	check(err)

	err = os.WriteFile(fileName, data, os.ModePerm)
	check(err)

	return data
}

// sources stores all the input text files
// defining Unicode data
type sources struct {
	ucdXML          []byte
	unicodeData     []byte
	emoji           []byte
	emojiTest       []byte
	bidiMirroring   []byte
	arabic          []byte
	scripts         []byte
	scriptNames     []byte
	indicSyllabic   []byte
	indicPositional []byte
	blocks          []byte
	eastAsianWidth  []byte
	derivedCore     []byte

	lineBreak     []byte
	sentenceBreak []byte
	graphemeBreak []byte

	lineBreakTest     []byte
	graphemeBreakTest []byte
	wordBreakTest     []byte
}

// download and return files in memory
func fetchAll(fromCache bool) (out sources) {
	out.ucdXML = fetchData(urlUCDXML, fromCache)
	out.unicodeData = fetchData(urlUnicodeData, fromCache)
	out.emoji = fetchData(urlEmoji, fromCache)
	out.emojiTest = fetchData(urlEmojiTest, fromCache)
	out.bidiMirroring = fetchData(urlBidiMirroring, fromCache)
	out.arabic = fetchData(urlArabic, fromCache)
	out.scripts = fetchData(urlScripts, fromCache)
	out.scriptNames = fetchData(urlScriptNames, fromCache)
	out.indicSyllabic = fetchData(urlIndicSyllabic, fromCache)
	out.indicPositional = fetchData(urlIndicPositional, fromCache)
	out.blocks = fetchData(urlBlocks, fromCache)
	out.eastAsianWidth = fetchData(urlEastAsianWidth, fromCache)
	out.derivedCore = fetchData(urlDerivedCore, fromCache)
	out.lineBreak = fetchData(urlLineBreak, fromCache)
	out.sentenceBreak = fetchData(urlSentenceBreak, fromCache)
	out.graphemeBreak = fetchData(urlGraphemeBreak, fromCache)
	out.lineBreakTest = fetchData(urlLineBreakTest, fromCache)
	out.wordBreakTest = fetchData(urlWordBreakTest, fromCache)
	out.graphemeBreakTest = fetchData(urlGraphemeBreakTest, fromCache)

	return out
}
