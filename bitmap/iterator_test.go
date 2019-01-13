package bitmap_test

import (
	"math/rand"
	"testing"

	"github.com/funny-falcon/highloadcup2018/alloc"

	"github.com/funny-falcon/highloadcup2018/bitmap"
	"github.com/stretchr/testify/require"
)

func TestIterator(t *testing.T) {
	rnd := rand.New(rand.NewSource(99))
	var al alloc.Simple
	newIts := []func(ds dumbSet) bitmap.Iterator{
		func(ds dumbSet) bitmap.Iterator {
			return ds.Iterator()
		},
		func(ds dumbSet) bitmap.Iterator {
			smset := bitmap.Wrap(&al, nil, bitmap.SmallEmpty)
			for _, v := range ds {
				smset.Set(v)
			}
			return smset.Iterator(1 << 20)
		},
		func(ds dumbSet) bitmap.Iterator {
			smset := bitmap.Wrap(&al, nil, bitmap.LargeEmpty)
			for _, v := range ds {
				smset.Set(v)
			}
			return smset.Iterator(1 << 20)
		},
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
}

func testIter(t *testing.T, k int, itk int, gen func() int32, newit func(ds dumbSet) bitmap.Iterator) {
	dss := make([]dumbSet, itk)
	itsOr := make([]bitmap.Iterator, itk)
	itsAnd := make([]bitmap.Iterator, itk)
	var dsOr, dsAnd dumbSet
	for i := range dss {
		dss[i].generate(k, gen)
		itsOr[i] = dss[i].Iterator()
		itsAnd[i] = dss[i].Iterator()
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

	require.Equal(t, dsOr, dsItOr)
	require.Equal(t, dsAnd, dsItAnd)
}
