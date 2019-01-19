package main

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"

	bitmap "github.com/funny-falcon/highloadcup2018/bitmap2"

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
	Likes         uintptr
	Interests     bitmap.Block
}

func (acc *Account) SetInterest(ix uint32) {
	acc.Interests[ix/64] |= 1 << (ix & 63)
}

var Accounts = make([]Account, 1536*1024)
var AccountsMap bitmap.Huge
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
	AccountsMap.Set(i)
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

var EmailGtIndexes [26]bitmap.Bitmap
var EmailLtIndexes [26]bitmap.Bitmap

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

var BirthYearIndexes [60]bitmap.Bitmap

func GetBirthYear(ts int32) int32 {
	return int32(time.Unix(int64(ts), 0).Year() - 1950)
}

var JoinYearIndexes [10]bitmap.Bitmap

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

//var InterestStrings = SomeStrings{Huge: true}
var InterestStrings = SomeStrings{}

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

var Likers = make([]uintptr, 1536*1024)

func SureLikers(i int32) *bitmap.Likes {
	if int(i) >= len(Likers) {
		ln := int32(1)
		for ; ln < i; ln *= 2 {
		}
		newLikers := make([]uintptr, ln, ln)
		copy(newLikers, Likers)
		Likers = newLikers
	}
	return bitmap.GetLikes(&Likers[i])
}

func GetLikers(i int32) *bitmap.Likes {
	if int(i) >= len(Likers) {
		return nil
	}
	if Likers[i] == 0 {
		return nil
	}
	return bitmap.GetLikes(&Likers[i])
}

const LikeUidShift = 8
const LikeUidMask = (^int32(0)) << LikeUidShift
const LikeCntMask = (1 << LikeUidShift) - 1
