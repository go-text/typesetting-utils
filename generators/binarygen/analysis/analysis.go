// Package analysis uses go/types and go/packages to extract
// information about the structures to convert to binary form.
package analysis

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Analyser provides information about types,
// shared by the parser and writer code generator
type Analyser struct {
	// Source is the path of the origin go source file.
	Source string

	sourceAbsPath string

	pkg *packages.Package

	// Tables contains the resolved struct definitions, coming from the
	// go source file [Source] and its dependencies.
	Tables map[*types.Named]Struct

	// additional information used to retrieve aliases
	forAliases syntaxFieldTypes

	// additional special directives provided by comments
	commentsMap map[*types.Named]commments

	// used to link union member and indicator flag :
	// constant type -> constant values
	unionFlags map[*types.Named][]*types.Const

	// get the structs which are member of an interface
	interfaces map[*types.Interface][]*types.Named

	// map type string to data storage
	constructors map[string]*types.Basic

	// StandaloneUnions returns the union with an implicit union tag scheme,
	// for which standalone parsing/writing function should be generated
	StandaloneUnions map[*types.Named]Union

	// ChildTypes contains types that are used in other types.
	// For instance, top-level tables have a [false] value.
	ChildTypes map[*types.Named]bool
}

// ImportSource loads the source go file with go/packages,
// also returning the absolute path.
func ImportSource(path string) (*packages.Package, string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedName | packages.NeedFiles | packages.NeedDeps | packages.NeedImports,
	}
	tmp, err := packages.Load(cfg, "file="+absPath)
	if err != nil {
		return nil, "", err
	}
	if len(tmp) != 1 {
		return nil, "", fmt.Errorf("multiple packages not supported")
	}

	return tmp[0], absPath, nil
}

// NewAnalyserFromPkg uses [pkg] to analyse the tables defined in
// [sourcePath].
func NewAnalyserFromPkg(pkg *packages.Package, sourcePath, sourceAbsPath string) Analyser {
	an := Analyser{
		Source:        sourcePath,
		sourceAbsPath: sourceAbsPath,
		pkg:           pkg,
	}

	an.fetchStructsComments()
	an.fetchFieldAliases()
	an.fetchUnionFlags()
	an.fetchInterfaces()
	an.fetchConstructors()

	// perform the actual analysis
	an.Tables = make(map[*types.Named]Struct)
	an.StandaloneUnions = make(map[*types.Named]Union)
	an.ChildTypes = make(map[*types.Named]bool)
	for _, ty := range an.fetchSource() {
		an.handleTable(ty, false)
	}

	return an
}

// NewAnalyser load the package of `path` and
// analyze the defined structs, filling the fields
// [Source] and [Tables].
func NewAnalyser(path string) (Analyser, error) {
	pkg, absPath, err := ImportSource(path)
	if err != nil {
		return Analyser{}, err
	}

	return NewAnalyserFromPkg(pkg, path, absPath), nil
}

type syntaxFieldTypes = map[*types.Named]map[string]ast.Expr

func getSyntaxFields(scope *types.Scope, ty *ast.TypeSpec, st *ast.StructType) (*types.Named, map[string]ast.Expr) {
	named := scope.Lookup(ty.Name.Name).Type().(*types.Named)
	fieldTypes := make(map[string]ast.Expr)
	for _, field := range st.Fields.List {
		for _, fieldName := range field.Names {
			fieldTypes[fieldName.Name] = field.Type
		}
	}
	return named, fieldTypes
}

func (an *Analyser) PackageName() string { return an.pkg.Name }

// ByName returns the type with name [name], or panic
// if it does not exist
func (an *Analyser) ByName(name string) *types.Named {
	return an.pkg.Types.Scope().Lookup(name).Type().(*types.Named)
}

// go/types erase alias information, so we add it in a preliminary step
func (an *Analyser) fetchFieldAliases() {
	an.forAliases = make(syntaxFieldTypes)
	scope := an.pkg.Types.Scope()
	for _, file := range an.pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if ty, isType := n.(*ast.TypeSpec); isType {
				if st, isStruct := ty.Type.(*ast.StructType); isStruct {
					named, tys := getSyntaxFields(scope, ty, st)
					an.forAliases[named] = tys
					return false
				}
			}
			return true
		})
	}
}

func (an *Analyser) fetchStructsComments() {
	an.commentsMap = make(map[*types.Named]commments)
	scope := an.pkg.Types.Scope()
	for _, file := range an.pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if decl, isDecl := n.(*ast.GenDecl); isDecl {
				if len(decl.Specs) != 1 {
					return true
				}
				n = decl.Specs[0]
				if ty, isType := n.(*ast.TypeSpec); isType {
					typ := scope.Lookup(ty.Name.Name).Type()
					if named, ok := typ.(*types.Named); ok {
						an.commentsMap[named] = parseComments(decl.Doc)
					}
					return false
				}
			}
			return true
		})
	}
}

func (an *Analyser) allNamed() (out []*types.Named) {
	scope := an.pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)

		tn, isTypeName := obj.(*types.TypeName)
		if !isTypeName {
			continue
		}
		if tn.IsAlias() {
			// ignore top level aliases
			continue
		}
		out = append(out, tn.Type().(*types.Named))
	}
	return out
}

// register the structs in the given input file
func (an *Analyser) fetchSource() []*types.Named {
	var out []*types.Named
	for _, named := range an.allNamed() {
		obj := named.Obj()
		if _, isStruct := named.Underlying().(*types.Struct); isStruct {
			// filter by input file
			if an.pkg.Fset.File(obj.Pos()).Name() == an.sourceAbsPath {
				out = append(out, named)
			}
		}
	}
	return out
}

// look for integer constants with type <...>Version
// and values <...>Version<v>,
// which are mapped to concrete types <interfaceName><v>
func (an *Analyser) fetchUnionFlags() {
	an.unionFlags = make(map[*types.Named][]*types.Const)

	scope := an.pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)

		cst, isConst := obj.(*types.Const)
		if !isConst {
			continue
		}
		if cst.Val().Kind() != constant.Int {
			continue
		}

		named, ok := cst.Type().(*types.Named)
		if !ok {
			continue
		}

		if !strings.HasSuffix(named.Obj().Name(), "Version") {
			continue
		}

		an.unionFlags[named] = append(an.unionFlags[named], cst)
	}
}

func (an *Analyser) fetchInterfaces() {
	an.interfaces = make(map[*types.Interface][]*types.Named)

	named := an.allNamed()
	for _, name := range named {
		itf, isItf := name.Underlying().(*types.Interface)
		if !isItf {
			continue
		}

		// find the members of this interface
		for _, st := range named {
			// do not add the interface itself as member
			if _, isItf := st.Underlying().(*types.Interface); isItf {
				continue
			}
			if types.Implements(st, itf) {
				an.interfaces[itf] = append(an.interfaces[itf], st)
			}
		}
	}
}

func (an *Analyser) fetchConstructors() {
	an.constructors = make(map[string]*types.Basic)

	scope := an.pkg.Types.Scope()
	names := scope.Names()

	for _, name := range names {
		obj := scope.Lookup(name)

		fn, isFunction := obj.(*types.Func)
		if !isFunction {
			continue
		}

		sig := fn.Type().(*types.Signature)

		// look for <...>FromUint and patterns
		if typeName, _, ok := strings.Cut(fn.Name(), "FromUint"); ok {
			if sig.Params().Len() != 1 {
				panic("invalid signature for constructor " + fn.Name())
			}
			arg := sig.Params().At(0).Type().(*types.Basic)
			an.constructors[typeName] = arg
		}
	}
}

// handle table wraps [createFromStruct] by registering
// the type in [Tables]
func (an *Analyser) handleTable(ty *types.Named, isChildType bool) Struct {
	if isChildType {
		an.ChildTypes[ty] = true
	}

	if st, has := an.Tables[ty]; has {
		return st
	}

	st := an.createFromStruct(ty)
	an.Tables[ty] = st

	return st
}

// resolveName returns a string for the name of the given type,
// using [decl] to preserve aliases.
func (an *Analyser) resolveName(ty types.Type, decl ast.Expr) string {
	// check if we have an alias
	if ident, ok := decl.(*ast.Ident); ok {
		alias := an.pkg.Types.Scope().Lookup(ident.Name)
		if named, ok := alias.(*types.TypeName); ok && named.IsAlias() {
			return named.Name()
		}
	}
	// otherwise use the short name for Named types
	if named, ok := ty.(*types.Named); ok {
		return named.Obj().Name()
	}
	// defaut to the general string representation
	return ty.String()
}

// sliceElement returns the ast for the declaration of a slice or array element,
// or nil if the slice is for instance defined by a named type
func sliceElement(typeDecl ast.Expr) ast.Expr {
	if slice, ok := typeDecl.(*ast.ArrayType); ok {
		return slice.Elt
	}
	return nil
}

// createTypeFor analyse the given type `ty`.
// When it is found on a struct field, `tags` gives additional metadata.
// `decl` matches the syntax declaration of `ty` so that aliases
// can be retrieved.
func (an *Analyser) createTypeFor(ty types.Type, tags parsedTags, decl ast.Expr) Type {
	// first deals with special cases, defined by tags
	if tags.isOpaque {
		return Opaque{origin: ty, SubsliceStart: tags.subsliceStart}
	}

	if offset := tags.offsetSize; offset != 0 {
		// adjust the tags and "recurse" to the actual type
		tags.offsetSize = NoOffset

		// handle pointer types by dereferencing
		pointer, isPointer := ty.Underlying().(*types.Pointer)
		if isPointer {
			ty = pointer.Elem()
		}
		target := an.createTypeFor(ty, tags, decl)
		_, isFixedSize := target.IsFixedSize()
		_, isStruct := target.(Struct)
		if isFixedSize && !isStruct {
			panic("offset to (non struct) fixed size type is not supported")
		}
		if isPointer && !isStruct {
			panic("pointer are only supported for structs")
		}
		return Offset{Target: target, Size: offset.binary(), IsPointer: isPointer}
	}

	// now inspect the actual go type
	switch under := ty.Underlying().(type) {
	case *types.Basic:
		return an.createFromBasic(ty, decl)
	case *types.Array:
		elemDecl := sliceElement(decl)
		// handle array of offsets by adujsting [offsetSize]
		elemTags := parsedTags{offsetSize: tags.offsetsArray}
		// recurse on the element
		elem := an.createTypeFor(under.Elem(), elemTags, elemDecl)
		return Array{origin: ty, Len: int(under.Len()), Elem: elem}
	case *types.Struct:
		// anonymous structs are not supported
		return an.handleTable(ty.(*types.Named), true)
	case *types.Slice:
		elemDecl := sliceElement(decl)
		// handle array of offsets by adujsting [offsetSize]
		elemTags := parsedTags{offsetSize: tags.offsetsArray}
		// recurse on the element
		elem := an.createTypeFor(under.Elem(), elemTags, elemDecl)
		return Slice{
			origin: ty, Elem: elem,
			Count: tags.arrayCount, CountExpr: tags.arrayCountField,
			SubsliceStart: tags.subsliceStart,
		}
	case *types.Interface:
		// anonymous interface are not supported
		return an.createFromInterface(ty.(*types.Named), tags.unionField)
	default:
		panic(fmt.Sprintf("unsupported type %s", under))
	}
}

// [ty] has underlying type Basic
func (an *Analyser) createFromBasic(ty types.Type, decl ast.Expr) Type {
	// check for custom constructors
	name := an.resolveName(ty, decl)
	if binaryType, hasConstructor := an.constructors[name]; hasConstructor {
		size, _ := newBinarySize(binaryType)
		return DerivedFromBasic{origin: ty, Name: name, Size: size}
	}

	return Basic{origin: ty}
}

func (an *Analyser) createFromStruct(ty *types.Named) Struct {
	st := ty.Underlying().(*types.Struct)
	cm := an.commentsMap[ty]
	out := Struct{
		origin:    ty,
		Fields:    make([]Field, st.NumFields()),
		Arguments: cm.externalArguments,
	}

	customParseFunc := map[string]bool{}
	for i := 0; i < ty.NumMethods(); i++ {
		m := ty.Method(i)
		mName := m.Name()
		if mName == "parseEnd" {
			out.ParseEnd = m
		} else if _, field, ok := strings.Cut(mName, "parse"); ok {
			returnsLength := m.Type().(*types.Signature).Results().Len() == 2
			customParseFunc[field] = returnsLength
		}
	}

	for i := range out.Fields {
		field := st.Field(i)

		// process the struct tags
		tags := newTags(st, reflect.StructTag(st.Tag(i)))

		astDecl := an.forAliases[ty][field.Name()]

		fieldType := an.createTypeFor(field.Type(), tags, astDecl)
		if opaque, isOpaque := fieldType.(Opaque); isOpaque {
			opaque.ParserReturnsLength = customParseFunc[strings.Title(field.Name())]
			fieldType = opaque
		}

		out.Fields[i] = Field{
			Name:                      field.Name(),
			Type:                      fieldType,
			ArgumentsProvidedByFields: tags.requiredFieldArguments,
			UnionTag:                  tags.unionTag,
			OffsetRelativeTo:          tags.offsetRelativeTo,
		}
	}

	return out
}

func (an *Analyser) createFromInterface(ty *types.Named, unionField *types.Var) Union {
	itfName := ty.Obj().Name()
	itf := ty.Underlying().(*types.Interface)
	members := an.interfaces[itf]
	// this can't be correct in practice
	if len(members) == 0 {
		panic(fmt.Sprintf("interface %s does not have any member", itfName))
	}

	out := Union{origin: ty}
	for _, member := range members {
		// analyse the concrete type
		st := an.handleTable(member, true)

		out.Members = append(out.Members, st)
	}

	// resolve the union scheme, given priority to explicit
	if unionField != nil { // explicit
		flags := an.unionFlags[unionField.Type().(*types.Named)]
		// match flags and members
		byVersion := map[string]*types.Const{}
		for _, flag := range flags {
			_, version, _ := strings.Cut(flag.Name(), "Version")
			byVersion[version] = flag
		}

		scheme := UnionTagExplicit{FlagField: unionField.Name()}
		for _, member := range members {
			memberName := member.Obj().Name()
			// fetch the associated flag
			version := strings.TrimPrefix(memberName, itfName)
			flag, ok := byVersion[version]
			if !ok {
				panic(fmt.Sprintf("union flag %sVersion%s not defined", itfName, version))
			}
			scheme.Flags = append(scheme.Flags, flag)
		}
		out.UnionTag = scheme
	} else if scheme, ok := isTagImplicit(out.Members); ok {
		out.UnionTag = scheme
		an.StandaloneUnions[ty] = out
	} else {
		panic(fmt.Sprintf("union field with type %s is missing unionField tag", ty))
	}

	return out
}
