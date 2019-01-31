package main

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	bitmap "github.com/funny-falcon/highloadcup2018/bitmap3"
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
}

type SmallAccount struct {
	Birth int32
	SmallerAccount
}

type SmallerAccount struct {
	City             uint16
	Country          uint8
	StatusSexPremium uint8
}

func (a *Account) SmallAccount() SmallAccount {
	s := SmallAccount{
		SmallerAccount: SmallerAccount{City: a.City, Country: a.Country, StatusSexPremium: a.Status},
		Birth:          a.Birth,
	}
	if a.Sex {
		s.StatusSexPremium |= 4
	}
	if a.PremiumNow {
		s.StatusSexPremium |= 8
	}
	return s
}

func (s SmallerAccount) Status() int {
	return int(s.StatusSexPremium & 3)
}

func (s SmallerAccount) Sex() bool {
	return s.StatusSexPremium&4 != 0
}

func (s SmallerAccount) SexIx() int {
	return int((s.StatusSexPremium & 4) >> 2)
}

func (s SmallerAccount) Premium() bool {
	return s.StatusSexPremium&8 != 0
}

func (a *Account) SexIx() int {
	if a.Sex {
		return 1
	}
	return 0
}

func (a *Account) StatusIx() int {
	return int(a.Status - 1)
}

/*
func (acc *Account) SetInterest(ix uint32) {
	acc.Interests[ix/64] |= 1 << (ix & 63)
}
*/

const Init = 1536 * 1024

var Accounts = make([]Account, Init)
var SmallAccounts = make([]SmallAccount, Init)
var SmallerAccounts = make([]SmallerAccount, Init)
var AccountsMap bitmap.Bitmap
var MaxId int32

func SureCapa(slicePtr interface{}, capa int) {
	val := reflect.ValueOf(slicePtr)
	tpe := reflect.TypeOf(slicePtr).Elem()
	newVal := reflect.MakeSlice(tpe, capa, capa)
	reflect.Copy(newVal, val.Elem())
	val.Set(newVal)
}

func SureAccount(i int32) *Account {
	if int(i) >= len(Accounts) {
		ln := int32(1)
		for ; ln < i; ln *= 2 {
		}
		if ln-ln/4 > i {
			ln -= ln / 4
		}
		SureCapa(&Accounts, int(ln))
		SureCapa(&SmallAccounts, int(ln))
		SureCapa(&SmallerAccounts, int(ln))
		SureCapa(&Interests, int(ln))
	}
	if i >= MaxId {
		MaxId = i + 1
	}
	AccountsMap.Set(i)
	acc := &Accounts[i]
	acc.Uid = int32(i)
	return acc
}

func HasAccount(i int32) *Account {
	if i >= MaxId {
		//log.Printf("i > MaxId : %d > %d, Acc.Has:%v", i, MaxId, AccountsMap.Has(i))
		return nil
	}
	if Accounts[i].Uid == 0 {
		//log.Printf("Acc[%d].Uid == 0, Acc.Has:%v", i, AccountsMap.Has(i))
		return nil
	}
	return &Accounts[i]
}

func RefAccount(i int32) *Account {
	return &Accounts[i]
}

func SetSmallAccount(i int32, s SmallAccount) {
	SmallAccounts[i] = s
	SmallerAccounts[i] = s.SmallerAccount
}

func GetSmallAccount(i int32) SmallAccount {
	return SmallAccounts[i]
}

func GetSmallerAccount(i int32) SmallerAccount {
	return SmallerAccounts[i]
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

//var MaleMap = bitmap.Bitmap{}
//var FemaleMap = bitmap.Bitmap{}
var MaleMap = bitmap.SexMap{Mask: bitmap.MaleMask}
var FemaleMap = bitmap.SexMap{Mask: bitmap.FemaleMask}
var FreeMap = bitmap.Bitmap{}
var MeetingMap = bitmap.Bitmap{}
var ComplexMap = bitmap.Bitmap{}
var FreeOrMeetingMap = bitmap.Bitmap{}
var MeetingOrComplexMap = bitmap.Bitmap{}
var FreeOrComplexMap = bitmap.Bitmap{}
var StatusMaps = func() [4][3]*bitmap.Bitmap {
	r := [4][3]*bitmap.Bitmap{
		{},
	}
	r[StatusFreeIx] = [3]*bitmap.Bitmap{&FreeMap, &FreeOrMeetingMap, &FreeOrComplexMap}
	r[StatusComplexIx] = [3]*bitmap.Bitmap{&ComplexMap, &FreeOrComplexMap, &MeetingOrComplexMap}
	r[StatusMeetingIx] = [3]*bitmap.Bitmap{&MeetingMap, &FreeOrMeetingMap, &MeetingOrComplexMap}
	return r
}()

var PremiumNow = bitmap.Bitmap{}
var PremiumNotNow = bitmap.Bitmap{}
var PremiumNull = bitmap.Bitmap{}
var PremiumNotNull = bitmap.Bitmap{}

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

var BirthYearIndexes [61]bitmap.Bitmap

func GetBirthYear(ts int32) int32 {
	return int32(time.Unix(int64(ts), 0).UTC().Year() - 1950)
}

var JoinYearIndexes [10]bitmap.Bitmap

func GetJoinYear(ts int32) int32 {
	return int32(time.Unix(int64(ts), 0).UTC().Year() - 2011)
}

var DomainsStrings = SomeStrings{Huge: true}
var PhoneCodesStrings = SomeStrings{}
var FnameStrings = SomeStrings{}
var SnameStrings SomeStrings
var SnameSorted SnameSorting
var SnameOnce = NewOnce(SnameSorted.Init)
var CityStrings SomeStrings
var CountryStrings SomeStrings

//var InterestStrings = SomeStrings{}
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
	switch {
	case l >= 4:
		r = uint32(s[3])
		fallthrough
	case l >= 3:
		r |= uint32(s[2]) << 8
		fallthrough
	case l >= 2:
		r |= uint32(s[1]) << 16
		fallthrough
	case l >= 1:
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

func SureLikers(i int32, f func(*bitmap.Likes)) {
	if int(i) >= len(Likers) {
		ln := int32(1)
		for ; ln < i; ln *= 2 {
		}
		newLikers := make([]uintptr, ln, ln)
		copy(newLikers, Likers)
		Likers = newLikers
	}
	f(bitmap.GetLikes(&Likers[i]))
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

var CityGroups [1000][6]uint32
var CountryGroups [100][6]uint32
var InterestJoinedGroups [10][100]uint32
var InterestBirthGroups [61][100]uint32
var InterestCountryGroups [100][100]uint32
