package alloc2

import (
	"fmt"
	"log"
	"sync"
	"unsafe"
)

type Simple struct {
	Base
	sync.Mutex
	Cur        chunk
	Free       []chunk
	TotalFree  int
	TotalAlloc int
	Log        string
}

type chunk struct {
	chunk uintptr
	off   uintptr
	free  *int
}

func (s *Simple) Alloc(ln int) unsafe.Pointer {
	s.Lock()
	defer s.Unlock()
	return s.alloc(ln)
}

func (s *Simple) alloc(ln int) unsafe.Pointer {
	n := 4 + (ln+3)&^3
	if n >= ChunkSize-8 {
		panic("no")
	}
	if s.Cur.free == nil || int(s.Cur.off)+n > ChunkSize {
		if s.Cur.free != nil {
			*s.Cur.free += 8
			if *s.Cur.free == ChunkSize {
				s.Cur.off = 8
				s.Free = append(s.Free, s.Cur)
			}
		}
		if len(s.Free) > 0 {
			s.Cur = s.Free[len(s.Free)-1]
			s.Free = s.Free[:len(s.Free)-1]
			*s.Cur.free = ChunkSize - 8
		} else {
			s.Cur.chunk = uintptr(unsafe.Pointer(s.Base.ExtendChunks()))
			if s.Log != "" {
				fmt.Printf("%p chunk %s\n", unsafe.Pointer(s.Cur.chunk), s.Log)
			}
			if s.Cur.chunk&ChunkMask != 0 {
				panic("no")
			}
			s.Cur.off = 8
			s.Cur.free = (*int)(unsafe.Pointer(s.Cur.chunk))
			*s.Cur.free = ChunkSize - 8
			s.TotalFree += ChunkSize - 8
		}
	}
	*(*uint32)(unsafe.Pointer(s.Cur.chunk + s.Cur.off)) = uint32(n)
	res := s.Cur.chunk + s.Cur.off + 4
	s.Cur.off += uintptr(n)
	*s.Cur.free -= n
	s.TotalAlloc += n
	s.TotalFree -= n
	if s.Log != "" {
		fmt.Printf("%p alloc %d %s\n", unsafe.Pointer(res), n, s.Log)
	}
	return unsafe.Pointer(res)
}

func (s *Simple) Dealloc(ptr unsafe.Pointer) {
	s.Lock()
	defer s.Unlock()
	s.dealloc(ptr)
}

func (s *Simple) dealloc(ptr unsafe.Pointer) {
	up := uintptr(ptr)
	sz := *(*uint32)(unsafe.Pointer(up - 4))
	s.TotalFree += int(sz)
	s.TotalAlloc -= int(sz)
	chunkp := up &^ ChunkMask
	freep := (*int)(unsafe.Pointer(chunkp))
	*freep += int(sz)
	if s.Log != "" {
		fmt.Printf("%p dealloc %s\n", ptr, s.Log)
		fmt.Printf("%p free %s\n", freep, s.Log)
	}
	if *freep == ChunkSize {
		s.Free = append(s.Free, chunk{
			chunk: chunkp,
			off:   8,
			free:  (*int)(unsafe.Pointer(chunkp)),
		})
	}
}

func (s *Simple) ChunkSpace(ptr unsafe.Pointer) int {
	up := uintptr(ptr)
	chunkp := up &^ ChunkMask
	freep := (*int)(unsafe.Pointer(chunkp))
	return *freep
}

func (s *Simple) Compact(pptr *uintptr) {
	if *pptr == 0 {
		return
	}
	up := uintptr(*pptr)
	chunkp := up &^ ChunkMask
	freep := (*int)(unsafe.Pointer(chunkp))
	if *freep > ChunkSize/4 {
		sz := *(*uint32)(unsafe.Pointer(up - 4))
		nptr := s.alloc(int(sz - 4))
		optr := unsafe.Pointer(*pptr)
		copy((*Chunk)(nptr)[:sz], (*Chunk)(optr)[:sz])
		s.dealloc(optr)
		*pptr = uintptr(nptr)
	}
}

func (s *Simple) FreeFree() {
	for _, free := range s.Free {
		if err := munmap((*Chunk)(unsafe.Pointer(free.chunk))[:]); err != nil {
			log.Fatal(err)
		}
		s.TotalFree -= ChunkSize - 8
	}
	s.Free = nil
}
