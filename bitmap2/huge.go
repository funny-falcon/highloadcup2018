package bitmap2

import "math/bits"

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
	return arefBlock(h.p, k/BlockLen), span - BlockSize
}
