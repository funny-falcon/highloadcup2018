package main

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"

	"github.com/funny-falcon/highloadcup2018/alloc2"
	bitmap "github.com/funny-falcon/highloadcup2018/bitmap3"

	jsoniter "github.com/json-iterator/go"
)

var jsonConfig = jsoniter.Config{
	DisallowUnknownFields:         true,
	ObjectFieldMustBeSimpleString: true,
	OnlyTaggedField:               true,
	CaseSensitive:                 true,
}.Froze()

type AccountIn struct {
	Id        int32    `json:"id"`
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
	Likes []Like `json:"likes"`
}
type Like struct {
	Id int32 `json:"id"`
	Ts int32 `json:"ts"`
}

func loadAccount(iter *jsoniter.Iterator, accin *AccountIn) error {
Acc:
	for {
		field := iter.ReadObject()
		if iter.Error != nil {
			return iter.Error
		}
		switch field {
		case "":
			break Acc
		case "id":
			accin.Id = iter.ReadInt32()
		case "birth":
			accin.Birth = iter.ReadInt32()
		case "joined":
			accin.Joined = iter.ReadInt32()
		case "email":
			accin.Email = iter.ReadString()
		case "fname":
			accin.Fname = iter.ReadString()
		case "sname":
			accin.Sname = iter.ReadString()
		case "phone":
			accin.Phone = iter.ReadString()
		case "sex":
			accin.Sex = iter.ReadString()
		case "country":
			accin.Country = iter.ReadString()
		case "city":
			accin.City = iter.ReadString()
		case "status":
			accin.Status = iter.ReadString()
		case "premium":
		Prem:
			for {
				fld := iter.ReadObject()
				switch fld {
				case "":
					break Prem
				case "start":
					accin.Premium.Start = iter.ReadInt32()
				case "finish":
					accin.Premium.Finish = iter.ReadInt32()
				default:
					return fmt.Errorf("unknown premium %q %v", fld, fld == "finish")
				}
			}
		case "interests":
			for iter.ReadArray() {
				accin.Interests = append(accin.Interests, iter.ReadString())
			}
		case "likes":
			for iter.ReadArray() {
				var id, ts int32
			Likes:
				for {
					fld := iter.ReadObject()
					switch fld {
					case "":
						break Likes
					case "id":
						id = iter.ReadInt32()
					case "ts":
						ts = iter.ReadInt32()
					default:
						return errors.New("unknown like field " + fld)
					}
				}
				accin.Likes = append(accin.Likes, Like{Id: id, Ts: ts})
			}
		default:
			return errors.New("unknown acc field " + field)
		}
		if iter.Error != nil {
			return iter.Error
		}
	}
	return iter.Error
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
	sema := make(chan int, 1)
	var wg sync.WaitGroup
	var compactMtx sync.RWMutex
	for _nf, _f := range rdr.File {
		wg.Add(1)
		nf, f := _nf, _f
		go func() {
			sema <- 1
			compactMtx.RLock()
			defer func() {
				compactMtx.RUnlock()
				<-sema
				wg.Done()
			}()
			rc, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}
			defer rc.Close()
			iter := jsoniter.Parse(jsonConfig, rc, 256*1024)
			if attr := iter.ReadObject(); attr != "accounts" {
				log.Fatal("No accounts ", attr, iter.Error)
			}
			for iter.ReadArray() {
				var accin AccountIn
				if err := loadAccount(iter, &accin); err != nil {
					iter.Error = err
					break
				}
				//iter.ReadVal(&accin)
				if iter.Error != nil {
					break
				}
				if outfile != nil {
					fmt.Fprintf(outfile, "%+v\n", &accin)
				}
				InsertAccount(&accin)
			}
			if iter.Error != nil {
				log.Fatal("Error reading accounts: ", iter.Error)
			}
			if (nf+1)%2 == 0 {
				compactMtx.RUnlock()
				compactMtx.Lock()
				Compact()
				compactMtx.Unlock()
				compactMtx.RLock()
			}
		}()
	}
	wg.Wait()
	debug.SetGCPercent(30)

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

	fmt.Println("City ", len(CityStrings.Arr))
	fmt.Println("Country ", len(CountryStrings.Arr))
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

func InsertAccount(accin *AccountIn) {
	var ok bool
	acc := SureAccount(int32(accin.Id))
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
	for _, mp := range StatusMaps[acc.Status] {
		mp.Set(acc.Uid)
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
		} else {
			PremiumNotNow.Set(acc.Uid)
		}
		PremiumNotNull.Set(acc.Uid)
	} else {
		PremiumNotNow.Set(acc.Uid)
		PremiumNull.Set(acc.Uid)
	}
	for _, interest := range accin.Interests {
		ix := InterestStrings.Add(interest, acc.Uid)
		SetInterest(acc.Uid, uint8(ix))
		//acc.SetInterest(ix - 1)
		InterestJoinedGroups[GetJoinYear(acc.Joined)][ix-1]++
		InterestBirthGroups[GetBirthYear(acc.Birth)][ix-1]++
		InterestCountryGroups[acc.Country][ix-1]++
	}
	var smallImpl = likesImplPool.Get().(*bitmap.SmallImpl)
	smallImpl.Size = 0
	likes := bitmap.Small{smallImpl}
	for _, like := range accin.Likes {
		likes.Set(like.Id)
		SureLikers(like.Id, func(l *bitmap.Likes) { l.SetTs(like.Id, acc.Uid, like.Ts) })
	}
	acc.Likes = likes.ForceAlloc()
	likesImplPool.Put(smallImpl)

	CityGroups[acc.City][acc.StatusIx()+acc.SexIx()*3]++
	CountryGroups[acc.Country][acc.StatusIx()+acc.SexIx()*3]++

	SetSmallAccount(acc.Uid, acc.SmallAccount())
}

var likesImplPool = sync.Pool{
	New: func() interface{} { return &bitmap.SmallImpl{Cap: 256} },
}

func UpdateAccount(acc *Account, accin *AccountIn) bool {
	oldEmail := EmailIndex.GetStr(acc.Email)
	updateEmail := false
	if accin.Email != "" && oldEmail != accin.Email {
		if !EmailIndex.IsFree(accin.Email) {
			logf("email is not free %s", accin.Email)
			return false
		}
		updateEmail = true
	}
	oldPhone := PhoneIndex.GetStr(acc.Phone)
	updatePhone := false
	if accin.Phone != "" && oldPhone != accin.Phone {
		if accin.Phone != "" && !PhoneIndex.IsFree(accin.Phone) {
			logf("phone is not free %s", accin.Phone)
			return false
		}
		updatePhone = true
	}

	var ok bool
	if updateEmail {
		EmailIndex.ResetUser(acc.Email, acc.Uid)
		DomainsStrings.Unset(uint32(acc.Domain), acc.Uid)
		IndexGtLtEmail(oldEmail, acc.Uid, false)

		acc.Email, ok = EmailIndex.InsertUid(accin.Email, acc.Uid)
		if !ok {
			panic("email is not unique " + accin.Email)
		}

		acc.EmailStart = GetEmailStart(accin.Email)
		domain := DomainFromEmail(accin.Email)
		acc.Domain = uint8(DomainsStrings.Add(domain, acc.Uid))
		IndexGtLtEmail(accin.Email, acc.Uid, true)
	}
	if updatePhone {
		if acc.Phone != 0 {
			PhoneIndex.ResetUser(acc.Phone, acc.Uid)
			PhoneCodesStrings.Unset(uint32(acc.Code), acc.Uid)
		}

		acc.Phone, ok = PhoneIndex.InsertUid(accin.Phone, acc.Uid)
		if !ok {
			panic("phone is not unique " + accin.Phone)
		}

		code := CodeFromPhone(accin.Phone)
		acc.Code = uint8(PhoneCodesStrings.Add(code, acc.Uid))
	}

	GetInterest(acc.Uid).Unroll(func(ix int32) {
		if len(accin.Interests) > 0 {
			InterestStrings.Unset(uint32(ix), acc.Uid)
		}
		InterestJoinedGroups[GetJoinYear(acc.Joined)][ix-1]--
		InterestBirthGroups[GetBirthYear(acc.Birth)][ix-1]--
		InterestCountryGroups[acc.Country][ix-1]--
	})

	if accin.Birth != 0 {
		byear := GetBirthYear(acc.Birth)
		nbyear := GetBirthYear(accin.Birth)
		if byear != nbyear {
			BirthYearIndexes[byear].Unset(acc.Uid)
			BirthYearIndexes[nbyear].Set(acc.Uid)
			acc.Birth = accin.Birth
		}
	}

	if accin.Joined != 0 {
		jyear := GetJoinYear(acc.Joined)
		njyear := GetJoinYear(accin.Joined)
		if njyear != jyear {
			JoinYearIndexes[jyear].Unset(acc.Uid)
			JoinYearIndexes[njyear].Set(acc.Uid)
			acc.Joined = accin.Joined
		}
	}

	CountryGroups[acc.Country][acc.StatusIx()+acc.SexIx()*3]--
	CityGroups[acc.City][acc.StatusIx()+acc.SexIx()*3]--

	if accin.Country != "" {
		oldCountry := CountryStrings.GetStr(uint32(acc.Country))
		if oldCountry != accin.Country {
			if acc.Country != 0 {
				CountryStrings.Unset(uint32(acc.Country), acc.Uid)
			}
			acc.Country = uint8(CountryStrings.Add(accin.Country, acc.Uid))
		}
	}

	if accin.City != "" {
		oldCity := CityStrings.GetStr(uint32(acc.City))
		if oldCity != accin.City {
			if acc.City != 0 {
				CityStrings.Unset(uint32(acc.City), acc.Uid)
			}
			acc.City = uint16(CityStrings.Add(accin.City, acc.Uid))
		}
	}

	if accin.Premium.Finish != 0 {
		newPremiumNow := accin.Premium.Start <= CurTs && accin.Premium.Finish > CurTs
		if newPremiumNow != acc.PremiumNow {
			if acc.PremiumNow {
				PremiumNotNow.Set(acc.Uid)
				PremiumNow.Unset(acc.Uid)
			} else {
				PremiumNow.Set(acc.Uid)
				PremiumNotNow.Unset(acc.Uid)
			}
		}
		acc.PremiumStart = accin.Premium.Start
		acc.PremiumLength = GetPremiumLength(accin.Premium.Start, accin.Premium.Finish)
		acc.PremiumNow = accin.Premium.Start <= CurTs && accin.Premium.Finish > CurTs
		PremiumNotNull.Set(acc.Uid)
		PremiumNull.Unset(acc.Uid)
	}

	if accin.Fname != "" {
		oldFname := FnameStrings.GetStr(uint32(acc.Fname))
		if oldFname != accin.Fname {
			if acc.Fname != 0 {
				FnameStrings.Unset(uint32(acc.Fname), acc.Uid)
			}
			acc.Fname = uint8(FnameStrings.Add(accin.Fname, acc.Uid))
		}
	}

	if accin.Sname != "" {
		oldSname := SnameStrings.GetStr(uint32(acc.Sname))
		if oldSname != accin.Sname {
			if acc.Sname != 0 {
				SnameStrings.Unset(uint32(acc.Sname), acc.Uid)
			}
			acc.Sname = uint16(SnameStrings.Add(accin.Sname, acc.Uid))
		}
		SnameOnce.Reset()
	}

	if accin.Sex != "" {
		newSex := accin.Sex == "f"
		if newSex != acc.Sex {
			panic("no sex change")
			/*
				if acc.Sex {
					MaleMap.Unset(acc.Uid)
					FemaleMap.Set(acc.Uid)
				} else {
					MaleMap.Set(acc.Uid)
					FemaleMap.Unset(acc.Uid)
				}
				acc.Sex = newSex
			*/
		}
	}

	if accin.Status != "" {
		newStatus, _ := GetStatusIx(accin.Status)
		if newStatus != acc.Status {
			for _, mp := range StatusMaps[acc.Status] {
				mp.Unset(acc.Uid)
			}
			for _, mp := range StatusMaps[newStatus] {
				mp.Set(acc.Uid)
			}
			acc.Status = newStatus
		}
	}
	CountryGroups[acc.Country][acc.StatusIx()+acc.SexIx()*3]++
	CityGroups[acc.City][acc.StatusIx()+acc.SexIx()*3]++

	if len(accin.Interests) > 0 {
		var newIntersets InterestMask
		for _, interest := range accin.Interests {
			ix := InterestStrings.Add(interest, acc.Uid)
			newIntersets.Set(uint8(ix))
			InterestJoinedGroups[GetJoinYear(acc.Joined)][ix-1]++
			InterestBirthGroups[GetBirthYear(acc.Birth)][ix-1]++
			InterestCountryGroups[acc.Country][ix-1]++
		}
		SetInterests(acc.Uid, newIntersets)
	} else {
		GetInterest(acc.Uid).Unroll(func(ix int32) {
			InterestJoinedGroups[GetJoinYear(acc.Joined)][ix-1]++
			InterestBirthGroups[GetBirthYear(acc.Birth)][ix-1]++
			InterestCountryGroups[acc.Country][ix-1]++
		})
	}

	SetSmallAccount(acc.Uid, acc.SmallAccount())

	return true
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
