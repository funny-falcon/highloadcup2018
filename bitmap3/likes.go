package bitmap3

import (
	"unsafe"

	"github.com/funny-falcon/highloadcup2018/alloc2"
)

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
	Uid int32
	Ts  int32
}

func GetLikes(p *uintptr) *Likes {
	return (*Likes)(unsafe.Pointer(p))
}

func (s *Likes) Uintptr() uintptr {
	return uintptr(unsafe.Pointer(s.LikesImpl))
}

var LikesCnt = make(map[[2]int32]int32)

func (s *Likes) SetTs(likee, liker int32, ts int32) {
	if s.LikesImpl == nil {
		ncap := uint16(2)
		s.LikesImpl = (*LikesImpl)(LikesAlloc.Alloc(4 + int(ncap)*8))
		s.Size = 1
		s.Cap = ncap
		s.Data[0] = LikesElem{liker, ts}
		return
	}
	ix := searchSparseLikes(s.Data[:s.Size], liker)
	if ix < int(s.Size) && s.Data[ix].Uid == liker {
		el := &s.Data[ix]
		if el.Ts > 0 {
			LikesCnt[[2]int32{likee, liker}] = 2
			el.Ts = -int32((int64(el.Ts) + int64(ts)) / 2)
		} else {
			cnt := int64(LikesCnt[[2]int32{likee, liker}])
			LikesCnt[[2]int32{likee, liker}] = int32(cnt + 1)
			el.Ts = -int32((int64(-el.Ts)*cnt + int64(ts)) / (cnt + 1))
		}
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
	s.Data[ix] = LikesElem{liker, ts}
	s.Size++
}

func (s *Likes) GetTs(id int32) int32 {
	if s.LikesImpl == nil || s.Size == 0 {
		return 0
	}
	ix := searchSparseLikes(s.Data[:s.Size], id)
	if ix < int(s.Size) && s.Data[ix].Uid>>8 == id {
		return s.Data[ix].Ts
	}
	return 0
}

func AndLikes(likes []*Likes) []int32 {
	res := make([]int32, 0, 50)
	slices := make([][]LikesElem, len(likes))
	for i, l := range likes {
		slices[i] = l.Data[:l.Size]
	}
	cur := int32(1 << 30)
	curcnt := 0
	cursl := 0
	for {
		curSlice := slices[cursl]
		for len(curSlice) > 0 && curSlice[0].Uid > cur {
			curSlice = curSlice[1:]
		}
		if len(curSlice) == 0 {
			break
		}
		if curSlice[0].Uid == cur {
			curcnt++
			if curcnt == len(likes) {
				res = append(res, cur)
				cur = cur - 1
				curcnt = 0
			}
		} else { //if curSlice[0].Uid < cur
			cur = curSlice[0].Uid
			curcnt = 1
		}
		slices[cursl] = curSlice[1:]
		if cursl++; cursl == len(slices) {
			cursl = 0
		}
	}
	return res
}
