package bitmap

import (
	"container/heap"
)

type NilIterator struct{}

func (NilIterator) LastSpan() int32 {
	return NoNext
}

func (NilIterator) FetchAndNext(span int32) (Block, int32) {
	return Block{}, NoNext
}

var EmptyIt = &NilIterator{}

type OrIterator struct {
	Its  OrIterElems
	Last int32
}

type OrIterElem struct {
	It   Iterator
	Next int32
}

type OrIterElems []OrIterElem

func (o OrIterElems) Len() int           { return len(o) }
func (o OrIterElems) Less(i, j int) bool { return o[i].Next > o[j].Next }
func (o OrIterElems) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o OrIterElems) Push(x interface{}) {}
func (o OrIterElems) Pop() interface{}   { return nil }

func NewOrIterator(its []Iterator) Iterator {
	or := &OrIterator{Last: NoNext}
	for _, it := range its {
		if _, ok := it.(*NilIterator); ok {
			continue
		}
		last := it.LastSpan()
		if last > or.Last {
			or.Last = last
		}
		or.Its = append(or.Its, OrIterElem{
			It:   it,
			Next: last,
		})
	}

	if len(or.Its) == 0 {
		return EmptyIt
	}

	if len(or.Its) == 1 {
		return or.Its[0].It
	}

	heap.Init(&or.Its)
	return or
}

func (it *OrIterator) LastSpan() int32 {
	return it.Last
}

func (it *OrIterator) FetchAndNext(span int32) (Block, int32) {
	current := it.Its[0].Next
	var block Block
	if current < 0 {
		return block, NoNext
	}
	for it.Its[0].Next == current {
		cbl, cnext := it.Its[0].It.FetchAndNext(span)
		block.Union(cbl)
		it.Its[0].Next = cnext
		heap.Fix(&it.Its, 0)
	}
	next := it.Its[0].Next
	return block, next
}

type AndIterator struct {
	Its  []Iterator
	Last int32
}

func NewAndIterator(its []Iterator) Iterator {
	j := 0
	for _, it := range its {
		if _, ok := it.(*NilIterator); ok {
			return EmptyIt
		}
		its[j] = it
		j++
	}
	its = its[:j]

	if len(its) == 0 {
		return EmptyIt
	}

	if len(its) == 1 {
		return its[0]
	}

	last := its[0].LastSpan()
	for _, it := range its[1:] {
		if l := it.LastSpan(); l < last {
			last = l
		}
	}

	if last == NoNext {
		return EmptyIt
	}

	return &AndIterator{
		Its:  its,
		Last: last,
	}
}

func (it *AndIterator) LastSpan() int32 {
	return it.Last
}

func (it *AndIterator) FetchAndNext(span int32) (Block, int32) {
	next := span - SpanSize
	if next < 0 {
		next = NoNext
	}
	block := AllBlock
	for _, it := range it.Its {
		cbl, cnext := it.FetchAndNext(span)
		block.Intersect(cbl)
		if cnext < next {
			next = cnext
		}
	}
	return block, next
}

type NotIterator struct {
	It   Iterator
	Last int32
}

func NewNotIterator(it Iterator, nextId int32) Iterator {
	switch itt := it.(type) {
	case *NotIterator:
		return itt.It
	}
	if nextId == 0 {
		return EmptyIt
	}
	return &NotIterator{
		It:   it,
		Last: nextId - 1,
	}
}

func (it *NotIterator) LastSpan() int32 {
	return it.Last &^ SpanMask
}

func (it *NotIterator) FetchAndNext(span int32) (Block, int32) {
	mask := AllBlock
	if span == it.LastSpan() {
		mask = BlockMask(uint8(it.Last & SpanMask))
	}
	b, _ := it.It.FetchAndNext(span)
	b.Intersect(mask)
	nxt := span - SpanSize
	if nxt < 0 {
		nxt = NoNext
	}
	return b, nxt
}

type AllIterator struct {
	Last int32
}

func NewAllIterator(nextId int32) Iterator {
	if nextId == 0 {
		return EmptyIt
	}
	return &AllIterator{
		Last: nextId - 1,
	}
}

func (it *AllIterator) LastSpan() int32 {
	return it.Last &^ 63
}

func (it *AllIterator) FetchAndNext(span int32) (Block, int32) {
	mask := AllBlock
	if span == it.LastSpan() {
		mask = BlockMask(uint8(it.Last & SpanMask))
	}
	nxt := span - 64
	if nxt < 0 {
		nxt = NoNext
	}
	return mask, nxt
}
