package alloc

import "unsafe"

type Ptr uint32

type Allocator interface {
	Get(ref Ptr, ptr interface{})
	GetPtr(ref Ptr) unsafe.Pointer
	Alloc(ln int) Ptr
	Dealloc(ptr Ptr)
}
