package parser

import (
	"fmt"
	pa "go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/go-text/typesetting-utils/generators/binarygen/analysis"
	gen "github.com/go-text/typesetting-utils/generators/binarygen/generator"
)

func assertParseBlock(t *testing.T, code string) {
	t.Helper()
	code = fmt.Sprintf(`package main 
	func main() {
		%s
	}`, code)
	_, err := pa.ParseFile(token.NewFileSet(), "", code, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_staticLengthCheck_code(t *testing.T) {
	cc := gen.Context{
		ObjectVar: "item",
		Type:      "lookup",
		Slice:     "data",
	}
	tests := []analysis.BinarySize{1, 5, 12}
	for _, sl := range tests {
		got := staticLengthCheckAt(cc, sl)
		assertParseBlock(t, got)
	}
}

func Test_affineLengthCheck_code(t *testing.T) {
	cc := gen.Context{
		ObjectVar: "item",
		Type:      "lookup",
		Slice:     "data",
		Offset:    gen.NewOffset("n", 0),
	}
	tests := []struct {
		count gen.Expression
		size  analysis.BinarySize
	}{
		{"L2", 4},
		{"L2", 5},
	}
	for _, sl := range tests {
		got := affineLengthCheckAt(cc, sl.count, sl.size)
		assertParseBlock(t, got)
	}
}

func Test_conditionalLengthCheck_code(t *testing.T) {
	cc := gen.Context{
		ObjectVar: "item",
		Type:      "lookup",
		Slice:     "data",
	}
	tests := []conditionalLength{
		{
			"2",
			[]conditionalField{},
		},
		{
			"n",
			[]conditionalField{
				{"lookup", 4},
				{"name", 2},
			},
		},
	}
	for _, sl := range tests {
		got := conditionalLengthCheck(sl, cc)
		assertParseBlock(t, got)
	}
}

func TestCodeForLength(t *testing.T) {
	cc := gen.Context{
		Type:      "lookup",
		Slice:     "data",
		ObjectVar: "item",
		Offset:    gen.NewOffset("n", 0),
	}

	for _, ct := range [...]analysis.ArrayCount{
		analysis.FirstUint16, analysis.FirstUint32, analysis.ComputedField,
		analysis.ToEnd,
	} {
		_, code := codeForSliceCount(analysis.Slice{
			Count:     ct,
			CountExpr: "myVar",
		}, "dummy", &cc)
		assertParseBlock(t, code)
	}
}

func TestWithArray(t *testing.T) {
	// for the given struct WithArray
	//
	// a uint16
	// b [4]uint32
	// c [3]byte
	//
	// the expected output should be
	//
	expected := []string{
		"item.a = binary.BigEndian.Uint16(src[0:])",
		"item.b[0] = binary.BigEndian.Uint32(src[2:])",
		"item.b[1] = binary.BigEndian.Uint32(src[6:])",
		"item.b[2] = binary.BigEndian.Uint32(src[10:])",
		"item.b[3] = binary.BigEndian.Uint32(src[14:])",
		"item.c[0] = src[18]",
		"item.c[1] = src[19]",
		"item.c[2] = src[20]",
	}

	mustParse := parserForTable(ana.Tables[ana.ByName("WithArray")])[0].Content
	for _, line := range expected {
		if !strings.Contains(mustParse, line) {
			t.Fatalf("missing\n%s \nin \n %s", line, mustParse)
		}
	}
}
