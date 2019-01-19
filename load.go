package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"

	"github.com/funny-falcon/highloadcup2018/alloc2"
	bitmap "github.com/funny-falcon/highloadcup2018/bitmap2"

	jsoniter "github.com/json-iterator/go"
)

var config = jsoniter.Config{
	OnlyTaggedField: true,
	CaseSensitive:   true,
}.Froze()

type AccountIn struct {
	Id        uint32   `json:"id"`
	Email     string   `json:"email"`
	Fname     string   `json:"fname"`
	Sname     string   `json:"sname"`
	Phone     string   `json:"phone"`
	Sex       string   `json:"sex"`
	Birth     int32    `json:"birth"`
	Country   string   `json:"country"`
	City      string   `json:"city"`
	Joined    int32    `json:"joined"`
	Status    string   `json:"status"`
	Interests []string `json:"interests"`
	Premium   struct {
		Start  int32 `json:"start"`
		Finish int32 `json:"finish"`
	} `json:"premium"`
	Likes []struct {
		Id int32 `json:"id"`
		Ts int32 `json:"ts"`
	} `json:"likes"`
}

type AccountsIn struct {
	Accounts []AccountIn `json:"accounts"`
}

func Compact() {
	for i := range Accounts {
		acc := &Accounts[i]
		bitmap.SmallAlloc.Compact(&acc.Likes)
	}
	for i := range Likers {
		bitmap.LikesAlloc.Compact(&Likers[i])
	}
}

func Load() {
	var outfile io.Writer
	if *dumpload {
		f, err := os.Create("load.dump")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		buf := bufio.NewWriterSize(f, 128*1024)
		defer buf.Flush()
		outfile = buf
	}

	optfile, err := os.Open(*path + "options.txt")
	if err != nil {
		log.Fatal(err)
	}
	_, err = fmt.Fscan(optfile, &CurTs)
	if err != nil {
		log.Fatal(err)
	}
	optfile.Close()

	rdr, err := zip.OpenReader(*path + "data.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer rdr.Close()
	debug.SetGCPercent(5)
	for nf, f := range rdr.File {
		func() {
			rc, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}
			defer rc.Close()
			iter := jsoniter.Parse(config, rc, 256*1024)
			if attr := iter.ReadObject(); attr != "accounts" {
				log.Fatal("No accounts ", attr, iter.Error)
			}
			for iter.ReadArray() {
				var accin AccountIn
				iter.ReadVal(&accin)
				if iter.Error != nil {
					break
				}
				if outfile != nil {
					fmt.Fprintf(outfile, "%+v\n", &accin)
				}
				var ok bool
				acc := SureAccount(int32(accin.Id))
				if MaxId <= acc.Uid {
					MaxId = acc.Uid + 1
				}
				acc.Birth = accin.Birth
				byear := GetBirthYear(acc.Birth)
				BirthYearIndexes[byear].Set(acc.Uid)
				acc.Joined = accin.Joined
				jyear := GetJoinYear(acc.Joined)
				JoinYearIndexes[jyear].Set(acc.Uid)
				acc.Sex = accin.Sex == "m"
				if acc.Sex {
					MaleMap.Set(acc.Uid)
				} else {
					FemaleMap.Set(acc.Uid)
				}
				acc.Status, ok = GetStatusIx(accin.Status)
				if !ok {
					panic("status unknown " + accin.Status)
				}
				switch acc.Status {
				case StatusFreeIx:
					FreeMap.Set(acc.Uid)
					FreeOrComplexMap.Set(acc.Uid)
					FreeOrMeetingMap.Set(acc.Uid)
				case StatusMeetingIx:
					MeetingMap.Set(acc.Uid)
					FreeOrMeetingMap.Set(acc.Uid)
					MeetingOrComplexMap.Set(acc.Uid)
				case StatusComplexIx:
					ComplexMap.Set(acc.Uid)
					FreeOrComplexMap.Set(acc.Uid)
					MeetingOrComplexMap.Set(acc.Uid)
				}
				acc.Email, ok = EmailIndex.InsertUid(accin.Email, acc.Uid)
				if !ok {
					panic("email is not unique " + accin.Email)
				}
				acc.EmailStart = GetEmailStart(accin.Email)
				domain := DomainFromEmail(accin.Email)
				acc.Domain = uint8(DomainsStrings.Add(domain, acc.Uid))
				IndexGtLtEmail(accin.Email, acc.Uid, true)
				acc.Phone, ok = PhoneIndex.InsertUid(accin.Phone, acc.Uid)
				if accin.Phone != "" {
					if !ok {
						panic("phone is not unique " + accin.Phone)
					}
					code := CodeFromPhone(accin.Phone)
					acc.Code = uint8(PhoneCodesStrings.Add(code, acc.Uid))
				}
				acc.Fname = uint8(FnameStrings.Add(accin.Fname, acc.Uid))
				acc.Sname = uint16(SnameStrings.Add(accin.Sname, acc.Uid))
				SnameOnce.Reset()
				acc.City = uint16(CityStrings.Add(accin.City, acc.Uid))
				acc.Country = uint8(CountryStrings.Add(accin.Country, acc.Uid))
				acc.PremiumStart = accin.Premium.Start
				if accin.Premium.Finish != 0 {
					acc.PremiumLength = GetPremiumLength(accin.Premium.Start, accin.Premium.Finish)
					acc.PremiumNow = accin.Premium.Start <= CurTs && accin.Premium.Finish > CurTs
					if acc.PremiumNow {
						PremiumNow.Set(acc.Uid)
					}
					PremiumNotNull.Set(acc.Uid)
				} else {
					PremiumNull.Set(acc.Uid)
				}
				for _, interest := range accin.Interests {
					ix := InterestStrings.Add(interest, acc.Uid)
					acc.SetInterest(ix)
				}
				var likes bitmap.Small
				for _, like := range accin.Likes {
					likes.Set(like.Id)
					SureLikers(like.Id).SetTs(acc.Uid, like.Ts)
				}
				acc.Likes = likes.Uintptr()
			}
			if iter.Error != nil {
				log.Fatal("Error reading accounts: ", iter.Error)
			}
			if (nf+1)%2 == 0 {
				Compact()
			}
			/*
				switch nf {
				case 40:
					debug.SetGCPercent(10)
				}
			*/
		}()
	}
	debug.SetGCPercent(20)
	fmt.Println("LikesAlloc ", bitmap.LikesAlloc.TotalAlloc, bitmap.LikesAlloc.TotalFree,
		len(bitmap.LikesAlloc.Free)*alloc2.ChunkSize)
	fmt.Println("SmallAlloc ", bitmap.SmallAlloc.TotalAlloc, bitmap.SmallAlloc.TotalFree,
		len(bitmap.SmallAlloc.Free)*alloc2.ChunkSize)
	//bitmap.SmallAlloc.Log = "small"
	Compact()
	bitmap.SmallAlloc.FreeFree()
	bitmap.LikesAlloc.FreeFree()
	fmt.Println("LikesAlloc ", bitmap.LikesAlloc.TotalAlloc, bitmap.LikesAlloc.TotalFree,
		len(bitmap.LikesAlloc.Free)*alloc2.ChunkSize)
	fmt.Println("SmallAlloc ", bitmap.SmallAlloc.TotalAlloc, bitmap.SmallAlloc.TotalFree,
		len(bitmap.SmallAlloc.Free)*alloc2.ChunkSize)
	fmt.Println("StringsAlloc ", StringAlloc.TotalAlloc, StringAlloc.TotalFree,
		len(StringAlloc.Free)*alloc2.ChunkSize)
	fmt.Println("TotalAlloc ", alloc2.ChunkGenerator.TotalAlloc)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		f.Close()
	}
	/*
		fmt.Println("SnameStrings ", SnameStrings.Stat())
		fmt.Println("CityStrings ", CityStrings.Stat())
		fmt.Println("CountryStrings ", CountryStrings.Stat())
		fmt.Println("DomainsStrings ", DomainsStrings.Stat())
		fmt.Println("PhoneCodesStrings ", PhoneCodesStrings.Stat())
		fmt.Println("FnameStrings ", FnameStrings.Stat())
		fmt.Println("InterestStrings ", InterestStrings.Stat())
		fmt.Print("FreeMap ")
		fmt.Println(FreeMap.Stat())
		fmt.Print("ComplexMap ")
		fmt.Println(ComplexMap.Stat())
		fmt.Print("MeetingMap ")
		fmt.Println(MeetingMap.Stat())
		fmt.Print("PremiumNow ")
		fmt.Println(PremiumNow.Stat())
		fmt.Print("PremiumNull ")
		fmt.Println(PremiumNull.Stat())
		fmt.Print("PremiumNotNull ")
		fmt.Println(PremiumNotNull.Stat())
	*/
}

/*
func GetSomeStat() {
	rdr, err := zip.OpenReader(*datazip)
	if err != nil {
		log.Fatal(err)
	}
	defer rdr.Close()
	domains := make(map[string]bool)
	fnames := make(map[string]bool)
	snames := make(map[string]bool)
	phones := make(map[string]bool)
	countries := make(map[string]bool)
	cities := make(map[string]bool)
	interests := make(map[string]bool)
	intHist := make(map[int]int)
	likesHist := make(map[int]int)
	//var accss []AccountsIn
	strs := make(map[string]string)
	intern := func(s string) string {
		if ss, ok := strs[s]; ok {
			return ss
		}
		strs[s] = s
		return s
	}
	maxid := uint32(0)
	likee := make(map[uint32]int)
	for _, f := range rdr.File {
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		dec := jsoniter.NewDecoder(rc)
		var accs AccountsIn
		err = dec.Decode(&accs)
		if err != nil {
			log.Fatal(err)
		}
		for _, acc := range accs.Accounts {
			ix := strings.LastIndexByte(acc.Email, '@')
			if acc.Sex == "" || acc.Status == "" {
				panic("has empty")
			}
			domains[acc.Email[ix+1:]] = true
			fnames[acc.Fname] = true
			snames[acc.Sname] = true
			phones[acc.Phone] = true
			countries[acc.Country] = true
			cities[acc.City] = true
			for i, intr := range acc.Interests {
				interests[intr] = true
				acc.Interests[i] = intern(intr)
			}
			intHist[len(acc.Interests)]++
			likesHist[len(acc.Likes)]++

			for _, like := range acc.Likes {
				likee[like.Id]++
			}

			acc.Domain = intern(acc.Email[ix+1:])
			acc.Email = "" + acc.Email[:ix]
			acc.City = intern(acc.City)
			acc.Country = intern(acc.Country)
			acc.Fname = intern(acc.Fname)
			acc.Sname = intern(acc.Sname)
			acc.Sex = intern(acc.Sex)
			acc.Status = intern(acc.Status)
			if acc.Id > maxid {
				maxid = acc.Id
			}
		}
		//fmt.Printf("%s: %d\n", f.Name, len(accs.Accounts))
		rc.Close()
		//accss = append(accss, accs)
	}
	fmt.Printf("Domains: %d\nFnames: %d\nSnames: %d\nPhones: %d\n",
		len(domains), len(fnames), len(snames), len(phones))
	fmt.Printf("Countries: %d\nCities: %d\nInterests: %d\n",
		len(countries), len(cities), len(interests))
	//printHist("Interests:", intHist)
	//printHist("Likes:", likesHist)
	fmt.Printf("MaxId: %d\n", maxid)

	likecnt := make([]int, 0, len(likee))
	for _, cnt := range likee {
		likecnt = append(likecnt, cnt)
	}
	sort.Ints(likecnt)
	fmt.Println(likecnt[len(likecnt)-30:])
}

func printHist(name string, hist map[int]int) {
	srt := make([][2]int, 0, len(hist))
	for h, c := range hist {
		srt = append(srt, [2]int{h, c})
	}
	sort.Slice(srt, func(i, j int) bool { return srt[i][0] < srt[j][0] })
	fmt.Println(name)
	for _, c := range srt {
		fmt.Printf("\t%d\t%d\n", c[0], c[1])
	}
}
*/
