package bitmap2

type IMutBitmap interface {
	Set(id int32)
	Unset(id int32)
	IBitmap
}

type IBitmap interface {
	Iterator() (Iterator, int32)
}

type IBitmapSizer interface {
	IBitmap
	ISizer
}

type ISizer interface {
	GetSize() uint32
}

type Iterator interface {
	FetchAndNext(span int32) (*Block, int32)
}

type BitmapSpan struct {
	Span  uint8
	Dense bool
	Size  uint32
	Bits  []uint16
}

type Bitmap struct {
	B    []BitmapSpan
	Size uint32
}

func (b *Bitmap) GetSize() uint32 {
	return b.Size
}

func (b *Bitmap) Set(id int32) {
	bigspan := uint8(id >> 16)
	for i := range b.B {
		sp := &b.B[i]
		if sp.Span == bigspan {
			if sp.Set(id) {
				b.Size++
			}
			return
		} else if sp.Span < bigspan {
			b.B = append(b.B, BitmapSpan{})
			copy(b.B[i+1:], b.B[i:])
			b.B[i] = BitmapSpan{Span: bigspan}
			b.B[i].Set(id)
			b.Size++
		}
	}
	b.B = append(b.B, BitmapSpan{Span: bigspan})
	b.B[len(b.B)-1].Set(id)
	b.Size++
}

func (b *Bitmap) Unset(id int32) {
	bigspan := uint8(id >> 16)
	for i := range b.B {
		sp := &b.B[i]
		if sp.Span == bigspan {
			if sp.Unset(id) {
				if sp.Size == 0 {
					copy(b.B[i:], b.B[i+1:])
					b.B[len(b.B)-1] = BitmapSpan{}
					b.B = b.B[:len(b.B)-1]
				}
				b.Size--
			}
			return
		} else if sp.Span < bigspan {
			return
		}
	}
}

func (b *Bitmap) LastSpan() int32 {
	if len(b.B) == 0 {
		return NoNext
	}
	return b.B[0].LastSpan()
}

func (b *Bitmap) Iterator() (Iterator, int32) {
	if b.Size == 0 {
		return EmptyIt, NoNext
	}
	return &BitmapIterator{B: b}, b.LastSpan()
}

func (b *Bitmap) MyIterator() *BitmapIterator {
	return &BitmapIterator{B: b}
}

func (sp *BitmapSpan) Set(id int32) bool {
	sid := uint16(id)
	if !sp.Dense {
		i := searchSparse16(sp.Bits, sid)
		if i < len(sp.Bits) && sp.Bits[i] == sid {
			return false
		}
		sp.Size++
		if i == len(sp.Bits) {
			sp.Bits = append(sp.Bits, sid)
		} else if sp.Bits[i] < sid {
			sp.Bits = append(sp.Bits, 0)
			copy(sp.Bits[i+1:], sp.Bits[i:])
			sp.Bits[i] = sid
		}
		if cap(sp.Bits) >= (1<<16)/16 {
			old := sp.Bits
			sp.Bits = make([]uint16, (1<<16)/16)
			sp.Size = 0
			sp.Dense = true
			for _, sid := range old {
				sp.Set(int32(sid))
			}
		}
		return true
	} else {
		o, b := sid>>3, uint8(1)<<(sid&7)
		p := aref8(ptr0_16(sp.Bits), int(o))
		r := *p&b == 0
		if r {
			*p |= b
			sp.Size++
		}
		return r
	}
}

func (sp *BitmapSpan) Unset(id int32) bool {
	sid := uint16(id)
	if !sp.Dense {
		i := searchSparse16(sp.Bits, sid)
		if i >= len(sp.Bits) || sp.Bits[i] != sid {
			return false
		}
		if sp.Size == 1 {
			sp.Bits = nil
		} else {
			copy(sp.Bits[i:], sp.Bits[i+1:])
			sp.Bits = sp.Bits[:sp.Size-1]
			if len(sp.Bits) < cap(sp.Bits)/2-cap(sp.Bits)/8 {
				old := sp.Bits
				sp.Bits = make([]uint16, len(sp.Bits), cap(sp.Bits)/2)
				copy(sp.Bits, old)
			}
		}
		sp.Size--
		return true
	} else {
		o, b := sid>>3, uint8(1)<<(sid&7)
		p := aref8(ptr0_16(sp.Bits), int(o))
		r := *p&b != 0
		if r {
			*p &^= b
			sp.Size--
		}
		return r
	}
}

func (sp *BitmapSpan) FetchAndNext(bl *Block, sid uint16, ix int) (*Block, int) {
	p := ptr0_16(sp.Bits)
	if !sp.Dense {
		l := len(sp.Bits)
		if ix >= l {
			return &ZeroBlock, -1
		}
		if *aref16(p, ix) > sid+BlockMask {
			ix += 1 + searchSparse16(sp.Bits[ix+1:], sid+BlockMask)
		}
		if ix >= l {
			return &ZeroBlock, -1
		}
		*bl = ZeroBlock
		for ; ix < l; ix++ {
			cur := *aref16(p, ix)
			if cur < sid {
				break
			}
			bl.Set(int32(cur))
		}
		if ix >= l {
			return bl, -1
		} else {
			return bl, ix
		}
	} else {
		if sid > 0 {
			return arefBlock(p, int(sid/BlockSize)), ix + 1
		} else {
			return arefBlock(p, int(sid/BlockSize)), -1
		}
	}
}

func (sp *BitmapSpan) LastSpan() int32 {
	if !sp.Dense {
		return int32(sp.Span)<<16 + int32(sp.Bits[0])&^BlockMask
	} else {
		return int32(sp.Span+1)<<16 - BlockSize
	}
}

func (sp *BitmapSpan) GetSpan(ix int) int32 {
	if !sp.Dense {
		return int32(sp.Span)<<16 + int32(sp.Bits[ix])&^BlockMask
	} else {
		return int32(sp.Span+1)<<16 - int32(ix+1)*BlockSize
	}
}

type BitmapIterator struct {
	B  *Bitmap
	Sp int
	Ix int
	Bl Block
}

func (bi *BitmapIterator) FetchAndNext(span int32) (*Block, int32) {
	bigspan := uint8(span >> 16)
	B := bi.B.B
	for ; bi.Sp < len(B) && B[bi.Sp].Span > bigspan; bi.Sp++ {
		bi.Ix = 0
	}
	if bi.Sp >= len(B) {
		return &ZeroBlock, NoNext
	} else if B[bi.Sp].Span < bigspan {
		return &ZeroBlock, B[bi.Sp].LastSpan()
	}
	bl, ix := B[bi.Sp].FetchAndNext(&bi.Bl, uint16(span&^BlockMask), bi.Ix)
	if ix == -1 {
		bi.Sp++
		bi.Ix = 0
		if bi.Sp >= len(B) {
			return bl, NoNext
		}
		return bl, B[bi.Sp].LastSpan()
	} else {
		bi.Ix = ix
		return bl, B[bi.Sp].GetSpan(ix)
	}
}
