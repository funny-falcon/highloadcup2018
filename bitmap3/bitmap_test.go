package bitmap3_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/funny-falcon/highloadcup2018/bitmap3"
)

func TestBitmap_simple(t *testing.T) {
	mp := bitmap3.Bitmap{}
	mp.Set(1)
	mp.Set(2)
	mp.Set(20000)
	mp.Set(1020000)
	ids := unroll(&mp)
	assert.Equal(t, []int32{1020000, 20000, 2, 1}, ids)
	assert.True(t, mp.Has(1))
	assert.True(t, mp.Has(2))
	assert.True(t, mp.Has(20000))
	assert.True(t, mp.Has(1020000))
	assert.Equal(t, uint32(4), mp.Count())
	mp.Unset(20000)
	assert.False(t, mp.Has(20000))
	assert.Equal(t, uint32(3), mp.Count())
}

func TestBitmap_huge(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gens := []func() int32{
		func() int32 {
			return rng.Int31n(512)
		},
		func() int32 {
			return rng.Int31n(1 << 16)
		},
		func() int32 {
			return rng.Int31n(1 << 20)
		},
	}
	for m := 3; m < 10000; m = m*2 + 1 {
		for _, gen := range gens {
			for k := 2; k < 6; k++ {
				maps := make([]bitmap3.Bitmap, k)
				dumb := make([]dumpmap, k)
				imaps := make([]bitmap3.IBitmap, k)
				for j := 0; j < k; j++ {
					for i := 0; i < m; i++ {
						v := gen()
						maps[j].Set(v)
						dumb[j].Set(v)
					}
					imaps[j] = &maps[j]
				}
				for j := 0; j < k; j++ {
					equal(t, dumb[j], imaps[j])
				}
				andDumb := intersectDumb(dumb)
				orDumb := unionDumb(dumb)
				andMap := bitmap3.NewAndBitmap(imaps)
				orMap := bitmap3.NewOrBitmap(imaps, nil)
				equal(t, andDumb, andMap)
				equal(t, orDumb, orMap)

				if k >= 3 {
					andOrDumb := intersectDumb([]dumpmap{unionDumb(dumb[:2]), unionDumb(dumb[2:])})
					andOrMap := bitmap3.NewAndBitmap([]bitmap3.IBitmap{
						bitmap3.NewOrBitmap(imaps[:2], nil),
						bitmap3.NewOrBitmap(imaps[2:], nil),
					})
					equal(t, andOrDumb, andOrMap)
				}
			}
		}
	}
}

func equal(t *testing.T, dmb dumpmap, mp bitmap3.IBitmap) {
	if !assert.Equal(t, dmb.Count(), bitmap3.Count(mp)) {
		panic("no")
	}
	require.Equal(t, dmb.Array(), unroll(mp))
}

func unroll(mp bitmap3.IBitmap) []int32 {
	ids := make([]int32, 0, 4)
	bitmap3.Loop(mp, func(i []int32) bool {
		ids = append(ids, i...)
		return true
	})
	return ids
}

type dumpmap struct{ m map[int32]struct{} }

func (d *dumpmap) Set(ix int32) {
	if d.m == nil {
		d.m = make(map[int32]struct{}, 1)
	}
	d.m[ix] = struct{}{}
}
func (d dumpmap) Unset(ix int32) {
	delete(d.m, ix)
}
func (d dumpmap) Has(ix int32) bool {
	_, ok := d.m[ix]
	return ok
}
func (d dumpmap) Count() uint32 {
	return uint32(len(d.m))
}
func (d dumpmap) Array() []int32 {
	res := make([]int32, 0, len(d.m))
	for i := range d.m {
		res = append(res, i)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i] > res[j]
	})
	return res
}
func unionDumb(dmb []dumpmap) dumpmap {
	var res dumpmap
	for _, m := range dmb {
		for k := range m.m {
			res.Set(k)
		}
	}
	return res
}
func intersectDumb(dmb []dumpmap) dumpmap {
	var res dumpmap
	for k := range dmb[0].m {
		res.Set(k)
	}
	for _, m := range dmb[1:] {
		for k := range res.m {
			if !m.Has(k) {
				res.Unset(k)
			}
		}
	}
	return res
}
