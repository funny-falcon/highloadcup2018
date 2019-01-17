package alloc2

import (
	"sync"
	"unsafe"
)

type Simple struct {
	Base
	sync.Mutex
	Cur []byte
}

func (s *Simple) Alloc(ln int) unsafe.Pointer {
	s.Lock()
	defer s.Unlock()
	return s.alloc(ln)
}

func (s *Simple) alloc(ln int) unsafe.Pointer {
	n := (ln + 3) &^ 3
	if len(s.Cur) < n {
		s.Cur = s.Base.ExtendChunks()[:]
	}
	res := unsafe.Pointer(&s.Cur[0])
	s.Cur = s.Cur[n:]
	return res
}

func (s *Simple) Dealloc(ptr unsafe.Pointer) {
	// nothing
}
