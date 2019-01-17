package bitmap2

import (
	"math/bits"
	"unsafe"
)

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
		*arefInt32(r, k) = sp
		*arefInt32(r, k+1) = sp - 1
		k += 2
	case 2:
		*arefInt32(r, k) = sp
		k++
	case 1:
		*arefInt32(r, k) = sp - 1
		k++
	case 0:
	}
	return k
}

func arefInt32(p uintptr, i int) *int32 {
	return (*int32)(unsafe.Pointer(p + uintptr(i)*4))
}

func arefBlock(p uintptr, i int) *Block {
	return (*Block)(unsafe.Pointer(p + uintptr(i)*BlockLen))
}
