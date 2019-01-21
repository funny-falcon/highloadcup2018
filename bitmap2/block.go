package bitmap2

import (
	"math/bits"
	"unsafe"
)

//*
const BlockSize = 128
const BlockLen = BlockSize / 8
const BlockMask = BlockSize - 1
const NoNext = -BlockSize

type BlockUnroll [BlockSize]int32

type Block [2]uint64

const all = ^uint64(0)

var AllBlock = Block{all, all}
var ZeroBlock = Block{}

func (b *Block) Set(i int32) bool {
	r := b.Has(i)
	b[(i>>6)&1] |= 1 << byte(i&63)
	return !r
}

func (b *Block) Unset(i int32) bool {
	r := b.Has(i)
	b[(i>>6)&1] &^= 1 << byte(i&63)
	return r
}

func (b *Block) Has(i int32) bool {
	return b[(i>>6)&1]&(1<<byte(i&63)) != 0
}

func (b *Block) Empty() bool {
	return b[0]|b[1] == 0
}

func (b *Block) Count() uint32 {
	return uint32(bits.OnesCount64(b[0]) + bits.OnesCount64(b[1]))
}

func (b Block) CountV() uint32 {
	return uint32(bits.OnesCount64(b[0]) + bits.OnesCount64(b[1]))
}

func (b *Block) Intersect(o *Block) {
	b[0] &= o[0]
	b[1] &= o[1]
}

func (b *Block) IntersectNew(o *Block) Block {
	return Block{b[0] & o[0], b[1] & o[1]}
}

func (b *Block) RemoveNew(o *Block) Block {
	return Block{b[0] &^ o[0], b[1] &^ o[1]}
}

func (b *Block) Union(o *Block) {
	b[0] |= o[0]
	b[1] |= o[1]
}

func (b Block) Unroll(span int32, r *BlockUnroll) []int32 {
	p := uintptr(unsafe.Pointer(r))
	rp := p
	p = unroll32(uint32(b[1]>>32), span+127, p)
	p = unroll32(uint32(b[1]), span+95, p)
	p = unroll32(uint32(b[0]>>32), span+63, p)
	p = unroll32(uint32(b[0]), span+31, p)
	return r[:(p-rp)/4]
}

func unroll32(v uint32, sp int32, r uintptr) uintptr {
	if v&0xffff0000 == 0 {
		v <<= 16
		sp -= 16
	}
	for ; v != 0; v <<= 8 {
		*aref32(r, 0) = sp
		r += uintptr((v >> 29) & 4)
		*aref32(r, 0) = sp - 1
		r += uintptr((v >> 28) & 4)
		*aref32(r, 0) = sp - 2
		r += uintptr((v >> 27) & 4)
		*aref32(r, 0) = sp - 3
		r += uintptr((v >> 26) & 4)
		*aref32(r, 0) = sp - 4
		r += uintptr((v >> 25) & 4)
		*aref32(r, 0) = sp - 5
		r += uintptr((v >> 24) & 4)
		*aref32(r, 0) = sp - 6
		r += uintptr((v >> 23) & 4)
		*aref32(r, 0) = sp - 7
		r += uintptr((v >> 22) & 4)
		sp -= 8
		/*
			switch v >> 30 {
			case 3:
				*aref32(r, k) = sp
				*aref32(r, k+1) = sp - 1
				k += 2
			case 2:
				*aref32(r, k) = sp
				k++
			case 1:
				*aref32(r, k) = sp - 1
				k++
			case 0:
			}
			switch (v >> 28) & 3 {
			case 3:
				*aref32(r, k) = sp - 2
				*aref32(r, k+1) = sp - 3
				k += 2
			case 2:
				*aref32(r, k) = sp - 2
				k++
			case 1:
				*aref32(r, k) = sp - 3
				k++
			case 0:
			}
			sp -= 4
		*/
	}
	return r
}

func (b Block) UnrollCount(r *BlockUnroll) {
	p := uintptr(unsafe.Pointer(r))
	unrollCount32(uint32(b[1]>>32), p+96*4)
	unrollCount32(uint32(b[1]), p+64*4)
	unrollCount32(uint32(b[0]>>32), p+32*4)
	unrollCount32(uint32(b[0]), p)
}

func unrollCount32(v uint32, r uintptr) {
	if v&0xffff == 0 {
		v >>= 16
		r += 64
	}
	for ; v != 0; v >>= 8 {
		*aref32(r, 0) += int32(v & 1)
		*aref32(r, 1) += int32((v >> 1) & 1)
		*aref32(r, 2) += int32((v >> 2) & 1)
		*aref32(r, 3) += int32((v >> 3) & 1)
		*aref32(r, 4) += int32((v >> 4) & 1)
		*aref32(r, 5) += int32((v >> 5) & 1)
		*aref32(r, 6) += int32((v >> 6) & 1)
		*aref32(r, 7) += int32((v >> 7) & 1)
		r += 32
	}
}

//*/

/*
const BlockSize = 256
const BlockLen = BlockSize / 8
const BlockMask = BlockSize - 1
const NoNext = -BlockSize

type BlockUnroll [BlockSize]int32

type Block [8]uint32

const all = ^uint32(0)

var AllBlock = Block{all, all, all, all, all, all, all, all}
var ZeroBlock = Block{}

func (b *Block) Set(i int32) bool {
	r := b.Has(i)
	b[(i>>5)&7] |= 1 << byte(i&31)
	return !r
}

func (b *Block) Unset(i int32) bool {
	r := b.Has(i)
	b[(i>>5)&7] &^= 1 << byte(i&31)
	return r
}

func (b *Block) Has(i int32) bool {
	return b[(i>>5)&7]&(1<<byte(i&31)) != 0
}

func (b *Block) Empty() bool {
	return b[0]|b[1]|b[2]|b[3]|b[4]|b[5]|b[6]|b[7] == 0
}

func (b *Block) Count() uint32 {
	sum := 0
	for _, v := range b {
		sum += bits.OnesCount32(v)
	}
	return uint32(sum)
}

func (b *Block) Intersect(o *Block) {
	b4 := (*[4]uint64)(unsafe.Pointer(b))
	o4 := (*[4]uint64)(unsafe.Pointer(o))
	b4[0] &= o4[0]
	b4[1] &= o4[1]
	b4[2] &= o4[2]
	b4[3] &= o4[3]
}

func (b *Block) Union(o *Block) {
	b4 := (*[4]uint64)(unsafe.Pointer(b))
	o4 := (*[4]uint64)(unsafe.Pointer(o))
	b4[0] |= o4[0]
	b4[1] |= o4[1]
	b4[2] |= o4[2]
	b4[3] |= o4[3]
}

func (b *Block) Unroll(span int32, r *BlockUnroll) []int32 {
	k := uint32(0)
	span += 256
	for j := 7; j >= 0; j-- {
		v := b[j]
		span -= 32
		if v == 0 {
			continue
		}
		sp := span + 31
		for ; v != 0; v <<= 4 {
			k = unroll2(v>>30, sp, k, r)
			k = unroll2((v>>28)&3, sp-2, k, r)
			sp -= 4
		}
	}
	return r[:k]
}

func unroll2(v uint32, sp int32, k uint32, r *BlockUnroll) uint32 {
	switch v {
	case 3:
		r[k] = sp
		r[k+1] = sp - 1
		k += 2
	case 2:
		r[k] = sp
		k++
	case 1:
		r[k] = sp - 1
		k++
	case 0:
	}
	return k
}

//*/

/*
const BlockSize = 64
const BlockLen = BlockSize / 8
const BlockMask = BlockSize - 1
const NoNext = -BlockSize

type Block uint64
type BlockUnroll [BlockSize]int32

var ZeroBlock Block
var AllBlock = ^Block(0)

func (b *Block) Set(id int32) {
	*b |= 1 << uint32(id&BlockMask)
}

func (b *Block) Union(o *Block) {
	*b |= *o
}

func (b *Block) Intersect(o *Block) {
	*b &= *o
}

func (b Block) Empty() bool {
	return b == 0
}

func (b Block) Count() uint32 {
	return uint32(bits.OnesCount64(uint64(b)))
}

func (b Block) Unroll(span int32, u *BlockUnroll) []int32 {
	k := 0
	if b == 0 {
		return nil
	}
	span += 63
	if b < 1<<32 {
		b <<= 32
		span -= 32
	}
	ptr := uintptr(unsafe.Pointer(u))
	for ; b != 0; b <<= 4 {
		k = unroll2(b>>62, span, k, ptr)
		k = unroll2((b>>60)&3, span-2, k, ptr)
		span -= 4
	}
	return u[:k]
}

func unroll2(v Block, sp int32, k int, r uintptr) int {
	switch v {
	case 3:
		*aref32(r, k) = sp
		*aref32(r, k+1) = sp - 1
		k += 2
	case 2:
		*aref32(r, k) = sp
		k++
	case 1:
		*aref32(r, k) = sp - 1
		k++
	case 0:
	}
	return k
}

//*/

func arefBlock(p uintptr, i int) *Block {
	return (*Block)(unsafe.Pointer(p + uintptr(i)*BlockLen))
}
