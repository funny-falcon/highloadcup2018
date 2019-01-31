package main

import (
	"math/bits"

	"github.com/funny-falcon/highloadcup2018/bitmap3"
)

type InterestBlock [16]uint8
type InterestMask [4]uint32

var Interests = make([]InterestBlock, Init)

func GetInterest(i int32) *InterestBlock {
	return &Interests[i]
}

func (bl *InterestBlock) Set(ix uint8) {
	for j, intr := range bl {
		if intr == uint8(ix) {
			return
		}
		if intr == 0 {
			bl[j] = uint8(ix)
			return
		}
	}
	panic("interests overflow")
}

func SetInterest(i int32, ix uint8) {
	Interests[i].Set(ix)
}

func SetInterests(i int32, b InterestBlock) {
	Interests[i] = b
}

func (i *InterestBlock) Mask() InterestMask {
	var mi InterestMask
	for _, intr := range i {
		bitmap3.Set(mi[:], int32(intr))
	}
	mi[0] &^= 1
	return mi
}

func (mi InterestMask) IntersectCount(mo InterestMask) uint32 {
	return uint32(bits.OnesCount32(mi[0]&mo[0]) +
		bits.OnesCount32(mi[1]&mo[1]) +
		bits.OnesCount32(mi[2]&mo[2]) +
		bits.OnesCount32(mi[3]&mo[3]))
}
