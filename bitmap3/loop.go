package bitmap3

import "math/bits"

type Looper interface {
	Loop(func([]int32) bool)
}

type LoopBlocker interface {
	LoopBlock(func(span int32, bl uint64) bool)
}

func Loop(m LoopBlocker, f func([]int32) bool) {
	if lp, ok := m.(Looper); ok {
		lp.Loop(f)
		return
	}
	var un Unrolled
	m.LoopBlock(func(span int32, bl uint64) bool {
		return f(Unroll(bl, span, &un))
	})
}

type Counter interface {
	Count() uint32
}

func Count(m LoopBlocker) uint32 {
	if cnt, ok := m.(Counter); ok {
		return cnt.Count()
	}
	sum := 0
	if lp, ok := m.(Looper); ok {
		lp.Loop(func(i []int32) bool {
			sum += len(i)
			return true
		})
	} else {
		m.LoopBlock(func(_ int32, bl uint64) bool {
			sum += bits.OnesCount64(bl)
			return true
		})
	}
	return uint32(sum)
}
