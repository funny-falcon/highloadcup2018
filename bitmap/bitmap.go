package bitmap

import (
	"sync"
	"unsafe"

	"github.com/modern-go/reflect2"

	"github.com/funny-falcon/highloadcup2018/alloc"
)

const NoNext = -1

type Bitmap interface {
	Set(al alloc.Allocator, addr alloc.Ptr, i int32) alloc.Ptr
	Unset(al alloc.Allocator, addr alloc.Ptr, i int32) alloc.Ptr
	ApproxCapa() int32
	Iterator(al alloc.Allocator, max int32) Iterator
}

type Iterator interface {
	LastSpan() int32
	FetchAndNext(span int32) (uint64, int32)
}

func Decompress(span int32, block uint64, to *[]int32) {
	*to = (*to)[:0]
	if block == 0 {
		return
	}
	for i := uint8(64); i > 0; i-- {
		if block&(1<<(i-1)) != 0 {
			*to = append(*to, span+int32(i-1))
		}
	}
}

func LoopIter(it Iterator, f func(u []int32) bool) {
	indx := make([]int32, 0, 256)
	last := it.LastSpan()
	for last >= 0 {
		block, next := it.FetchAndNext(last)
		Decompress(last, block, &indx)
		if !f(indx) {
			break
		}
		last = next
	}
}

type Wrapper struct {
	sync.Mutex
	Alloc  alloc.Allocator
	Ptr    *alloc.Ptr
	Tpe    reflect2.Type
	Bitmap Bitmap
}

func Wrap(al alloc.Allocator, ptr *alloc.Ptr, pat interface{}) *Wrapper {
	tpe := reflect2.TypeOfPtr(pat).Elem()
	wr := &Wrapper{
		Alloc: al,
		Tpe:   tpe,
		Ptr:   ptr,
	}
	if wr.Ptr == nil {
		wr.Ptr = new(alloc.Ptr)
	}
	if *wr.Ptr == 0 {
		wr.Bitmap = pat.(Bitmap)
	} else {
		var p unsafe.Pointer
		al.Get(*wr.Ptr, &p)
		wr.Bitmap = tpe.PackEFace(p).(Bitmap)
	}
	return wr
}

func (w *Wrapper) IsEmpty() bool {
	return *w.Ptr == 0
}

func (w *Wrapper) Set(i int32) {
	w.Lock()
	w.remap(w.Bitmap.Set(w.Alloc, *w.Ptr, i))
	w.Unlock()
}

func (w *Wrapper) Unset(i int32) {
	w.Lock()
	w.remap(w.Bitmap.Unset(w.Alloc, *w.Ptr, i))
	w.Unlock()
}

func (w *Wrapper) GetIterator(max int32) Iterator {
	if w == nil {
		return EmptyIt
	}
	return w.Bitmap.Iterator(w.Alloc, max)
}

func (w *Wrapper) remap(ptr alloc.Ptr) {
	if ptr != *w.Ptr {
		var p unsafe.Pointer
		*w.Ptr = ptr
		w.Alloc.Get(*w.Ptr, &p)
		w.Bitmap = w.Tpe.PackEFace(p).(Bitmap)
	}
}

func (w *Wrapper) ApproxCapa() int32 {
	return w.Bitmap.ApproxCapa()
}

func (w *Wrapper) Iterator(max int32) Iterator {
	return w.Bitmap.Iterator(w.Alloc, max)
}
