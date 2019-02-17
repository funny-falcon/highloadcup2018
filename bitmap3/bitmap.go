package bitmap3

import (
	"math/bits"
	"unsafe"
)

type Bitmap struct {
	Size uint32
	L2   [384]uint64

	L2Ptr [384]*[64]uint64
}

const UpLimit = (1 << 20) | (1 << 19)

func (bm *Bitmap) Set(ix int32) {
	p := ix / (64 * 64)
	x := bm.L2[p]
	lptr := bm.L2Ptr[p]
	bit := uint64(1) << uint32((ix/64)&63)
	if x&bit == 0 {
		x |= bit
		bm.L2[p] = x
		bm.Size++
		if lptr == nil {
			nlptr := make([]uint64, 1)
			Set(nlptr[:1], ix&63)
			bm.L2Ptr[p] = (*[64]uint64)(unsafe.Pointer(&nlptr[0]))
			return
		}
		k := bits.OnesCount64(x)
		nlptr := make([]uint64, k)
		i := bits.OnesCount64(x & (bit - 1))
		copy(nlptr[:i], lptr[:i])
		copy(nlptr[i+1:k], lptr[i:k-1])
		Set(nlptr[i:i+1], ix&63)
		bm.L2Ptr[p] = (*[64]uint64)(unsafe.Pointer(&nlptr[0]))
	} else {
		i := bits.OnesCount64(x & (bit - 1))
		if Set(lptr[i:i+1], ix&63) {
			bm.Size++
		}
	}
}

func (bm *Bitmap) SetBlock(ix int32, bl uint64) {
	p := ix / (64 * 64)
	x := bm.L2[p]
	lptr := bm.L2Ptr[p]
	bit := uint64(1) << uint32((ix/64)&63)
	if x&bit == 0 {
		x |= bit
		bm.L2[p] = x
		bm.Size += uint32(bits.OnesCount64(bl))
		if lptr == nil {
			nlptr := make([]uint64, 1)
			nlptr[0] = bl
			bm.L2Ptr[p] = (*[64]uint64)(unsafe.Pointer(&nlptr[0]))
			return
		}
		k := bits.OnesCount64(x)
		nlptr := make([]uint64, k)
		i := bits.OnesCount64(x & (bit - 1))
		copy(nlptr[:i], lptr[:i])
		copy(nlptr[i+1:k], lptr[i:k-1])
		nlptr[i] = bl
		bm.L2Ptr[p] = (*[64]uint64)(unsafe.Pointer(&nlptr[0]))
	} else {
		panic("NO")
	}
}

func (bm *Bitmap) Unset(ix int32) {
	p := ix / (64 * 64)
	x := bm.L2[p]
	lptr := bm.L2Ptr[p]
	bit := uint64(1) << uint32((ix/64)&63)
	if lptr == nil || x&bit == 0 {
		return
	}
	i := bits.OnesCount64(x & (bit - 1))
	if Unset(lptr[i:i+1], ix&63) {
		bm.Size--
	}
}

func (bm *Bitmap) Has(ix int32) bool {
	if ix > UpLimit {
		return false
	}
	p := ix / (64 * 64)
	x := bm.L2[p]
	bit := uint64(1) << uint32((ix/64)&63)
	if x&bit == 0 {
		return false
	}
	i := bits.OnesCount64(x & (bit - 1))
	lptr := bm.L2Ptr[p]
	return Has(lptr[:], int32(i*64)+(ix&63))
}

func (bm *Bitmap) LoopBlock(f func(int32, uint64) bool) {
	var l2u Unrolled
	for l2ix := int32(len(bm.L2) - 1); l2ix >= 0; l2ix-- {
		l2v := bm.L2[l2ix]
		if l2v == 0 {
			continue
		}
		l2ixb := l2ix * 64
		lptr := bm.L2Ptr[l2ix]
		k := bits.OnesCount64(l2v) - 1
		for i, l3ix := range Unroll(l2v, l2ixb, &l2u) {
			l3v := lptr[k-i]
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
	//return bm.L3[span/64]
	if span > UpLimit {
		return 0
	}
	p := span / (64 * 64)
	x := bm.L2[p]
	bit := uint64(1) << uint32((span/64)&63)
	if x&bit == 0 {
		return 0
	}
	i := bits.OnesCount64(x & (bit - 1))
	lptr := bm.L2Ptr[p]
	return lptr[i]
}

func (bm *Bitmap) Count() uint32 {
	return bm.Size
}
