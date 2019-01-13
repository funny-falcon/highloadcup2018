package bitmap

import (
	"sort"

	"github.com/funny-falcon/highloadcup2018/alloc"
)

type Small struct {
	cnt  uint16
	cap  uint16
	nums [256]int32
}

var SmallEmpty = &Small{}

func (s *Small) ApproxCapa() int32 {
	return int32(s.cnt)
}

func (s *Small) Set(allocator alloc.Allocator, old alloc.Ptr, ix int32) alloc.Ptr {
	i := sort.Search(int(s.cnt), func(i int) bool {
		return ix <= s.nums[i]
	})
	if i < int(s.cnt) && s.nums[i] == ix {
		return old
	}
	if s.cnt == s.cap {
		nextcap := s.cap * 2
		if nextcap == 0 {
			nextcap = 2
		}
		nptr := allocator.Alloc(4 + 4*int(nextcap))
		var ns *Small
		allocator.Get(nptr, &ns)
		ns.cap = nextcap
		ns.cnt = s.cnt + 1
		copy(ns.nums[:i], s.nums[:i])
		ns.nums[i] = ix
		copy(ns.nums[i+1:], s.nums[i:s.cnt])
		allocator.Dealloc(old)
		return nptr
	} else {
		copy(s.nums[i+1:], s.nums[i:s.cnt])
		s.cnt++
		s.nums[i] = ix
		return old
	}
}

func (s *Small) Unset(allocator alloc.Allocator, old alloc.Ptr, ix int32) alloc.Ptr {
	i := sort.Search(int(s.cnt), func(i int) bool {
		return ix <= s.nums[i]
	})
	if i < int(s.cnt) && s.nums[i] == ix {
		copy(s.nums[i:], s.nums[i+1:s.cnt])
		s.cnt--
	}
	return old
}

func (s *Small) Iterator(al alloc.Allocator, max int32) Iterator {
	if s.cnt == 0 {
		return EmptyIt
	}
	return &SmallIter{S: s, Li: int(s.cnt)}
}

type SmallIter struct {
	S  *Small
	Li int
}

func (si *SmallIter) LastSpan() int32 {
	last := si.S.nums[si.S.cnt-1]
	return last &^ SpanMask
}

func (si *SmallIter) FetchAndNext(span int32) (Block, int32) {
	li, ll := si.Li, si.Li
	if li < int(si.S.cnt) && si.S.nums[li] < span {
		li = int(si.S.cnt)
	}
	if li == 0 {
		return Block{}, NoNext
	}
	for ; li > 0 && si.S.nums[li-1] >= span; li-- {
	}
	si.Li = li
	var block Block
	for ; li < ll && si.S.nums[li] < span+SpanSize; li++ {
		block.Set(uint8(si.S.nums[li] & SpanMask))
	}
	if li == 0 {
		return block, NoNext
	} else {
		return block, si.S.nums[li-1] &^ SpanMask
	}
}
