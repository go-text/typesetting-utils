package src

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// download the database files from the Unicode source

const (
	version            = "15.0.0"
	versionEmoji       = "15.0"
	urlUCDXML          = "https://unicode.org/Public/" + version + "/ucdxml/ucd.nounihan.grouped.zip"
	urlUnicodeData     = "https://unicode.org/Public/" + version + "/ucd/UnicodeData.txt"
	urlEmoji           = "https://unicode.org/Public/" + version + "/ucd/emoji/emoji-data.txt"
	urlEmojiTest       = "https://unicode.org/Public/emoji/" + versionEmoji + "/emoji-test.txt"
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
	data, err := ioutil.ReadAll(resp.Body)
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
	indicSyllabic   []byte
	indicPositional []byte
	blocks          []byte
	lineBreak       []byte
	eastAsianWidth  []byte
	sentenceBreak   []byte
	graphemeBreak   []byte
	derivedCore     []byte
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
	out.indicSyllabic = fetchData(urlIndicSyllabic, fromCache)
	out.indicPositional = fetchData(urlIndicPositional, fromCache)
	out.blocks = fetchData(urlBlocks, fromCache)
	out.lineBreak = fetchData(urlLineBreak, fromCache)
	out.eastAsianWidth = fetchData(urlEastAsianWidth, fromCache)
	out.sentenceBreak = fetchData(urlSentenceBreak, fromCache)
	out.graphemeBreak = fetchData(urlGraphemeBreak, fromCache)
	out.derivedCore = fetchData(urlDerivedCore, fromCache)

	return out
}
