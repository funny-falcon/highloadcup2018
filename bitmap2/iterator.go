package bitmap2

import "sort"

type looperIt interface {
	Loop(func(u []int32) bool)
}

func LoopMap(bm IBitmap, f func(u []int32) bool) {
	var indx BlockUnroll
	it, last := bm.Iterator()
	if lp, ok := it.(looperIt); ok {
		lp.Loop(f)
		return
	}
	for last >= 0 {
		block, next := it.FetchAndNext(last)
		if next >= last {
			panic("no")
		}
		if !block.Empty() && !f(block.Unroll(last, &indx)) {
			break
		}
		last = next
	}
}

func CountMap(bm IBitmap) uint32 {
	var sum uint32
	it, last := bm.Iterator()
	for last >= 0 {
		block, next := it.FetchAndNext(last)
		if next >= last {
			panic("no")
		}
		sum += block.Count()
		last = next
	}
	return sum
}

func LoopMapBlock(bm IBitmap, f func(block Block, span int32) bool) {
	it, last := bm.Iterator()
	for last >= 0 {
		block, next := it.FetchAndNext(last)
		if next >= last {
			panic("no")
		}
		if !block.Empty() && !f(*block, last) {
			break
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
	if len(bm) == 0 {
		return EmptyMap
	}
	if len(bm) == 1 {
		return bm[0]
	}
	am := &AndBitmap{append([]IBitmap(nil), bm...)}
	sort.Slice(am.B, func(i, j int) bool {
		return SizeOf(am.B[i]) < SizeOf(am.B[j])
	})
	if SizeOf(am.B[0]) == 0 {
		return EmptyMap
	}
	if len(bm) == 2 {
		hg1, ok1 := am.B[0].(*Huge)
		hg2, ok2 := am.B[1].(*Huge)
		if ok1 && ok2 {
			return &And2HugeBitmap{hg1, hg2}
		} else {
			return &And2Bitmap{am.B[0], am.B[1]}
		}
	}
	return am
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

type And2HugeBitmap struct{ A, B *Huge }

func (h2 *And2HugeBitmap) Iterator() (Iterator, int32) {
	_, lasta := h2.A.Iterator()
	_, lastb := h2.B.Iterator()
	it := And2HugeBitmapIter{And2HugeBitmap: *h2}
	if lasta < lastb {
		return &it, lasta
	}
	return &it, lastb
}

type And2HugeBitmapIter struct {
	And2HugeBitmap
	Block
}

func (it *And2HugeBitmapIter) FetchAndNext(span int32) (*Block, int32) {
	bla, lasta := it.A.FetchAndNext(span)
	blb, _ := it.B.FetchAndNext(span)
	it.Block = bla.IntersectNew(blb)
	return &it.Block, lasta
}

func (it *And2HugeBitmapIter) Loop(f func(u []int32) bool) {
	var indx BlockUnroll
	l := len(it.A.B)
	if len(it.B.B) < l {
		l = len(it.B.B)
	}
	l /= BlockLen
	ap, bp := it.A.p, it.B.p
	for l > 0 {
		l--
		bl := arefBlock(ap, l).IntersectNew(arefBlock(bp, l))
		if !bl.Empty() && !f(bl.Unroll(int32(l*BlockSize), &indx)) {
			break
		}
	}
}

type And2Bitmap [2]IBitmap

func (h2 *And2Bitmap) Iterator() (Iterator, int32) {
	ita, lasta := h2[0].Iterator()
	itb, lastb := h2[1].Iterator()
	it := And2BitmapIter{A: ita, B: itb}
	if lasta < lastb {
		return &it, lasta
	}
	return &it, lastb
}

type And2BitmapIter struct {
	A, B Iterator
	Block
}

func (it *And2BitmapIter) FetchAndNext(span int32) (*Block, int32) {
	bla, lasta := it.A.FetchAndNext(span)
	blb, lastb := it.B.FetchAndNext(span)
	it.Block = bla.IntersectNew(blb)
	if lasta < lastb {
		return &it.Block, lasta
	}
	return &it.Block, lastb
}

type OrBitmap struct {
	B []IBitmap
}

func NewOrBitmap(bm []IBitmap) IBitmap {
	ob := &OrBitmap{B: make([]IBitmap, 0, len(bm))}
	oh := make([]*Huge, 0, len(bm))
	for _, b := range bm {
		if SizeOf(b) == 0 {
			continue
		}
		ob.B = append(ob.B, b)
		if h, ok := b.(*Huge); ok {
			oh = append(oh, h)
		}
	}
	if len(ob.B) == 0 {
		return EmptyMap
	}
	if len(ob.B) == 1 {
		return ob.B[0]
	}
	if len(ob.B) == len(oh) {
		return &OrHugeBitmap{M: oh}
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
	oi.Heapify()
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
		oi.SiftUp(0)
	}
	return &oi.B, cur.Last
}

func (ob *OrIterator) Heapify() {
	for i := len(ob.It); i > 0; i-- {
		ob.SiftUp(i - 1)
	}
}

func (ob *OrIterator) SiftUp(i int) {
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

type OrHugeBitmap struct {
	M []*Huge
}

func (oh *OrHugeBitmap) Iterator() (Iterator, int32) {
	last := int32(NoNext)
	for _, m := range oh.M {
		l := m.LastSpan()
		if l > last {
			last = l
		}
	}
	return &OrHugeIterator{M: oh.M}, last
}

type OrHugeIterator struct {
	M []*Huge
	B Block
}

func (oh *OrHugeIterator) FetchAndNext(span int32) (*Block, int32) {
	oh.B = ZeroBlock
	last := int32(NoNext)
	for _, m := range oh.M {
		b, l := m.FetchAndNext(span)
		oh.B.Union(b)
		if l > last {
			last = l
		}
	}
	return &oh.B, last
}
