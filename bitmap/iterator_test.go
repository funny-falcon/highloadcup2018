package bitmap_test

import (
	"math/rand"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/funny-falcon/highloadcup2018/alloc"

	"github.com/funny-falcon/highloadcup2018/bitmap"
)

func TestIterator(t *testing.T) {
	for rrr := 99; rrr < 102; rrr++ {
		rnd := rand.New(rand.NewSource(int64(rrr)))
		var al alloc.Simple
		newIts := []func(ds dumbSet) bitmap.Iterator{
			func(ds dumbSet) bitmap.Iterator {
				return ds.Iterator()
			},
			func(ds dumbSet) bitmap.Iterator {
				smset := bitmap.Wrap(&al, nil, bitmap.LargeEmpty)
				for _, v := range ds {
					smset.Set(v)
				}
				return smset.Iterator(1 << 20)
			},
		}
		for itk := 2; itk < 6; itk++ {
			for _, newIt := range newIts {
				testIter(t, 500, itk, func() int32 { return rnd.Int31n(512) }, newIt)
			}
		}
		for k := 1; k < 10000; k += k/2 + 1 {
			for itk := 2; itk < 6; itk++ {
				for _, newIt := range newIts {
					testIter(t, k, itk, func() int32 { return rnd.Int31n(1 << 20) }, newIt)
					testIter(t, k, itk, func() int32 { return rnd.Int31n(1 << 14) }, newIt)
					testIter(t, k, itk, func() int32 {
						n := rnd.Int31n(1 << 16)
						n = n%256 + n/256*497
						return n
					}, newIt)
				}
			}
		}
		newIt := func(ds dumbSet) bitmap.Iterator {
			smset := bitmap.Wrap(&al, nil, bitmap.SmallEmpty)
			for _, v := range ds {
				smset.Set(v)
			}
			return smset.Iterator(1 << 20)
		}
		for k := 1; k < 230; k += k/2 + 1 {
			for itk := 2; itk < 6; itk++ {
				testIter(t, k, itk, func() int32 { return rnd.Int31n(1 << 20) }, newIt)
				testIter(t, k, itk, func() int32 { return rnd.Int31n(1 << 10) }, newIt)
			}
		}
	}
}

func testIter(t *testing.T, k int, itk int, gen func() int32, newit func(ds dumbSet) bitmap.Iterator) {
	dss := make([]dumbSet, itk)
	itsOr := make([]bitmap.Iterator, itk)
	var dsOr, dsAnd dumbSet
	uniqOr := make(map[int32]bool, k)
	for i := range dss {
		dss[i].generate(k, gen)
		it := newit(dss[i])
		sort.Sort(dss[i])
		for _, u := range dss[i] {
			uniqOr[u] = true
		}
		itsOr[i] = it
		if i == 0 {
			dsOr = dss[0]
			dsAnd = dss[0]
		} else {
			dsOr = dsOr.Union(dss[i])
			dsAnd = dsAnd.Intersect(dss[i])
		}
	}
	dsItOr := dumbFromIter(bitmap.NewOrIterator(itsOr))
	dsItAnd := dumbFromIter(bitmap.NewAndIterator(itsOr))
	matItOr := dumbFromIter(bitmap.Materialize(bitmap.NewOrIterator(itsOr)))
	matItAnd := dumbFromIter(bitmap.Materialize(bitmap.NewAndIterator(itsOr)))

	require.Len(t, dsOr, len(uniqOr))
	for i := range dss {
		require.Equal(t, dss[i], dumbFromIter(itsOr[i]))
	}
	require.Equal(t, dsOr, dsItOr)
	runtime.KeepAlive(&itsOr)
	runtime.KeepAlive(&dss)
	require.Equal(t, dsAnd, dsItAnd)
	require.Equal(t, dsOr, matItOr)
	require.Equal(t, dsAnd, matItAnd)
}
