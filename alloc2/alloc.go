package alloc2

import (
	"syscall"
	"unsafe"
)

type Allocator interface {
	Alloc(ln int) unsafe.Pointer
	Dealloc(ptr unsafe.Pointer)
}

func mmap(addr uintptr, length uintptr, prot int, flags int, fd int, offset int64) (slice []byte, err error) {
	r0, _, e1 := syscall.Syscall6(syscall.SYS_MMAP, uintptr(addr), uintptr(length), uintptr(prot), uintptr(flags), uintptr(fd), uintptr(offset))
	slice = (*[1 << 30]byte)(unsafe.Pointer(r0))[:length]
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}

func munmap(slice []byte) (err error) {
	ptr := unsafe.Pointer(&slice[0])
	_, _, e1 := syscall.Syscall(syscall.SYS_MUNMAP, uintptr(ptr), uintptr(len(slice)), 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	default:
		return e
	}
	return e
}
