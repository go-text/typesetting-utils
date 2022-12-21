package parser

import (
	"fmt"
	"go/types"
	"strings"

	an "github.com/go-text/typesetting-utils/generators/binarygen/analysis"
	gen "github.com/go-text/typesetting-utils/generators/binarygen/generator"
)

// mustParser is only valid for type [ty] with a fixed sized,
// it will panic otherwise
func mustParser(ty an.Type, cc gen.Context, target string) string {
	switch ty := ty.(type) {
	case an.Basic:
		return mustParserBasic(ty, cc, target)
	case an.DerivedFromBasic:
		return mustParserDerived(ty, cc, target)
	case an.Struct:
		return mustParserStruct(ty, cc, target)
	case an.Array:
		return mustParserArray(ty, cc, target)
	case an.Offset:
		return mustParserOffset(ty, cc, target)
	case an.Slice:
		return mustParseSlice(ty, cc, target)
	default:
		// other types are never fixed sized
		panic(fmt.Sprintf("invalid type %T in mustParser", ty))
	}
}

func mustParserBasic(bt an.Basic, cc gen.Context, target string) string {
	size, _ := bt.IsFixedSize()
	readCode := readBasicTypeAt(cc, size)

	name := gen.Name(bt)

	switch name {
	case "uint8", "byte", "uint16", "uint32", "uint64": // simplify by removing the unnecessary conversion
		return fmt.Sprintf("%s = %s", target, readCode)
	default:
		return fmt.Sprintf("%s = %s(%s)", target, name, readCode)
	}
}

func mustParserDerived(de an.DerivedFromBasic, cc gen.Context, target string) string {
	readCode := readBasicTypeAt(cc, de.Size)
	return fmt.Sprintf("%s = %sFromUint(%s)", target, de.Name, readCode)
}

// only valid for fixed size structs, call the `mustParse` method
func mustParserStruct(st an.Struct, cc gen.Context, target string) string {
	return fmt.Sprintf("%s.mustParse(%s[%s:])", target, cc.Slice, cc.Offset.Value())
}

func mustParserArray(ar an.Array, cc gen.Context, target string) string {
	elemSize, ok := ar.Elem.IsFixedSize()
	if !ok {
		panic("mustParserArray only support fixed size elements")
	}

	statements := make([]string, ar.Len)
	for i := range statements {
		// adjust the selector
		elemSelector := fmt.Sprintf("%s[%d]", target, i)
		// generate the code
		statements[i] = mustParser(ar.Elem, cc, elemSelector)
		// update the context offset
		cc.Offset.Increment(elemSize)
	}
	return strings.Join(statements, "\n")
}

func offsetName(target string) string {
	_, name, ok := strings.Cut(target, ".")
	if !ok {
		name = target
	}
	return "offset" + strings.Title(name)
}

// parse the offset value (not the target) in a temporary variable
func mustParserOffset(of an.Offset, cc gen.Context, target string) string {
	return fmt.Sprintf("%s := int(%s)", offsetName(target), readBasicTypeAt(cc, of.Size))
}

func arrayCountName(target string) string {
	_, name, ok := strings.Cut(target, ".")
	if !ok {
		name = target
	}
	return "arrayLength" + strings.Title(name)
}

func mustParseSlice(sl an.Slice, cc gen.Context, target string) string {
	size := sl.Count.Size()
	if size == 0 {
		return ""
	}
	return fmt.Sprintf("%s := int(%s)", arrayCountName(target), readBasicTypeAt(cc, size))
}

// extension to a scope

// returns the reading instructions, without bounds check
// it can be used for example when parsing a slice of such fields
func mustParserFields(fs an.StaticSizedFields, cc *gen.Context) string {
	var code []string

	// optimize following slice access
	if len(fs) >= 2 {
		code = append(code, fmt.Sprintf("_ = %s[%s] // early bound checking", cc.Slice, cc.Offset.With(fs.Size()-1)))
	}

	for _, field := range fs {
		code = append(code, mustParser(field.Type, *cc, cc.Selector(field.Name)))

		fieldSize, _ := field.Type.IsFixedSize()
		// adjust the offset
		cc.Offset.Increment(fieldSize)
	}

	return strings.Join(code, "\n")
}

// return the mustParse method and the body of the parse function
func mustParserFieldsFunction(ta an.Struct, cc gen.Context) (mustParse gen.Declaration, parseBody string) {
	fs := ta.Scopes()[0].(an.StaticSizedFields)

	contextCopy := cc
	mustParseBody := mustParserFields(fs, &contextCopy) // pass a copy of context not influence the next calls

	mustParse.Origin = ta.Origin().(*types.Named)
	mustParse.ID = string(cc.Type) + ".mustParse"
	mustParse.Content = fmt.Sprintf(`func (%s *%s) mustParse(%s []byte) {
		%s
	}
	`, cc.ObjectVar, cc.Type, cc.Slice, mustParseBody)

	// for the parsing function: check length, call mustParse, and update the offset
	check := staticLengthCheckAt(cc, fs.Size())
	mustParseCall := fmt.Sprintf("%s.mustParse(%s)", cc.ObjectVar, cc.Slice)
	updateOffset := cc.Offset.UpdateStatement(fs.Size())

	parseBody = strings.Join([]string{
		check,
		mustParseCall,
		string(updateOffset),
	}, "\n")

	return mustParse, parseBody
}
