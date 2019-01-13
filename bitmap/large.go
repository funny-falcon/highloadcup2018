package bitmap

import (
	"encoding/binary"
	"sort"

	"github.com/funny-falcon/highloadcup2018/alloc"
)

type Large struct {
	cnt    uint16
	cap    uint16
	chunks [1 << 16]LargeChunk
}

type LargeChunk struct {
	chnkAndCnt int32
	refsmall   [4]uint8
}

var LargeEmpty = &Large{}

func (l *Large) ApproxCapa() int32 {
	return int32(l.cnt) * 4
}

func (l *Large) Set(allocator alloc.Allocator, ptr alloc.Ptr, ix int32) alloc.Ptr {
	chix := (ix &^ 255) << 1
	chi := sort.Search(int(l.cnt), func(i int) bool {
		return chix >= l.chunks[i].chnkAndCnt&^511
	})
	if chi >= int(l.cnt) || l.chunks[chi].chnkAndCnt&^511 != chix {
		if l.cnt == l.cap {
			nextcap := l.cap * 2
			if nextcap == 0 {
				nextcap = 2
			}
			nptr := allocator.Alloc(4 + 8*int(nextcap))
			nl := (*Large)(allocator.GetPtr(nptr))
			nl.cap = nextcap
			nl.cnt = l.cnt + 1
			copy(nl.chunks[:chi], l.chunks[:chi])
			copy(nl.chunks[chi+1:], l.chunks[chi:l.cnt])
			allocator.Dealloc(ptr)
			l = nl
			ptr = nptr
		} else {
			copy(l.chunks[chi+1:], l.chunks[chi:l.cnt])
			l.cnt++
		}
		l.chunks[chi] = LargeChunk{chnkAndCnt: chix, refsmall: [4]uint8{uint8(ix & 255)}}
	} else {
		l.chunks[chi].Set(allocator, uint8(ix))
	}
	return ptr
}

func (l *Large) Unset(allocator alloc.Allocator, ptr alloc.Ptr, ix int32) alloc.Ptr {
	chix := (ix &^ 255) << 1
	chi := sort.Search(int(l.cnt), func(i int) bool {
		return chix >= l.chunks[i].chnkAndCnt&^511
	})
	if chi < int(l.cnt) && l.chunks[chi].chnkAndCnt&^511 == chix {
		if l.chunks[chi].Unset(allocator, uint8(ix)) {
			copy(l.chunks[chi:], l.chunks[chi+1:l.cnt])
			l.cnt--
		}
	}
	return ptr
}

func (ch *Large) Iterator(allocator alloc.Allocator, max int32) Iterator {
	if ch.cnt == 0 {
		return EmptyIt
	}
	return &LargeIterator{
		L:  ch,
		Al: allocator,
	}
}

func (ch *LargeChunk) Set(allocator alloc.Allocator, ix uint8) {
	switch ch.chnkAndCnt & 256 {
	case 0:
		pos := 0
		cnt := int(ch.chnkAndCnt&255) + 1
		for ; pos < cnt; pos++ {
			if ch.refsmall[pos] == ix {
				return
			}
			if ch.refsmall[pos] > ix {
				break
			}
		}
		if cnt == 4 {
			ch.chnkAndCnt |= 256
			ptr := allocator.Alloc(32)
			mask := (*Block)(allocator.GetPtr(ptr))
			mask.Set(ch.refsmall[0])
			mask.Set(ch.refsmall[1])
			mask.Set(ch.refsmall[2])
			mask.Set(ch.refsmall[3])
			mask.Set(ix)
			binary.LittleEndian.PutUint32(ch.refsmall[:], uint32(ptr))
		} else {
			for i := cnt; i > pos; i-- {
				ch.refsmall[i] = ch.refsmall[i-1]
			}
			ch.refsmall[pos] = ix
		}
		ch.chnkAndCnt++
	default:
		ptr := binary.LittleEndian.Uint32(ch.refsmall[:])
		mask := (*Block)(allocator.GetPtr(alloc.Ptr(ptr)))
		if mask.Set(ix) {
			ch.chnkAndCnt++
			if ch.chnkAndCnt&255 == 0 {
				panic("overflow")
			}
		}
	}
}

func (ch *LargeChunk) Unset(allocator alloc.Allocator, ix uint8) bool {
	switch ch.chnkAndCnt & 256 {
	case 0:
		pos := 0
		cnt := int(ch.chnkAndCnt&255) + 1
		for ; pos < cnt; pos++ {
			if ch.refsmall[pos] == ix {
				for pos++; pos < cnt; pos++ {
					ch.refsmall[pos-1] = ch.refsmall[pos]
				}
				if ch.chnkAndCnt&255 == 0 {
					return true
				}
				ch.chnkAndCnt--
			}
		}
	default:
		ptr := alloc.Ptr(binary.LittleEndian.Uint32(ch.refsmall[:]))
		var mask *Block
		allocator.Get(ptr, &mask)
		if mask.Unset(ix) {
			if ch.chnkAndCnt&255 == 0 {
				allocator.Dealloc(ptr)
				return true
			}
			ch.chnkAndCnt--
		}
	}
	return false
}

func (ch *LargeChunk) Span() int32 {
	return (ch.chnkAndCnt &^ 511) >> 1
}

type LargeIterator struct {
	L  *Large
	Al alloc.Allocator
	Li int
}

func (it *LargeIterator) LastSpan() int32 {
	return it.L.chunks[0].Span()
}

func (it *LargeIterator) Reset() {
	it.Li = 0
}

func (it *LargeIterator) FetchAndNext(span int32) (Block, int32) {
	var block Block
	var bl = &block
	chixn := ((span &^ 255) + SpanSize) << 1
	/*
		li := it.Li + JumpSearch(int(it.L.cnt)-it.Li, func(i int) bool {
			return chixn > it.L.chunks[it.Li+i].chnkAndCnt
		})
	*/
	li := it.Li
	L := it.L
	cnt := int(L.cnt)
	if cnt >= len(L.chunks) {
		return block, NoNext
	}
	for ; li < cnt && chixn <= L.chunks[li].chnkAndCnt; li++ {
	}
	it.Li = li
	if li < cnt {
		ch := L.chunks[li]
		if span == ch.Span() {
			it.Li = li + 1
			switch ch.chnkAndCnt & 256 {
			case 0:
				cnt := int(ch.chnkAndCnt&255) + 1
				for i := 0; i < cnt; i++ {
					block.Set(ch.refsmall[i])
				}
			default:
				ptr := alloc.Ptr(binary.LittleEndian.Uint32(ch.refsmall[:]))
				/*
					var bl *Block
					it.Al.Get(ptr, &bl)
					block = *bl
				*/
				bl = (*Block)(it.Al.GetPtr(ptr))
			}
		}
	}
	if it.Li >= cnt {
		return *bl, NoNext
	}
	return *bl, L.chunks[it.Li].Span()
}

func JumpSearch(n int, f func(i int) bool) int {
	if n == 0 || f(0) {
		return 0
	}
	if n == 1 || f(1) {
		return 1
	}
	if n == 2 || f(2) {
		return 2
	}
	if n == 3 || f(3) {
		return 3
	}
	var j int
	for j = 8; j <= n; j *= 2 {
		c := j - 1
		if f(c) {
			h := j / 2
			return h + sort.Search(h, func(k int) bool { return f(h + k) })
		}
	}
	h := j / 2
	return h + sort.Search(n-h, func(k int) bool { return f(h + k) })
}
