package bitmap_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/funny-falcon/highloadcup2018/alloc"
	"github.com/funny-falcon/highloadcup2018/bitmap"
)

func TestSmall(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	testIt(t, bitmap.SmallEmpty, 200, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, bitmap.SmallEmpty, 120, func() int32 { return rnd.Int31n(1 << 7) })
	testIt(t, bitmap.SmallEmpty, 200, func() int32 { return rnd.Int31n(1 << 9) })
}

func testIt(t *testing.T, b bitmap.Bitmap, maxcap int, gen func() int32) {
	var al alloc.Simple
	for k := 1; k <= maxcap; k += k/4 + 1 {
		sm := bitmap.Wrap(&al, nil, b)
		var set dumbSet
		set.generate(k, gen)
		for _, v := range set {
			sm.Set(v)
		}

		nset := dumbFromIter(sm.Iterator(1 << 20))
		sort.Sort(set)
		sort.Sort(nset)

		require.Equal(t, set, nset)

		set.shuffle()
		for i := k / 4; i >= 0; i-- {
			sm.Unset(set[i])
		}
		set = set[k/4+1:]

		nset = dumbFromIter(sm.Iterator(1 << 20))
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
			return v <= set[i]
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

func dumbFromIter(it bitmap.Iterator) dumbSet {
	set := dumbSet{}
	bitmap.LoopIter(it, func(u []int32) bool {
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

func (ds dumbSet) Iterator() bitmap.Iterator {
	sort.Sort(ds)
	return ds
}

func (ds dumbSet) LastSpan() int32 {
	if len(ds) == 0 {
		return bitmap.NoNext
	}
	return ds[0] &^ bitmap.SpanMask
}

func (ds dumbSet) FetchAndNext(span int32) (bitmap.Block, int32) {
	var block bitmap.Block
	if span < 0 {
		return block, bitmap.NoNext
	}
	ix := sort.Search(len(ds), func(i int) bool {
		return span > ds[i]
	})
	for i := ix - 1; i >= 0 && ds[i] < span+bitmap.SpanSize; i-- {
		block.Set(uint8(ds[i] - span))
	}
	if ix == len(ds) {
		return block, bitmap.NoNext
	}
	return block, ds[ix] &^ bitmap.SpanMask
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
