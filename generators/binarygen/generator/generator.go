package generator

import (
	"fmt"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-text/typesetting-utils/generators/binarygen/analysis"
)

type Accu map[string]bool

// Buffer is used to accumulate
// and de-deduplicate function declarations
type Buffer struct {
	seen  Accu
	decls []Declaration
}

// NewBuffer returns a ready to use buffer,
// which will add the new decleration to [accu].
func NewBuffer(accu Accu) Buffer { return Buffer{seen: accu} }

func (db *Buffer) Add(decl Declaration) {
	if db.seen[decl.ID] {
		return
	}
	db.decls = append(db.decls, decl)
	db.seen[decl.ID] = true
}

// remove non exported, unused function declaration
func (db Buffer) filterUnused(childTypes map[*types.Named]bool) []Declaration {
	var filtered []Declaration
	for i, decl := range db.decls {
		isUsed := false
		for j, other := range db.decls {
			if i == j {
				continue
			}
			if strings.Contains(other.Content, decl.ID) {
				// the function is used, keep it
				isUsed = true
				break
			}
		}
		// remove unused declaration, unless it is an exported top level one or a method
		if isUsed || strings.Contains(decl.ID, ".") || (decl.IsExported && !childTypes[decl.Origin]) {
			filtered = append(filtered, decl)
		}
	}

	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })
	return filtered
}

// Code removes the unused declaration and returns
// the final code.
func (db Buffer) Code(childTypes map[*types.Named]bool) string {
	var builder strings.Builder
	for _, decl := range db.filterUnused(childTypes) {
		builder.WriteString(decl.Content + "\n")
	}
	return builder.String()
}

// Declaration is a chunk of generated go code,
// with an id used to avoid duplication
type Declaration struct {
	ID         string
	Content    string
	IsExported bool
	Origin     *types.Named
}

// Name returns the representation of the given type in generated code,
// either its local name or its String
func Name(ty analysis.Type) string {
	if named, isNamed := ty.Origin().(*types.Named); isNamed {
		return named.Obj().Name()
	}

	return ty.Origin().String()
}

// Expression is a Go expression, such as a variable name, a static number, or an expression
type Expression = string

// Context holds the names of the objects used in the
// generated code
type Context struct {
	// <variableName> = parse<Type>(<byteSliceName>)
	// <byteSliceName> = append<Type>To(<variableName>, <byteSliceName>)

	// Type is the name of the type being generated
	Type Expression

	// ObjectVar if the name of the variable being parsed or dumped
	ObjectVar Expression

	// Slice is the name of the []byte being read or written
	Slice Expression

	// Offset holds the variable name for the current offset,
	// and its value when known at compile time
	Offset Offset

	// IgnoreUpdateOffset is true if the update offset statement
	// should not be written
	IgnoreUpdateOffset bool
}

type Err interface {
	wrap(context string) string
}

// simple 'err' statement
type ErrVariable string

func (ev ErrVariable) wrap(context string) string {
	return fmt.Sprintf(`fmt.Errorf("reading %s: %%s", %s)`, context, ev)
}

// represent a fmt.Errorf(..., args) statement
type ErrFormated string

func (ef ErrFormated) wrap(context string) string {
	return fmt.Sprintf(`fmt.Errorf("reading %s: " + %s)`, context, ef)
}

// ErrReturn returns a "return ..., err" statement
func (cc Context) ErrReturn(errVariable Err) string {
	return fmt.Sprintf("return %s, 0, %s", cc.ObjectVar, errVariable.wrap(cc.Type))
}

// Selector returns a "<ObjectVar>.<field>" statement
func (cc Context) Selector(field string) string {
	return fmt.Sprintf("%s.%s", cc.ObjectVar, field)
}

// SubSlice slices the current input slice at the current offset
// and assigns it to `subSlice`.
// It also updates the [Context.Slice] field
func (cc *Context) SubSlice(subSlice Expression) string {
	out := fmt.Sprintf("%s := %s[%s:]", subSlice, cc.Slice, cc.Offset.Name)
	cc.Slice = subSlice
	return out
}

func IsExported(typeName string) bool {
	return unicode.IsUpper([]rune(typeName)[0])
}

func ParseFunctionName(typeName string) string {
	prefix := "parse"
	if IsExported(typeName) {
		prefix = "Parse"
	}
	return prefix + strings.Title(typeName)
}

// ParsingFunc adds the context to the given [scopes] and [args], also
// adding the given comment as documentation
func (cc Context) ParsingFuncComment(origin *types.Named, args, scopes []string, comment string) Declaration {
	funcName := ParseFunctionName(cc.Type)
	if comment != "" {
		comment = "// " + comment + "\n"
	}
	content := fmt.Sprintf(`%sfunc %s(%s) (%s, int, error) {
		var %s %s
		%s
		return %s, %s, nil
	}
	`, comment, funcName, strings.Join(args, ","), cc.Type, cc.ObjectVar,
		cc.Type, strings.Join(scopes, "\n"), cc.ObjectVar, cc.Offset.Name)
	return Declaration{
		Origin:     origin,
		ID:         funcName,
		Content:    content,
		IsExported: IsExported(cc.Type),
	}
}

// ParsingFunc adds the context to the given [scopes] and [args]
func (cc Context) ParsingFunc(origin *types.Named, args, scopes []string) Declaration {
	return cc.ParsingFuncComment(origin, args, scopes, "")
}

// offset management

// Offset represents an offset in a byte of slice.
// It is designed to produce the optimal output regardless
// of whether its value is known at compile time or not.
type Offset struct {
	// Name is the name of the variable containing the offset
	Name Expression

	// part of the value known at compile time, or -1
	value int

	tmpIncrement analysis.BinarySize
}

func NewOffset(name Expression, initialValue int) Offset {
	return Offset{Name: name, value: initialValue}
}

func NewOffsetDynamic(name Expression) Offset {
	return Offset{Name: name, value: -1}
}

// Value returns the optimal Go expression for the offset current value.
func (of Offset) Value() Expression {
	if of.value != -1 {
		// use the compile time value
		return strconv.Itoa(of.value + int(of.tmpIncrement))
	}

	if of.tmpIncrement != 0 {
		return fmt.Sprintf("%s + %d", of.Name, of.tmpIncrement)
	}

	return of.Name
}

// With returns the optimal expression for <offset> + <size>
func (of Offset) With(size analysis.BinarySize) Expression {
	of.tmpIncrement += size // note the copy, so the receiver is not modified
	return of.Value()
}

// WithAffine returns the optimal expression for <offset> + <count>*<size>
func (of Offset) WithAffine(count Expression, size analysis.BinarySize) Expression {
	return arrayOffsetExpr(of.Value(), count, int(size))
}

// Increment updates the current value, adding [size].
// It is a no-op if the tracked value is unknown.
func (of *Offset) Increment(size analysis.BinarySize) {
	if of.value == -1 {
		of.tmpIncrement += size
		return
	}
	of.value += int(size)
}

// UpdateStatement returns a statement for <offset> += size,
// without changing the tracked value, since
// it has already been done with [Increment] calls.
// It also reset the temporary increment.
func (of *Offset) UpdateStatement(size analysis.BinarySize) Expression {
	of.tmpIncrement = 0
	return fmt.Sprintf("%s += %d", of.Name, size)
}

// UpdateStatementDynamic returns a statement for <offset> += size,
// and remove the tracked value which is now unknown.
func (of *Offset) UpdateStatementDynamic(size Expression) Expression {
	of.value = -1
	return fmt.Sprintf("%s += %s", of.Name, size)
}

// SetStatement returns the code for <offset> = <value>,
// and remove the tracked value which is now unknown
func (of *Offset) SetStatement(value Expression) Expression {
	of.value = -1
	return fmt.Sprintf("%s = %s", of.Name, value)
}

// ArrayOffset returns the expression for <offset> + <count> * <elementSize>,
// usable for offsets or array length
func ArrayOffset(offset Expression, count Expression, elementSize int) Expression {
	return arrayOffsetExpr(offset, count, elementSize)
}

func arrayOffsetExpr(offset Expression, count Expression, elementSize int) Expression {
	if elementSize == 1 {
		if offset == "0" || offset == "" {
			return count
		}
		return fmt.Sprintf("%s + %s", offset, count)
	} else {
		if offset == "0" || offset == "" {
			return fmt.Sprintf("%s * %d", count, elementSize)
		}
		return fmt.Sprintf("%s + %s * %d", offset, count, elementSize)
	}
}
