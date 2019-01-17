package bitmap2

import "sort"

func LoopMap(bm IBitmap, f func(u []int32) bool) {
	var indx BlockUnroll
	it, last := bm.Iterator()
	for last >= 0 {
		block, next := it.FetchAndNext(last)
		if !block.Empty() && !f(block.Unroll(last, &indx)) {
			break
		}
		if next >= last {
			panic("no")
		}
		last = next
	}
}

type EmptyMapT struct{}

var EmptyMap = &EmptyMapT{}

func (e EmptyMapT) GetSize() int32 {
	return 0
}

func (e EmptyMapT) Iterator() (Iterator, int32) {
	return EmptyIt, NoNext
}

type NoopIterator struct{}

var EmptyIt = &NoopIterator{}

func (n NoopIterator) FetchAndNext(int32) (*Block, int32) {
	return &ZeroBlock, NoNext
}

type AndBitmap struct {
	B []IBitmap
}

func NewAndBitmap(bm []IBitmap) IBitmap {
	sort.Slice(bm, func(i, j int) bool {
		return SizeOf(bm[i]) < SizeOf(bm[j])
	})
	if SizeOf(bm[0]) == 0 {
		return EmptyMap
	}
	if len(bm) == 1 {
		return bm[0]
	}
	return &AndBitmap{bm}
}

type AndIterator struct {
	It []Iterator
	B  Block
}

func SizeOf(b IBitmap) uint32 {
	if szr, ok := b.(ISizer); ok {
		return szr.GetSize()
	}
	return 1 << 30
}

func (a AndBitmap) Iterator() (Iterator, int32) {
	last := int32(1 << 30)
	var r AndIterator
	for _, b := range a.B {
		it, l := b.Iterator()
		if l < last {
			last = l
		}
		if last == NoNext {
			return EmptyIt, NoNext
		}
		r.It = append(r.It, it)
	}
	return &r, last
}

func (it *AndIterator) FetchAndNext(span int32) (*Block, int32) {
	it.B = AllBlock
	next := span - BlockSize
	for _, cur := range it.It {
		bl, nxt := cur.FetchAndNext(span)
		if nxt < next {
			next = nxt
		}
		it.B.Intersect(bl)
		if it.B.Empty() {
			return &ZeroBlock, next
		}
	}
	return &it.B, next
}

type OrBitmap struct {
	B []IBitmap
}

func NewOrBitmap(bm []IBitmap) IBitmap {
	ob := &OrBitmap{B: make([]IBitmap, 0, len(bm))}
	for _, b := range bm {
		if SizeOf(b) == 0 {
			continue
		}
		ob.B = append(ob.B, b)
	}
	if len(ob.B) == 0 {
		return EmptyMap
	}
	return ob
}

func (ob *OrBitmap) Iterator() (Iterator, int32) {
	last := int32(NoNext)
	oi := &OrIterator{}
	for _, b := range ob.B {
		if SizeOf(b) == 0 {
			continue
		}
		it, lst := b.Iterator()
		if lst == NoNext {
			continue
		}
		oi.It = append(oi.It, OrElem{it, lst})
		if lst > last {
			last = lst
		}
	}
	if last == NoNext {
		return EmptyIt, NoNext
	}
	if len(oi.It) == 0 {
		return EmptyIt, NoNext
	}
	oi.heapify()
	return oi, last
}

type OrIterator struct {
	It []OrElem
	B  Block
}

type OrElem struct {
	Iterator
	Last int32
}

func (oi *OrIterator) FetchAndNext(span int32) (*Block, int32) {
	if len(oi.It) == 0 {
		return &ZeroBlock, NoNext
	}
	oi.B = ZeroBlock
	cur := &oi.It[0]
	for cur.Last >= span {
		b, n := cur.FetchAndNext(span)
		oi.B.Union(b)
		cur.Last = n
		oi.siftUp(0)
	}
	return &oi.B, cur.Last
}

func (ob *OrIterator) heapify() {
	for i := len(ob.It); i > 0; i-- {
		ob.siftUp(i - 1)
	}
}

func (ob *OrIterator) siftUp(i int) {
	el := ob.It[i]
	l := len(ob.It)
	for i*2+1 < l {
		c1 := i*2 + 1
		c2 := c1 + 1
		if c2 < l && ob.It[c2].Last > ob.It[c1].Last {
			c1 = c2
		}
		if el.Last >= ob.It[c1].Last {
			break
		}
		ob.It[i] = ob.It[c1]
		i = c1
	}
	ob.It[i] = el
}
