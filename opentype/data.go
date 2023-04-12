package opentype

import "embed"

// Files provides Opentype fonts.
// The subfolders included are :
//   - morx : contains AAT 'morx' tables
//   - collections : contains opentype font collections
//   - common : contains opentype fonts
//
//go:embed *
var Files embed.FS

// WithOTLayout is the path of font files
// containing Opentype GSUB/GPOS tables,
// with path expressed from the root of [Files]
var WithOTLayout = []string{
	"common/Raleway-v4020-Regular.otf",
	"common/Commissioner-VF.ttf",
	"common/Estedad-VF.ttf",
	"common/Mada-VF.ttf",
}

var WithGlyphs = []struct {
	Path        string
	GlyphNumber int
}{
	{"common/Commissioner-VF.ttf", 1123},
	{"common/Roboto-BoldItalic.ttf", 3359},
	{"common/open-sans-v15-latin-regular.woff", 221},
}

var WithSbix = []struct {
	Path          string
	StrikesNumber int
}{
	{"toys/Sbix1.ttf", 1},
	{"toys/Sbix2.ttf", 1},
	{"toys/Sbix3.ttf", 1},
}

var WithCBLC = []struct {
	Path          string
	StrikesNumber int
	GlyphRange    [2]int // GID with bitmap data
}{
	{"toys/CBLC1.ttf", 1, [2]int{2, 4}},
	{"toys/CBLC2.ttf", 1, [2]int{1, 1}},
	{"bitmap/NotoColorEmoji.ttf", 1, [2]int{4, 17}},
}

var WithEBLC = []struct {
	Path          string
	StrikesNumber int
}{
	{"toys/KacstQurn.ttf", 2},
	{"bitmap/IBM3161-bitmap.otb", 1},
}

var WithMVAR = []string{
	"toys/Var1.ttf",
	"common/SourceSans-VF.ttf",
}

var WithAvar = []string{
	"common/Selawik-VF.ttf",
	"common/Commissioner-VF.ttf",
	"common/SourceSans-VF.ttf",
}

var WithFvar = []struct {
	Path      string
	AxisCount int
}{
	{"common/Selawik-VF.ttf", 1},
	{"common/Commissioner-VF.ttf", 4},
	{"common/SourceSans-VF.ttf", 1},
	{"toys/Var1.ttf", 15},
}

var Monospace = map[string]bool{
	"common/SourceSans-VF.ttf":         true,
	"common/Go-Mono-Bold-Italic.ttf":   true,
	"common/LiberationMono-Italic.ttf": true,
	"common/Lmmono-italic.otf":         true,
	"common/DejaVuSansMono.ttf":        true,
}
