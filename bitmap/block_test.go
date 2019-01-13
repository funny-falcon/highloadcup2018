package bitmap_test

import (
	"testing"

	"github.com/funny-falcon/highloadcup2018/bitmap"
	"github.com/stretchr/testify/require"
)

func TestBlock(t *testing.T) {
	var bl bitmap.Block

	bl.Set(13)
	bl.Set(24)
	bl.Set(133)

	var r [256]int32
	require.Equal(t, []int32{133, 24, 13}, bl.Unroll(0, &r))
	require.Equal(t, []int32{1133, 1024, 1013}, bl.Unroll(1000, &r))
}

var resunroll [256]int32

func BenchmarkBlock_Unroll(b *testing.B) {
	benchmarkBlockUnroll(b, (*bitmap.Block).Unroll)
}

func benchmarkBlockUnroll(b *testing.B, unr func(b *bitmap.Block, span int32, res *[256]int32) []int32) {
	var rv rng
	rv.next()
	rs := rng(rv.next())
	rr := rng(rv.next())
	for i := 0; i < b.N; i += 10 {
		var bl bitmap.Block
		for k := range bl {
			v, s, r := rv.next(), rs.next()%31, rr.next()&15
			v &= (^uint32(0)) >> s
			v = v<<r | v>>(32-r)
			bl[k] = v
		}
		for j := 0; j < 10; j++ {
			unr(&bl, int32(i), &resunroll)
		}
	}
}

type rng uint32

func (r *rng) next() uint32 {
	k := *r
	*r = k*5 + 1
	k *= 0x53215995
	return uint32(k ^ (k<<7 | k>>25) ^ (k<<13 | k>>19))
}
