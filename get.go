package main

import (
	"bytes"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/funny-falcon/highloadcup2018/bitmap"

	"github.com/valyala/fasthttp"
)

var FilterPath = []byte("/accounts/filter/")

func getHandler(ctx *fasthttp.RequestCtx) {
	path := ctx.Path()
	switch {
	case bytes.Equal(path, FilterPath):
		doFilter(ctx)
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
		if !correct {
			return
		}
		skey := string(key)
		switch skey {
		case "sex_eq":
			outFields.Sex = true
			switch string(val) {
			case "m":
				iterators = append(iterators, MaleMap.Iterator(MaxId))
			case "f":
				iterators = append(iterators, FemaleMap.Iterator(MaxId))
			default:
				logf("sex_eq incorrect")
				correct = false
			}
		case "email_domain":
			domain := string(val)
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
			email := string(val)
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
			email := string(val)
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
			switch string(val) {
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
			switch string(val) {
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
			ix := FnameStrings.Find(string(val))
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, FnameStrings.GetIter(ix, MaxId))
		case "fname_any":
			outFields.Fname = true
			names := strings.Split(string(val), ",")
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
			switch string(val) {
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
			ix := SnameStrings.Find(string(val))
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, SnameStrings.GetIter(ix, MaxId))
		case "sname_starts":
			outFields.Sname = true
			SnameOnce.Sure()
			pref := string(val)
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
			switch string(val) {
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
			code := string(val)
			ix := PhoneCodesStrings.Find(code)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, PhoneCodesStrings.GetIter(ix, MaxId))
		case "phone_null":
			outFields.Phone = true
			switch string(val) {
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
			ix := CountryStrings.Find(string(val))
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, CountryStrings.GetIter(ix, MaxId))
		case "country_null":
			outFields.Country = true
			switch string(val) {
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
			ix := CityStrings.Find(string(val))
			if ix == 0 {
				emptyRes = true
				return
			}
			iterators = append(iterators, CityStrings.GetIter(ix, MaxId))
		case "city_any":
			outFields.City = true
			cities := strings.Split(string(val), ",")
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
			switch string(val) {
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
			n, err := strconv.Atoi(string(val))
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
			n, err := strconv.Atoi(string(val))
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
			year, err := strconv.Atoi(string(val))
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
			interests := strings.Split(string(val), ",")
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
			switch string(val) {
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
