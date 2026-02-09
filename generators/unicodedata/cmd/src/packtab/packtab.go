package packtab

import (
	"fmt"
	"io"
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

// in bytes
func sizeOf(typ string) int {
	switch typ {
	case "uint8", "int8":
		return 1
	case "uint16", "int16":
		return 2
	case "uint32", "int32":
		return 4
	case "uint64", "int64":
		return 8
	default:
		panic(typ)
	}
}

const (
	bytesPerOp       = 4
	lookupOps        = 4
	subByteAccessOps = 4
)

type innerSolution struct {
	layer     *layer
	next      *innerSolution
	nLookups  int
	nExtraOps int
	cost      int

	bits int
}

func newInnerSolution(
	layer *layer,
	next *innerSolution,
	nLookups int,
	nExtraOps int,
	cost int,
	bits int,
) innerSolution {
	return innerSolution{layer, next, nLookups, nExtraOps, cost, bits}
}

func (sl innerSolution) fullCost() int {
	return sl.cost + (sl.nLookups*lookupOps+sl.nExtraOps)*bytesPerOp
}

func expand(v int, stack []*layer, i int, out *[]int) {
	if i < 0 {
		*out = append(*out, v)
		return
	}
	v2 := stack[i].mapping.getBack(v)
	i -= 1
	expand(v2[0], stack, i, out)
	expand(v2[1], stack, i, out)
}

func combine(data []int, bits int) []int {
	if bits <= 1 {
		data = combineBy2(data, func(a, b int) int { return (b << 1) | a })
	}
	if bits <= 2 {
		data = combineBy2(data, func(a, b int) int { return (b << 2) | a })
	}
	if bits <= 4 {
		data = combineBy2(data, func(a, b int) int { return (b << 4) | a })
	}
	return data
}

func combineBy2(data []int, f func(a, b int) int) []int {
	data2 := make([]int, len(data)/2)
	for i := range data2 {
		a, b := data[2*i], data[2*i+1]
		data2[i] = f(a, b)
	}
	return data2
}

type layer struct {
	data       []int
	minV, maxV int
	next       *layer
	solutions  []innerSolution

	extraOps, unitBits, bytes int

	default_, bias, mult int // for OuterLayer

	mapping *autoMapping
}

// A layer that can reproduce @data passed to its constructor, by
// using multiple lookup tables that split the domain by powers
// of two.
func newInnerLayer(data []int) *layer {
	var self layer
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

func (self *layer) split() {
	if len(self.data)&1 != 0 {
		self.data = append(self.data, 0)
	}

	mapping := newAutoMapping()
	self.mapping = mapping
	data2 := combineBy2(self.data, mapping.get)

	self.next = newInnerLayer(data2)
}

// Remove dominated solutions.
func (self *layer) prune_solutions() {
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
	var self layer
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

// @default is value to be used for keys not specified in @data.  Defaults
// to zero.
// default=0, compression=1, mapping=nil
func PackTable(data []int, default_ int, compression float64) OuterSolution {
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

func (self innerSolution) genCode(code *code, name, var_ string) (string, string) {
	inputVar := var_
	if name != "" {
		var_ = "u"
	}
	expr := var_

	isVarComposite := strings.ContainsRune(var_, '(')

	retType := typeFor(self.layer.minV, self.layer.maxV)
	unitBits := self.layer.unitBits
	if unitBits == 0 {
		expr = fmt.Sprintf("%d", self.layer.data[0])
		return retType, expr
	}

	shift := self.bits
	mask := (1 << shift) - 1

	if self.next != nil {
		if isVarComposite {
			_, expr = self.next.genCode(code, "", fmt.Sprintf("((%s)>>%d)", var_, shift))
		} else {
			_, expr = self.next.genCode(code, "", fmt.Sprintf("(%s>>%d)", var_, shift))
		}
	}
	// Generate data.
	var layers []*layer
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
			expand(d, layers, len(layers)-1, &data)
		}
	}

	data = combine(data, self.layer.unitBits)

	arrName, start := code.addArray(retType, data)

	// Generate expression.
	var index0 string
	if expr == "0" {
		index0 = ""
	} else if shift == 0 {
		index0 = expr
	} else {
		index0 = fmt.Sprintf("((%s)<<%d)", asUsize(expr), shift)
	}
	index1 := ""
	if mask != 0 {
		if isVarComposite {
			index1 = fmt.Sprintf("((%s)&%d)", var_, mask)
		} else {
			index1 = fmt.Sprintf("(%s&%d)", var_, mask)
		}
	}
	index := asUsize(index0) + asUsize(index1)
	if index0 != "" && index1 != "" {
		index = asUsize(index0) + "+" + asUsize(index1)
	}
	if unitBits >= 8 {
		if start != 0 {
			index = fmt.Sprintf("%d+%s", start, asUsize(index))
		}
		expr = fmt.Sprintf("%s[%s]", arrName, index)
	} else {
		shift1 := int(math.Round(math.Log2(float64(8 / unitBits))))
		mask1 := (8 / unitBits) - 1
		shift2 := int(math.Round(math.Log2(float64(unitBits))))
		mask2 := (1 << unitBits) - 1
		funcBody := fmt.Sprintf("return (a[i>>%d]>>((i&%d)<<%d))&0b%b", shift1, mask1, shift2, mask2)
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

type OuterSolution struct {
	innerSolution
}

func newOuterSolution(
	layer *layer,
	next *innerSolution,
	nLookups int,
	nExtraOps int,
	cost int,
) OuterSolution {
	return OuterSolution{innerSolution{layer, next, nLookups, nExtraOps, cost, 0}}
}

func (self OuterSolution) String() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "%d %d %d\n", self.nLookups, self.nExtraOps, self.cost)
	for next := self.next; next != nil; next = next.next {
		fmt.Fprintf(&buf, "\t%d %d %d\n", next.nLookups, next.nExtraOps, next.cost)
	}
	return buf.String()
}

func (self OuterSolution) Code(name string) string {
	code := newCode(name)

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
	expr = tertiary(fmt.Sprintf("0 <= %s && %s<%d", var_, var_, len(self.layer.data)), "return "+expr, fmt.Sprintf("return %d", self.layer.default_))
	// TODO Map default?

	code.addFunction(retType, "lookup", [][2]string{{"u", "rune"}}, expr)

	return code.print()
}

// we actually use the idiomatique int type
func asUsize(expr string) string {
	_, err := strconv.Atoi(expr)
	if expr == "" || err == nil {
		return expr
	}
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return "int" + expr
	}
	return fmt.Sprintf("int(%s)", expr)
}

func cast(typ, expr string) string {
	return fmt.Sprintf("%s(%s)", typ, expr)
}

func tertiary(cond, trueExpr, falseExpr string) string {
	return fmt.Sprintf("if %s { %s } else { %s }", cond, trueExpr, falseExpr)
}

// Array is a compile time known array
type Array struct {
	Type   string // integer type
	Values []int
}

func (ar Array) Size() int {
	elemSize := sizeOf(ar.Type)
	return elemSize * len(ar.Values)
}

type function struct {
	name string

	retType string
	args    [][2]string // (name, type) pairs
	body    string
}

// code is an accumulator for output code
type code struct {
	namespace string // prefix
	functions []function
	arrays    map[string]Array
}

func newCode(namespace string) *code {
	return &code{namespace, nil, make(map[string]Array)}
}

func (self *code) nameFor(name string) string {
	return fmt.Sprintf("%s%s", self.namespace, strings.Title(name))
}

func (self *code) addFunction(retType, name string, args [][2]string, body string) string {
	name = self.nameFor(name)
	for _, existing := range self.functions {
		if existing.name == name {
			return name
		}
	}

	self.functions = append(self.functions, function{name, retType, args, body})
	return name
}

func (self *code) addArray(typ string, values []int) (_ string, start int) {
	name := self.nameFor(typ)
	var existing []int
	if ar, has := self.arrays[name]; has {
		existing = ar.Values
	}
	// extends existing values
	start = len(existing)
	self.arrays[name] = Array{typ, append(existing, values...)}
	return name, start
}

func PrintArray(w io.Writer, name string, array Array, hex bool) {
	fmt.Fprintf(w, "var %s = [%d]%s{", name, len(array.Values), array.Type)
	for i, v := range array.Values {
		if i%20 == 0 {
			fmt.Fprintln(w, "")
		}
		if hex {
			fmt.Fprintf(w, "0x%x,", v)
		} else {
			fmt.Fprintf(w, "%d,", v)
		}
	}
	fmt.Fprintln(w, "\n}")
	fmt.Fprintln(w)
}

func (self code) print() string {
	var out strings.Builder

	var arrayKeys []string
	for key := range self.arrays {
		arrayKeys = append(arrayKeys, key)
	}
	sort.Strings(arrayKeys)

	totalSize := 0
	for _, name := range arrayKeys {
		array := self.arrays[name]
		totalSize += array.Size()
		PrintArray(&out, name, array, false)
	}

	out.WriteString(fmt.Sprintf("// Total size %d B.\n\n", totalSize))

	for _, function := range self.functions {
		var code string
		for _, arg := range function.args {
			code += fmt.Sprintf("%s %s,", arg[0], arg[1])
		}
		out.WriteString(fmt.Sprintf("func %s(%s) %s {\n", function.name, code, function.retType))
		out.WriteString(function.body)
		out.WriteString("\n}\n")
	}

	return out.String()
}
