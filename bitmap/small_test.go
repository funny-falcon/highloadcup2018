package bitmap_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/funny-falcon/highloadcup2018/alloc"
	"github.com/funny-falcon/highloadcup2018/bitmap"
	"github.com/stretchr/testify/assert"
)

func TestSmall(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	testIt(t, bitmap.SmallEmpty, 200, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, bitmap.SmallEmpty, 120, func() int32 { return rnd.Int31n(1 << 7) })
	testIt(t, bitmap.SmallEmpty, 200, func() int32 { return rnd.Int31n(1 << 9) })
}

func testIt(t *testing.T, b bitmap.Bitmap, maxcap int, gen func() int32) {
	var al alloc.Simple
	for k := 1; k < maxcap; k += k/2 + 1 {
		sm := bitmap.Wrap(&al, nil, b)
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
			sm.Set(v)
		}

		nset := make([]int32, 0, k)
		bitmap.LoopIter(sm.Iterator(1<<20), func(u []int32) bool {
			nset = append(nset, u...)
			return true
		})
		sort.Slice(set, func(i, j int) bool { return set[i] < set[j] })
		sort.Slice(nset, func(i, j int) bool { return nset[i] < nset[j] })

		assert.Equal(t, set, nset)

		rand.Shuffle(k, func(i, j int) { set[i], set[j] = set[j], set[i] })
		for i := k / 4; i >= 0; i-- {
			sm.Unset(set[i])
		}
		set = set[k/4+1:]

		nset = make([]int32, 0, k)
		bitmap.LoopIter(sm.Iterator(1<<20), func(u []int32) bool {
			nset = append(nset, u...)
			return true
		})
		sort.Slice(set, func(i, j int) bool { return set[i] < set[j] })
		sort.Slice(nset, func(i, j int) bool { return nset[i] < nset[j] })

		assert.Equal(t, set, nset)
	}
}
