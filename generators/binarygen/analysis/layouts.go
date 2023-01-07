package analysis

import (
	"go/constant"
	"go/types"
)

// BinarySize indicates how many bytes
// are needed to store a value
type BinarySize int

const (
	Byte BinarySize = 1 << iota
	Uint16
	Uint32
	Uint64
)

func newBinarySize(t *types.Basic) (BinarySize, bool) {
	switch t.Kind() {
	case types.Bool, types.Int8, types.Uint8:
		return Byte, true
	case types.Int16, types.Uint16:
		return Uint16, true
	case types.Int32, types.Uint32, types.Float32:
		return Uint32, true
	case types.Int64, types.Uint64, types.Float64:
		return Uint64, true
	default:
		return 0, false
	}
}

// Type is the common interface for struct field types
// supported by the package,
// describing the binary layout of a type.
type Type interface {
	// Origin returns the Go type yielding the type
	Origin() types.Type

	// IsFixedSize returns the number of byte needed to store an element,
	// or false if it is not known at compile time.
	IsFixedSize() (BinarySize, bool)
}

// ---------------------------- Concrete types ----------------------------

func (t Struct) Origin() types.Type           { return t.origin }
func (t Basic) Origin() types.Type            { return t.origin }
func (t DerivedFromBasic) Origin() types.Type { return t.origin }
func (t Offset) Origin() types.Type           { return t.Target.Origin() }
func (t Array) Origin() types.Type            { return t.origin }
func (t Slice) Origin() types.Type            { return t.origin }
func (t Union) Origin() types.Type            { return t.origin }
func (t Opaque) Origin() types.Type           { return t.origin }

// Struct defines the the binary layout
// of a struct
type Struct struct {
	origin *types.Named
	Fields []Field

	// Arguments is not empty if the struct parsing/writting function
	// requires data not provided in the input slice
	Arguments []Argument

	// HasParseEnd is non nil if the table has an
	// additional "parseEnd" method which must be called
	// at the end of parsing
	ParseEnd *types.Func
}

type ProvidedArgument struct {
	Value string // a go expression of the value
	For   string // the argument name this value is providing
}

// Field is a struct field.
// Embeded fields are not resolved.
type Field struct {
	Type Type
	Name string

	// name of other fields which will be provided
	// to parsing/writing functions
	ArgumentsProvidedByFields []ProvidedArgument

	// Non empty for fields indicating the kind of union
	// (usually the first field)
	UnionTag constant.Value

	// Non zero if the offset must be resolved into
	// the parent (or grand-parent) slice
	OffsetRelativeTo OffsetRelative
}

// IsFixedSize returns true if all the fields have fixed size.
func (st Struct) IsFixedSize() (BinarySize, bool) {
	var totalSize BinarySize
	for _, field := range st.Fields {
		size, ok := field.Type.IsFixedSize()
		if !ok {
			return 0, false
		}
		totalSize += size
	}
	return totalSize, true
}

// ResolveOffsetRelative return the union flag of all
// the fields.
func ResolveOffsetRelative(ty Type) OffsetRelative {
	switch ty := ty.(type) {
	case Struct:
		return ty.resolveOffsetRelative()
	case Slice:
		return ResolveOffsetRelative(ty.Elem)
	case Offset:
		return ResolveOffsetRelative(ty.Target)
	case Union:
		var out OffsetRelative
		for _, member := range ty.Members {
			out |= member.resolveOffsetRelative()
		}
		return out
	default:
		return 0
	}
}

func (st Struct) resolveOffsetRelative() (out OffsetRelative) {
	for _, field := range st.Fields {
		if field.OffsetRelativeTo == Parent {
			out |= Parent
		} else if field.OffsetRelativeTo == GrandParent {
			out |= GrandParent
		}

		// recurse
		child := ResolveOffsetRelative(field.Type)
		if child&GrandParent != 0 {
			out |= Parent
		}
	}
	return out
}

// Basic is a fixed size type, directly
// convertible from and to uintXX
type Basic struct {
	origin types.Type // may be named, but with underlying Basic
}

func (ba Basic) IsFixedSize() (BinarySize, bool) {
	return newBinarySize(ba.origin.Underlying().(*types.Basic))
}

// DerivedFromBasic is stored as a an uintXX, but
// uses custom constructor to perform the convertion :
// <typeString>FromUintXX ; <typeString>ToUintXX
type DerivedFromBasic struct {
	origin types.Type // may be named, but with underlying Basic

	// For aliases, it is the Name of the defined (not the "underlying" type)
	// For named types, the Name of the defined type
	// Otherwise, it is the string representation
	Name string

	// Size is the size as read and written in binary files
	Size BinarySize
}

func (de DerivedFromBasic) IsFixedSize() (BinarySize, bool) {
	return de.Size, true
}

// Offset is a fixed size integer pointing to
// an other type, which has never a fixed size.
type Offset struct {
	// Target if the type the offset is pointing at
	Target Type

	// Size if the size of the offset field
	Size BinarySize

	// IsPointer is true if the target type is actually
	// a pointer. In this case, [Target] is a [Struct].
	IsPointer bool
}

// IsFixedSize returns [Size], `false`, since, even if the offset itself has a fixed size,
// the whole data has not and requires additional length check.
func (of Offset) IsFixedSize() (BinarySize, bool) { return of.Size, false }

// Array is a fixed length array.
type Array struct {
	origin types.Type

	// Len is the length of elements in the array
	Len int

	// Elem is the type of the element
	Elem Type
}

func (ar Array) IsFixedSize() (BinarySize, bool) {
	elementSize, isElementFixed := ar.Elem.IsFixedSize()
	if !isElementFixed {
		return 0, false
	}
	return BinarySize(ar.Len) * elementSize, true
}

// Slice is a variable size array
// If Elem is [Offset], it represents a slice of (variable sized) elements
// written in binary as a slice of offsets
type Slice struct {
	origin types.Type

	// Elem is the type of the element
	Elem Type

	// Count indicates how to read/write the length of the array
	Count ArrayCount
	// CountExpr is used when [Count] is [ComputedField] or [ToComputedField]
	CountExpr string

	// SubsliceStart is only used for raw data ([]byte).
	SubsliceStart SubsliceStart
}

// IsFixedSize returns false and the length of the fixed size length prefix, if any.
func (sl Slice) IsFixedSize() (BinarySize, bool) {
	return sl.Count.Size(), false
}

// IsRawData returns true for []byte
func (sl Slice) IsRawData() bool {
	elem := sl.Elem.Origin().Underlying()
	if basic, isBasic := elem.(*types.Basic); isBasic {
		return basic.Kind() == types.Byte
	}
	return false
}

// UnionTagScheme is a union type for the two schemes
// supported : [UnionTagExplicit] or [UnionTagImplicit]
type UnionTagScheme interface {
	// TagsCode return the tags go code (like a constant name or a valid constant expression)
	TagsCode() []string
}

func (ut UnionTagExplicit) TagsCode() []string {
	out := make([]string, len(ut.Flags))
	for i, t := range ut.Flags {
		out[i] = t.Name()
	}
	return out
}

func (ut UnionTagImplicit) TagsCode() []string {
	out := make([]string, len(ut.Flags))
	for i, t := range ut.Flags {
		out[i] = t.ExactString()
	}
	return out
}

// UnionTagExplicit uses a field and defined constants.
// For instance :
//
//	type myStruct struct {
//		kind unionTag
//		data itf `unionField:"kind"`
//	}
//	type unionTag uint16
//	const (
//		unionTag1 = iota +1
//		unionTag2
//	 )
type UnionTagExplicit struct {
	// Flags are the possible flag values, in the same order as `Members`
	Flags []*types.Const

	// FlagField is the struct field indicating which
	// member is to be read
	FlagField string
}

// UnionTagImplicit uses a common field and values defined by struct tags
type UnionTagImplicit struct {
	Tag   Type
	Flags []constant.Value // in the same order as `Members`
}

// Union represents an union of several types,
// which are identified by constant flags.
type Union struct {
	origin *types.Named // with underlying type Interface

	// Members stores the possible members
	Members []Struct

	UnionTag UnionTagScheme
}

func (Union) IsFixedSize() (BinarySize, bool) { return 0, false }

// isTagImplicit checks for a common tag in each members, which must be the
// first field, and have same type.
// If so, it returns the tag [Type]
func isTagImplicit(members []Struct) (UnionTagImplicit, bool) {
	out := UnionTagImplicit{
		Flags: make([]constant.Value, len(members)),
	}

	all := map[types.Type]bool{}
	for i, member := range members {
		if len(member.Fields) == 0 {
			return out, false
		}
		firstField := member.Fields[0]
		all[firstField.Type.Origin()] = true
		out.Flags[i] = firstField.UnionTag
	}
	if len(all) != 1 {
		return out, false
	}
	out.Tag = members[0].Fields[0].Type
	return out, true
}

// Opaque represents a type with no binary structure.
// The parsing and writting step will be replaced by placeholder methods.
type Opaque struct {
	origin types.Type

	// ParserReturnsLength is true if the custom parsing
	// function returns the length read.
	ParserReturnsLength bool

	// How should the slice be passed
	SubsliceStart SubsliceStart
}

func (Opaque) IsFixedSize() (BinarySize, bool) { return 0, false }
