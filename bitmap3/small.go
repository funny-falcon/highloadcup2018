package bitmap3

import (
	"unsafe"

	"github.com/funny-falcon/highloadcup2018/alloc2"
)

var SmallAlloc = alloc2.Simple{Log: ""}

type Small struct {
	*SmallImpl
}

type SmallImpl struct {
	Size uint16
	Cap  uint16
	Data [256]int32
}

func GetSmall(p *uintptr) *Small {
	return (*Small)(unsafe.Pointer(p))
}

func (s *Small) Uintptr() uintptr {
	return uintptr(unsafe.Pointer(s.SmallImpl))
}

func (s *Small) ForceAlloc() uintptr {
	ncap := s.Size + 3
	ptr := SmallAlloc.Alloc(4 + int(ncap)*4)
	impl := (*SmallImpl)(ptr)
	impl.Size = s.Size
	impl.Cap = ncap
	copy(impl.Data[:], s.Data[:s.Size])
	s.SmallImpl = impl
	return uintptr(ptr)
}

func (s *Small) GetSize() uint32 {
	return uint32(s.Size)
}

func (s *Small) Set(id int32) {
	if s.SmallImpl == nil {
		ncap := uint16(4)
		s.SmallImpl = (*SmallImpl)(SmallAlloc.Alloc(4 + int(ncap)*4))
		s.Size = 1
		s.Cap = ncap
		s.Data[0] = id
		return
	}
	ix := searchSparse32(s.Data[:s.Size], id)
	if ix < int(s.Size) && s.Data[ix] == id {
		return
	}
	if s.Size == s.Cap {
		ncap := s.Cap * 2
		newImpl := (*SmallImpl)(SmallAlloc.Alloc(int(ncap+1) * 4))
		newImpl.Size = s.Size
		newImpl.Cap = ncap
		copy(newImpl.Data[:s.Size], s.Data[:s.Size])
		SmallAlloc.Dealloc(unsafe.Pointer(s.SmallImpl))
		s.SmallImpl = newImpl
	}
	copy(s.Data[ix+1:s.Size+1], s.Data[ix:s.Size])
	s.Data[ix] = id
	s.Size++
}
