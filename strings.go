package main

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/funny-falcon/highloadcup2018/alloc2"
	bitmap "github.com/funny-falcon/highloadcup2018/bitmap3"
)

var StringAlloc alloc2.Simple

const StringShards = 64
const ShardFind = (1 << 32) / StringShards

type StringsTable struct {
	Tbl     []uint32
	Arr     []StringHandle
	Null    bitmap.Bitmap
	NotNull bitmap.Bitmap
}

type StringHandle struct {
	Ptr    uintptr
	Handle uintptr
}

type String struct {
	Hash uint32
	Len  uint8
	Data [256]uint8
}

func (h *StringHandle) Hash() uint32 {
	ustr := (*String)(unsafe.Pointer(h.Ptr))
	return ustr.Hash
}

func (h *StringHandle) Str() string {
	ustr := (*String)(unsafe.Pointer(h.Ptr))
	return ustr.String()
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
		if hndl.Hash() == h && hndl.Str() == s {
			return apos, false
		}
		pos = (pos + d) & mask
		d++
	}

	ptr := StringAlloc.Alloc(len(s) + 1 + 4)
	ustr := (*String)(ptr)
	ustr.Hash = h
	ustr.Len = uint8(len(s))
	copy(ustr.Data[:], s)

	us.Arr = append(us.Arr, StringHandle{Ptr: uintptr(ptr)})
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
		if hndl.Hash() == h && hndl.Str() == s {
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
		pos, d := hndl.Hash()&mask, uint32(1)
		for newTbl[pos] != 0 {
			pos = (pos + d) & mask
			d++
		}
		newTbl[pos] = uint32(i) + 1
	}
	ush.Tbl = newTbl
}

func (ush *StringsTable) SetNull(uid int32, isNull bool) {
	if isNull {
		ush.Null.Set(uid)
		ush.NotNull.Unset(uid)
	} else {
		ush.Null.Unset(uid)
		ush.NotNull.Set(uid)
	}
}

type UniqStrings struct {
	StringsTable
}

func (us *UniqStrings) InsertUid(s string, uid int32) (uint32, bool) {
	if len(s) > 255 {
		panic("String is too long " + s)
	}

	us.SetNull(uid, s == "")
	if s == "" {
		return 0, true
	}

	ix, isNew := us.Insert(s)
	hndl := us.GetHndl(ix)
	if isNew {
		hndl.Handle = 0
	}
	if hndl.Handle == 0 {
		hndl.Handle = uintptr(uid)
		return ix, true
	}
	return ix, false
}

func (us *UniqStrings) IsFree(email string) bool {
	ix := us.Find(email)
	if ix == 0 {
		return true
	}
	return us.GetHndl(ix).Handle == 0
}

func (us *UniqStrings) ResetUser(ix uint32, uid int32) {
	if ix == 0 {
		return
	}
	hndl := us.GetHndl(ix)
	if hndl.Handle != uintptr(uid) {
		panic(fmt.Sprintf("User %d is not owner of string %s", uid, hndl.Str()))
	}
	hndl.Handle = 0
}

type SomeStrings struct {
	sync.Mutex
	StringsTable
	Huge bool
	Maps []*bitmap.Bitmap
}

func (ss *SomeStrings) Add(str string, uid int32) uint32 {
	if len(str) > 255 {
		panic("String is too long " + str)
	}

	ss.SetNull(uid, str == "")
	if str == "" {
		return 0
	}

	ix, _ := ss.Insert(str)
	for int(ix) > len(ss.Maps) {
		ss.Maps = append(ss.Maps, &bitmap.Bitmap{})
	}
	ss.Maps[ix-1].Set(uid)
	return ix
}

/*
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
*/

func (ss *SomeStrings) GetMap(ix uint32) *bitmap.Bitmap {
	if ix == 0 {
		return nil
	}
	return ss.Maps[ix-1]
}

func (ss *SomeStrings) Set(ix uint32, i int32) {
	ss.GetMap(ix).Set(i)
}

func (ss *SomeStrings) Unset(ix uint32, i int32) {
	ss.GetMap(ix).Unset(i)
}
