package main

import (
	"bytes"
	"math/bits"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/funny-falcon/highloadcup2018/bitmap"

	"github.com/valyala/fasthttp"
)

var FilterPath = []byte("/accounts/filter/")
var GroupsPath = []byte("/accounts/group/")

func getHandler(ctx *fasthttp.RequestCtx) {
	path := ctx.Path()
	logf("ctx.Path: %s, args: %s", path, ctx.QueryArgs())
	switch {
	case bytes.Equal(path, FilterPath):
		doFilter(ctx)
	case bytes.Equal(path, GroupsPath):
		doGroup(ctx)
	}

}

type OutFields struct {
	Sex     bool
	Status  bool
	Fname   bool
	Sname   bool
	Phone   bool
	Country bool
	City    bool
	Birth   bool
	Joined  bool
	Premium bool
}

var EmptyFilterRes = []byte(`{"accounts":[]}`)
var EmptyGroupRes = []byte(`{"groups":[]}`)

func logf(format string, args ...interface{}) {
	//log.Printf(format, args...)
}

func doFilter(ctx *fasthttp.RequestCtx) {
	args := ctx.QueryArgs()
	iterators := make([]bitmap.Iterator, 0, 4)
	filters := []func(*Account) bool{}

	correct := true
	emptyRes := false
	limit := -1
	var outFields OutFields

	args.VisitAll(func(key []byte, val []byte) {
		if !correct {
			return
		}
		logf("arg %s: %s", key, val)
		if bytes.Equal(key, []byte("limit")) {
			var err error
			limit, err = strconv.Atoi(string(val))
			if err != nil {
				logf("limit: %s", err)
				correct = false
			} else if limit == 0 {
				emptyRes = true
			}
			return
		}
		skey := string(key)
		sval := string(val)
		switch skey {
		case "sex_eq":
			outFields.Sex = true
			switch sval {
			case "m":
				iterators = append(iterators, MaleMap.Iterator(MaxId))
			case "f":
				iterators = append(iterators, FemaleMap.Iterator(MaxId))
			default:
				logf("sex_eq incorrect")
				correct = false
			}
		case "email_domain":
			domain := sval
			ix := DomainsStrings.Find(domain)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterator := DomainsStrings.GetIter(ix, MaxId)
			iterators = append(iterators, iterator)
		case "email_gt":
			if len(val) == 0 {
				return // all are greater
			}
			email := sval
			emailgt := GetEmailPrefix(email)
			filters = append(filters, func(acc *Account) bool {
				return acc.EmailStart >= emailgt
			})
			if len(email) > 4 {
				filters = append(filters, func(acc *Account) bool {
					accEmail := EmailIndex.GetStr(acc.Email)
					return accEmail > email
				})
			}
			chix := int(email[0]) - 'a'
			if chix < 0 {
				return
			} else if chix > 25 {
				chix = 25
			}
			iterators = append(iterators, EmailGtIndexes[chix].Iterator(MaxId))
		case "email_lt":
			if len(val) == 0 {
				emptyRes = true
				return // all are greater
			}
			email := sval
			emaillt := GetEmailPrefix(email)
			filters = append(filters, func(acc *Account) bool {
				return acc.EmailStart < emaillt
			})
			if len(email) > 4 {
				filters = append(filters, func(acc *Account) bool {
					accEmail := EmailIndex.GetStr(acc.Email)
					return accEmail < email
				})
			}
			chix := int(email[0]) - 'a'
			if chix < 0 {
				chix = 0
			} else if chix > 25 {
				return
			}
			iterators = append(iterators, EmailLtIndexes[chix].Iterator(MaxId))
		case "status_eq":
			outFields.Status = true
			switch sval {
			case StatusFree:
				iterators = append(iterators, FreeMap.Iterator(MaxId))
			case StatusMeeting:
				iterators = append(iterators, MeetingMap.Iterator(MaxId))
			case StatusComplex:
				iterators = append(iterators, ComplexMap.Iterator(MaxId))
			default:
				logf("status_eq incorrect")
				correct = false
				return
			}
		case "status_neq":
			outFields.Status = true
			switch sval {
			case StatusFree:
				iterators = append(iterators, MeetingOrComplexMap.Iterator(MaxId))
			case StatusMeeting:
				iterators = append(iterators, FreeOrComplexMap.Iterator(MaxId))
			case StatusComplex:
				iterators = append(iterators, FreeOrMeetingMap.Iterator(MaxId))
			default:
				logf("status_neq incorrect")
				correct = false
				return
			}
		case "fname_eq":
			outFields.Fname = true
			ix := FnameStrings.Find(sval)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, FnameStrings.GetIter(ix, MaxId))
		case "fname_any":
			outFields.Fname = true
			names := strings.Split(sval, ",")
			orIters := make([]bitmap.Iterator, 0, len(names))
			for _, name := range names {
				ix := FnameStrings.Find(name)
				if ix == 0 {
					continue
				}
				orIters = append(orIters, FnameStrings.GetIter(ix, MaxId))
			}
			if len(orIters) == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, bitmap.NewOrIterator(orIters))
		case "fname_null":
			outFields.Fname = true
			switch sval {
			case "1":
				iterators = append(iterators, FnameStrings.Null.GetIterator(MaxId))
			case "0":
				filters = append(filters, func(acc *Account) bool {
					return acc.Fname != 0
				})
			default:
				logf("fname_null incorrect")
				correct = false
			}
		case "sname_eq":
			outFields.Sname = true
			ix := SnameStrings.Find(sval)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, SnameStrings.GetIter(ix, MaxId))
		case "sname_starts":
			outFields.Sname = true
			SnameOnce.Sure()
			pref := sval
			i, j := SnameSorted.PrefixRange(pref)
			if i == j {
				emptyRes = true
				return
			}
			orIters := make([]bitmap.Iterator, j-i)
			for k := i; k < j; k++ {
				orIters[k-i] = SnameStrings.GetIndex(SnameSorted.Ix[k]).Iterator(MaxId)
			}
			iterators = append(iterators, bitmap.NewOrIterator(orIters))
		case "sname_null":
			outFields.Sname = true
			switch sval {
			case "1":
				iterators = append(iterators, SnameStrings.Null.GetIterator(MaxId))
			case "0":
				filters = append(filters, func(acc *Account) bool {
					return acc.Sname != 0
				})
			default:
				logf("sname_null incorrect")
				correct = false
			}
		case "phone_code":
			outFields.Phone = true
			code := sval
			ix := PhoneCodesStrings.Find(code)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, PhoneCodesStrings.GetIter(ix, MaxId))
		case "phone_null":
			outFields.Phone = true
			switch sval {
			case "1":
				iterators = append(iterators, PhoneIndex.Null.GetIterator(MaxId))
			case "0":
				iterators = append(iterators, PhoneIndex.NotNull.GetIterator(MaxId))
			default:
				logf("phone_null incorrect")
				correct = false
			}
		case "country_eq":
			outFields.Country = true
			ix := CountryStrings.Find(sval)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, CountryStrings.GetIter(ix, MaxId))
		case "country_null":
			outFields.Country = true
			switch sval {
			case "1":
				iterators = append(iterators, CountryStrings.Null.GetIterator(MaxId))
			case "0":
				iterators = append(iterators, CountryStrings.NotNull.GetIterator(MaxId))
			default:
				logf("country_null incorrect")
				correct = false
			}
		case "city_eq":
			outFields.City = true
			ix := CityStrings.Find(sval)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, CityStrings.GetIter(ix, MaxId))
		case "city_any":
			outFields.City = true
			cities := strings.Split(sval, ",")
			orIters := make([]bitmap.Iterator, 0, len(cities))
			for _, name := range cities {
				ix := CityStrings.Find(name)
				if ix == 0 {
					continue
				}
				orIters = append(orIters, CityStrings.GetIter(ix, MaxId))
			}
			if len(orIters) == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, bitmap.NewOrIterator(orIters))
		case "city_null":
			outFields.City = true
			switch sval {
			case "1":
				iterators = append(iterators, CityStrings.Null.GetIterator(MaxId))
			case "0":
				iterators = append(iterators, CityStrings.NotNull.GetIterator(MaxId))
			default:
				logf("city_null incorrect")
				correct = false
			}
		case "birth_gt":
			outFields.Birth = true
			n, err := strconv.Atoi(sval)
			if err != nil {
				logf("birth_gt incorrect")
				correct = false
				return
			}
			birth := int32(n)
			filters = append(filters, func(acc *Account) bool {
				return acc.Birth > birth
			})
			birthYear := GetBirthYear(birth)
			if birthYear < 1995-1950 {
				return
			}
			orIters := make([]bitmap.Iterator, 0, len(BirthYearIndexes)-int(birthYear)+1)
			for ; int(birthYear) < len(BirthYearIndexes); birthYear++ {
				orIters = append(orIters, BirthYearIndexes[birthYear].Iterator(MaxId))
			}
			iterators = append(iterators, bitmap.NewOrIterator(orIters))
		case "birth_lt":
			outFields.Birth = true
			n, err := strconv.Atoi(sval)
			if err != nil {
				logf("birth_lt incorrect")
				correct = false
				return
			}
			birth := int32(n)
			filters = append(filters, func(acc *Account) bool {
				return acc.Birth > birth
			})
			birthYear := GetBirthYear(birth)
			if birthYear > 1988-1950 {
				return
			}
			orIters := make([]bitmap.Iterator, 0, birthYear+1)
			for ; birthYear >= 0; birthYear-- {
				orIters = append(orIters, BirthYearIndexes[birthYear].Iterator(MaxId))
			}
			iterators = append(iterators, bitmap.NewOrIterator(orIters))
		case "birth_year":
			outFields.Birth = true
			year, err := strconv.Atoi(sval)
			if err != nil {
				logf("birth_year incorrect")
				correct = false
				return
			}
			if year < 1950 || year-1950 > len(BirthYearIndexes) {
				emptyRes = true
				return
			}
			iterators = append(iterators, BirthYearIndexes[year].Iterator(MaxId))
		case "interests_contains", "interests_any":
			interests := strings.Split(sval, ",")
			iters := make([]bitmap.Iterator, 0, len(interests))
			for _, interest := range interests {
				ix := InterestStrings.Find(interest)
				if ix == 0 {
					if skey == "interests_contains" {
						emptyRes = true
						return
					}
					continue
				}
				iters = append(iters, InterestStrings.GetIter(ix, MaxId))
			}
			if len(iters) == 0 {
				if skey == "interests_any" {
					emptyRes = true
				}
				return
			}
			if skey == "interests_any" {
				iterators = append(iterators, bitmap.NewOrIterator(iters))
			} else {
				iterators = append(iterators, iters...)
			}
		case "likes_contains":
			likesStrs := bytes.Split(val, []byte(","))
			for _, likeS := range likesStrs {
				n, err := strconv.Atoi(string(likeS))
				if err != nil || n <= 0 || n >= int(MaxId) {
					logf("likes_contains incorrect")
					correct = false
					return
				}
				w := GetLikers(int32(n))
				if w == nil {
					emptyRes = true
					return
				}
				iterators = append(iterators, w.Iterator(MaxId))
			}
		case "premium_now":
			outFields.Premium = true
			iterators = append(iterators, PremiumNow.Iterator(MaxId))
		case "premium_null":
			outFields.Premium = true
			switch sval {
			case "1":
				filters = append(filters, func(acc *Account) bool {
					return acc.PremiumLength != 0
				})
			case "0":
				iterators = append(iterators, PremiumNotNull.GetIterator(MaxId))
			default:
				logf("premium_null incorrect")
				correct = false
			}
		case "query_id":
			// ignore
		default:
			logf("default incorrect")
			correct = false
		}
	})
	if !correct || limit < 0 {
		logf("correct ", correct, " limit ", limit)
		ctx.SetStatusCode(400)
		return
	} else if emptyRes {
		logf("empty result")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyFilterRes)
		return
	}

	logf("Iterator %#v, filters %#v", iterators, filters)

	iterator := bitmap.NewAllIterator(MaxId)
	if len(iterators) > 0 {
		iterator = bitmap.NewAndIterator(iterators)
		iterators = nil
	}
	filter := combineFilters(filters)

	resAccs := make([]*Account, 0, limit)
	if filter == nil {
		bitmap.LoopIter(iterator, func(uids []int32) bool {
			for _, uid := range uids {
				resAccs = append(resAccs, &Accounts[uid])
				if len(resAccs) == limit {
					return false
				}
			}
			return true
		})
	} else {
		bitmap.LoopIter(iterator, func(uids []int32) bool {
			for _, uid := range uids {
				acc := &Accounts[uid]
				if !filter(acc) {
					continue
				}
				resAccs = append(resAccs, acc)
				if len(resAccs) == limit {
					return false
				}
			}
			return true
		})
	}

	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json")
	stream := config.BorrowStream(nil)
	stream.Write([]byte(`{"accounts":[`))
	for i, acc := range resAccs {
		outAccount(outFields, acc, stream)
		if i != len(resAccs)-1 {
			stream.WriteMore()
		}
	}
	stream.Write([]byte(`]}`))
	ctx.SetBody(stream.Buffer())
	config.ReturnStream(stream)
}

const (
	GroupBySex       = 1
	GroupByStatus    = 2
	GroupByCity      = 4
	GroupByCountry   = 8
	GroupByInterests = 16

	GroupByCitySex       = GroupByCity | GroupBySex
	GroupByCityStatus    = GroupByCity | GroupByStatus
	GroupByCountrySex    = GroupByCountry | GroupBySex
	GroupByCountryStatus = GroupByCountry | GroupByStatus
)

type GroupBy struct {
	Sex       bool
	Status    bool
	City      bool
	Country   bool
	Interests bool
}

func doGroup(ctx *fasthttp.RequestCtx) {
	args := ctx.QueryArgs()
	logf("doGroup")
	iterators := make([]bitmap.Iterator, 0, 4)
	groupBy := uint32(0)

	correct := true
	emptyRes := false
	limit := -1
	order := 0

	args.VisitAll(func(key []byte, val []byte) {
		if !correct {
			return
		}
		//logf("arg %s: %s", key, val)

		skey := string(key)
		sval := string(val)
		switch skey {
		case "limit":
			var err error
			limit, err = strconv.Atoi(sval)
			if err != nil {
				logf("limit: %s", err)
				correct = false
			} else if limit == 0 {
				emptyRes = true
			}
		case "order":
			var err error
			order, err = strconv.Atoi(sval)
			if err != nil {
				logf("limit: %s", err)
				correct = false
			} else if order != -1 && order != 1 {
				logf("limit: %s", sval)
				correct = false
			}
		case "keys":
			fields := strings.Split(sval, ",")
			for _, field := range fields {
				switch field {
				case "sex":
					groupBy |= GroupBySex
				case "status":
					groupBy |= GroupByStatus
				case "city":
					groupBy |= GroupByCity
				case "country":
					groupBy |= GroupByCountry
				case "interests":
					groupBy |= GroupByInterests
				default:
					correct = false
					return
				}
			}
			switch bits.OnesCount32(groupBy) {
			case 1:
			case 2:
				if groupBy&GroupByInterests != 0 {
					logf("group interests with other: %s", sval)
					correct = false
				} else if groupBy&^(GroupByCity|GroupByCountry) == 0 {
					logf("group city with country: %s", sval)
					correct = false
				} else if groupBy&^(GroupBySex|GroupByStatus) == 0 {
					logf("group sex with status: %s", sval)
					correct = false
				}
			default:
				logf("too many keys: %s", val)
				correct = false
			}
		case "sex":
			switch sval {
			case "m":
				iterators = append(iterators, MaleMap.Iterator(MaxId))
			case "f":
				iterators = append(iterators, FemaleMap.Iterator(MaxId))
			default:
				logf("sex_eq incorrect")
				correct = false
			}
		case "status":
			switch sval {
			case StatusFree:
				iterators = append(iterators, FreeMap.Iterator(MaxId))
			case StatusMeeting:
				iterators = append(iterators, MeetingMap.Iterator(MaxId))
			case StatusComplex:
				iterators = append(iterators, ComplexMap.Iterator(MaxId))
			default:
				logf("status_eq incorrect")
				correct = false
				return
			}
		case "country":
			ix := CountryStrings.Find(sval)
			if ix == 0 {
				logf("country %s not found", sval)
				emptyRes = true
				return
			}
			iterators = append(iterators, CountryStrings.GetIter(ix, MaxId))
		case "city":
			ix := CityStrings.Find(sval)
			if ix == 0 {
				logf("city %s not found", sval)
				emptyRes = true
				return
			}
			iterators = append(iterators, CityStrings.GetIter(ix, MaxId))
		case "birth":
			year, err := strconv.Atoi(sval)
			if err != nil {
				logf("birth_year incorrect")
				correct = false
				return
			}
			if year < 1950 || year-1950 >= len(BirthYearIndexes) {
				logf("birth_year out of range: %s", sval)
				emptyRes = true
				return
			}
			iterators = append(iterators, BirthYearIndexes[year-1950].Iterator(MaxId))
		case "joined":
			year, err := strconv.Atoi(sval)
			if err != nil {
				logf("join_year incorrect")
				correct = false
				return
			}
			if year < 2011 || year-2011 >= len(JoinYearIndexes) {
				logf("join_year out of range: %s", sval)
				emptyRes = true
				return
			}
			iterators = append(iterators, JoinYearIndexes[year-2011].Iterator(MaxId))
		case "interests":
			ix := InterestStrings.Find(sval)
			if ix == 0 {
				logf("interest %s not found", sval)
				emptyRes = true
				return
			}
			iterators = append(iterators, InterestStrings.GetIter(ix, MaxId))
		case "likes":
			n, err := strconv.Atoi(sval)
			if err != nil || n <= 0 || n >= int(MaxId) {
				logf("likes_contains incorrect")
				correct = false
				return
			}
			w := GetLikers(int32(n))
			if w == nil {
				logf("likes for %d not found", n)
				emptyRes = true
				return
			}
			iterators = append(iterators, w.Iterator(MaxId))
		case "query_id":
			// ignore
		default:
			logf("default incorrect")
			correct = false
		}
	})
	logf("groupBy %d iterators %#v", groupBy, iterators)

	if !correct || limit < 0 || order == 0 {
		logf("correct ", correct, " limit ", limit, " order ", order)
		ctx.SetStatusCode(400)
		return
	} else if emptyRes {
		logf("empty result")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyGroupRes)
		return
	}

	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json")
	stream := config.BorrowStream(nil)
	stream.Write([]byte(`{"groups":[`))

	var groups []counter
	switch {
	case groupBy == GroupByInterests:
		iterator := bitmap.NewAllIterator(MaxId)
		switch len(iterators) {
		case 0:
		case 1:
			iterator = iterators[0]
		default:
			iterator = bitmap.Materialize(bitmap.NewAndIterator(iterators))
		}
		groups = make([]counter, len(InterestStrings.Arr))
		for i := range InterestStrings.Arr {
			iter := InterestStrings.GetIter(uint32(i+1), MaxId)
			cnt := bitmap.CountIter(bitmap.NewAndIterator([]bitmap.Iterator{iterator, iter}))
			groups[i] = counter{uint32(i + 1), float64(cnt)}
		}
		groups = SortGroupLimit(limit, order, groups, func(idi, idj uint32) bool {
			return InterestStrings.GetStr(idi) < InterestStrings.GetStr(idj)
		})
		for i, gr := range groups {
			stream.Write([]byte(`{"interests":`))
			stream.WriteString(InterestStrings.GetStr(gr.u))
			stream.Write([]byte(`,"count":`))
			stream.WriteInt32(int32(gr.s))
			if i == len(groups)-1 {
				stream.WriteObjectEnd()
			} else {
				stream.Write([]byte("},"))
			}
		}
	/*
		case groupBy == GroupBySex:
			iterators = append(iterators, MaleMap.Iterator(MaxId))
			maleCount := bitmap.CountIter(bitmap.NewAndIterator(iterators))
			iterators[len(iterators)-1] = FemaleMap.Iterator(MaxId)
			femaleCount := bitmap.CountIter(bitmap.NewAndIterator(iterators))
			if order == 1 && femaleCount <= maleCount || order == -1 && femaleCount > maleCount {
				stream.Write([]byte(`{"sex":"f","count":`))
				stream.WriteUint32(femaleCount)
				if limit > 1 {
					stream.Write([]byte(`},{"sex":"m","count":`))
					stream.WriteUint32(maleCount)
				}
			} else {
				stream.Write([]byte(`{"sex":"m","count":`))
				stream.WriteUint32(maleCount)
				if limit > 1 {
					stream.Write([]byte(`},{"sex":"f","count":`))
					stream.WriteUint32(femaleCount)
				}
			}
			stream.WriteObjectEnd()
		case groupBy == GroupByStatus:
			iterators = append(iterators, FreeMap.Iterator(MaxId))
			freeCount := bitmap.CountIter(bitmap.NewAndIterator(iterators))
			iterators[len(iterators)-1] = MeetingMap.Iterator(MaxId)
			meetingCount := bitmap.CountIter(bitmap.NewAndIterator(iterators))
			iterators[len(iterators)-1] = ComplexMap.Iterator(MaxId)
			complexCount := bitmap.CountIter(bitmap.NewAndIterator(iterators))
			groups = []counter{
				{StatusFreeIx, float64(freeCount)},
				{StatusMeetingIx, float64(meetingCount)},
				{StatusComplexIx, float64(complexCount)},
			}
			SortGroupLimit(limit, order, groups, func(i uint32, j uint32) bool {
				return GetStatus(uint8(i)) < GetStatus(uint8(j))
			})
			if len(groups) > limit {
				groups = groups[:limit]
			}
			for i, gr := range groups {
				stream.Write([]byte(`{"status":`))
				stream.WriteString(GetStatus(uint8(gr.u)))
				stream.Write([]byte(`,"count":`))
				stream.WriteInt32(int32(gr.s))
				if i == len(groups)-1 {
					stream.WriteObjectEnd()
				} else {
					stream.Write([]byte("},"))
				}
			}
	*/
	//case groupBy&(GroupByCity|GroupByCountry) != 0:
	default:
		cityMult := 1
		if groupBy&GroupBySex != 0 {
			// female = 0, male = 1
			cityMult = 2
		} else if groupBy&GroupByStatus != 0 {
			cityMult = 3
		}
		var ngroups int
		var ncity int
		var nullIt bitmap.Iterator
		var notNullIt bitmap.Iterator
		if groupBy&GroupByCity != 0 {
			ncity = len(CityStrings.Arr) + 1
			nullIt = CityStrings.Null.GetIterator(MaxId)
			notNullIt = CityStrings.NotNull.GetIterator(MaxId)
		} else if groupBy&GroupByCountry != 0 {
			ncity = len(CountryStrings.Arr) + 1
			nullIt = CountryStrings.Null.GetIterator(MaxId)
			notNullIt = CountryStrings.NotNull.GetIterator(MaxId)
		} else {
			ncity = 1
		}
		ngroups = ncity * cityMult
		groups = make([]counter, ngroups+3)
		for i := 0; i < ncity; i++ {
			k := i * cityMult
			groups[k].u = uint32(i << 8)
			groups[k+1].u = uint32(i<<8) + 1
			groups[k+2].u = uint32(i<<8) + 2
		}
		groups = groups[:ngroups]
		mapper := func(u []int32) bool {
			for _, uid := range u {
				k := 0
				acc := Accounts[uid]
				if groupBy&GroupByCity != 0 {
					k = int(acc.City) * cityMult
				} else if groupBy&GroupByCountry != 0 {
					k = int(acc.Country) * cityMult
				}
				if cityMult == 2 {
					if acc.Sex {
						k++
					}
				} else if cityMult == 3 {
					k += int(acc.Status) - 1
				}
				groups[k].s++
			}
			return true
		}
		if groupBy&(GroupByCity|GroupByCountry) != 0 {
			bitmap.LoopIter(bitmap.NewAndIterator(append(iterators, notNullIt)), mapper)
		}
		if nullIt != nil {
			iterators = append(iterators, nullIt)
		}
		switch cityMult {
		case 1:
			groups[0].s = float64(bitmap.CountIter(bitmap.NewAndIterator(iterators)))
		case 2:
			groups[0].s = float64(bitmap.CountIter(
				bitmap.NewAndIterator(append(iterators, FemaleMap.Iterator(MaxId)))))
			groups[1].s = float64(bitmap.CountIter(
				bitmap.NewAndIterator(append(iterators, MaleMap.Iterator(MaxId)))))
		case 3:
			groups[StatusFreeIx-1].s = float64(bitmap.CountIter(
				bitmap.NewAndIterator(append(iterators, FreeMap.Iterator(MaxId)))))
			groups[StatusMeetingIx-1].s = float64(bitmap.CountIter(
				bitmap.NewAndIterator(append(iterators, MeetingMap.Iterator(MaxId)))))
			groups[StatusComplexIx-1].s = float64(bitmap.CountIter(
				bitmap.NewAndIterator(append(iterators, ComplexMap.Iterator(MaxId)))))
		}
		groups = SortGroupLimit(limit, order, groups, func(idi, idj uint32) bool {
			var cityi string
			var cityj string
			if groupBy&GroupByCity != 0 {
				cityi = CityStrings.GetStr(idi >> 8)
				cityj = CityStrings.GetStr(idj >> 8)
			} else if groupBy&GroupByCountry != 0 {
				cityi = CountryStrings.GetStr(idi >> 8)
				cityj = CountryStrings.GetStr(idj >> 8)
			}
			if cityi < cityj {
				return true
			} else if cityi > cityj {
				return false
			}
			return idi&0xff < idj&0xff
		})
		for i, gr := range groups {
			stream.Write([]byte(`{"count":`))
			stream.WriteInt32(int32(gr.s))
			if gr.u>>8 != 0 {
				if groupBy&GroupByCity != 0 {
					stream.Write([]byte(`,"city":`))
					stream.WriteString(CityStrings.GetStr(gr.u >> 8))
				} else if groupBy&GroupByCountry != 0 {
					stream.Write([]byte(`,"country":`))
					stream.WriteString(CountryStrings.GetStr(gr.u >> 8))
				}
			}
			if cityMult == 2 {
				if gr.u&1 != 0 {
					stream.Write([]byte(`,"sex":"m"`))
				} else {
					stream.Write([]byte(`,"sex":"f"`))
				}
			} else if cityMult == 3 {
				stream.Write([]byte(`,"status":`))
				stream.WriteString(GetStatus(uint8(gr.u) + 1))
			}
			if i == len(groups)-1 {
				stream.WriteObjectEnd()
			} else {
				stream.Write([]byte("},"))
			}
		}
	}

	stream.Write([]byte("]}"))
	ctx.SetBody(stream.Buffer())
	config.ReturnStream(stream)
}

func outAccount(out OutFields, acc *Account, stream *jsoniter.Stream) {
	stream.Write([]byte(`{"id":`))
	stream.WriteInt32(acc.Uid)
	stream.Write([]byte(`,"email":`))
	stream.WriteString(EmailIndex.GetStr(acc.Email))

	if out.Sex {
		if acc.Sex {
			stream.Write([]byte(`,"sex":"m"`))
		} else {
			stream.Write([]byte(`,"sex":"f"`))
		}
	}
	if out.Status {
		stream.Write([]byte(`,"status":`))
		stream.WriteString(GetStatus(acc.Status))
	}
	if out.Phone && acc.Phone != 0 {
		stream.Write([]byte(`,"phone":`))
		stream.WriteString(PhoneIndex.GetStr(acc.Phone))
	}
	if out.Fname && acc.Fname != 0 {
		stream.Write([]byte(`,"fname":`))
		stream.WriteString(FnameStrings.GetStr(uint32(acc.Fname)))
	}
	if out.Sname && acc.Sname != 0 {
		stream.Write([]byte(`,"sname":`))
		stream.WriteString(SnameStrings.GetStr(uint32(acc.Sname)))
	}
	if out.City && acc.City != 0 {
		stream.Write([]byte(`,"city":`))
		stream.WriteString(CityStrings.GetStr(uint32(acc.City)))
	}
	if out.Country && acc.Country != 0 {
		stream.Write([]byte(`,"country":`))
		stream.WriteString(CountryStrings.GetStr(uint32(acc.Country)))
	}
	if out.Birth {
		stream.Write([]byte(`,"birth":`))
		stream.WriteInt32(acc.Birth)
	}
	if out.Joined {
		stream.Write([]byte(`,"joined":`))
		stream.WriteInt32(acc.Joined)
	}
	if out.Premium && acc.PremiumLength != 0 {
		stream.Write([]byte(`,"premium":{"finish":`))
		stream.WriteInt32(acc.PremiumStart + PremiumLengths[acc.PremiumLength])
		stream.Write([]byte(`,"start":`))
		stream.WriteInt32(acc.PremiumStart)
		stream.WriteObjectEnd()
	}
	stream.WriteObjectEnd()
}

func combineFilters(filters []func(*Account) bool) func(*Account) bool {
	if len(filters) == 0 {
		return nil
	}
	for len(filters) >= 3 {
		l := len(filters)
		filters[l-3] = combineFilters3(filters[l-3], filters[l-2], filters[l-1])
		filters = filters[:l-2]
	}
	if len(filters) == 2 {
		return combineFilters2(filters[0], filters[1])
	}
	return filters[0]
}

func combineFilters2(f1, f2 func(*Account) bool) func(*Account) bool {
	return func(acc *Account) bool {
		return f1(acc) && f2(acc)
	}
}

func combineFilters3(f1, f2, f3 func(*Account) bool) func(*Account) bool {
	return func(acc *Account) bool {
		return f1(acc) && f2(acc) && f3(acc)
	}
}
