package src

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// download the database files and save them locally

const (
	version            = "13.0.0"
	urlUCDXML          = "https://unicode.org/Public/" + version + "/ucdxml/ucd.nounihan.grouped.zip"
	urlUnicodeData     = "https://unicode.org/Public/" + version + "/ucd/UnicodeData.txt"
	urlEmoji           = "https://unicode.org/Public/" + version + "/ucd/emoji/emoji-data.txt"
	urlEmojiTest       = "https://unicode.org/Public/emoji/13.1/emoji-test.txt"
	urlBidiMirroring   = "https://unicode.org/Public/" + version + "/ucd/BidiMirroring.txt"
	urlArabic          = "https://unicode.org/Public/" + version + "/ucd/ArabicShaping.txt"
	urlScripts         = "https://unicode.org/Public/" + version + "/ucd/Scripts.txt"
	urlIndicSyllabic   = "https://unicode.org/Public/" + version + "/ucd/IndicSyllabicCategory.txt"
	urlIndicPositional = "https://unicode.org/Public/" + version + "/ucd/IndicPositionalCategory.txt"
	urlBlocks          = "https://unicode.org/Public/" + version + "/ucd/Blocks.txt"
	urlLineBreak       = "https://unicode.org/Public/" + version + "/ucd/LineBreak.txt"
	urlEastAsianWidth  = "https://unicode.org/Public/" + version + "/ucd/EastAsianWidth.txt"
	urlSentenceBreak   = "https://unicode.org/Public/" + version + "/ucd/auxiliary/SentenceBreakProperty.txt"
	urlGraphemeBreak   = "https://unicode.org/Public/" + version + "/ucd/auxiliary/GraphemeBreakProperty.txt"
	urlDerivedCore     = "https://unicode.org/Public/" + version + "/ucd/DerivedCoreProperties.txt"
)

func fetchData(url string) []byte {
	fmt.Println("Downloading", url, "...")
	resp, err := http.Get(url)
	check(err)

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
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
	indicSyllabic   []byte
	indicPositional []byte
	blocks          []byte
	lineBreak       []byte
	eastAsianWidth  []byte
	sentenceBreak   []byte
	graphemeBreak   []byte
	derivedCore     []byte
}

// download and save files in ../data/
func fetchAll() (out sources) {
	out.ucdXML = fetchData(urlUCDXML)
	out.unicodeData = fetchData(urlUnicodeData)
	out.emoji = fetchData(urlEmoji)
	out.emojiTest = fetchData(urlEmojiTest)
	out.bidiMirroring = fetchData(urlBidiMirroring)
	out.arabic = fetchData(urlArabic)
	out.scripts = fetchData(urlScripts)
	out.indicSyllabic = fetchData(urlIndicSyllabic)
	out.indicPositional = fetchData(urlIndicPositional)
	out.blocks = fetchData(urlBlocks)
	out.lineBreak = fetchData(urlLineBreak)
	out.eastAsianWidth = fetchData(urlEastAsianWidth)
	out.sentenceBreak = fetchData(urlSentenceBreak)
	out.graphemeBreak = fetchData(urlGraphemeBreak)
	out.derivedCore = fetchData(urlDerivedCore)

	return out
}
