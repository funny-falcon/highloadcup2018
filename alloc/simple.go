package alloc

import (
	"sync"
)

type Simple struct {
	Base
	sync.Mutex
	CurOff uint32
}

func (s *Simple) Alloc(ln int) Ptr {
	s.Lock()
	defer s.Unlock()
	return s.alloc(ln)
}

func (s *Simple) alloc(ln int) Ptr {
	if s.CurOff == 0 {
		s.CurOff = 8
		s.ExtendChunks()
	}
	n := uint32(ln)
	if (s.CurOff+n+3)&^3 >= s.CurEnd {
		s.CurOff = s.ExtendChunks()
	}
	res := s.CurOff
	s.CurOff = (s.CurOff + n + 3) &^ 3
	if s.CurOff >= s.CurEnd {
		s.CurOff = s.ExtendChunks()
	}
	return Ptr(res)
}

func (s *Simple) Dealloc(ptr Ptr) {
	// nothing
}
