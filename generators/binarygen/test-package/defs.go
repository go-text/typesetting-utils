package testpackage

import "math"

type withFixedSize struct {
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

type toBeEmbeded struct {
	a, b byte
	c    []uint16 `arrayCount:"FirstUint16"`
}

type tag uint32

type float214 float32 // representated as 2.14 fixed point

func (f *float214) fromUint(v uint16) {
	*f = float214(math.Float32frombits(uint32(v)))
}

func (f float214) toUint() uint16 {
	return uint16(math.Float32bits(float32(f)))
}

type fl32 = float32

func fl32FromUint(v uint32) fl32 {
	return math.Float32frombits(uint32(v))
}

func fl32ToUint(f fl32) uint32 {
	return math.Float32bits(f)
}

type fl1616 = float32

func fl1616FromUint(v uint32) fl1616 {
	// value are actually signed integers
	return fl1616(int32(v)) / (1 << 16)
}

func fl1616ToUint(f fl1616) uint32 {
	return uint32(int32(f * (1 << 16)))
}

// other constants not interpreted as flags

type flagNotVersion_ uint

const _dummy1 = ""

const _dummy2 = 2

const _dummy3 flagNotVersion_ = 8

func (WithChildArgument) parseCustomWithArg(_ []byte, arrayCount int, kind uint16, version uint16) (int, error) {
	return 0, nil
}
