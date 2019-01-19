package bitmap2_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/funny-falcon/highloadcup2018/bitmap2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitmap(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	gen := func() bitmap2.IMutBitmap { return new(bitmap2.Bitmap) }
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, gen, 120, func() int32 { return rnd.Int31n(1 << 7) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 9) })
	testIt(t, gen, 1000, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, gen, 60000, func() int32 { return rnd.Int31n(1 << 17) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 8) })
	testIt(t, gen, 800, func() int32 {
		n := rnd.Int31n(1 << 10)
		n = n%256 + n/256*911
		return n
	})
}

func testIt(t *testing.T, genBm func() bitmap2.IMutBitmap, maxcap int, gen func() int32) {
	for k := 1; k <= maxcap; k += k/4 + 1 {
		sm := genBm()
		var set dumbSet
		set.generate(k, gen)
		for _, v := range set {
			sm.Set(v)
		}

		nset := dumbFromIter(sm)
		sort.Sort(set)
		sort.Sort(nset)

		if !assert.Equal(t, set, nset) {
			panic("no")
		}

		set.shuffle()
		for i := k / 4; i >= 0; i-- {
			sm.Unset(set[i])
		}
		set = set[k/4+1:]

		nset = dumbFromIter(sm)
		sort.Sort(set)
		sort.Sort(nset)

		require.Equal(t, set, nset)
	}
}

type dumbSet []int32

func (ds *dumbSet) generate(k int, gen func() int32) {
	set := make([]int32, 0, k)
	for len(set) < k {
		v := gen()
		ix := sort.Search(len(set), func(i int) bool {
			return v >= set[i]
		})
		if ix < len(set) && set[ix] == v {
			continue
		}
		set = append(set, 0)
		copy(set[ix+1:], set[ix:])
		set[ix] = v
	}
	*ds = set
}

func dumbFromIter(it bitmap2.IBitmap) dumbSet {
	set := dumbSet{}
	bitmap2.LoopMap(it, func(u []int32) bool {
		if len(set) > 0 && u[0] > set[len(set)-1] {
			panic("no")
		}
		set = append(set, u...)
		return true
	})
	return set
}

func (ds dumbSet) shuffle() {
	rand.Shuffle(len(ds), ds.Swap)
}

func (ds dumbSet) Len() int           { return len(ds) }
func (ds dumbSet) Less(i, j int) bool { return ds[i] > ds[j] }
func (ds dumbSet) Swap(i, j int)      { ds[i], ds[j] = ds[j], ds[i] }

func (ds dumbSet) Bitmap() bitmap2.IBitmapSizer {
	sort.Sort(ds)
	return dumbMap(ds)
}

type dumbMap []int32

func (ds dumbMap) GetSize() uint32 {
	return uint32(len(ds))
}

func (ds dumbMap) Iterator() (bitmap2.Iterator, int32) {
	return ds, ds.LastSpan()
}

func (ds dumbMap) LastSpan() int32 {
	if len(ds) == 0 {
		return bitmap2.NoNext
	}
	return ds[0] &^ bitmap2.BlockMask
}

func (ds dumbMap) Reset() {}

func (ds dumbMap) FetchAndNext(span int32) (*bitmap2.Block, int32) {
	var block bitmap2.Block
	if span < 0 {
		return &block, bitmap2.NoNext
	}
	ix := sort.Search(len(ds), func(i int) bool {
		return span > ds[i]
	})
	for i := ix - 1; i >= 0 && ds[i] < span+bitmap2.BlockSize; i-- {
		block.Set(ds[i])
	}
	if ix == len(ds) {
		return &block, bitmap2.NoNext
	}
	return &block, ds[ix] &^ bitmap2.BlockMask
}

func (ds dumbSet) Intersect(other dumbSet) dumbSet {
	res := dumbSet{}
	i, j := 0, 0
	for i < len(ds) && j < len(other) {
		if ds[i] == other[j] {
			res = append(res, ds[i])
			i++
			j++
		} else if ds[i] > other[j] {
			i++
		} else {
			j++
		}
	}
	return res
}

func (ds dumbSet) Union(other dumbSet) dumbSet {
	res := dumbSet{}
	i, j := 0, 0
	for i < len(ds) && j < len(other) {
		if ds[i] == other[j] {
			res = append(res, ds[i])
			i++
			j++
		} else if ds[i] > other[j] {
			res = append(res, ds[i])
			i++
		} else {
			res = append(res, other[j])
			j++
		}
	}
	res = append(res, ds[i:]...)
	res = append(res, other[j:]...)
	return res
}
