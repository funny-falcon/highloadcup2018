package main

import (
	"math/bits"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/funny-falcon/highloadcup2018/bitmap3"
	bitmap "github.com/funny-falcon/highloadcup2018/bitmap3"
)

func getHandler(ctx *Request, path string) {
	switch {
	case path == "filter/":
		doFilter(ctx)
	case path == "group/":
		doGroup(ctx)
	case strings.HasSuffix(path, "/suggest/"):
		ids := path[:strings.IndexByte(path, '/')]
		id, err := strconv.Atoi(ids)
		if err != nil {
			ctx.SetStatusCode(400)
			return
		}
		doSuggest(ctx, id)
	case strings.HasSuffix(path, "/recommend/"):
		ids := path[:strings.IndexByte(path, '/')]
		id, err := strconv.Atoi(string(ids))
		if err != nil {
			ctx.SetStatusCode(400)
			return
		}
		doRecommend(ctx, id)
	default:
		ctx.SetStatusCode(404)
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

	PremiumNow bool
}

var EmptyFilterRes = []byte(`{"accounts":[]}`)
var EmptyGroupRes = []byte(`{"groups":[]}`)

func doFilter(ctx *Request) {
	var r bitmap.ReleaseHolder
	defer r.Release()

	maps := make([]bitmap.IBitmap, 0, 4)
	filters := []func(int32, *Account) bool{}

	correct := true
	emptyRes := false
	limit := -1
	var outFields OutFields

	for _, kv := range ctx.Args {
		key, val := kv.k, kv.v
		func() {
			if !correct {
				return
			}
			logf("arg %s: %s", key, val)
			if key == "limit" {
				var err error
				limit, err = strconv.Atoi(string(val))
				if err != nil || limit == 0 {
					logf("limit: %s", err)
					correct = false
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
					maps = append(maps, &MaleMap)
					//filters = append(filters, func(uid int32, _ *Account) bool { return uid&1 == 1 })
				case "f":
					maps = append(maps, &FemaleMap)
					//filters = append(filters, func(uid int32, _ *Account) bool { return uid&1 == 0 })
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
				iterator := DomainsStrings.GetMap(ix)
				maps = append(maps, iterator)
			case "email_gt":
				if len(val) == 0 {
					return // all are greater
				}
				email := sval
				emailgt := GetEmailPrefix(email)
				filters = append(filters, func(_ int32, acc *Account) bool {
					return acc.EmailStart >= emailgt
				})
				if len(email) > 4 {
					filters = append(filters, func(_ int32, acc *Account) bool {
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
				maps = append(maps, &EmailGtIndexes[chix])
			case "email_lt":
				if len(val) == 0 {
					emptyRes = true
					return // all are greater
				}
				email := sval
				emaillt := GetEmailPrefix(email)
				logf("emaillt %08x", emaillt)
				filters = append(filters, func(_ int32, acc *Account) bool {
					//logf("EmailStart %08x", acc.EmailStart)
					return acc.EmailStart < emaillt
				})
				if len(email) > 4 {
					filters = append(filters, func(_ int32, acc *Account) bool {
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
				maps = append(maps, &EmailLtIndexes[chix])
			case "status_eq":
				outFields.Status = true
				switch sval {
				case StatusFree:
					maps = append(maps, &FreeMap)
				case StatusMeeting:
					maps = append(maps, &MeetingMap)
				case StatusComplex:
					maps = append(maps, &ComplexMap)
				default:
					logf("status_eq incorrect")
					correct = false
					return
				}
			case "status_neq":
				outFields.Status = true
				switch sval {
				case StatusFree:
					maps = append(maps, &MeetingOrComplexMap)
				case StatusMeeting:
					maps = append(maps, &FreeOrComplexMap)
				case StatusComplex:
					maps = append(maps, &FreeOrMeetingMap)
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
				maps = append(maps, FnameStrings.GetMap(ix))
			case "fname_any":
				outFields.Fname = true
				names := strings.Split(sval, ",")
				orIters := make([]bitmap.IBitmap, 0, len(names))
				for _, name := range names {
					ix := FnameStrings.Find(name)
					if ix == 0 {
						continue
					}
					orIters = append(orIters, FnameStrings.GetMap(ix))
				}
				if len(orIters) == 0 {
					emptyRes = true
					return
				}
				maps = append(maps, bitmap.NewOrBitmap(orIters, &r))
			case "fname_null":
				outFields.Fname = true
				switch sval {
				case "1":
					maps = append(maps, &FnameStrings.Null)
				case "0":
					maps = append(maps, &FnameStrings.NotNull)
					/*
						filters = append(filters, func(acc *Account) bool {
							return acc.Fname != 0
						})
					*/
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
				maps = append(maps, SnameStrings.GetMap(ix))
			case "sname_starts":
				outFields.Sname = true
				SnameOnce.Sure()
				pref := sval
				i, j := SnameSorted.PrefixRange(pref)
				if i == j {
					emptyRes = true
					return
				}
				orIters := make([]bitmap.IBitmap, j-i)
				for k := i; k < j; k++ {
					orIters[k-i] = SnameStrings.GetMap(SnameSorted.Ix[k])
				}
				maps = append(maps, bitmap.NewOrBitmap(orIters, &r))
			case "sname_null":
				outFields.Sname = true
				switch sval {
				case "1":
					maps = append(maps, &SnameStrings.Null)
				case "0":
					maps = append(maps, &SnameStrings.NotNull)
					/*
						filters = append(filters, func(acc *Account) bool {
							return acc.Sname != 0
						})
					*/
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
				maps = append(maps, PhoneCodesStrings.GetMap(ix))
			case "phone_null":
				outFields.Phone = true
				switch sval {
				case "1":
					maps = append(maps, &PhoneIndex.Null)
				case "0":
					maps = append(maps, &PhoneIndex.NotNull)
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
				maps = append(maps, CountryStrings.GetMap(ix))
			case "country_null":
				outFields.Country = true
				switch sval {
				case "1":
					maps = append(maps, &CountryStrings.Null)
				case "0":
					maps = append(maps, &CountryStrings.NotNull)
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
				maps = append(maps, CityStrings.GetMap(ix))
			case "city_any":
				outFields.City = true
				cities := strings.Split(sval, ",")
				orIters := make([]bitmap.IBitmap, 0, len(cities))
				for _, name := range cities {
					ix := CityStrings.Find(name)
					if ix == 0 {
						continue
					}
					orIters = append(orIters, CityStrings.GetMap(ix))
				}
				if len(orIters) == 0 {
					emptyRes = true
					return
				}
				maps = append(maps, bitmap.NewOrBitmap(orIters, &r))
			case "city_null":
				outFields.City = true
				switch sval {
				case "1":
					maps = append(maps, &CityStrings.Null)
				case "0":
					maps = append(maps, &CityStrings.NotNull)
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
				filters = append(filters, func(_ int32, acc *Account) bool {
					return acc.Birth > birth
				})
				birthYear := GetBirthYear(birth)
				if birthYear < 1995-1950 {
					return
				}
				orIters := make([]bitmap.IBitmap, 0, len(BirthYearIndexes)-int(birthYear)+1)
				for ; int(birthYear) < len(BirthYearIndexes); birthYear++ {
					orIters = append(orIters, &BirthYearIndexes[birthYear])
				}
				maps = append(maps, bitmap.NewOrBitmap(orIters, &r))
			case "birth_lt":
				outFields.Birth = true
				n, err := strconv.Atoi(sval)
				if err != nil {
					logf("birth_lt incorrect")
					correct = false
					return
				}
				birth := int32(n)
				filters = append(filters, func(_ int32, acc *Account) bool {
					return acc.Birth < birth
				})
				birthYear := GetBirthYear(birth)
				if birthYear > 1988-1950 {
					return
				}
				orIters := make([]bitmap.IBitmap, 0, birthYear+1)
				for ; birthYear >= 0; birthYear-- {
					orIters = append(orIters, &BirthYearIndexes[birthYear])
				}
				maps = append(maps, bitmap.NewOrBitmap(orIters, &r))
			case "birth_year":
				outFields.Birth = true
				year, err := strconv.Atoi(sval)
				if err != nil {
					logf("birth_year incorrect")
					correct = false
					return
				}
				if year < 1950 || year-1950 >= len(BirthYearIndexes) {
					emptyRes = true
					return
				}
				maps = append(maps, &BirthYearIndexes[year-1950])
			case "interests_contains", "interests_any":
				interests := strings.Split(sval, ",")
				iters := make([]bitmap.IBitmap, 0, len(interests))
				for _, interest := range interests {
					ix := InterestStrings.Find(interest)
					if ix == 0 {
						if skey == "interests_contains" {
							emptyRes = true
							return
						}
						continue
					}
					iters = append(iters, InterestStrings.GetMap(ix))
				}
				if len(iters) == 0 {
					if skey == "interests_any" {
						emptyRes = true
					}
					return
				}
				if skey == "interests_any" {
					maps = append(maps, bitmap.NewOrBitmap(iters, &r))
				} else {
					maps = append(maps, iters...)
				}
			case "likes_contains":
				likesStrs := strings.Split(val, ",")
				likesMaps := make([]*bitmap.Likes, 0, len(likesStrs))
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
					likesMaps = append(likesMaps, w)
				}
				likers := bitmap.AndLikes(likesMaps)
				if len(likers) == 0 {
					emptyRes = true
					return
				}
				maps = append(maps, likers)
			case "premium_now":
				outFields.Premium = true
				maps = append(maps, &PremiumNow)
			case "premium_null":
				outFields.Premium = true
				switch sval {
				case "1":
					maps = append(maps, &PremiumNull)
				case "0":
					maps = append(maps, &PremiumNotNull)
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
		}()
		if !correct {
			break
		}
	}
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

	logf("Iterator %#v, filters %#v", maps, filters)

	iterator := bitmap.IBitmap(&AccountsMap)
	if len(maps) > 0 {
		if len(maps) == 1 {
			if _, ok := maps[0].(*bitmap.SexMap); ok {
				//filters = append(filters, func(_ int32, acc *Account) bool { return acc.Uid != 0 })
				maps = append(maps, &AccountsMap)
			}
		}
		iterator = bitmap.NewAndBitmap(maps)
		maps = nil
	}
	filter := combineFilters(filters)

	resAccs := make([]*Account, 0, limit)
	if filter == nil {
		bitmap.Loop(iterator, func(uids []int32) bool {
			for _, uid := range uids {
				resAccs = append(resAccs, RefAccount(uid))
				if len(resAccs) == limit {
					return false
				}
			}
			return true
		})
	} else {
		bitmap.Loop(iterator, func(uids []int32) bool {
			for _, uid := range uids {
				acc := RefAccount(uid)
				if !filter(uid, acc) {
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
	stream := jsonConfig.BorrowStream(nil)
	stream.Write([]byte(`{"accounts":[`))
	for i, acc := range resAccs {
		outAccount(&outFields, acc, stream)
		if i != len(resAccs)-1 {
			stream.WriteMore()
		}
	}
	stream.Write([]byte(`]}`))
	ctx.SetBody(stream.Buffer())
	jsonConfig.ReturnStream(stream)
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

func doGroup(ctx *Request) {
	logf("doGroup")
	iterators := make([]bitmap.IBitmap, 0, 4)
	groupBy := uint32(0)

	correct := true
	emptyRes := false
	limit := -1
	order := 0
	cityId := 0
	countryId := 0
	sexId := 0
	statusId := 0
	birthId := 0
	joinId := 0
	otherFilters := false

	for _, kv := range ctx.Args {
		key, val := kv.k, kv.v
		func() {
			//logf("arg %s: %s", key, val)

			skey := string(key)
			sval := string(val)
			switch skey {
			case "limit":
				var err error
				limit, err = strconv.Atoi(sval)
				if err != nil || limit == 0 {
					logf("limit: %s", err)
					correct = false
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
					sexId = 2
					iterators = append(iterators, &MaleMap)
				case "f":
					sexId = 1
					iterators = append(iterators, &FemaleMap)
				default:
					logf("sex_eq incorrect")
					correct = false
				}
			case "status":
				switch sval {
				case StatusFree:
					statusId = StatusFreeIx
					iterators = append(iterators, &FreeMap)
				case StatusMeeting:
					statusId = StatusMeetingIx
					iterators = append(iterators, &MeetingMap)
				case StatusComplex:
					statusId = StatusComplexIx
					iterators = append(iterators, &ComplexMap)
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
				countryId = int(ix)
				iterators = append(iterators, CountryStrings.GetMap(ix))
			case "city":
				ix := CityStrings.Find(sval)
				if ix == 0 {
					logf("city %s not found", sval)
					emptyRes = true
					return
				}
				cityId = int(ix)
				iterators = append(iterators, CityStrings.GetMap(ix))
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
				birthId = year + 1 - 1950
				otherFilters = true
				iterators = append(iterators, &BirthYearIndexes[year-1950])
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
				joinId = year + 1 - 2011
				otherFilters = true
				iterators = append(iterators, &JoinYearIndexes[year-2011])
			case "interests":
				ix := InterestStrings.Find(sval)
				if ix == 0 {
					logf("interest %s not found", sval)
					emptyRes = true
					return
				}
				otherFilters = true
				iterators = append(iterators, InterestStrings.GetMap(ix))
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
				otherFilters = true
				iterators = append(iterators, bitmap.AndLikes([]*bitmap3.Likes{w}))
			case "query_id":
				// ignore
			default:
				logf("default incorrect")
				correct = false
			}
		}()
		if !correct {
			break
		}
	}
	logf("groupBy %d iterators %#v limit %d order %d", groupBy, iterators, limit, order)

	if !correct || limit < 0 || order == 0 {
		logf("correct %v", correct)
		ctx.SetStatusCode(400)
		return
	} else if emptyRes {
		logf("empty result")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyGroupRes)
		return
	}

	ctx.SetStatusCode(200)
	stream := jsonConfig.BorrowStream(nil)
	stream.Write([]byte(`{"groups":[`))

	var groups []groupCounter
	switch {
	case groupBy == GroupByInterests:
		groups = make([]groupCounter, len(InterestStrings.Arr)+1)
		for i := range InterestStrings.Arr {
			groups[i+1] = groupCounter{u: uint32(i + 1)}
		}

		var iterator bitmap.IBitmap
		switch len(iterators) {
		case 0:
			for i := range InterestStrings.Arr {
				groups[i+1].s = InterestStrings.Maps[i].Count()
			}
		case 1:
			ok := true
			switch {
			case joinId != 0:
				for ix, cnt := range InterestJoinedGroups[joinId-1][:len(groups)-1] {
					groups[ix+1].s = cnt
				}
			case birthId != 0:
				for ix, cnt := range InterestBirthGroups[birthId-1][:len(groups)-1] {
					groups[ix+1].s = cnt
				}
			case countryId != 0:
				for ix, cnt := range InterestCountryGroups[countryId][:len(groups)-1] {
					groups[ix+1].s = cnt
				}
			default:
				ok = false
			}
			if ok {
				break
			}
			fallthrough
		default:
			iterator = bitmap.NewAndBitmap(iterators)
			bitmap.Loop(iterator, func(u []int32) bool {
				for _, uid := range u {
					GetInterest(uid).Unroll(func(int int32) {
						groups[int].s++
					})
				}
				return true
			})
		}
		groups = groups[1:]

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
		var maps []bitmap.IBitmap
		var nullIt bitmap.IBitmap
		//var notNullIt bitmap.IBitmap
		if groupBy&GroupByCity != 0 {
			ncity = len(CityStrings.Arr) + 1
			maps = make([]bitmap.IBitmap, 0, len(CityStrings.Maps))
			for _, m := range CityStrings.Maps {
				maps = append(maps, m)
			}
			nullIt = &CityStrings.Null
			//notNullIt = &CityStrings.NotNull
		} else if groupBy&GroupByCountry != 0 {
			maps = make([]bitmap.IBitmap, 0, len(CountryStrings.Maps))
			for _, m := range CountryStrings.Maps {
				maps = append(maps, m)
			}
			ncity = len(CountryStrings.Arr) + 1
			nullIt = &CountryStrings.Null
			//notNullIt = &CountryStrings.NotNull
		} else {
			ncity = 1
		}
		ngroups = ncity * cityMult
		groups = make([]groupCounter, ngroups+3)
		for i := 0; i < ncity; i++ {
			k := i * cityMult
			groups[k].u = uint32(i << 8)
			groups[k+1].u = uint32(i<<8) + 1
			groups[k+2].u = uint32(i<<8) + 2
		}
		groups = groups[:ngroups]
		var mapSmallAcc func(acc SmallerAccount, c uint32)
		switch groupBy {
		case GroupByCity:
			mapSmallAcc = func(acc SmallerAccount, c uint32) { groups[acc.City].s += c }
		case GroupByCountry:
			mapSmallAcc = func(acc SmallerAccount, c uint32) { groups[acc.Country].s += c }
		case GroupBySex:
			mapSmallAcc = func(acc SmallerAccount, c uint32) { groups[acc.SexIx()].s += c }
		case GroupByStatus:
			mapSmallAcc = func(acc SmallerAccount, c uint32) { groups[acc.Status()-1].s += c }
		case GroupByCitySex:
			mapSmallAcc = func(acc SmallerAccount, c uint32) {
				groups[int(acc.City)*2+acc.SexIx()].s += c
			}
		case GroupByCityStatus:
			mapSmallAcc = func(acc SmallerAccount, c uint32) {
				groups[int(acc.City)*3+acc.Status()-1].s += c
			}
		case GroupByCountrySex:
			mapSmallAcc = func(acc SmallerAccount, c uint32) {
				groups[int(acc.Country)*2+acc.SexIx()].s += c
			}
		case GroupByCountryStatus:
			mapSmallAcc = func(acc SmallerAccount, c uint32) {
				groups[int(acc.Country)*3+acc.Status()-1].s += c
			}
		}
		if !otherFilters {
			if groupBy&GroupByCountry != 0 && cityId != 0 {
				otherFilters = true
			} else if groupBy&GroupByCity != 0 && countryId != 0 {
				otherFilters = true
			}
		}
		if len(iterators) == 0 && groupBy&(GroupByCountry|GroupByCity) == 0 {
			switch cityMult {
			case 2:
				groups[0].s = FemaleMap.Size
				groups[1].s = MaleMap.Size
			case 3:
				groups[StatusFreeIx-1].s = FreeMap.Size
				groups[StatusMeetingIx-1].s = MeetingMap.Size
				groups[StatusComplexIx-1].s = ComplexMap.Size
			}
		} else if len(iterators) == 0 && groupBy&(GroupByCountry|GroupByCity) == groupBy {
			groups[0].s = bitmap.Count(nullIt)
			for i, mp := range maps {
				groups[i+1].s = bitmap.Count(mp)
			}
		} else if !otherFilters {
			var iterLine func(*[6]uint32)
			var acc SmallerAccount
			if sexId != 0 && statusId != 0 {
				iterLine = func(line *[6]uint32) {
					i := statusId - 1
					acc.StatusSexPremium = uint8(statusId)
					if sexId == 2 {
						acc.StatusSexPremium |= 4
						i += 3
					}
					mapSmallAcc(acc, line[i])
				}
			} else if statusId != 0 {
				iterLine = func(line *[6]uint32) {
					i := statusId - 1
					acc.StatusSexPremium = uint8(statusId)
					mapSmallAcc(acc, line[i])
					acc.StatusSexPremium |= 4
					mapSmallAcc(acc, line[i+3])
				}
			} else if sexId != 0 {
				iterLine = func(line *[6]uint32) {
					acc.StatusSexPremium = uint8(1 + (sexId-1)*4)
					i := (sexId - 1) * 3
					mapSmallAcc(acc, line[i])
					acc.StatusSexPremium++
					mapSmallAcc(acc, line[i+1])
					acc.StatusSexPremium++
					mapSmallAcc(acc, line[i+2])
				}
			} else {
				iterLine = func(line *[6]uint32) {
					acc.StatusSexPremium = 1
					mapSmallAcc(acc, line[0])
					acc.StatusSexPremium = 2
					mapSmallAcc(acc, line[1])
					acc.StatusSexPremium = 3
					mapSmallAcc(acc, line[2])
					acc.StatusSexPremium = 5
					mapSmallAcc(acc, line[3])
					acc.StatusSexPremium = 6
					mapSmallAcc(acc, line[4])
					acc.StatusSexPremium = 7
					mapSmallAcc(acc, line[5])
				}
			}
			lineToAcc := func(int) {}
			var massive [][6]uint32
			var base int
			if groupBy&GroupByCountry != 0 {
				lineToAcc = func(i int) { acc.Country = uint8(i) }
				massive = CountryGroups[:len(CountryStrings.Arr)+1]
			} else if groupBy&GroupByCity != 0 {
				lineToAcc = func(i int) { acc.City = uint16(i) }
				massive = CityGroups[:len(CityStrings.Arr)+1]
			}
			if cityId != 0 {
				massive = CityGroups[cityId : cityId+1]
				base = cityId
			} else if countryId != 0 {
				massive = CountryGroups[countryId : countryId+1]
				base = countryId
			}
			for i := range massive {
				lineToAcc(base + i)
				iterLine(&massive[i])
			}
		} else {
			mapper := func(u []int32) bool {
				for _, uid := range u {
					mapSmallAcc(GetSmallerAccount(uid), 1)
				}
				return true
			}
			if groupBy&(GroupByCity|GroupByCountry) != 0 {
				bitmap.Loop(bitmap.NewAndBitmap(iterators), mapper)
			} else {
				/*
					if nullIt != nil {
						iterators = append(iterators, nullIt)
					}
				*/
				switch cityMult {
				case 1:
					groups[0].s = bitmap.Count(bitmap.NewAndBitmap(iterators))
				case 2:
					groups[0].s = bitmap.Count(bitmap.NewAndBitmap(append(iterators, &FemaleMap)))
					groups[1].s = bitmap.Count(bitmap.NewAndBitmap(append(iterators, &MaleMap)))
				case 3:
					groups[StatusFreeIx-1].s = bitmap.Count(
						bitmap.NewAndBitmap(append(iterators, &FreeMap)))
					groups[StatusMeetingIx-1].s = bitmap.Count(
						bitmap.NewAndBitmap(append(iterators, &MeetingMap)))
					groups[StatusComplexIx-1].s = bitmap.Count(
						bitmap.NewAndBitmap(append(iterators, &ComplexMap)))
					logf("ComplexMap size %d %v", ComplexMap.Size, groups[StatusComplexIx-1])
					logf("Status counts: %v", groups[:3])
				}
			}
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
			//logf("cityi %s cityj %s", cityi, cityj)
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
	jsonConfig.ReturnStream(stream)
}

func doSuggest(ctx *Request, iid int) {
	id := int32(iid)
	if int(id) != iid {
		ctx.SetStatusCode(404)
		return
	}

	acc := HasAccount(id)
	if acc == nil {
		ctx.SetStatusCode(404)
		return
	}

	//iterators := make([]bitmap.IBitmap, 0, 4)
	var filter func(uid int32) bool
	correct := true
	emptyRes := false
	limit := -1

	ctx.VisitArgs(func(key string, val string) {
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
			if err != nil || limit == 0 {
				logf("limit: %s", err)
				correct = false
			}
		case "country":
			if len(sval) == 0 {
				correct = false
				return
			}
			ix := CountryStrings.Find(sval)
			if ix == 0 {
				logf("country %s not found", sval)
				emptyRes = true
				return
			}
			if filter != nil {
				old := filter
				filter = func(uid int32) bool { return old(uid) && uint32(RefAccount(uid).Country) == ix }
			} else {
				filter = func(uid int32) bool { return uint32(RefAccount(uid).Country) == ix }
			}
			//iterators = append(iterators, CountryStrings.GetMap(ix))
		case "city":
			if len(sval) == 0 {
				correct = false
				return
			}
			ix := CityStrings.Find(sval)
			if ix == 0 {
				logf("city %s not found", sval)
				emptyRes = true
				return
			}
			if filter != nil {
				old := filter
				filter = func(uid int32) bool { return old(uid) && uint32(RefAccount(uid).City) == ix }
			} else {
				filter = func(uid int32) bool { return uint32(RefAccount(uid).City) == ix }
			}
			//iterators = append(iterators, CityStrings.GetMap(ix))
		case "query_id":
			// ignore
		default:
			logf("default incorrect")
			correct = false
		}
	})
	logf("correct %v limit %d filter %v", correct, limit, filter)

	if !correct || limit <= 0 {
		logf("correct %v", correct)
		ctx.SetStatusCode(400)
		return
	} else if emptyRes {
		logf("empty result")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyFilterRes)
		return
	}

	/*
		sameSex := &MaleMap
		if !acc.Sex {
			sameSex = &FemaleMap
		}
	*/

	small := bitmap.GetSmall(&acc.Likes)
	if small.SmallImpl == nil {
		logf("empty likes")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyFilterRes)
		return
	}

	likerss := make([]*bitmap.Likes, 0, small.Size)
	sz := 0
	for _, uid := range small.Data[:small.Size] {
		likers := GetLikers(uid)
		likerss = append(likerss, likers)
		sz += int(likers.Size)
	}
	//logf("likerss %d", len(likerss))

	hsh := newCntHash(sz)
	for _, likers := range likerss {
		ts := likers.GetTs(id)
		if ts == 0 {
			panic("no")
		}
		for _, ucnt := range likers.Data[:likers.Size] {
			oid := ucnt.Uid
			if filter != nil && !filter(oid) {
				continue
			}
			//logf("oid %d", oid)
			ots := ucnt.Ts
			if ots < 0 {
				ots = -ots
			}
			dlt := ots - ts
			if dlt < 0 {
				dlt = -dlt
			} else if dlt == 0 {
				dlt = 1
			}
			cnt := hsh.Insert(uint32(oid))
			cnt.s += 1.0 / float64(dlt)
		}
	}

	/*
		groups := SortGroupLimit(len(hsh), -1, hsh, func(idi, idj uint32) bool {
			return idi > idj
		})
	*/
	groups := Heapify(hsh)
	//logf("groups %v", groups)

	uidHash := newUidHash(limit + int(small.Size))
	for _, oid := range small.Data[:small.Size] {
		uidHash.Insert(oid)
	}
	//logf("uidHash %v", uidHash)
	uids := make([]int32, 0, limit)
Outter:
	for len(groups) > 0 {
		cnt := groups[0]
		osmall := bitmap.GetSmall(&RefAccount(int32(cnt.u)).Likes)
		//logf("osmall %d %v", osmall.Size, osmall.Data[:osmall.Size])
		for _, oid := range osmall.Data[:osmall.Size] {
			//logf("osmall oid %d", oid)
			/*
				if sameSex.Has(oid) {
					panic("no")
				}
			*/
			if oid == id || !uidHash.Insert(oid) {
				continue
			}
			uids = append(uids, oid)
			if len(uids) == limit {
				break Outter
			}
		}
		groups = CntPop(groups)
	}
	logf("uids len %d", len(uids))

	ctx.SetStatusCode(200)
	stream := jsonConfig.BorrowStream(nil)
	stream.Write([]byte(`{"accounts":[`))
	outFields := OutFields{Status: true, Fname: true, Sname: true,
		Country: false}
	for i, id := range uids {
		outAccount(&outFields, RefAccount(id), stream)
		if i != len(uids)-1 {
			stream.WriteMore()
		}
	}
	stream.Write([]byte(`]}`))
	ctx.SetBody(stream.Buffer())
	jsonConfig.ReturnStream(stream)
}

func doRecommend(ctx *Request, iid int) {
	var r bitmap.ReleaseHolder
	defer r.Release()

	id := int32(iid)
	if int(id) != iid {
		ctx.SetStatusCode(404)
		return
	}

	acc := HasAccount(id)
	if acc == nil {
		ctx.SetStatusCode(404)
		return
	}

	var maps []bitmap.IBitmap
	correct := true
	emptyRes := false
	limit := -1

	ctx.VisitArgs(func(key string, val string) {
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
			if err != nil || limit == 0 {
				logf("limit: %s", err)
				correct = false
			}
		case "country":
			if len(sval) == 0 {
				correct = false
				return
			}
			ix := CountryStrings.Find(sval)
			if ix == 0 {
				logf("country %s not found", sval)
				emptyRes = true
				return
			}
			maps = append(maps, CountryStrings.GetMap(ix))
		case "city":
			if len(sval) == 0 {
				correct = false
				return
			}
			ix := CityStrings.Find(sval)
			if ix == 0 {
				logf("city %s not found", sval)
				emptyRes = true
				return
			}
			maps = append(maps, CityStrings.GetMap(ix))
		case "query_id":
			// ignore
		default:
			logf("default incorrect")
			correct = false
		}
	})
	logf("correct %v limit %d maps %v", correct, limit, maps)

	if !correct || limit <= 0 {
		logf("correct %v", correct)
		ctx.SetStatusCode(400)
		return
	} else if emptyRes {
		logf("empty result")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyFilterRes)
		return
	}

	interests := GetInterest(id)
	ormaps := make([]bitmap.IBitmap, 0, 16)
	interests.Unroll(func(ii int32) {
		ormaps = append(ormaps, InterestStrings.Maps[ii-1])
	})
	if len(ormaps) == 0 {
		logf("empty result")
		ctx.SetStatusCode(200)
		ctx.SetBody(EmptyFilterRes)
		return
	}
	orr := bitmap.NewOrBitmap(ormaps, &r)

	recs := Recommends{
		Birth:     acc.Birth,
		Limit:     limit,
		Interests: *interests,
	}

	var tmaps [][]bitmap.IBitmap
	maps = append(maps, orr)

	maps = maps[:len(maps):len(maps)]
	if acc.Sex {
		tmaps = [][]bitmap.IBitmap{
			append(maps, PremiumFreeFemale),
			append(maps, PremiumMeetingOrComplexFemale),
			/*
				append(maps, PremiumComplexFemale),
				append(maps, PremiumMeetingFemale),
			*/
		}
		maps = append(maps, &FemaleMap)
	} else {
		tmaps = [][]bitmap.IBitmap{
			append(maps, PremiumFreeMale),
			append(maps, PremiumMeetingOrComplexMale),
			/*
				append(maps, PremiumComplexMale),
				append(maps, PremiumMeetingMale),
			*/
		}
		maps = append(maps, &MaleMap)
	}
	maps = maps[:len(maps):len(maps)]
	tmaps = append(tmaps,
		append(maps, &PremiumNotNow, &FreeMap),
		append(maps, &PremiumNotNow, &MeetingOrComplexMap),
	)

	for _, tmap := range tmaps {
		bitmap.Loop(bitmap.NewAndBitmap(tmap), func(uids []int32) bool {
			for _, uid := range uids {
				/*
					cnt := interests.IntersectCount(*GetInterest(uid))
					if cnt == 0 {
						continue
					}
				*/
				othacc := GetSmallAccount(uid)
				recs.Add(othacc, uid, 0)
			}
			return true
		})
		if len(recs.Accs) == limit {
			break
		}
	}

	recs.Heapify()

	uids := make([]int32, len(recs.Accs))
	l := len(uids) - 1
	for i := range uids {
		uids[l-i] = recs.Accs[0].Uid
		recs.Pop()
	}

	ctx.SetStatusCode(200)
	stream := jsonConfig.BorrowStream(nil)
	stream.Write([]byte(`{"accounts":[`))
	outFields := OutFields{Status: true, Fname: true, Sname: true, Birth: true,
		Premium: true,
		Country: false}
	for i, id := range uids {
		outAccount(&outFields, RefAccount(id), stream)
		if i != len(uids)-1 {
			stream.WriteMore()
		}
	}
	stream.Write([]byte(`]}`))
	ctx.SetBody(stream.Buffer())
	jsonConfig.ReturnStream(stream)
}

func outAccount(out *OutFields, acc *Account, stream *jsoniter.Stream) {
	stream.Write([]byte(`{"id":`))
	stream.WriteInt32(acc.Uid)

	if out.Premium && acc.PremiumLength != 0 {
		stream.Write([]byte(`,"premium":{"start":`))
		stream.WriteInt32(acc.PremiumStart)
		stream.Write([]byte(`,"finish":`))
		stream.WriteInt32(acc.PremiumStart + PremiumLengths[acc.PremiumLength])
		stream.WriteObjectEnd()
	}

	if out.Sname && acc.Sname != 0 {
		stream.Write([]byte(`,"sname":`))
		stream.WriteString(SnameStrings.GetStr(uint32(acc.Sname)))
	}
	if out.Fname && acc.Fname != 0 {
		stream.Write([]byte(`,"fname":`))
		stream.WriteString(FnameStrings.GetStr(uint32(acc.Fname)))
	}

	if out.Status {
		stream.Write([]byte(`,"status":`))
		stream.WriteString(GetStatus(acc.Status))
	}

	stream.Write([]byte(`,"email":`))
	stream.WriteString(EmailIndex.GetStr(acc.Email))

	if out.Sex {
		if acc.Sex {
			stream.Write([]byte(`,"sex":"m"`))
		} else {
			stream.Write([]byte(`,"sex":"f"`))
		}
	}
	if out.Phone && acc.Phone != 0 {
		stream.Write([]byte(`,"phone":`))
		stream.WriteString(PhoneIndex.GetStr(acc.Phone))
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
	stream.WriteObjectEnd()
}

func combineFilters(filters []func(int32, *Account) bool) func(int32, *Account) bool {
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

func combineFilters2(f1, f2 func(int32, *Account) bool) func(int32, *Account) bool {
	return func(id int32, acc *Account) bool {
		return f1(id, acc) && f2(id, acc)
	}
}

func combineFilters3(f1, f2, f3 func(int32, *Account) bool) func(int32, *Account) bool {
	return func(id int32, acc *Account) bool {
		return f1(id, acc) && f2(id, acc) && f3(id, acc)
	}
}
