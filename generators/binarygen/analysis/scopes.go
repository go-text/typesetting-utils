package analysis

// Scope defines one step of parsing/writting,
// which may come from several fields.
// It is an optimisation to reduce length checks
type Scope interface {
	isScope()
}

func (SingleField) isScope()       {}
func (StaticSizedFields) isScope() {}

type SingleField Field

// StaticSizedFields is a list of fields which all have a static size.
// Slice and Offset may be used to denote the fixed size part item.
type StaticSizedFields []Field

// Size return the cumulated size of all fields
func (fs StaticSizedFields) Size() BinarySize {
	var out BinarySize
	for _, field := range fs {
		s, _ := field.Type.IsFixedSize()
		out += s
	}
	return out
}

func (st Struct) Scopes() (out []Scope) {
	// as an optimization groups the contiguous fixed-size fields
	var (
		fixedSize     StaticSizedFields
		offsetsFields []Scope
	)
	for _, field := range st.Fields {
		// append to the static fields
		if _, isFixedSize := field.Type.IsFixedSize(); isFixedSize {
			fixedSize = append(fixedSize, field)
			continue
		}

		// special case for Offset and Slice
		if _, isOffset := field.Type.(Offset); isOffset {
			fixedSize = append(fixedSize, field)
			offsetsFields = append(offsetsFields, SingleField(field))
			continue
		} else if slice, isSlice := field.Type.(Slice); isSlice && slice.Count.Size() != 0 {
			fixedSize = append(fixedSize, field)
			// and also start a new scope
		}

		// else, close the current fixedSize array ...
		if len(fixedSize) != 0 {
			out = append(out, fixedSize)
			out = append(out, offsetsFields...)
			offsetsFields = nil
			fixedSize = nil
		}

		// and add a standalone field
		out = append(out, SingleField(field))
	}

	// close the current fixedSize array if needed
	if len(fixedSize) != 0 {
		out = append(out, fixedSize)
	}

	out = append(out, offsetsFields...)

	return out
}

// Size returns the binary size occupied by the count field,
// or zero if it is specified externally
func (c ArrayCount) Size() BinarySize {
	switch c {
	case FirstUint16:
		return Uint16
	case FirstUint32:
		return Uint32
	}
	return 0
}
