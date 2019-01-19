package main

import (
	"bytes"
	"math/bits"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	bitmap "github.com/funny-falcon/highloadcup2018/bitmap2"

	"github.com/valyala/fasthttp"
)

func getHandler(ctx *fasthttp.RequestCtx, path []byte) {
	logf("ctx.Path: %s, args: %s", path, ctx.QueryArgs())
	switch {
	case bytes.Equal(path, []byte("filter/")):
		doFilter(ctx)
	case bytes.Equal(path, []byte("group/")):
		doGroup(ctx)
	case bytes.HasSuffix(path, []byte("/suggest/")):
		ids := path[:bytes.IndexByte(path, '/')]
		id, err := strconv.Atoi(string(ids))
		if err != nil {
			ctx.SetStatusCode(400)
			return
		}
		doSuggest(ctx, id)
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

func doFilter(ctx *fasthttp.RequestCtx) {
	args := ctx.QueryArgs()
	maps := make([]bitmap.IBitmap, 0, 4)
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
			case "f":
				maps = append(maps, &FemaleMap)
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
			maps = append(maps, &EmailGtIndexes[chix])
		case "email_lt":
			if len(val) == 0 {
				emptyRes = true
				return // all are greater
			}
			email := sval
			emaillt := GetEmailPrefix(email)
			logf("emaillt %08x", emaillt)
			filters = append(filters, func(acc *Account) bool {
				logf("EmailStart %08x", acc.EmailStart)
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
			maps = append(maps, bitmap.NewOrBitmap(orIters))
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
			maps = append(maps, bitmap.NewOrBitmap(orIters))
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
			maps = append(maps, bitmap.NewOrBitmap(orIters))
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
			filters = append(filters, func(acc *Account) bool {
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
			maps = append(maps, bitmap.NewOrBitmap(orIters))
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
			maps = append(maps, bitmap.NewOrBitmap(orIters))
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
				maps = append(maps, bitmap.NewOrBitmap(iters))
			} else {
				maps = append(maps, iters...)
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
				maps = append(maps, w)
			}
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

	logf("Iterator %#v, filters %#v", maps, filters)

	iterator := bitmap.IBitmap(&AccountsMap)
	if len(maps) > 0 {
		iterator = bitmap.NewAndBitmap(maps)
		maps = nil
	}
	filter := combineFilters(filters)

	resAccs := make([]*Account, 0, limit)
	if filter == nil {
		bitmap.LoopMap(iterator, func(uids []int32) bool {
			for _, uid := range uids {
				resAccs = append(resAccs, &Accounts[uid])
				if len(resAccs) == limit {
					return false
				}
			}
			return true
		})
	} else {
		bitmap.LoopMap(iterator, func(uids []int32) bool {
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
		outAccount(&outFields, acc, stream)
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
	iterators := make([]bitmap.IBitmap, 0, 4)
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
				iterators = append(iterators, &MaleMap)
			case "f":
				iterators = append(iterators, &FemaleMap)
			default:
				logf("sex_eq incorrect")
				correct = false
			}
		case "status":
			switch sval {
			case StatusFree:
				iterators = append(iterators, &FreeMap)
			case StatusMeeting:
				iterators = append(iterators, &MeetingMap)
			case StatusComplex:
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
			iterators = append(iterators, CountryStrings.GetMap(ix))
		case "city":
			ix := CityStrings.Find(sval)
			if ix == 0 {
				logf("city %s not found", sval)
				emptyRes = true
				return
			}
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
			iterators = append(iterators, &JoinYearIndexes[year-2011])
		case "interests":
			ix := InterestStrings.Find(sval)
			if ix == 0 {
				logf("interest %s not found", sval)
				emptyRes = true
				return
			}
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
			iterators = append(iterators, w)
		case "query_id":
			// ignore
		default:
			logf("default incorrect")
			correct = false
		}
	})
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
	ctx.SetContentType("application/json")
	stream := config.BorrowStream(nil)
	stream.Write([]byte(`{"groups":[`))

	var groups []counter
	switch {
	case groupBy == GroupByInterests:
		groups = make([]counter, len(InterestStrings.Arr))
		for i := range InterestStrings.Arr {
			groups[i] = counter{u: uint32(i + 1)}
		}

		var iterator bitmap.IBitmap
		switch len(iterators) {
		case 0:
			for i := range InterestStrings.Arr {
				groups[i].s = float64(InterestStrings.Maps[i].GetSize())
			}
		default:
			iterators = append(iterators, &InterestStrings.NotNull)
			//iterator = bitmap.Materialize(bitmap.NewAndBitmap(iterators))
			iterator = bitmap.NewAndBitmap(iterators)
			//*
			var counts bitmap.BlockUnroll
			bitmap.LoopMap(iterator, func(u []int32) bool {
				for _, uid := range u {
					//Accounts[uid].Interests.UnrollCount(&counts)
					Interests[uid].UnrollCount(&counts)
				}
				return true
			})
			for i, c := range counts[:len(groups)] {
				groups[i].s = float64(c)
			}
			//*/
			/*
				intIters := make([]bitmap.Iterator, len(InterestStrings.Maps))
				for i, m := range InterestStrings.Maps {
					intIters[i], _ = m.Iterator()
				}
				bitmap.LoopMapBlock(iterator, func(bl bitmap.Block, span int32) bool {
					for i, m := range intIters {
						pbl, _ := m.FetchAndNext(span)
						block := *pbl
						block.Intersect(&bl)
						groups[i].s += float64(block.Count())
					}
					return true
				})
			//*/
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
		var maps []bitmap.IMutBitmap
		var nullIt bitmap.IBitmapSizer
		var notNullIt bitmap.IBitmapSizer
		if groupBy&GroupByCity != 0 {
			ncity = len(CityStrings.Arr) + 1
			maps = CityStrings.Maps
			nullIt = &CityStrings.Null
			notNullIt = &CityStrings.NotNull
		} else if groupBy&GroupByCountry != 0 {
			maps = CountryStrings.Maps
			ncity = len(CountryStrings.Arr) + 1
			nullIt = &CountryStrings.Null
			notNullIt = &CountryStrings.NotNull
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
		if len(iterators) == 0 && groupBy&(GroupByCountry|GroupByCity) == 0 {
			switch cityMult {
			case 2:
				groups[0].s = float64(FemaleMap.Size)
				groups[1].s = float64(MaleMap.Size)
			case 3:
				groups[StatusFreeIx-1].s = float64(FreeMap.Size)
				groups[StatusMeetingIx-1].s = float64(MeetingMap.Size)
				groups[StatusComplexIx-1].s = float64(ComplexMap.Size)
			}
		} else if len(iterators) == 0 && groupBy&(GroupByCountry|GroupByCity) == groupBy {
			groups[0].s = float64(nullIt.GetSize())
			for i, mp := range maps {
				groups[i+1].s = float64(mp.GetSize())
			}
		} else {
			mapper := func(u []int32) bool {
				for _, uid := range u {
					k := 0
					acc := &Accounts[uid]
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

					/*
						cityi := CountryStrings.GetStr(uint32(acc.Country))
						if cityi == "Росмаль" {
							logf("city %s status %s", cityi, GetStatus(acc.Status))
						}
					*/
				}
				return true
			}
			if groupBy&(GroupByCity|GroupByCountry) != 0 {
				bitmap.LoopMap(bitmap.NewAndBitmap(append(iterators, notNullIt)), mapper)
			}
			if nullIt != nil {
				iterators = append(iterators, nullIt)
			}
			switch cityMult {
			case 1:
				groups[0].s = float64(bitmap.CountMap(bitmap.NewAndBitmap(iterators)))
			case 2:
				groups[0].s = float64(bitmap.CountMap(
					bitmap.NewAndBitmap(append(iterators, &FemaleMap))))
				groups[1].s = float64(bitmap.CountMap(
					bitmap.NewAndBitmap(append(iterators, &MaleMap))))
			case 3:
				groups[StatusFreeIx-1].s = float64(bitmap.CountMap(
					bitmap.NewAndBitmap(append(iterators, &FreeMap))))
				groups[StatusMeetingIx-1].s = float64(bitmap.CountMap(
					bitmap.NewAndBitmap(append(iterators, &MeetingMap))))
				groups[StatusComplexIx-1].s = float64(bitmap.CountMap(
					bitmap.NewAndBitmap(append(iterators, &ComplexMap))))
				logf("ComplexMap size %d %v", ComplexMap.Size, groups[StatusComplexIx-1])
				logf("Status counts: %v", groups[:3])
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
	config.ReturnStream(stream)
}

func doSuggest(ctx *fasthttp.RequestCtx, iid int) {
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

	args := ctx.QueryArgs()
	//iterators := make([]bitmap.IBitmap, 0, 4)
	var filter func(uid int32) bool
	correct := true
	emptyRes := false
	limit := -1

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
				filter = func(uid int32) bool { return old(uid) && uint32(Accounts[uid].Country) == ix }
			} else {
				filter = func(uid int32) bool { return uint32(Accounts[uid].Country) == ix }
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
				filter = func(uid int32) bool { return old(uid) && uint32(Accounts[uid].City) == ix }
			} else {
				filter = func(uid int32) bool { return uint32(Accounts[uid].City) == ix }
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
	logf("likerss %d", len(likerss))

	hsh := newCntHash(sz)
	for _, likers := range likerss {
		ts := likers.GetTs(id)
		if ts == 0 {
			panic("no")
		}
		for _, ucnt := range likers.Data[:likers.Size] {
			oid := ucnt.UidAndCnt >> 8
			/*
				if !sameSex.Has(oid) {
					panic("no")
					continue
				}
			*/
			if filter != nil && !filter(oid) {
				continue
			}
			logf("oid %d", oid)
			ots := ucnt.Ts
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

	groups := SortGroupLimit(len(hsh), -1, hsh, func(idi, idj uint32) bool {
		return idi > idj
	})
	logf("groups %v", groups)

	uidHash := newUidHash(limit + int(small.Size))
	for _, oid := range small.Data[:small.Size] {
		uidHash.Insert(oid)
	}
	logf("uidHash %v", uidHash)
	uids := make([]int32, 0, limit)
Outter:
	for _, cnt := range groups {
		osmall := bitmap.GetSmall(&Accounts[cnt.u].Likes)
		logf("osmall %d %v", osmall.Size, osmall.Data[:osmall.Size])
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
	}
	logf("uids len %d", len(uids))

	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json")
	stream := config.BorrowStream(nil)
	stream.Write([]byte(`{"accounts":[`))
	outFields := OutFields{Status: true, Fname: true, Sname: true,
		Country: false}
	for i, id := range uids {
		outAccount(&outFields, &Accounts[id], stream)
		if i != len(uids)-1 {
			stream.WriteMore()
		}
	}
	stream.Write([]byte(`]}`))
	ctx.SetBody(stream.Buffer())
	config.ReturnStream(stream)
}

func outAccount(out *OutFields, acc *Account, stream *jsoniter.Stream) {
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
