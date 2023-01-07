package testpackage

// Used to test that aliases are correctly retrieved
type WithAlias struct {
	f fl32
}

// Used to test interface support
type WithUnion struct {
	version    subtableFlagVersion
	otherField byte
	data       subtableITF `unionField:"version"`
}

type subtableFlagVersion uint16

const (
	subtableFlagVersion1 subtableFlagVersion = iota
	subtableFlagVersion2
)

type subtableITF interface {
	isSubtableITF()
}

type subtableITF1 struct {
	F uint64
}
type subtableITF2 struct {
	F uint8
}

func (subtableITF1) isSubtableITF() {}
func (subtableITF2) isSubtableITF() {}

// Used to test Offset support
type WithOffset struct {
	version           uint16
	offsetToSlice     []uint64 `offsetSize:"Offset32"`
	offsetToStruct    varSize  `offsetSize:"Offset32"`
	a, b, c           byte
	offsetToUnbounded []byte   `offsetSize:"Offset16" arrayCount:"ToEnd"`
	optional          *varSize `offsetSize:"Offset32"`
}

type WithOffsetArray struct {
	array []WithSlices `arrayCount:"FirstUint16" offsetsArray:"Offset32"`
}

// Used to test []byte support
type WithRawdata struct {
	length          uint32
	defaut          []byte // default, external length
	startTo         []byte `subsliceStart:"AtStart"` // external length
	currentToEnd    []byte `arrayCount:"ToEnd"`
	startToEnd      []byte `arrayCount:"ToEnd" subsliceStart:"AtStart"`
	currentToOffset []byte `arrayCount:"To-length"` // cut the origin early
}

// Used to check that static fields yields
// only one scope
type singleScope struct {
	a, b, c int32
	d       uint32
	e       int64
	g, h    byte
	t       tag
	v       float214
	w       fl32
	array1  [5]byte
	array2  [5]uint16
}

type multipleScopes struct {
	version  uint16
	coverage []byte `offsetSize:"Offset16"`
	x, y     int16
	lookups  []withFixedSize `arrayCount:"FirstUint16"`
	array2   []uint32        `arrayCount:"FirstUint16"`
}

type customType map[string]int

type WithOpaque struct {
	f                uint16
	opaque           customType `isOpaque:""`
	opaqueWithLength customType `isOpaque:""`
}

// parseOpaque is called by the generated parsing code
func (wo *WithOpaque) parseOpaque(src []byte) error {
	return nil
}

// parseOpaqueWithLength is called by the generated parsing code
func (wo *WithOpaque) parseOpaqueWithLength(src []byte) (int, error) {
	return 0, nil
}

type WithSlices struct {
	length uint16
	s1     []varSize `arrayCount:"ComputedField-length"`
}

type varSize struct {
	f1     uint32
	array  []uint32    `arrayCount:"FirstUint16"`
	stucts []WithAlias `arrayCount:"FirstUint32"`
}

func (v *varSize) parseEnd(src []byte) (int, error) {
	return len(src), nil
}

type withEmbeded struct {
	a, b, c byte
	toBeEmbeded
}

// uses type not defined in the origin source file
type withFromExternalFile struct {
	a withFixedSize
	b withFixedSize
}

type WithArray struct {
	a uint16
	b [4]uint32
	c [3]byte
}

// binarygen: argument=kind uint16
// binarygen: argument=version uint16
type withArgument struct {
	array []uint16 // count is required
}

type WithChildArgument struct {
	child  withArgument
	child2 withArgument
}

type PassArg struct {
	kind          uint16
	version       uint16
	count         int32
	customWithArg withArgument `arguments:"arrayCount=.count, kind=.kind, version=.version"`
}

type WithImplicitITF struct {
	field1 uint32
	itf    ImplicitITF
}

type ImplicitITF interface {
	isImplicitITF()
}

func (ImplicitITF1) isImplicitITF() {}
func (ImplicitITF2) isImplicitITF() {}
func (ImplicitITF3) isImplicitITF() {}

type ImplicitITF1 struct {
	kind uint16 `unionTag:"1"`
	data [5]byte
}
type ImplicitITF2 struct {
	kind uint16 `unionTag:"2"`
	data [5]byte
}
type ImplicitITF3 struct {
	kind uint16 `unionTag:"3"`
	data [5]uint64
}

type RootTable struct {
	E  Element   `offsetSize:"Offset16"`
	Es []Element `arrayCount:"FirstUint16"`
}

type Element struct {
	A        int32
	v        varSize      `offsetSize:"Offset32" offsetRelativeTo:"Parent"`
	VarSizes []varSize    `arrayCount:"FirstUint16" offsetsArray:"Offset32" offsetRelativeTo:"Parent"`
	sl       []SubElement `arrayCount:"FirstUint32"`
}

type SubElement struct {
	v varSize `offsetSize:"Offset16" offsetRelativeTo:"GrandParent"`
}

type VariableThenFixed struct {
	v varSize
	a uint16
	b uint32
	c [5]byte
}
