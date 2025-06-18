package parser

import (
	"fmt"
	"go/types"

	an "github.com/go-text/typesetting-utils/generators/binarygen/analysis"
	gen "github.com/go-text/typesetting-utils/generators/binarygen/generator"
)

// ParsersForFile write the parsing functions required by [ana.Tables] in [dst]
func ParsersForFile(ana an.Analyser, dst *gen.Buffer) {
	for _, table := range ana.Tables {
		for _, decl := range parserForTable(table) {
			dst.Add(decl)
		}
	}

	for _, standaloneUnion := range ana.StandaloneUnions {
		dst.Add(parserForStanaloneUnion(standaloneUnion))
	}
}

// parserForTable returns the parsing function for the given table.
// The required methods for fields shall be generated in a separated step.
func parserForTable(ta an.Struct) []gen.Declaration {
	origin := ta.Origin().(*types.Named)
	context := &gen.Context{
		Type:      origin.Obj().Name(),
		ObjectVar: "item",
		Slice:     "src",                 // defined in args
		Offset:    gen.NewOffset("n", 0), // defined later
	}

	scopes := ta.Scopes()
	if len(scopes) == 0 {
		// empty struct are useful : generate the trivial parser
		return []gen.Declaration{context.ParsingFunc(origin, []string{"[]byte"}, []string{"n := 0"})}
	}

	body, args := []string{fmt.Sprintf("n := %s", context.Offset.Value())}, []string{"src []byte"}
	flag := an.ResolveOffsetRelative(ta)
	if flag&an.Parent != 0 {
		args = append(args, "parentSrc []byte")
	}
	if flag&an.GrandParent != 0 {
		args = append(args, "grandParentSrc []byte")
	}
	for _, arg := range requiredArgs(ta, "") {
		args = append(args, arg.asSignature())
	}

	// important special case when all fields have fixed size (with no offset) :
	// generate a mustParse method
	if _, isFixedSize := ta.IsFixedSize(); isFixedSize {
		mustParse, parseBody := mustParserFieldsFunction(ta, *context)
		body = append(body, parseBody)

		return []gen.Declaration{mustParse, context.ParsingFuncComment(origin, args, body, "")}
	}

	for _, scope := range scopes {
		body = append(body, parser(scope, ta, context))
	}
	// add the parseEnd when present
	if ta.ParseEnd != nil {
		body = append(body, fmt.Sprintf(`var err error 
		n, err = %s.%s(%s, %s)
		if err != nil {
			%s
		}
		`, context.ObjectVar, ta.ParseEnd.Name(), context.Slice, resolveArguments(context.ObjectVar, nil, requiredArgs(ta, "")),
			context.ErrReturn(gen.ErrVariable("err"))))
	}

	finalCode := context.ParsingFuncComment(origin, args, body, "")

	return []gen.Declaration{finalCode}
}

// parserForStanaloneUnion returns the parsing function for the given union.
func parserForStanaloneUnion(un an.Union) gen.Declaration {
	context := &gen.Context{
		Type:      un.Origin().(*types.Named).Obj().Name(),
		ObjectVar: "item",
		Slice:     "src",                    // defined in args
		Offset:    gen.NewOffset("read", 0), // defined later
	}

	body, args := []string{}, []string{"src []byte"}
	for _, arg := range requiredArgsForUnion(un, "") {
		args = append(args, arg.asSignature())
	}

	cases := unionCases(un, context, nil, context.ObjectVar)
	code := standaloneUnionBody(un, context, cases)
	body = append(body, code)

	finalCode := context.ParsingFunc(un.Origin().(*types.Named), args, body)

	return finalCode
}

func parser(scope an.Scope, parent an.Struct, cc *gen.Context) string {
	var code string
	switch scope := scope.(type) {
	case an.StaticSizedFields:
		code = parserForFixedSize(scope, cc)
	case an.SingleField:
		code = parserForSingleField(scope, parent, cc)
	default:
		panic("exhaustive type switch")
	}
	return code
}

// add the length check
func parserForFixedSize(fs an.StaticSizedFields, cc *gen.Context) string {
	totalSize := fs.Size()
	return fmt.Sprintf(`%s
		%s
		%s
		`,
		staticLengthCheckAt(*cc, totalSize),
		mustParserFields(fs, cc),
		cc.Offset.UpdateStatement(totalSize),
	)
}

// delegate to the type
func parserForSingleField(field an.SingleField, parent an.Struct, cc *gen.Context) string {
	code := parserForVariableSize(an.Field(field), parent, cc)
	return fmt.Sprintf(`{
		%s}`, code)
}
