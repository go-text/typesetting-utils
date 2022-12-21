package parser

import (
	"fmt"
	"strings"

	an "github.com/go-text/typesetting-utils/generators/binarygen/analysis"
	gen "github.com/go-text/typesetting-utils/generators/binarygen/generator"
)

func parserForStructTo(field an.Field, cc *gen.Context, target string) string {
	ty, _ := field.Type.(an.Struct)
	args := resolveSliceArgument(field.Type, *cc)
	args += resolveArguments(cc.ObjectVar, field.ArgumentsProvidedByFields, requiredArgs(ty, field.Name))
	start := cc.Offset.Value()
	updateOffset := cc.Offset.UpdateStatementDynamic("read")
	vars := `var (
		err error
		read int
	)`
	readTarget := "read"
	if cc.IgnoreUpdateOffset {
		updateOffset = ""
		vars = "var err error"
		readTarget = "_"
	}
	return fmt.Sprintf(`%s
		%s, %s, err = %s(%s[%s:], %s)
		if err != nil {
			%s 
		}
		%s
		`,
		vars,
		target, readTarget, gen.ParseFunctionName(gen.Name(field.Type)), cc.Slice, start, args,
		cc.ErrReturn(gen.ErrVariable("err")),
		updateOffset,
	)
}

func parserForVariableSize(field an.Field, parent an.Struct, cc *gen.Context) string {
	switch field.Type.(type) {
	case an.Slice:
		return parserForSlice(field, cc)
	case an.Opaque:
		return parserForOpaque(field, parent, cc)
	case an.Offset:
		return parserForOffset(field, parent, cc)
	case an.Union:
		return parserForUnion(field, cc)
	case an.Struct:
		return parserForStructTo(field, cc, cc.Selector(field.Name))
	}
	return ""
}

// delegate the parsing to a user written method of the form
// <structName>.parse<fieldName>
func parserForOpaque(field an.Field, parent an.Struct, cc *gen.Context) string {
	op := field.Type.(an.Opaque)
	start := cc.Offset.Value()
	updateOffset := cc.Offset.UpdateStatementDynamic("read")
	if op.SubsliceStart == an.AtStart { // do not use the current offset as start
		start = ""
		updateOffset = cc.Offset.SetStatement("read")
	}
	// the offset for the opaque "child" type must be shifted by one level
	args := sliceArgs(field.OffsetRelativeTo<<1, *cc)
	args += resolveArguments(cc.ObjectVar, field.ArgumentsProvidedByFields, requiredArgs(parent, field.Name))
	if op.ParserReturnsLength {
		return fmt.Sprintf(`
		read, err := %s.parse%s(%s[%s:], %s)
		if err != nil {
			%s
		}
		%s
		`, cc.ObjectVar, strings.Title(field.Name), cc.Slice, start, args,
			cc.ErrReturn(gen.ErrVariable("err")),
			updateOffset,
		)
	} else {
		return fmt.Sprintf(`
		err := %s.parse%s(%s[%s:], %s)
		if err != nil {
			%s
		}
		`, cc.ObjectVar, strings.Title(field.Name), cc.Slice, start, args,
			cc.ErrReturn(gen.ErrVariable("err")),
		)
	}
}

// ------------------------- slices -------------------------

// we distinguish the following cases for a Slice :
//   - elements have a static sized : we can check the length early
//     and use mustParse on each element
//   - elements have a variable length : we have to check the length at each iteration
//   - as an optimization, we special case raw bytes (see [Slice.IsRawData])
//   - slice of offsets are handled is in dedicated function
//   - opaque types, whose interpretation is defered are represented by an [an.Opaque] type,
//     and handled in a separate function
func parserForSlice(field an.Field, cc *gen.Context) string {
	sl := field.Type.(an.Slice)
	// no matter the kind of element, resolve the count
	countExpr, countCode := codeForSliceCount(sl, field.Name, cc)

	codes := []string{countCode}

	if sl.IsRawData() { // special case for bytes data
		// adjust the start offset if needed
		if sl.SubsliceStart == an.AtStart { // do not use the current offset as start
			cc.Offset = gen.NewOffset(cc.Offset.Name, 0)
		}
		codes = append(codes, parserForSliceBytes(sl, cc, countExpr, field.Name))
	} else if offset, isOffset := sl.Elem.(an.Offset); isOffset { // special case for slice of offsets
		codes = append(codes, parserForSliceOfOffsets(offset, cc, countExpr, field))
	} else if _, isFixedSize := sl.Elem.IsFixedSize(); isFixedSize { // else, check for fixed size elements
		codes = append(codes, parserForSliceFixedSizeElement(sl, cc, countExpr, field.Name))
	} else {
		codes = append(codes, parserForSliceVariableSizeElement(sl, cc, countExpr, field))
	}

	return strings.Join(codes, "\n")
}

func codeForSliceCount(sl an.Slice, fieldName string, cc *gen.Context) (countVar gen.Expression, code string) {
	var statements []string
	switch sl.Count {
	case an.NoLength: // the length is provided as an external variable
		countVar = externalCountVariable(fieldName)
	case an.FirstUint16, an.FirstUint32: // the length is at the start of the array
		countVar = arrayCountName(cc.Selector(fieldName))
	case an.ComputedField:
		countVar = "arrayLength"
		statements = append(statements, fmt.Sprintf("%s := int(%s)", countVar, cc.Selector(sl.CountExpr)))
	case an.ToEnd, an.ToComputedField:
		// count is ignored in this case
	}

	return countVar, strings.Join(statements, "\n")
}

func parserForSliceBytes(sl an.Slice, cc *gen.Context, count gen.Expression, fieldName string) string {
	target := cc.Selector(fieldName)
	start := cc.Offset.Value()
	// special case for ToEnd : do not use an intermediate variable
	if sl.Count == an.ToEnd {
		readStatement := fmt.Sprintf("%s = %s[%s:]", target, cc.Slice, start)
		if cc.IgnoreUpdateOffset {
			return readStatement
		}
		offsetStatemtent := cc.Offset.SetStatement(fmt.Sprintf("len(%s)", cc.Slice))
		return readStatement + "\n" + offsetStatemtent
	}

	lengthDefinition := fmt.Sprintf("L := int(%s + %s)", start, count)
	if sl.Count == an.ToComputedField { // the length is not relative to the start
		lengthDefinition = fmt.Sprintf("L := int(%s)", cc.Selector(sl.CountExpr))
	}

	errorStatement := fmt.Sprintf(`"EOF: expected length: %%d, got %%d", L, len(%s)`, cc.Slice)

	updateOffset := cc.Offset.SetStatement("L")
	if cc.IgnoreUpdateOffset {
		updateOffset = ""
	}
	return fmt.Sprintf(` 
			%s
			if len(%s) < L {
				%s
			}
			%s = %s[%s:L]
			%s
			`,
		lengthDefinition,
		cc.Slice,
		cc.ErrReturn(gen.ErrFormated(errorStatement)),
		target, cc.Slice, start,
		updateOffset,
	)
}

// The field is a slice of structs (or basic type), whose size is known at compile time.
// We can thus check for the whole slice length, and use mustParseXXX functions.
// The generated code will look like
//
//	if len(data) < n + arrayLength * size {
//		return err
//	}
//	out = make([]MorxChain, arrayLength)
//	for i := range out {
//		out[i] = mustParseMorxChain(data[])
//	}
//	n += arrayLength * size
func parserForSliceFixedSizeElement(sl an.Slice, cc *gen.Context, count gen.Expression, fieldName string) string {
	target := cc.Selector(fieldName)
	out := []string{""}

	// step 1 : check the expected length
	elementSize, _ := sl.Elem.IsFixedSize()
	out = append(out, affineLengthCheckAt(*cc, count, elementSize))

	// step 2 : allocate the slice - it is garded by the check above
	out = append(out, fmt.Sprintf("%s = make([]%s, %s) // allocation guarded by the previous check",
		target, gen.Name(sl.Elem), count))

	// step 3 : loop to parse every elements,
	// temporarily changing the offset
	startOffset := cc.Offset
	cc.Offset = gen.NewOffsetDynamic(cc.Offset.WithAffine("i", elementSize))
	loopBody := mustParser(sl.Elem, *cc, fmt.Sprintf("%s[i]", cc.Selector(fieldName)))
	out = append(out, fmt.Sprintf(`for i := range %s {
		%s
	}`, target, loopBody))

	// step 4 : update the offset
	cc.Offset = startOffset
	out = append(out,
		cc.Offset.UpdateStatementDynamic(fmt.Sprintf("%s * %d", count, elementSize)))

	return strings.Join(out, "\n")
}

// The field is a slice of structs, whose size is only known at run time
// The generated code will look like
//
//	offset := 2
//	for i := 0; i < arrayLength; i++ {
//		chain, read, err := parseMorxChain(data[offset:])
//		if err != nil {
//			return nil, err
//		}
//		out = append(out, chain)
//		offset += read
//	}
//	n = offset
func parserForSliceVariableSizeElement(sl an.Slice, cc *gen.Context, count gen.Expression, field an.Field) string {
	// if start is a constant, we have to use an additional variable

	args := resolveSliceArgument(field.Type, *cc)
	if st, isStruct := sl.Elem.(an.Struct); isStruct {
		args += resolveArguments(cc.ObjectVar, field.ArgumentsProvidedByFields, requiredArgs(st, field.Name))
	}
	// loop and update the offset
	return fmt.Sprintf(`
		offset := %s
		for i := 0; i < %s; i++ {
		elem, read, err := %s(%s[offset:], %s)
		if err != nil {
			%s
		}
		%s = append(%s, elem)
		offset += read
		}
		%s`,
		cc.Offset.Value(),
		count,
		gen.ParseFunctionName(gen.Name(sl.Elem)), cc.Slice, args,
		cc.ErrReturn(gen.ErrVariable("err")),
		cc.Selector(field.Name), cc.Selector(field.Name),
		cc.Offset.SetStatement("offset"),
	)
}

// ------------------------ Offsets ------------------------

func parserForOffset(fi an.Field, parent an.Struct, cc *gen.Context) string {
	of := fi.Type.(an.Offset)
	// Step 1 - Reading the offset value is already handled in fixed sized

	// Step 2 - check the length for the pointed value
	offsetVarName := offsetName(cc.Selector(fi.Name))

	// Step 3 - for pointer types, allocate memory,
	// and change the target
	allocate, updatePointer, tmpVarName := "", "", ""
	if of.IsPointer {
		tmpVarName = "tmp" + strings.Title(fi.Name)
		allocate = fmt.Sprintf("var  %s %s", tmpVarName, gen.Name(of.Target))
		updatePointer = fmt.Sprintf("\n%s = &%s", cc.Selector(fi.Name), tmpVarName)
	}

	// Step 4 - if needed adjust the source for the offset
	savedSlice := cc.Slice
	if fi.OffsetRelativeTo == an.Parent {
		cc.Slice = "parentSrc"
	} else if fi.OffsetRelativeTo == an.GrandParent {
		cc.Slice = "grandParentSrc"
	}
	lengthCheck := lengthCheck(*cc, offsetVarName)

	// Step 5 - finally delegate to the target parser
	savedOffset := cc.Offset
	cc.Offset = gen.NewOffsetDynamic(offsetVarName)
	cc.IgnoreUpdateOffset = true

	var readTarget string
	targetField := an.Field{
		Type:                      of.Target,
		Name:                      fi.Name,
		ArgumentsProvidedByFields: fi.ArgumentsProvidedByFields,
		UnionTag:                  fi.UnionTag,
		OffsetRelativeTo:          fi.OffsetRelativeTo,
	}
	if of.IsPointer {
		readTarget = parserForStructTo(targetField, cc, tmpVarName)
	} else {
		readTarget = parserForVariableSize(targetField, parent, cc)
	}

	// restore value
	cc.Slice = savedSlice
	cc.Offset = savedOffset

	return fmt.Sprintf(` 
	if %s != 0 { // ignore null offset
		%s
		%s
		%s%s
	}
	`,
		offsetVarName,
		lengthCheck,
		allocate,
		readTarget,
		updatePointer,
	)
}

// slice of offsets: this is somewhat a mix of [parserForSliceVariableSizeElement] and [parserForOffset].
// The generated code looks like :
//
//	if len(src) < arrayCount * offsetSize {
//		return err
//	}
//	elems := make([]ElemType, arrayCount)
//	for i := range elems {
//		offset := readUint()
//		if len(src) < offset {
//			return err
//		}
//		elems[i] = parseElemType(src[offset:])
//	}
func parserForSliceOfOffsets(of an.Offset, cc *gen.Context, count gen.Expression, fi an.Field) string {
	target := cc.Selector(fi.Name)
	out := []string{""}

	// step 1 : check the expected length
	elementSize := of.Size
	out = append(out, affineLengthCheckAt(*cc, count, elementSize))

	// step 2 : allocate the slice of offsets target - it is garded by the check above
	out = append(out, fmt.Sprintf("%s = make([]%s, %s) // allocation guarded by the previous check",
		target, gen.Name(of.Target), count))

	// step 3 : loop to parse every elements,
	// temporarily changing the offset
	startOffset := cc.Offset
	cc.Offset = gen.NewOffsetDynamic(cc.Offset.WithAffine("i", elementSize))

	args := resolveSliceArgument(of.Target, *cc)
	args += resolveArguments(cc.ObjectVar, fi.ArgumentsProvidedByFields, requiredArgs(of.Target, fi.Name))

	// Loop body :
	// Step 1 - read the offset value
	readOffset := readBasicTypeAt(*cc, elementSize)
	// Step 2 - adjust the source slice
	savedSlice := cc.Slice
	if fi.OffsetRelativeTo == an.Parent {
		cc.Slice = "parentSrc"
	} else if fi.OffsetRelativeTo == an.GrandParent {
		cc.Slice = "grandParentSrc"
	}
	// Step 3 - check the length for the pointed value
	check := lengthCheck(*cc, "offset")
	// Step 4 - finally delegate to the target parser
	targetParse := fmt.Sprintf("%s[i], _, err = %s(%s[offset:], %s)", target, gen.ParseFunctionName(gen.Name(of.Target)), cc.Slice, args)

	out = append(out, fmt.Sprintf(`for i := range %s {
		offset := int(%s)
		// ignore null offsets 
		if offset == 0 {
			continue
		}
		
		%s
		var err error
		%s
		if err != nil {
			%s
		}
	}`, target,
		readOffset,
		check,
		targetParse,
		cc.ErrReturn(gen.ErrVariable("err"))))

	// step 5 : update the offset
	cc.Slice = savedSlice
	cc.Offset = startOffset
	out = append(out,
		cc.Offset.UpdateStatementDynamic(fmt.Sprintf("%s * %d", count, elementSize)))

	return strings.Join(out, "\n")
}

// -- unions --

func unionCases(u an.Union, cc *gen.Context, providedArguments []an.ProvidedArgument, target string) []string {
	start := cc.Offset.Value()
	flags := u.UnionTag.TagsCode()
	var cases []string
	for i, flag := range flags {
		member := u.Members[i]
		args := resolveSliceArgument(member, *cc)
		args += resolveArguments(cc.ObjectVar, providedArguments, requiredArgs(member, target))
		cases = append(cases, fmt.Sprintf(`case %s :
		%s, read, err = %s(%s[%s:], %s)`,
			flag,
			target, gen.ParseFunctionName(gen.Name(member)), cc.Slice,
			start, args,
		))
	}
	return cases
}

func standaloneUnionBody(u an.Union, cc *gen.Context, cases []string) string {
	// steps :
	// 	1 : check the length for the format tag
	//	2 : read the format tag
	//	3 : defer to the corresponding member parsing function
	scheme := u.UnionTag.(an.UnionTagImplicit)
	tagSize, _ := scheme.Tag.IsFixedSize()
	return fmt.Sprintf(`
			%s
			format := %s(%s)
			var (
				read int
				err error
			)
			switch format {
			%s
			default:
				err = fmt.Errorf("unsupported %s format %%d", format)
			}
			if err != nil {
				%s
			}
			`,
		staticLengthCheckAt(*cc, tagSize),
		gen.Name(scheme.Tag), readBasicTypeAt(*cc, tagSize),
		strings.Join(cases, "\n"),
		gen.Name(u),
		cc.ErrReturn(gen.ErrVariable("err")),
	)
}

func parserForUnion(field an.Field, cc *gen.Context) string {
	u := field.Type.(an.Union)

	cases := unionCases(u, cc, field.ArgumentsProvidedByFields, cc.Selector(field.Name))

	var code string
	switch scheme := u.UnionTag.(type) {
	case an.UnionTagExplicit:
		kindVariable := cc.Selector(scheme.FlagField)
		code = fmt.Sprintf(`var (
				read int
				err error
			)
			switch %s {
			%s
			default:
				err = fmt.Errorf("unsupported %sVersion %%d", %s)
			}
			if err != nil {
				%s
			}
			`, kindVariable,
			strings.Join(cases, "\n"),
			gen.Name(u),
			kindVariable,
			cc.ErrReturn(gen.ErrVariable("err")),
		)
	case an.UnionTagImplicit:
		// defed to the generated standalone function
		args := resolveSliceArgument(field.Type, *cc)
		args += resolveArguments(cc.ObjectVar, field.ArgumentsProvidedByFields, requiredArgsForUnion(u, field.Name))

		code = fmt.Sprintf(`var (
			err error
			read int
		)
		%s, read, err = %s(%s[%s:], %s)
		if err != nil {
			%s 
		}
 		`, cc.Selector(field.Name), gen.ParseFunctionName(gen.Name(field.Type)), cc.Slice, cc.Offset.Value(), args,
			cc.ErrReturn(gen.ErrVariable("err")))
	default:
		panic("exhaustive type switch")
	}

	updateOffset := cc.Offset.UpdateStatementDynamic("read")
	return code + updateOffset
}
