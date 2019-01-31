package bitmap3

type Bitmap struct {
	Size uint32
	L2   [384]uint64
	L3   [24576]uint64
}

const UpLimit = (1 << 20) | (1 << 19)

func (bm *Bitmap) Set(ix int32) {
	if Set(bm.L3[:], ix) {
		bm.Size++
		Set(bm.L2[:], ix/64)
	}
}

func (bm *Bitmap) Unset(ix int32) {
	if Unset(bm.L3[:], ix) {
		bm.Size--
	}
}

func (bm *Bitmap) Has(ix int32) bool {
	return ix < UpLimit && Has(bm.L2[:], ix/64) && Has(bm.L3[:], ix)
}

func (bm *Bitmap) LoopBlock(f func(int32, uint64) bool) {
	var l2u Unrolled
	for l2ix := int32(len(bm.L2) - 1); l2ix >= 0; l2ix-- {
		l2v := bm.L2[l2ix]
		if l2v == 0 {
			continue
		}
		l2ixb := l2ix * 64
		for _, l3ix := range Unroll(l2v, l2ixb, &l2u) {
			l3v := bm.L3[l3ix]
			if l3v != 0 && !f(l3ix*64, l3v) {
				return
			}
		}
	}
}

func (bm *Bitmap) GetL2() *[384]uint64 {
	return &bm.L2
}

func (bm *Bitmap) GetBlock(span int32) uint64 {
	/*
		if !Has(bm.L1[:], span/(32*32)) {
			return 0
		}
		if !Has(bm.L2[:], span/32) {
			return 0
		}
	*/
	return bm.L3[span/64]
	//return *arefu32(ptr0_u32(bm.L3[:]), int(span/32))
}

func (bm *Bitmap) Count() uint32 {
	return bm.Size
}
