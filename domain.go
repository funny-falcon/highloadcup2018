package main

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/funny-falcon/highloadcup2018/bitmap"

	"github.com/funny-falcon/highloadcup2018/alloc"
)

const (
	SexBitsMale = 0x01
	SexMale     = "m"
	SexFemale   = "f"

	StatusBitsMask    = 0x06
	StatusBitsFree    = 0x02
	StatusBitsMeeting = 0x04
	StatusBitsComplex = 0x06
	StatusComplexIx   = 1
	StatusMeetingIx   = 2
	StatusFreeIx      = 3
	StatusComplex     = "всё сложно"
	StatusMeeting     = "заняты"
	StatusFree        = "свободны"

	StatusPremiumNow   = 0x08
	StatusPremiumHas   = 0x10
	StatusPremiumMask  = 0x60
	StatusPremiumShift = 0x20
)

var PremiumLengths = [5]int32{0, 30 * 24 * 3600, 91 * 24 * 3600, 182 * 24 * 3600, 365 * 24 * 3600}

var CurTs int32

type Account struct {
	Uid           int32
	Email         uint32
	EmailStart    uint32
	Phone         uint32
	Domain        uint8
	Code          uint8
	Sex           bool
	Status        uint8
	Sname         uint16
	Fname         uint8
	Country       uint8
	City          uint16
	PremiumNow    bool
	PremiumLength uint8
	PremiumStart  int32
	Birth         int32
	Joined        int32
	Likes         alloc.Ptr
	Interests     [4]uint32
}

func (acc *Account) SetInterest(ix uint32) {
	acc.Interests[ix/32] |= 1 << (ix & 31)
}

var Accounts []Account
var MaxId int32

func SureAccount(i int32) *Account {
	if int(i) >= len(Accounts) {
		ln := int32(1)
		for ; ln < i; ln *= 2 {
		}
		if ln-ln/4 > i {
			ln -= ln / 4
		}
		newAccs := make([]Account, ln, ln)
		copy(newAccs, Accounts)
		Accounts = newAccs
	}
	acc := &Accounts[i]
	acc.Uid = int32(i)
	return acc
}

func HasAccount(i int32) *Account {
	if int(i) >= len(Accounts) {
		return nil
	}
	if Accounts[i].Uid == 0 {
		return nil
	}
	return &Accounts[i]
}

func DomainFromEmail(e string) string {
	ix := strings.LastIndexByte(e, '@')
	return e[ix+1:]
}

func CodeFromPhone(p string) string {
	ixl := strings.IndexByte(p, '(')
	ixr := strings.IndexByte(p, ')')
	return p[ixl+1 : ixr]
}

var EmailIndex UniqStrings
var PhoneIndex UniqStrings

var BitmapAlloc alloc.Simple

var MaleMap = bitmap.Huge{}
var FemaleMap = bitmap.Huge{}
var FreeMap = bitmap.Huge{}
var MeetingMap = bitmap.Huge{}
var ComplexMap = bitmap.Huge{}
var FreeOrMeetingMap = bitmap.Huge{}
var MeetingOrComplexMap = bitmap.Huge{}
var FreeOrComplexMap = bitmap.Huge{}

var PremiumNow = bitmap.Huge{}
var PremiumNull = bitmap.Huge{}
var PremiumNotNull = bitmap.Huge{}

var EmailGtIndexes [26]bitmap.Huge
var EmailLtIndexes [26]bitmap.Huge

func IndexGtLtEmail(e string, uid int32, set bool) {
	ch := e[0]
	// lt
	start, end := int(ch)-'a', 25
	if start < 0 {
		start = 0
	}
	for ; start <= end; start++ {
		if set {
			EmailLtIndexes[start].Set(uid)
		} else {
			EmailLtIndexes[start].Unset(uid)
		}
	}
	// gt
	start, end = 0, int(ch)-'a'
	if end > 25 {
		end = 25
	}
	for ; start <= end; start++ {
		if set {
			EmailGtIndexes[start].Set(uid)
		} else {
			EmailGtIndexes[start].Unset(uid)
		}
	}
}

var BirthYearIndexes = func() []*bitmap.Wrapper {
	res := make([]*bitmap.Wrapper, 60)
	for i := range res {
		res[i] = bitmap.Wrap(&BitmapAlloc, nil, bitmap.LargeEmpty)
	}
	return res
}()

func GetBirthYear(ts int32) int32 {
	return int32(time.Unix(int64(ts), 0).Year() - 1950)
}

var JoinYearIndexes = func() []*bitmap.Wrapper {
	res := make([]*bitmap.Wrapper, 10)
	for i := range res {
		res[i] = bitmap.Wrap(&BitmapAlloc, nil, bitmap.LargeEmpty)
	}
	return res
}()

func GetJoinYear(ts int32) int32 {
	return int32(time.Unix(int64(ts), 0).Year() - 2011)
}

var DomainsStrings = SomeStrings{Huge: true}
var PhoneCodesStrings = SomeStrings{}
var FnameStrings = SomeStrings{}
var SnameStrings SomeStrings
var SnameSorted SnameSorting
var SnameOnce = NewOnce(SnameSorted.Init)
var CityStrings SomeStrings
var CountryStrings SomeStrings
var InterestStrings = SomeStrings{Huge: true}

func GetStatusIx(status string) (uint8, bool) {
	switch status {
	case StatusFree:
		return StatusFreeIx, true
	case StatusMeeting:
		return StatusMeetingIx, true
	case StatusComplex:
		return StatusComplexIx, true
	default:
		return 0, false
	}
}

func GetStatus(status uint8) string {
	switch status {
	case StatusFreeIx:
		return StatusFree
	case StatusMeetingIx:
		return StatusMeeting
	case StatusComplexIx:
		return StatusComplex
	default:
		panic(fmt.Sprintf("Unknown statusIx %d", status))
	}
}

func GetPremiumLength(start, finish int32) uint8 {
	switch lngth := finish - start; lngth {
	case 0:
		return 0
	case 30 * 24 * 3600:
		return 1
	case 91 * 24 * 3600:
		return 2
	case 182 * 24 * 3600:
		return 3
	case 365 * 24 * 3600:
		return 4
	default:
		panic(fmt.Sprintf("wrong premium length %d %d %d", lngth, start, finish))
	}
}

func GetEmailStart(s string) uint32 {
	if len(s) >= 4 {
		return binary.BigEndian.Uint32([]byte(s))
	}
	return GetEmailPrefix(s)
}

func GetEmailPrefix(s string) uint32 {
	l := len(s)
	r := uint32(0)
	if l >= 4 {
		r = uint32(s[3])
	}
	if l == 3 {
		r |= uint32(s[2]) << 8
	}
	if l == 2 {
		r |= uint32(s[1]) << 16
	}
	if l == 1 {
		r |= uint32(s[0]) << 24
	}
	return r
}

/*
func GetEmailLte(s string) uint32 {
	l := len(s)
	r := uint32(0)
	if l >= 4 {
		r = uint32(s[3])
	} else {
		r = 0xff
	}
	if l == 3 {
		r |= uint32(s[2]) << 8
	} else {
		r |= 0xff00
	}
	if l == 2 {
		r |= uint32(s[1]) << 16
	} else {
		r |= 0xff0000
	}
	if l == 1 {
		r |= uint32(s[0]) << 24
	} else {
		r |= 0xff000000
	}
	return r
}
*/

type SnameSorting struct {
	Ix  []uint32
	Str []string
}

func (s *SnameSorting) Init() {
	s.Ix = make([]uint32, len(SnameStrings.Arr))
	s.Str = make([]string, len(SnameStrings.Arr))
	for i := range s.Ix {
		s.Ix[i] = uint32(i + 1)
		s.Str[i] = SnameStrings.GetStr(uint32(i + 1))
	}
	sort.Sort(s)
}

func (s *SnameSorting) Len() int           { return len(s.Ix) }
func (s *SnameSorting) Less(i, j int) bool { return s.Str[i] < s.Str[j] }
func (s *SnameSorting) Swap(i, j int) {
	s.Ix[i], s.Ix[j] = s.Ix[j], s.Ix[i]
	s.Str[i], s.Str[j] = s.Str[j], s.Str[i]
}

func (s *SnameSorting) PrefixRange(pref string) (i, j int) {
	i = sort.Search(len(s.Str), func(i int) bool {
		return pref <= s.Str[i]
	})
	for j = i; j < len(s.Str) && strings.HasPrefix(s.Str[j], pref); j++ {
	}
	return
}

var Likers []alloc.Ptr

var LikersAlloc alloc.Simple

func SureLikers(i int32) *bitmap.Wrapper {
	if int(i) >= len(Likers) {
		ln := int32(1)
		for ; ln < i; ln *= 2 {
		}
		newLikers := make([]alloc.Ptr, ln, ln)
		copy(newLikers, Likers)
		Likers = newLikers
	}
	return bitmap.Wrap(&BitmapAlloc, &Likers[i], bitmap.SmallEmpty)
}

func GetLikers(i int32) *bitmap.Wrapper {
	if int(i) >= len(Likers) {
		return nil
	}
	if Likers[i] == 0 {
		return nil
	}
	return bitmap.Wrap(&BitmapAlloc, &Likers[i], bitmap.SmallEmpty)
}

type Likes struct {
	Cnt   uint16
	Cap   uint16
	Likes [256]Like
}

const LikeUidShift = 8
const LikeUidMask = (^int32(0)) << LikeUidShift
const LikeCntMask = (1 << LikeUidShift) - 1

type Like struct {
	UidAndCnt int32
	AvgTs     int32
}

var LikesAlloc alloc.Simple

func (lks *Likes) Simplify() {
	sort.Slice(lks.Likes[:lks.Cnt], func(i, j int) bool {
		return lks.Likes[i].UidAndCnt < lks.Likes[j].UidAndCnt
	})
	j := 0
	for i := 1; i < int(lks.Cnt); i++ {
		if lks.Likes[i].UidAndCnt&LikeUidMask == lks.Likes[j].UidAndCnt&LikeUidMask {
			lks.Likes[j].AddOne(lks.Likes[i].AvgTs)
		} else {
			j++
			lks.Likes[j] = lks.Likes[i]
		}
	}
	lks.Cnt = uint16(j)
}

func (lks *Likes) Alloc() alloc.Ptr {
	cap := (lks.Cnt + 2) &^ 3
	var alks *Likes
	ptr := LikesAlloc.Alloc(4 + 8*int(cap))
	LikesAlloc.Get(ptr, &alks)
	alks.Cnt = lks.Cnt
	alks.Cap = cap
	copy(alks.Likes[:lks.Cnt], lks.Likes[:lks.Cnt])
	return ptr
}

func (lks *Likes) Insert(old alloc.Ptr, uid int32, ts int32) alloc.Ptr {
	uidShift := uid << LikeUidShift
	i := sort.Search(int(lks.Cnt), func(i int) bool {
		return uidShift <= lks.Likes[i].UidAndCnt
	})
	if i < int(lks.Cnt) && lks.Likes[i].UidAndCnt&LikeUidMask == uidShift {
		lks.Likes[i].AddOne(ts)
		return old
	}
	ptr := old
	if lks.Cnt == lks.Cap {
		ptr = lks.Alloc()
		var nlks *Likes
		LikesAlloc.Get(ptr, &nlks)
		lks = nlks
	}
	copy(lks.Likes[i+1:], lks.Likes[i:lks.Cnt])
	lks.Likes[i] = Like{UidAndCnt: uidShift, AvgTs: ts}
	return ptr
}

func (lks *Likes) Similarity(oth *Likes) float64 {
	i, j := 0, 0
	la, lb := int(lks.Cnt), int(oth.Cnt)
	sum := float64(0)
	for i < la && j < lb {
		lka := lks.Likes[i]
		lkb := lks.Likes[j]
		uida := lka.UidAndCnt & LikeUidMask
		uidb := lkb.UidAndCnt & LikeUidMask
		if uida == uidb {
			tsdiff := int(lka.AvgTs) - int(lkb.AvgTs)
			if tsdiff < 0 {
				tsdiff = -tsdiff
			} else if tsdiff == 0 {
				tsdiff = 1
			}
			sum += 1.0 / float64(tsdiff)
		} else if uida < uidb {
			i++
		} else {
			j++
		}
	}
	return sum
}

func (lk *Like) AddOne(othTs int32) {
	oldCnt := uint64((lk.UidAndCnt & LikeCntMask) + 1)
	tsSum := uint64(lk.AvgTs)*oldCnt + uint64(othTs)
	lk.AvgTs = int32(tsSum / (oldCnt + 1))
	lk.UidAndCnt++
}
