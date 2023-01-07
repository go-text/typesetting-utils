package analysis

import (
	"go/ast"
	"go/constant"
	"go/types"
	"reflect"
	"strconv"
	"strings"
)

// parsedTags is the result of parsing a field tag string
type parsedTags struct {
	arrayCountField string // used by [ComputedField], [ToComputedField]
	arrayCount      ArrayCount

	subsliceStart SubsliceStart

	offsetSize       OffsetSize
	offsetsArray     OffsetSize
	offsetRelativeTo OffsetRelative

	requiredFieldArguments []ProvidedArgument

	unionField *types.Var
	unionTag   constant.Value

	// isCustom is true if the field has
	// a custom parser/writter
	isOpaque bool
}

func newTags(st *types.Struct, tags reflect.StructTag) (out parsedTags) {
	_, out.isOpaque = tags.Lookup("isOpaque")

	switch tag := tags.Get("subsliceStart"); tag {
	case "AtStart":
		out.subsliceStart = AtStart
	case "AtCurrent":
		out.subsliceStart = AtCurrent
	case "":
		// make AtStart the default for opaque types
		if out.isOpaque {
			out.subsliceStart = AtStart
		} else {
			out.subsliceStart = AtCurrent
		}
	default:
		panic("invalic tag for subsliceStart : " + tag)
	}

	switch tag := tags.Get("arrayCount"); tag {
	case "FirstUint16":
		out.arrayCount = FirstUint16
	case "FirstUint32":
		out.arrayCount = FirstUint32
	case "ToEnd":
		out.arrayCount = ToEnd
	default:
		if _, field, hasComputedField := strings.Cut(tag, "ComputedField-"); hasComputedField {
			out.arrayCount = ComputedField
			out.arrayCountField = field
		} else if _, field, hasToField := strings.Cut(tag, "To-"); hasToField {
			out.arrayCount = ToComputedField
			out.arrayCountField = field
		} else if tag == "" {
			// default to NoLength
			out.arrayCount = NoLength
		} else {
			panic("invalid tag for arrayCount: " + tag)
		}
	}

	switch tag := tags.Get("offsetSize"); tag {
	case "Offset16":
		out.offsetSize = Offset16
	case "Offset32":
		out.offsetSize = Offset32
	case "":
	default:
		panic("invalid tag for offsetSize: " + tag)
	}

	switch tag := tags.Get("offsetsArray"); tag {
	case "Offset16":
		out.offsetsArray = Offset16
	case "Offset32":
		out.offsetsArray = Offset32
	case "":
	default:
		panic("invalid tag for offsetsArray: " + tag)
	}

	switch tag := tags.Get("offsetRelativeTo"); tag {
	case "Parent":
		out.offsetRelativeTo = Parent
	case "GrandParent":
		out.offsetRelativeTo = GrandParent
	case "":
	default:
		panic("invalid tag for offsetRelativeTo: " + tag)
	}

	unionField := tags.Get("unionField")
	if unionField != "" {
		for i := 0; i < st.NumFields(); i++ {
			if fi := st.Field(i); fi.Name() == unionField {
				out.unionField = fi
				break
			}
		}
		if out.unionField == nil {
			panic("unknow field for union version: " + unionField)
		}
	}

	if unionTag := tags.Get("unionTag"); unionTag != "" {
		value, err := strconv.Atoi(unionTag)
		if err != nil {
			panic(err)
		}
		out.unionTag = constant.MakeInt64(int64(value))
	}

	if args := tags.Get("arguments"); args != "" {
		chunks := strings.Split(tags.Get("arguments"), ",")

		for _, chunk := range chunks {
			forName, value, ok := strings.Cut(chunk, "=")
			if !ok {
				panic("expected <argName>=<value>, got " + chunk)
			}
			out.requiredFieldArguments = append(out.requiredFieldArguments, ProvidedArgument{
				Value: strings.TrimSpace(value),
				For:   strings.TrimSpace(forName),
			})
		}
	}

	return out
}

type Argument struct {
	VariableName string
	TypeName     string
}

type commments struct {
	// externalArguments may be provided it the type parsing/writting function
	// requires data not provided in the input slice
	externalArguments []Argument
}

// parse the type documentation looking for special comments
// of the following form :
//
//	// binarygen: argument=<name> <type>
func parseComments(doc *ast.CommentGroup) (out commments) {
	if doc == nil {
		return out
	}
	for _, comment := range doc.List {
		if _, value, ok := strings.Cut(comment.Text, " binarygen:"); ok {
			if _, argDef, ok := strings.Cut(value, "argument="); ok {
				name, typeN, _ := strings.Cut(argDef, " ")
				out.externalArguments = append(out.externalArguments, Argument{VariableName: name, TypeName: typeN})
			}
		}
	}
	return out
}

// ArrayCount defines how the number of elements in an array is defined
type ArrayCount uint8

const (
	// The length must be provided by the context and is not found in the binary
	NoLength ArrayCount = iota

	// The length is written at the start of the array, as an uint16
	FirstUint16
	// The length is written at the start of the array, as an uint32
	FirstUint32

	// The length is deduced from an other field, parsed previously,
	// or computed by a method or an expression
	ComputedField

	// For raw data, that is slice of bytes, this special value
	// indicates that the data must be copied until the end of the
	// given slice
	ToEnd

	// For raw data, that is slice of bytes, this special value
	// indicates that the data must be copied until the offset (not the length)
	// given by an other field, parsed previously,
	// or computed by a method or an expression
	ToComputedField
)

// SubsliceStart indicates where the start of the subslice
// given to the field parsing function shall be computed
type SubsliceStart uint8

const (
	// The current slice is sliced at the current offset for the field
	AtCurrent SubsliceStart = iota
	// The current slice is not resliced
	AtStart
)

// OffsetSize is the size (in bits) of the storage
// of an offset to a field type, or 0
type OffsetSize uint8

const (
	NoOffset OffsetSize = iota
	// The offset is written as uint16
	Offset16
	// The offset is written as uint32
	Offset32
)

func (os OffsetSize) binary() BinarySize {
	switch os {
	case Offset16:
		return Uint16
	case Offset32:
		return Uint32
	default:
		return 0
	}
}

// OffsetRelative indicates if the offset is related
// to the current slice or the one of its parent type (or grand parent)
type OffsetRelative uint8

const (
	Current     OffsetRelative = iota
	Parent                     = 1 << 0
	GrandParent                = 1 << 1
)
