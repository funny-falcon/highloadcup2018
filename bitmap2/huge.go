package bitmap2

import (
	"math/bits"
)

type Huge struct {
	Size uint32
	p    uintptr
	B    []uint8
}

func (h *Huge) GetSize() uint32 {
	return h.Size
}

func (h *Huge) Set(i int32) {
	k := int(i >> 3)
	if k >= len(h.B) {
		l := (k + BlockLen) &^ (BlockLen - 1)
		if l >= cap(h.B) {
			c := 1 << uint(bits.Len32(uint32(l)))
			if c-c/4 > l {
				c -= c / 4
			}
			b := make([]uint8, l, c)
			copy(b, h.B)
			h.B = b
			h.p = ptr0_8(h.B)
		}
		h.B = h.B[:l]
	}
	p := aref8(h.p, k)
	bit := uint8(1) << uint32(i&7)
	if *p&bit == 0 {
		*p |= bit
		h.Size++
	}
}

func (h *Huge) Unset(i int32) {
	k := int(i >> 3)
	if k >= len(h.B) {
		return
	}
	p := aref8(h.p, k)
	bit := uint8(1) << uint32(i&7)
	if *p&bit != 0 {
		*p &^= bit
		h.Size--
	}
}

func (h *Huge) Has(i int32) bool {
	k := int(i >> 3)
	if k >= len(h.B) {
		return false
	}
	p := aref8(h.p, k)
	bit := uint8(1) << uint32(i&7)
	return *p&bit != 0
}

func (h *Huge) LastSpan() int32 {
	return int32((len(h.B)<<3)-1) &^ BlockMask
}

func (h *Huge) Iterator() (Iterator, int32) {
	if h.Size == 0 {
		return EmptyIt, NoNext
	}
	return h, h.LastSpan()
}

func (h *Huge) Reset() {}

func (h *Huge) FetchAndNext(span int32) (*Block, int32) {
	k := int((span >> 3) &^ (BlockLen - 1))
	if k >= len(h.B) {
		return &ZeroBlock, h.LastSpan()
	}
	last := span - BlockSize
	if k > BlockLen && arefBlock(h.p, k/BlockLen-1).Empty() {
		last -= BlockSize
		/*
			if k > 2*BlockLen && arefBlock(h.p, k/BlockLen-2).Empty() {
				last -= BlockSize
			}
		*/
	}
	return arefBlock(h.p, k/BlockLen), last
}

func (h *Huge) Loop(f func([]int32) bool) {
	var un BlockUnroll
	for k := len(h.B) - BlockLen; k >= 0; k -= BlockLen {
		span := int32(k * 8)
		bl := arefBlock(h.p, k/BlockLen)
		if !bl.Empty() && !f(bl.Unroll(span, &un)) {
			break
		}
	}
}

/*
func And2Huge(a, b *Huge) *Huge {
	r := &Huge{}
	l := len(a.B)
	if len(b.B) < l {
		l = len(b.B)
	}
	r.B = make([]byte, l)
	r.p = ptr0_8(r.B)
	l /= BlockLen
	rp, ap, bp := r.p, a.p, b.p
	for i := 0; i < l; i++ {
		bl := arefBlock(ap, i).IntersectNew(arefBlock(bp, i))
		*arefBlock(rp, i) = bl
		r.Size += bl.Count()
	}
	return r
}
*/

func And2Huge(a, b *Huge) *Materialized {
	r := &Materialized{}
	l := len(a.B)
	if len(b.B) < l {
		l = len(b.B)
	}
	l /= BlockLen
	r.B = make([]MElem, l, l)
	ap, bp := a.p, b.p
	k := 0
	for l > 0 {
		l--
		bl := arefBlock(ap, l).IntersectNew(arefBlock(bp, l))
		if !bl.Empty() {
			r.B[k].B = bl
			r.B[k].Span = int32(l * BlockSize)
			r.Size += bl.Count()
			k++
		}
	}
	r.B = r.B[:k]
	return r
}
