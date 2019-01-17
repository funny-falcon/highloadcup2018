package alloc2

import "unsafe"

type Allocator interface {
	Alloc(ln int) unsafe.Pointer
	Dealloc(ptr unsafe.Pointer)
}
