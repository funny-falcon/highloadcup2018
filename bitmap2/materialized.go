package bitmap2

type Materialized struct {
	Size uint32
	B    []MElem
}

type MElem struct {
	Span int32
	B    Block
}

func Materialize(bm IBitmap) *Materialized {
	m := &Materialized{}

	it, last := bm.Iterator()
	for last >= 0 {
		block, next := it.FetchAndNext(last)
		if !block.Empty() {
			m.B = append(m.B, MElem{last, *block})
			m.Size += block.Count()
		}
		if next >= last {
			panic("no")
		}
		last = next
	}
	return m
}

func (m *Materialized) GetSize() uint32 {
	return m.Size
}

func (m *Materialized) LastSpan() int32 {
	if m.Size == 0 {
		return NoNext
	}
	return m.B[0].Span
}

func (m *Materialized) Iterator() (Iterator, int32) {
	if m.Size == 0 {
		return EmptyIt, NoNext
	}
	return &MatIterator{B: m}, m.LastSpan()
}

type MatIterator struct {
	B  *Materialized
	Ix int
}

func (mi *MatIterator) FetchAndNext(span int32) (*Block, int32) {
	bl := &ZeroBlock
	ix := mi.Ix
	l := len(mi.B.B)
	if ix >= l {
		return bl, NoNext
	}
	for ; ix < l && mi.B.B[ix].Span > span; ix++ {
	}
	if ix < l && mi.B.B[ix].Span == span {
		bl = &mi.B.B[ix].B
		ix++
	}
	mi.Ix = ix
	if ix == l {
		return bl, NoNext
	}
	return bl, mi.B.B[ix].Span
}

func (mi *MatIterator) Loop(f func(u []int32) bool) {
	var indx BlockUnroll
	for _, el := range mi.B.B {
		if !f(el.B.Unroll(el.Span, &indx)) {
			break
		}
	}
}
