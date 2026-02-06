package packtab

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Pack a static table of integers into compact lookup tables to save space.
// This is Go port of https://github.com/harfbuzz/packtab

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func ceilDivBy8(v int) int { return int(math.Ceil(float64(v) / 8)) }

func min(l []int) int {
	v := l[0]
	for _, a := range l {
		if a < v {
			v = a
		}
	}
	return v
}

func max(l []int) int {
	v := l[0]
	for _, a := range l {
		if a > v {
			v = a
		}
	}
	return v
}

type autoMapping struct {
	dict    map[[2]int]int
	reverse map[int][2]int
	next    int
}

func newAutoMapping() *autoMapping {
	return &autoMapping{dict: make(map[[2]int]int), reverse: make(map[int][2]int)}
}

func (self *autoMapping) get(a, b int) int {
	key := [2]int{a, b}
	if v, ok := self.dict[key]; ok {
		return v
	}
	v := self.next
	self.next += 1
	self.dict[key] = v
	self.reverse[v] = key
	return v
}

func (self *autoMapping) getBack(v int) [2]int { return self.reverse[v] }

// Returns smallest power-of-two number of bits needed to represent n different values.
// binaryBitsFor(0, 0) == 0
// binaryBitsFor(0, 1) == 1
// binaryBitsFor(0, 6) == 4
// binaryBitsFor(0, 14) == 4
// binaryBitsFor(0, 15) == 4
// binaryBitsFor(0, 16) == 8
// binaryBitsFor(0, 100) == 8
func binaryBitsFor(minV, maxV int) int {
	if 0 <= minV && maxV <= 0 {
		return 0
	}
	if 0 <= minV && maxV <= 1 {
		return 1
	}
	if 0 <= minV && maxV <= 3 {
		return 2
	}
	if 0 <= minV && maxV <= 15 {
		return 4
	}

	if 0 <= minV && maxV <= 255 {
		return 8
	}
	if -128 <= minV && maxV <= 127 {
		return 8
	}

	if 0 <= minV && maxV <= 65535 {
		return 16
	}
	if -32768 <= minV && maxV <= 32767 {
		return 16
	}

	if 0 <= minV && maxV <= 4294967295 {
		return 32
	}
	if -2147483648 <= minV && maxV <= 2147483647 {
		return 32
	}

	if 0 <= minV && maxV <= math.MaxInt {
		return 64
	}
	if -9223372036854775808 <= minV && maxV <= 9223372036854775807 {
		return 64
	}
	panic("unreachable")
}

func typeFor(minV, maxV int) string {
	bits := binaryBitsFor(minV, maxV)
	if bits <= 8 {
		bits = 8
	}
	if minV < 0 {
		return fmt.Sprintf("int%d", bits)
	}
	return fmt.Sprintf("uint%d", bits)
}

func cast(typ, expr string) string {
	return fmt.Sprintf("%s(%s)", typ, expr)
}

func tertiary(cond, trueExpr, falseExpr string) string {
	return fmt.Sprintf("if %s { %s } else { %s }", cond, trueExpr, falseExpr)
}

const (
	bytesPerOp       = 4
	lookupOps        = 4
	subByteAccessOps = 4
)

type innerSolution struct {
	layer     *Layer
	next      *innerSolution
	nLookups  int
	nExtraOps int
	cost      int

	bits int
}

func newInnerSolution(
	layer *Layer,
	next *innerSolution,
	nLookups int,
	nExtraOps int,
	cost int,
	bits int,
) innerSolution {
	return innerSolution{layer, next, nLookups, nExtraOps, cost, bits}
}

type OuterSolution struct {
	innerSolution
}

func newOuterSolution(
	layer *Layer,
	next *innerSolution,
	nLookups int,
	nExtraOps int,
	cost int,
) OuterSolution {
	return OuterSolution{innerSolution{layer, next, nLookups, nExtraOps, cost, 0}}
}

func (sl innerSolution) fullCost() int {
	return sl.cost + (sl.nLookups*lookupOps+sl.nExtraOps)*bytesPerOp
}

func (self innerSolution) genCode(code *Code, name, var_ string) (string, string) {
	inputVar := var_
	if name != "" {
		var_ = "u"
	}
	expr := var_

	retType := typeFor(self.layer.minV, self.layer.maxV)
	unitBits := self.layer.unitBits
	if unitBits == 0 {
		expr = fmt.Sprintf("%d", self.layer.data[0])
		return retType, expr
	}

	shift := self.bits
	mask := (1 << shift) - 1

	if self.next != nil {
		_, expr = self.next.genCode(code, "", fmt.Sprintf("((%s)>>%d)", var_, shift))
	}
	// Generate data.
	var layers []*Layer
	layer := self.layer
	bits := self.bits
	for bits != 0 {
		layers = append(layers, layer)
		layer = layer.next
		bits -= 1
	}

	var data []int
	if len(layers) == 0 {
		data = append(data, layer.data...)
	} else {
		for d := 0; d < layer.maxV+1; d++ {
			_expand(d, layers, len(layers)-1, &data)
		}
	}

	data = _combine(data, self.layer.unitBits)

	arrName, start := code.addArray(retType, data)

	// Generate expression.
	var index0 string
	if expr == "0" {
		index0 = ""
	} else if shift == 0 {
		index0 = expr
	} else {
		index0 = fmt.Sprintf("((%s)<<%d)", expr, shift)
	}
	index1 := ""
	if mask != 0 {
		index1 = fmt.Sprintf("((%s)&%d)", var_, mask)
	}
	index := index0 + index1
	if index0 != "" && index1 != "" {
		index = index0 + "+" + index1
	}
	if unitBits >= 8 {
		if start != 0 {
			index = fmt.Sprintf("%d+%s", start, index)
		}
		expr = fmt.Sprintf("%s[%s]", arrName, index)
	} else {
		shift1 := int(math.Round(math.Log2(float64(8 / unitBits))))
		mask1 := (8 / unitBits) - 1
		shift2 := int(math.Round(math.Log2(float64(unitBits))))
		mask2 := (1 << unitBits) - 1
		funcBody := fmt.Sprintf("return (a[i>>%d]>>((i&%d)<<%d))&%d", shift1, mask1, shift2, mask2)
		funcName := code.addFunction("uint8", fmt.Sprintf("bits%d", unitBits), [][2]string{{"a", "[]uint8"}, {"i", "int"}}, funcBody)
		slicedArray := fmt.Sprintf("%s[:]", arrName)
		if start != 0 {
			slicedArray = fmt.Sprintf("%s[%d:]", arrName, start)
		}
		expr = fmt.Sprintf("%s(%s,%s)", funcName, slicedArray, index)
	}
	// Wrap up.

	if name != "" {
		funcName := code.addFunction(retType, name, [][2]string{{"u", "uint32"}}, expr)
		expr = fmt.Sprintf("%s(%s)", funcName, inputVar)
	}

	return retType, expr
}

func _expand(v int, stack []*Layer, i int, out *[]int) {
	if i < 0 {
		*out = append(*out, v)
		return
	}
	v2 := stack[i].mapping.getBack(v)
	i -= 1
	_expand(v2[0], stack, i, out)
	_expand(v2[1], stack, i, out)
}

func _combine(data []int, bits int) []int {
	if bits <= 1 {
		return _combine2(data, func(a, b int) int { return (b << 1) | a })
	}
	if bits <= 2 {
		return _combine2(data, func(a, b int) int { return (b << 2) | a })
	}
	if bits <= 4 {
		return _combine2(data, func(a, b int) int { return (b << 4) | a })
	}
	return data
}

func _combine2(data []int, f func(a, b int) int) []int {
	data2 := make([]int, len(data)/2)
	for i := range data2 {
		a, b := data[2*i], data[2*i+1]
		data2[i] = f(a, b)
	}
	return data2
}

type Layer struct {
	data       []int
	minV, maxV int
	next       *Layer
	solutions  []innerSolution

	extraOps, unitBits, bytes int

	default_, bias, mult int // for OuterLayer

	mapping *autoMapping
}

// A layer that can reproduce @data passed to its constructor, by
// using multiple lookup tables that split the domain by powers
// of two.
func newInnerLayer(data []int) *Layer {
	var self Layer
	self.data = data
	self.maxV = max(data)
	self.minV = min(data)
	self.unitBits = binaryBitsFor(self.minV, self.maxV)
	if self.unitBits < 8 {
		self.extraOps = subByteAccessOps
	}
	self.bytes = ceilDivBy8(self.unitBits * len(self.data))

	if self.maxV == 0 {
		return &self
	}

	self.split()

	self.solutions = append(self.solutions, newInnerSolution(&self, nil, 1, self.extraOps, self.bytes, 0))

	bits := 1
	layer := self.next
	for layer != nil {
		extraCost := ceilDivBy8((layer.maxV + 1) * (1 << bits) * self.unitBits)
		for i, s := range layer.solutions {
			nLookups := s.nLookups + 1
			nExtraOps := s.nExtraOps + self.extraOps
			cost := s.cost + extraCost
			self.solutions = append(self.solutions, newInnerSolution(&self, &layer.solutions[i], nLookups, nExtraOps, cost, bits))
		}

		layer = layer.next
		bits += 1
	}

	self.prune_solutions()

	return &self
}

func (self *Layer) split() {
	if len(self.data)&1 != 0 {
		self.data = append(self.data, 0)
	}

	mapping := newAutoMapping()
	self.mapping = mapping
	data2 := _combine2(self.data, mapping.get)

	self.next = newInnerLayer(data2)
}

// Remove dominated solutions.
func (self *Layer) prune_solutions() {
	// Doing it the slowest, O(N^2), way for now.
	sols := self.solutions
	for _, a := range sols {
		if a.cost == -1 {
			continue
		}
		for j, b := range sols {
			if a == b {
				continue
			}
			if b.cost == -1 {
				continue
			}

			// Rules of dominance: a being not worse than b
			if a.nLookups <= b.nLookups && a.fullCost() <= b.fullCost() {
				sols[j].cost = -1
				continue
			}
		}
	}
	var tmp []innerSolution
	for _, s := range self.solutions {
		if s.cost != -1 {
			tmp = append(tmp, s)
		}
	}
	sort.Slice(tmp, func(i, j int) bool { return tmp[i].nLookups < tmp[j].nLookups })
	self.solutions = tmp
}

// gcd([]) == 		1
// gcd([48]) == 		48
// gcd([-48]) == 		48
// gcd([48, 60]) == 		12
// gcd([48, 60, 6]) == 		6
// gcd([48, 61, 6]) == 		1
func gcd(lst []int) int {
	if len(lst) == 0 {
		return 1
	}
	x := abs(lst[0])
	for y := range lst[1:] {
		y = abs(y)
		for y != 0 {
			x, y = y, x%y
		}
		if x == 1 {
			break
		}
	}
	return x
}

// A layer that can reproduce @data passed to its constructor, by
// simple arithmetic tricks to reduce its size.
func newOuterLayer(data []int, default_ int) []OuterSolution {
	for data[len(data)-1] == default_ {
		data = data[:len(data)-1]
	}
	var self Layer
	self.data = data
	self.default_ = default_

	self.minV, self.maxV = min(data), max(data)

	bias := 0
	mult := 1
	unitBits := binaryBitsFor(self.minV, self.maxV)

	b := self.minV
	candidateBits := binaryBitsFor(0, self.maxV-b)
	if unitBits > candidateBits {
		unitBits = candidateBits
		bias = b
	}

	m := gcd(data)
	candidateBits = binaryBitsFor(self.minV/m, self.maxV/m)
	if unitBits > candidateBits {
		unitBits = candidateBits
		bias = 0
		mult = m
	}

	if b != 0 {
		tmp := make([]int, len(data))
		for i, d := range data {
			tmp[i] = d - b
		}
		m = gcd(tmp)
		candidateBits = binaryBitsFor(0, (self.maxV-b)/m)
		if unitBits > candidateBits {
			unitBits = candidateBits
			bias = b
			mult = m
		}
	}

	data = make([]int, len(self.data))
	for i, d := range self.data {
		data[i] = (d - bias) / mult
	}
	default_ = (self.default_ - bias) / mult

	self.unitBits = unitBits
	if self.unitBits < 8 {
		self.extraOps = subByteAccessOps
	}
	self.bias = bias
	if bias != 0 {
		self.extraOps += 1
	}
	self.mult = mult
	if mult != 0 {
		self.extraOps += 1
	}

	self.bytes = ceilDivBy8(self.unitBits * len(self.data))
	self.next = newInnerLayer(data)

	extraCost := 0

	solutions := make([]OuterSolution, len(self.next.solutions))
	for i, s := range self.next.solutions {
		nLookups := s.nLookups
		nExtraOps := s.nExtraOps + self.extraOps
		cost := s.cost + extraCost
		solutions[i] = newOuterSolution(&self, &self.next.solutions[i], nLookups, nExtraOps, cost)
	}

	return solutions
}

// @data is either a dictionary mapping integer keys to values, of an
// iterable containing values for keys starting at zero. Values must
// all be integers, or all strings.
//
// @mapping, if set, should be either a mapping from integer keys to
// string values, or vice versa.  Either way, it's first augmented by its
// own inverse.  After that it's used to map any value in @data that is
// not an integer.  If @mapping is not provided and data values are
// strings, the strings are written out verbatim.
//
// If mapping is not provided and values are strings, it is assumed that they
// all fit in an unsigned char.
//
// @default is value to be used for keys not specified in @data.  Defaults
// to zero.
// default=0, compression=1, mapping=nil
func PackTable(data []int, default_ int, compression float64) OuterSolution {
	// // Set up mapping.  See docstring.
	// if mapping is not nil:
	//     // assert (all(isinstance(k, int) and not isinstance(v, int) for k,v in mapping.items()) or
	//     //        all(not isinstance(k, int) and isinstance(v, int) for k,v in mapping.items()))
	//     mapping2 = mapping.copy()
	//     for k, v in mapping.items():
	//         mapping2[v] = k
	//     mapping = mapping2
	//     del mapping2

	// // Set up data as a list.
	// if isinstance(data, dict):
	//     assert all(isinstance(k, int) for k, v in data.items())
	//     minK = min(data.keys())
	//     maxK = max(data.keys())
	//     assert minK >= 0
	//     data2 = [default] * (maxK + 1)
	//     for k, v in data.items():
	//         data2[k] = v
	//     data = data2
	//     del data2

	// // Convert all to integers
	// assert all(isinstance(v, int) for v in data) or all(
	//     not isinstance(v, int) for v in data
	// )
	// if not isinstance(data[0], int) and mapping is not nil:
	//     data = [mapping[v] for v in data]
	// if not isinstance(default, int) and mapping is not nil:
	//     default = mapping[default]

	solutions := newOuterLayer(data, default_)
	return pickSolution(solutions, compression)
}

func pickSolution(solutions []OuterSolution, compression float64) OuterSolution {
	bestSol := solutions[0]
	bestScore := math.Inf(1)
	for _, s := range solutions {
		score := float64(s.nLookups) + compression*math.Log2(float64(s.fullCost()))
		if score < bestScore {
			bestSol, bestScore = s, score
		}
	}
	return bestSol
}

const packageHeader = `// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package unicodedata

// Code generated by typesetting-utils/generators/packtab. DO NOT EDIT

// Unicode version: %s

`

func (self OuterSolution) code(name string) string {
	code := NewCode(name)

	var_ := "u"
	expr := var_

	retType := typeFor(self.layer.minV, self.layer.maxV)
	unitBits := self.layer.unitBits
	if unitBits == 0 {
		panic("TODO")
	}

	if self.next != nil {
		_, expr = self.next.genCode(code, "", var_)
	}

	if self.layer.mult != 1 {
		expr = fmt.Sprintf("%d*%s", self.layer.mult, expr)
	}
	if self.layer.bias != 0 {
		if self.layer.bias < 0 {
			expr = cast(retType, expr)
		}
		expr = fmt.Sprintf("%d+%s", self.layer.bias, expr)
	}
	expr = tertiary(fmt.Sprintf("%s<%d", var_, len(self.layer.data)), "return "+expr, fmt.Sprintf("return %d", self.layer.default_))
	// TODO Map default?

	code.addFunction(retType, "lookup", [][2]string{{"u", "int"}}, expr)

	return code.Print()
}

// compile time know array
type array struct {
	typ    string // integer type
	values []int
}

type function struct {
	retType string
	args    [][2]string // (name, type) pairs
	body    string
}

// Code is an accumulator for output code
type Code struct {
	namespace string // prefix
	functions map[string]function
	arrays    map[string]array
}

func NewCode(namespace string) *Code {
	return &Code{namespace, make(map[string]function), make(map[string]array)}
}

func (self *Code) nameFor(name string) string {
	return fmt.Sprintf("%s%s", self.namespace, strings.Title(name))
}

func (self *Code) addFunction(retType, name string, args [][2]string, body string) string {
	name = self.nameFor(name)
	self.functions[name] = function{retType, args, body}
	return name
}

func (self *Code) addArray(typ string, values []int) (_ string, start int) {
	name := self.nameFor(typ)
	var existing []int
	if ar, has := self.arrays[name]; has {
		existing = ar.values
	}
	// extends existing values
	start = len(existing)
	self.arrays[name] = array{typ, append(existing, values...)}
	return name, start
}

func (self Code) Print() string {
	var out strings.Builder

	out.WriteString(`// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

		package unicodedata
	
		// Code generated by typesetting-utils/generators/packtab. DO NOT EDIT

		// Unicode version: %s

		`)

	for name, array := range self.arrays {
		out.WriteString(fmt.Sprintf("var %s = [%d]%s{", name, len(array.values), array.typ))
		for i, v := range array.values {
			if i%20 == 0 {
				out.WriteString("\n")
			}
			out.WriteString(strconv.Itoa(v) + ",")
		}
		out.WriteString("\n}\n\n")
	}

	for name, function := range self.functions {
		var code string
		for _, arg := range function.args {
			code += fmt.Sprintf("%s %s,", arg[0], arg[1])
		}
		out.WriteString(fmt.Sprintf("func %s(%s) %s {\n", name, code, function.retType))
		out.WriteString(function.body)
		out.WriteString("\n}\n")
	}

	return out.String()
}
