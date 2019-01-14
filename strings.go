package main

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/funny-falcon/highloadcup2018/bitmap"

	"github.com/funny-falcon/highloadcup2018/alloc"
)

var StringAlloc alloc.Simple

const StringShards = 64
const ShardFind = (1 << 32) / StringShards

type StringsTable struct {
	Tbl     []uint32
	Arr     []StringHandle
	Null    *bitmap.Wrapper
	NotNull *bitmap.Wrapper
}

type StringHandle struct {
	Hash   uint32
	Ptr    alloc.Ptr
	Handle int32
}

type String struct {
	Len  uint8
	Data [256]uint8
}

func (h *StringHandle) Str() string {
	ustr := (*String)(StringAlloc.GetPtr(h.Ptr))
	return ustr.String()
}

func (h *StringHandle) HndlAsPtr() *alloc.Ptr {
	return (*alloc.Ptr)(unsafe.Pointer(&h.Handle))
}

func (us *String) String() string {
	sl := us.Data[:us.Len]
	str := *(*string)(unsafe.Pointer(&sl))
	return str
}

func hash(s string) uint32 {
	res := uint32(0x123456)
	for _, b := range []byte(s) {
		res ^= uint32(b)
		res *= 0x51235995
	}
	res ^= (res<<8 | res>>24) ^ (res<<19 | res>>13)
	res *= 0x62435345
	return res ^ res>>16
}

func (us *StringsTable) Insert(s string) (uint32, bool) {
	if s == "" {
		return 0, false
	}
	if len(s) > 255 {
		panic("String is too long " + s)
	}
	if len(us.Arr) >= len(us.Tbl)*5/8 {
		us.Rebalance()
	}
	h := hash(s)
	mask := uint32(len(us.Tbl) - 1)
	pos, d := h&mask, uint32(1)
	for us.Tbl[pos] != 0 {
		apos := us.Tbl[pos]
		hndl := us.Arr[apos-1]
		if hndl.Hash == h && hndl.Str() == s {
			return apos, false
		}
		pos = (pos + d) & mask
		d++
	}

	ptr := StringAlloc.Alloc(len(s) + 1)
	ustr := (*String)(StringAlloc.GetPtr(ptr))
	ustr.Len = uint8(len(s))
	copy(ustr.Data[:], s)

	us.Arr = append(us.Arr, StringHandle{Hash: h, Ptr: ptr})
	apos := uint32(len(us.Arr))
	us.Tbl[pos] = apos
	return apos, true
}

func (us *StringsTable) Find(s string) uint32 {
	if s == "" {
		return 0
	}
	h := hash(s)
	mask := uint32(len(us.Tbl) - 1)
	pos, d := h&mask, uint32(1)
	for us.Tbl[pos] != 0 {
		apos := us.Tbl[pos]
		hndl := us.Arr[apos-1]
		if hndl.Hash == h && hndl.Str() == s {
			return apos
		}
		pos = (pos + d) & mask
		d++
	}
	return 0
}

func (us *StringsTable) GetHndl(i uint32) *StringHandle {
	return &us.Arr[i-1]
}

func (us *StringsTable) GetStr(i uint32) string {
	if i == 0 {
		return ""
	}
	return us.Arr[i-1].Str()
}

func (ush *StringsTable) Rebalance() {
	newcapa := len(ush.Tbl) * 2
	if newcapa == 0 {
		newcapa = 256
	}
	mask := uint32(newcapa - 1)
	newTbl := make([]uint32, newcapa, newcapa)
	for i, hndl := range ush.Arr {
		pos, d := hndl.Hash&mask, uint32(1)
		for newTbl[pos] != 0 {
			pos = (pos + d) & mask
			d++
		}
		newTbl[pos] = uint32(i) + 1
	}
	ush.Tbl = newTbl
}

func (ush *StringsTable) SetNull(uid int32, isNull bool) {
	if ush.Null == nil {
		ush.Null = bitmap.Wrap(&BitmapAlloc, nil, bitmap.LargeEmpty)
		ush.NotNull = bitmap.Wrap(&BitmapAlloc, nil, bitmap.LargeEmpty)
	}
	if isNull {
		ush.Null.Set(uid)
		ush.NotNull.Unset(uid)
	} else {
		ush.Null.Unset(uid)
		ush.NotNull.Set(uid)
	}
}

type UniqStrings struct {
	sync.Mutex
	StringsTable
	Null    *bitmap.Wrapper
	NotNull *bitmap.Wrapper
}

func (us *UniqStrings) InsertUid(s string, uid int32) (uint32, bool) {
	if len(s) > 255 {
		panic("String is too long " + s)
	}

	us.Lock()
	defer us.Unlock()

	us.SetNull(uid, s == "")
	if s == "" {
		return 0, true
	}

	ix, isNew := us.Insert(s)
	hndl := us.GetHndl(ix)
	if isNew {
		hndl.Handle = -1
	}
	if hndl.Handle == -1 {
		hndl.Handle = uid
		return ix, true
	}
	return ix, false
}

func (us *UniqStrings) ResetUser(ix uint32, uid int32) {
	if ix == 0 {
		return
	}
	us.Lock()
	defer us.Unlock()
	hndl := us.GetHndl(ix)
	if hndl.Handle != uid {
		panic(fmt.Sprintf("User %d is not owner of string %s", uid, hndl.Str()))
	}
	hndl.Handle = -1
}

type SomeStrings struct {
	sync.Mutex
	StringsTable
	Huge         bool
	OtherStrings []string
	HugeMaps     []bitmap.Huge
}

func (ss *SomeStrings) Add(str string, uid int32) uint32 {
	if len(str) > 255 {
		panic("String is too long " + str)
	}

	ss.Lock()
	defer ss.Unlock()

	ss.SetNull(uid, str == "")
	if str == "" {
		return 0
	}

	ix, _ := ss.Insert(str)
	/*
		if int(ix) == len(ss.OtherStrings)+1 {
			ss.OtherStrings = append(ss.OtherStrings, ss.StringsTable.GetStr(ix))
		}
	*/
	if ss.Huge {
		for int(ix) >= len(ss.HugeMaps) {
			ss.HugeMaps = append(ss.HugeMaps, bitmap.Huge{})
		}
		ss.HugeMaps[ix].Set(uid)
	} else {
		hndl := ss.GetHndl(ix)
		wr := bitmap.Wrap(&BitmapAlloc, hndl.HndlAsPtr(), bitmap.LargeEmpty)
		wr.Set(uid)
	}
	return ix
}

func (ss *SomeStrings) Stat() [9]int {
	nsz, ncmp, ncnt := ss.Null.Bitmap.(*bitmap.Large).Stat()
	nnsz, nncmp, nncnt := ss.NotNull.Bitmap.(*bitmap.Large).Stat()
	var size, compact, count int
	for i := range ss.Arr {
		var sz, cmp, cnt int
		if ss.Huge {
			sz, cmp, cnt = ss.HugeMaps[i].Stat()
		} else {
			lrg := ss.GetIndex(uint32(i + 1)).(*bitmap.Wrapper).Bitmap.(*bitmap.Large)
			sz, cmp, cnt = lrg.Stat()
		}
		size += sz
		compact += cmp
		count += cnt
	}
	return [9]int{nsz, ncmp, ncnt, nnsz, nncmp, nncnt, size, compact, count}
}

type Index interface {
	Set(i int32)
	Unset(i int32)
	Iterator(m int32) bitmap.Iterator
}

func (ss *SomeStrings) GetIndex(ix uint32) Index {
	if ix == 0 {
		return ss.Null
	}
	if !ss.Huge {
		return bitmap.Wrap(&BitmapAlloc, ss.GetHndl(ix).HndlAsPtr(), bitmap.LargeEmpty)
	} else {
		return &ss.HugeMaps[ix]
	}
}

func (ss *SomeStrings) GetIter(ix uint32, max int32) bitmap.Iterator {
	if ix == 0 {
		return bitmap.EmptyIt
	}
	if !ss.Huge {
		return bitmap.Wrap(&BitmapAlloc, ss.GetHndl(ix).HndlAsPtr(), bitmap.LargeEmpty).Iterator(max)
	} else {
		return ss.HugeMaps[ix].Iterator(max)
	}
}

/*
func (ss *SomeStrings) GetStr(ix uint32) string {
	if ix == 0 {
		return ""
	}
	return ss.OtherStrings[ix-1]
}
*/
