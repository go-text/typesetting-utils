package main

// copied from typesetting/fontscan/rune_coverage.go

// pageSet is the base storage for a compact rune set.
// A rune is first reduced to its lower byte 'b'. Then the index
// of 'b' in the page is given by the 3 high bits (from 0 to 7)
// and the position in the resulting uint32 is given by the 5 lower bits (from 0 to 31)
type pageSet [8]uint32

// pageRef stores the second and third bytes of a rune (uint16(r >> 8)),
// shared by all the runes in a page.
type pageRef = uint16

type runePage struct {
	ref pageRef
	set pageSet
}

// runeSet is an efficient implementation of a rune set (that is a map[rune]bool),
// used to store the Unicode points supported by a font, and optimized to deal with consecutive
// runes.
type runeSet []runePage

// findPageFrom is the same as findPagePos, but
// start the binary search with the given `low` index
func (rs runeSet) findPageFrom(low int, ref pageRef) int {
	high := len(rs) - 1
	for low <= high {
		mid := (low + high) >> 1
		page := rs[mid].ref
		if page == ref {
			return mid // found the page
		}
		if page < ref {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if high < 0 || (high < len(rs) && rs[high].ref < ref) {
		high++
	}
	return -(high + 1) // the page is not in the set, but should be inserted at high
}

// findPagePos searches for the leaf containing the specified number.
// It returns its index if it exists, otherwise it returns the negative of
// the (`position` + 1) where `position` is the index where it should be inserted
func (rs runeSet) findPagePos(page pageRef) int { return rs.findPageFrom(0, page) }

// findPage returns the page containing the specified char, or nil
// if it doesn't exists
func (rs runeSet) findPage(ref pageRef) *pageSet {
	pos := rs.findPagePos(ref)
	if pos >= 0 {
		return &rs[pos].set
	}
	return nil
}

// findOrCreatePage locates the page containing the specified char, creating it if needed,
// and returns a pointer to it
func (rs *runeSet) findOrCreatePage(ref pageRef) *pageSet {
	pos := rs.findPagePos(ref)
	if pos < 0 { // the page doest not exists, create it
		pos = -pos - 1
		rs.insertPage(runePage{ref: ref}, pos)
	}

	return &(*rs)[pos].set
}

// insertPage inserts the given `page` at `pos`, meaning the resulting page can be accessed via &rs[pos]
func (rs *runeSet) insertPage(page runePage, pos int) {
	// insert in slice
	*rs = append(*rs, runePage{})
	copy((*rs)[pos+1:], (*rs)[pos:])
	(*rs)[pos] = page
}

// Add adds `r` to the rune set.
func (rs *runeSet) Add(r rune) {
	leaf := rs.findOrCreatePage(uint16(r >> 8))
	b := &leaf[(r&0xff)>>5] // (r&0xff)>>5 is the index in the page
	*b |= (1 << (r & 0x1f)) // r & 0x1f is the bit in the uint32
}
