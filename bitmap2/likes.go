package bitmap2

import (
	"unsafe"

	"github.com/funny-falcon/highloadcup2018/alloc2"
)

const LikeUidShift = 8
const LikeUidMask = (^int32(0)) << LikeUidShift
const LikeCntMask = (1 << LikeUidShift) - 1

var LikesAlloc = alloc2.Simple{Log: ""}

type Likes struct {
	*LikesImpl
}

type LikesImpl struct {
	Size uint16
	Cap  uint16
	Data [256]LikesElem
}

type LikesElem struct {
	UidAndCnt int32
	Ts        int32
}

func GetLikes(p *uintptr) *Likes {
	return (*Likes)(unsafe.Pointer(p))
}

func (s *Likes) Uintptr() uintptr {
	return uintptr(unsafe.Pointer(s.LikesImpl))
}

func (s *Likes) GetSize() uint32 {
	return uint32(s.Size)
}

func (s *Likes) Set(id int32) {
	s.SetTs(id, 0)
}

/*
const likesMtxCnt = 64

var likesMtx [likesMtxCnt]sync.Mutex

func (s *Likes) Lock() {
	ix := uintptr(unsafe.Pointer(s)) >> 3
	likesMtx[ix&(likesMtxCnt-1)].Lock()
}

func (s *Likes) Unlock() {
	ix := uintptr(unsafe.Pointer(s)) >> 3
	likesMtx[ix&(likesMtxCnt-1)].Unlock()
}
*/

func (s *Likes) SetTs(id int32, ts int32) {
	if s.LikesImpl == nil {
		ncap := uint16(2)
		s.LikesImpl = (*LikesImpl)(LikesAlloc.Alloc(4 + int(ncap)*8))
		//fmt.Printf("%p alloc ncap %d\n", s.LikesImpl, 20)
		s.Size = 1
		s.Cap = ncap
		s.Data[0] = LikesElem{id << 8, ts}
		return
	}
	ix := searchSparseLikes(s.Data[:s.Size], id)
	if ix < int(s.Size) && s.Data[ix].UidAndCnt>>8 == id {
		el := &s.Data[ix]
		cnt := int64(el.UidAndCnt&255) + 1
		el.Ts = int32((int64(el.Ts)*cnt + int64(ts)) / (cnt + 1))
		el.UidAndCnt++
		return
	}
	if s.Size == s.Cap {
		ncap := s.Cap * 2
		ptr := LikesAlloc.Alloc(4 + int(ncap)*8)
		//fmt.Printf("%p alloc ncap %d\n", ptr, 4+int(ncap)*8)
		newImpl := (*LikesImpl)(ptr)
		newImpl.Size = s.Size
		newImpl.Cap = ncap
		copy(newImpl.Data[:s.Size], s.Data[:s.Size])
		//fmt.Printf("%p dealloc ncap %d\n", s.LikesImpl, 4+int(s.Cap)*8)
		LikesAlloc.Dealloc(unsafe.Pointer(s.LikesImpl))
		s.LikesImpl = newImpl
	}
	copy(s.Data[ix+1:s.Size+1], s.Data[ix:s.Size])
	s.Data[ix] = LikesElem{id << 8, ts}
	s.Size++
}

func (s *Likes) Unset(id int32) {
	if s.LikesImpl == nil || s.Size == 0 {
		return
	}
	ix := searchSparseLikes(s.Data[:s.Size], id)
	if ix == int(s.Size) || s.Data[ix].UidAndCnt>>8 != id {
		return
	}
	copy(s.Data[ix:s.Size-1], s.Data[ix+1:s.Size])
	s.Size--
}

func (s *Likes) GetTs(id int32) int32 {
	if s.LikesImpl == nil || s.Size == 0 {
		return 0
	}
	ix := searchSparseLikes(s.Data[:s.Size], id)
	if ix < int(s.Size) && s.Data[ix].UidAndCnt>>8 == id {
		return s.Data[ix].Ts
	}
	return 0
}

func (s *Likes) Iterator() (Iterator, int32) {
	if s.Size == 0 {
		return EmptyIt, NoNext
	}
	return &LikesIterator{S: s.LikesImpl}, (s.Data[0].UidAndCnt >> 8) &^ BlockMask
}

type LikesIterator struct {
	S *LikesImpl
	B Block
	I uint16
}

func (s *LikesIterator) FetchAndNext(span int32) (*Block, int32) {
	if s.I >= s.S.Size {
		return &ZeroBlock, NoNext
	}
	ix := s.I
	for _, el := range s.S.Data[ix:s.S.Size] {
		if el.UidAndCnt>>8 <= span+BlockMask {
			break
		}
		ix++
	}
	s.B = ZeroBlock
	for _, el := range s.S.Data[ix:s.S.Size] {
		if el.UidAndCnt>>8 < span {
			break
		}
		s.B.Set(el.UidAndCnt >> 8)
		ix++
	}
	s.I = ix
	if ix < s.S.Size {
		return &s.B, (s.S.Data[ix].UidAndCnt >> 8) &^ BlockMask
	}
	return &s.B, NoNext
}
