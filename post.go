package main

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/funny-falcon/highloadcup2018/bitmap2"
)

var globMutex sync.RWMutex

func postHandler(ctx *Request, path string) {
	switch {
	case path == "new/":
		if !doNew(ctx) {
			ctx.SetStatusCode(400)
		}
	case path == "likes/":
		if !doLikes(ctx) {
			ctx.SetStatusCode(400)
		}
	case strings.HasSuffix(path, "/"):
		ids := path[:len(path)-1]
		id, err := strconv.Atoi(string(ids))
		if err != nil {
			ctx.SetStatusCode(404)
			return
		}
		if !doUpdate(ctx, id) {
			ctx.SetStatusCode(400)
		}
	default:
		ctx.SetStatusCode(404)
	}
}

var unix1950 = int32(time.Date(1950, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
var unix2005 = int32(time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
var unix2011 = int32(time.Date(2011, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
var unix2018 = int32(time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC).Unix())

func doNew(ctx *Request) bool {
	var accin AccountIn

	ok := func() bool {
		globMutex.RLock()
		defer globMutex.RUnlock()

		iter := jsonConfig.BorrowIterator(ctx.Body)
		iter.ReadVal(&accin)
		if iter.Error != nil {
			logf("doNew iter error: %v", iter.Error)
			return false
		}

		if accin.Id == 0 {
			logf("id is not set")
			return false
		}
		if HasAccount(int32(accin.Id)) != nil {
			logf("id is already used %d", accin.Id)
			return false
		}
		if !commonValidate(&accin, false) {
			return false
		}
		otherMap := &MaleMap
		if accin.Sex == "m" {
			otherMap = &FemaleMap
		}
		for _, like := range accin.Likes {
			if like.Ts < accin.Joined {
				logf("like ts %d less than joined %d", like.Ts, accin.Joined)
				return false
			}
			if !AccountsMap.Has(like.Id) {
				logf("user %d doesn't exists to be liked by %d", like.Id, accin.Id)
				return false
			}
			if !otherMap.Has(like.Id) {
				logf("user %d is not of other sex", like.Id, accin.Id)
				return false
			}
		}
		if !EmailIndex.IsFree(accin.Email) {
			logf("email is not free %s", accin.Email)
			return false
		}
		if accin.Phone != "" && !PhoneIndex.IsFree(accin.Phone) {
			logf("phone is not free %s", accin.Phone)
			return false
		}
		return true
	}()
	if !ok {
		return false
	}

	globMutex.Lock()
	InsertAccount(&accin)
	globMutex.Unlock()
	ctx.SetStatusCode(201)
	ctx.SetBody([]byte("{}"))
	return true
}

type DoLike struct {
	Liker int32
	Likee int32
	Ts    int32
}

func doLikes(ctx *Request) bool {
	var likes []DoLike
	var like struct {
		Liker uint32 `json:"liker"`
		Likee uint32 `json:"likee"`
		Ts    int32  `json:"ts"`
	}

	ok := func() bool {
		globMutex.RLock()
		defer globMutex.RUnlock()

		iter := jsonConfig.BorrowIterator(ctx.Body)
		defer jsonConfig.ReturnIterator(iter)

		if attr := iter.ReadObject(); attr != "likes" {
			logf("likes doesn't likes")
			return false
		}
		for iter.ReadArray() {
			iter.ReadVal(&like)
			if !AccountsMap.Has(int32(like.Likee)) {
				logf("there is no likee %d", like.Likee)
				return false
			}
			if !AccountsMap.Has(int32(like.Liker)) {
				logf("there is no liker %d", like.Liker)
				return false
			}
			likes = append(likes, DoLike{
				Liker: int32(like.Liker),
				Likee: int32(like.Likee),
				Ts:    like.Ts,
			})
		}
		if iter.Error != nil || iter.ReadObject() != "" || iter.Error != nil {
			logf("parsing likes fails: %v", iter.Error)
			return false
		}
		return true
	}()
	if !ok {
		return false
	}

	globMutex.Lock()
	for _, like := range likes {
		bitmap2.GetSmall(&HasAccount(like.Liker).Likes).Set(like.Likee)
		SureLikers(like.Likee, func(l *bitmap2.Likes) { l.SetTs(like.Liker, like.Ts) })
	}
	globMutex.Unlock()

	logf("doLikes Looks to be ok")
	ctx.SetStatusCode(202)
	ctx.SetBody([]byte("{}"))
	return true
}

func doUpdate(ctx *Request, id int) bool {
	var accin AccountIn
	var acc *Account

	var res bool
	ok := func() bool {
		globMutex.RLock()
		defer globMutex.RUnlock()

		iter := jsonConfig.BorrowIterator(ctx.Body)
		iter.ReadVal(&accin)
		if iter.Error != nil {
			logf("doNew iter error: %v", iter.Error)
			return false
		}

		if accin.Id != 0 {
			logf("id should not be set in update")
			return false
		}
		acc = HasAccount(int32(id))
		if acc == nil {
			logf("user is not found %d", accin.Id)
			ctx.SetStatusCode(404)
			res = true
			return false
		}
		if !commonValidate(&accin, true) {
			return false
		}

		if len(accin.Likes) != 0 {
			logf("could not update likes")
			return false
		}
		return true
	}()
	if !ok {
		return res
	}

	globMutex.Lock()
	ok = UpdateAccount(acc, &accin)
	globMutex.Unlock()
	if !ok {
		return false
	}

	logf("doLikes Looks to be ok")
	ctx.SetStatusCode(202)
	ctx.SetBody([]byte("{}"))
	return true
}

func commonValidate(accin *AccountIn, update bool) bool {
	if (!update && accin.Email == "") || len(accin.Email) > 100 {
		logf("email is invalid %s", accin.Email)
		return false
	}
	if accin.Email != "" {
		ixdog := strings.IndexByte(accin.Email, '@')
		if ixdog == -1 {
			logf("email has no @ %s", accin.Email)
			return false
		}
		ixdot := strings.IndexByte(accin.Email[ixdog:], '.')
		if ixdot == -1 || ixdog+ixdot > len(accin.Email)-2 {
			logf("email has no . %s", accin.Email)
			return false
		}
	}
	if len(accin.Phone) > 16 {
		logf("phone is too long %s", accin.Phone)
		return false
	}
	_, ok := GetStatusIx(accin.Status)
	if !ok && (!update || accin.Status != "") {
		logf("status is not ok %s", accin.Status)
		return false
	}
	if (accin.Birth < unix1950 || accin.Birth > unix2005) && (!update || accin.Birth != 0) {
		logf("birth is not ok %d", accin.Birth)
		return false
	}
	if (accin.Joined < unix2011 || accin.Joined > unix2018) && (!update || accin.Birth != 0) {
		logf("birth is not ok %d", accin.Birth)
		return false
	}
	if accin.Sex != "m" && accin.Sex != "f" && (!update || accin.Sex != "") {
		logf("sex is not ok %s", accin.Sex)
		return false
	}
	if len(accin.Country) > 50 {
		logf("to long country %s", accin.Country)
		return false
	}
	if len(accin.City) > 50 {
		logf("to long city %s", accin.Country)
		return false
	}
	if len(accin.Fname) > 50 {
		logf("too long fname %s", accin.Fname)
		return false
	}
	if len(accin.Sname) > 50 {
		logf("too long sname %s", accin.Fname)
		return false
	}
	for _, int := range accin.Interests {
		if int == "" || len(int) > 100 {
			logf("invalid interest %s", int)
			return false
		}
	}
	if (accin.Premium.Start != 0 || accin.Premium.Finish != 0) &&
		(accin.Premium.Start < unix2018 || accin.Premium.Finish < unix2018) {
		logf("invalid premium %v", accin.Premium)
		return false
	}

	return true
}
