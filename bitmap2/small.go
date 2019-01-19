package bitmap2

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

func (s *Small) Unset(id int32) {
	if s.SmallImpl == nil || s.Size == 0 {
		return
	}
	ix := searchSparse32(s.Data[:s.Size], id)
	if ix == int(s.Size) || s.Data[ix] != id {
		return
	}
	copy(s.Data[ix:s.Size-1], s.Data[ix+1:s.Size])
	s.Size--
}

func (s *Small) Iterator() (Iterator, int32) {
	if s.Size == 0 {
		return EmptyIt, NoNext
	}
	return &SmallIterator{S: s.SmallImpl}, s.Data[0] &^ BlockMask
}

type SmallIterator struct {
	S *SmallImpl
	B Block
	I uint16
}

func (s *SmallIterator) FetchAndNext(span int32) (*Block, int32) {
	if s.I >= s.S.Size {
		return &ZeroBlock, NoNext
	}
	ix := s.I
	p := ptr0_32(s.S.Data[:])
	for ; ix < s.S.Size && *aref32(p, int(ix)) > span+BlockMask; ix++ {
	}
	s.B = ZeroBlock
	for ; ix < s.S.Size && *aref32(p, int(ix)) >= span; ix++ {
		s.B.Set(*aref32(p, int(ix)))
	}
	s.I = ix
	if ix < s.S.Size {
		return &s.B, *aref32(p, int(ix)) &^ BlockMask
	}
	return &s.B, NoNext
}
