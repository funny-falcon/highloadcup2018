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
		return chix <= l.chunks[i].chnkAndCnt
	})
	if chi >= int(l.cnt) || l.chunks[chi].chnkAndCnt&^511 != chix {
		if l.cnt == l.cap {
			nextcap := l.cap * 2
			if nextcap == 0 {
				nextcap = 2
			}
			nptr := allocator.Alloc(4 + 8*int(nextcap))
			var nl *Large
			allocator.Get(nptr, &nl)
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
		return chix <= l.chunks[i].chnkAndCnt
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
		Li: int(ch.cnt),
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
			var mask *LargeChunkMask
			allocator.Get(ptr, &mask)
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
		var mask *LargeChunkMask
		allocator.Get(alloc.Ptr(ptr), &mask)
		if mask.Set(ix) {
			ch.chnkAndCnt++
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
		var mask *LargeChunkMask
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

func (ch *LargeChunk) LastSpan(al alloc.Allocator) int32 {
	base := (ch.chnkAndCnt &^ 511) >> 1
	switch ch.chnkAndCnt & 256 {
	case 0:
		return base + int32(ch.refsmall[ch.chnkAndCnt&255]&192)
	default:
		ptr := alloc.Ptr(binary.LittleEndian.Uint32(ch.refsmall[:]))
		var mask *LargeChunkMask
		al.Get(ptr, &mask)
		if mask[3] != 0 {
			return base + 64*3
		}
		if mask[2] != 0 {
			return base + 64*2
		}
		if mask[1] != 0 {
			return base + 64*1
		}
		return base + 64*0
	}
}

type LargeChunkMask [4]uint64

func (msk *LargeChunkMask) Has(i uint8) bool {
	return msk[i/64]&(1<<(i&63)) != 0
}

func (msk *LargeChunkMask) Set(i uint8) bool {
	p, b := i/64, uint64(1)<<(i&63)
	r := msk[p]&b == 0
	msk[p] |= b
	return r
}

func (msk *LargeChunkMask) Unset(i uint8) bool {
	p, b := i/64, uint64(1)<<(i&63)
	r := msk[p]&b != 0
	msk[p] &^= b
	return r
}

type LargeIterator struct {
	L  *Large
	Al alloc.Allocator
	Li int
}

func (it *LargeIterator) LastSpan() int32 {
	if it.L.cnt == 0 {
		return NoNext
	}
	return it.L.chunks[it.L.cnt-1].LastSpan(it.Al)
}

func (it *LargeIterator) FetchAndNext(span int32) (uint64, int32) {
	chix := (span &^ 255) << 1
	li := JumpSearch(it.Li, func(i int) bool {
		return chix <= it.L.chunks[i].chnkAndCnt&^511
	})
	it.Li = li + 1
	ch := it.L.chunks[li]
	var block uint64
	if chix == ch.chnkAndCnt&^511 {
		var here LargeChunkMask
		var mask *LargeChunkMask
		switch ch.chnkAndCnt & 256 {
		case 0:
			mask = &here
			cnt := int(ch.chnkAndCnt&255) + 1
			for i := 0; i < cnt; i++ {
				mask.Set(ch.refsmall[i])
			}
		default:
			ptr := alloc.Ptr(binary.LittleEndian.Uint32(ch.refsmall[:]))
			it.Al.Get(ptr, &mask)
		}
		k := (span / 64) & 3
		block = mask[k]
		base := (ch.chnkAndCnt &^ 511) >> 1
		switch k {
		case 3:
			if mask[2] != 0 {
				return block, base + 128
			}
			fallthrough
		case 2:
			if mask[1] != 0 {
				return block, base + 64
			}
			fallthrough
		case 1:
			if mask[0] != 0 {
				return block, base
			}
			fallthrough
		case 0:
		}
	}
	it.Li = li
	if li == 0 {
		return block, NoNext
	}
	return block, it.L.chunks[li-1].LastSpan(it.Al)
}

func JumpSearch(n int, f func(i int) bool) int {
	if n == 0 {
		return 0
	}
	var j int
	for j = 1; j <= n; j *= 2 {
		c := n - j
		if f(c) {
			switch j {
			case 1:
				return n - 1
			case 2:
				return n - 2
			default:
				return c + sort.Search(j/2, func(k int) bool { return f(c + k) })
			}
		}
	}
	return sort.Search(n-j/2, f)
}
