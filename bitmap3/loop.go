package bitmap3

import "math/bits"

func Loop(m IBitmap, f func([]int32) bool) {
	var un Unrolled
	m.LoopBlock(func(span int32, bl uint32) bool {
		return f(Unroll(bl, span, &un))
	})
}

type Counter interface {
	Count() uint32
}

func Count(m IBitmap) uint32 {
	if cnt, ok := m.(Counter); ok {
		return cnt.Count()
	}
	sum := 0
	m.LoopBlock(func(_ int32, bl uint32) bool {
		sum += bits.OnesCount32(bl)
		return true
	})
	return uint32(sum)
}
