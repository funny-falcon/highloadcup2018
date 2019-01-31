package bitmap3

type Bitmap struct {
	Size uint32
	L1   [48]uint32
	L2   [1536]uint32
	L3   [49152]uint32
}

func (bm *Bitmap) Set(ix int32) {
	if Set(bm.L3[:], ix) {
		bm.Size++
		Set(bm.L2[:], ix/32)
		Set(bm.L1[:], ix/(32*32))
	}
}

func (bm *Bitmap) Unset(ix int32) {
	if Unset(bm.L3[:], ix) {
		bm.Size--
	}
}

func (bm *Bitmap) Has(ix int32) bool {
	return Has(bm.L1[:], ix/(32*32)) && Has(bm.L2[:], ix/32) && Has(bm.L3[:], ix)
}

func (bm *Bitmap) LoopBlock(f func(int32, uint32) bool) {
	var l1u, l2u Unrolled
	for l1ix := int32(len(bm.L1) - 1); l1ix >= 0; l1ix-- {
		l1v := bm.L1[l1ix]
		if l1v == 0 {
			continue
		}
		l1ixb := l1ix * 32
		for _, l2ix := range Unroll(l1v, l1ixb, &l1u) {
			l2v := bm.L2[l2ix]
			l2ixb := l2ix * 32
			for _, l3ix := range Unroll(l2v, l2ixb, &l2u) {
				l3v := bm.L3[l3ix]
				if l3v != 0 && !f(l3ix*32, l3v) {
					return
				}
			}
		}
	}
}

func (bm *Bitmap) GetL1() *[48]uint32 {
	return &bm.L1
}

func (bm *Bitmap) GetL2(l2ix int32) uint32 {
	if !Has(bm.L1[:], l2ix) {
		return 0
	}
	return bm.L2[l2ix]
}

func (bm *Bitmap) GetBlock(span int32) uint32 {
	if !Has(bm.L1[:], span/(32*32)) {
		return 0
	}
	if !Has(bm.L2[:], span/32) {
		return 0
	}
	return bm.L3[span/32]
}

func (bm *Bitmap) Count() uint32 {
	return bm.Size
}
