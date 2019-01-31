package main

import (
	"math/bits"

	"github.com/funny-falcon/highloadcup2018/bitmap3"
)

type InterestBlock [16]uint8
type InterestMask [2]uint64

var Interests = make([]InterestMask, Init)

func GetInterest(i int32) *InterestMask {
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

func (bl *InterestMask) Set(ix uint8) {
	bitmap3.Set(bl[:], int32(ix))
}

func SetInterest(i int32, ix uint8) {
	bitmap3.Set(Interests[i][:], int32(ix))
}

func SetInterests(i int32, b InterestMask) {
	Interests[i] = b
}

/*
func (i *InterestBlock) Mask() InterestMask {
	var mi InterestMask
	for _, intr := range i {
		bitmap3.Set(mi[:], int32(intr))
	}
	mi[0] &^= 1
	return mi
}
*/
func (mi InterestMask) Unroll(f func(int32)) {
	var un bitmap3.Unrolled
	for _, ix := range bitmap3.Unroll(mi[0], 0, &un) {
		f(ix)
	}
	for _, ix := range bitmap3.Unroll(mi[1], 64, &un) {
		f(ix)
	}
}

func (mi InterestMask) IntersectCount(mo InterestMask) uint32 {
	return uint32(bits.OnesCount64(mi[0]&mo[0]) +
		bits.OnesCount64(mi[1]&mo[1]))
}
