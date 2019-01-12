package alloc

type Ptr uint32

type Allocator interface {
	Get(ref Ptr, ptr interface{})
	Alloc(ln int) Ptr
	Dealloc(ptr Ptr)
}
