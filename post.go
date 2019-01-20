package main

import (
	"bytes"
	"time"

	"github.com/funny-falcon/highloadcup2018/bitmap2"
	"github.com/valyala/fasthttp"
)

func postHandler(ctx *fasthttp.RequestCtx, path []byte) {
	logf("post Path: %s, args: %s", path, ctx.QueryArgs())
	switch {
	case bytes.Equal(path, []byte("new/")):
		if !doNew(ctx) {
			ctx.SetStatusCode(400)
		}
	case bytes.Equal(path, []byte("likes/")):
		if !doLikes(ctx) {
			ctx.SetStatusCode(400)
		}
	/*
		case bytes.HasSuffix(path, []byte("/")):
			ids := path[:bytes.IndexByte(path, '/')]
			id, err := strconv.Atoi(string(ids))
			if err != nil {
				ctx.SetStatusCode(400)
				return
			}
			doUpdate(ctx, id)
	*/
	default:
		ctx.SetStatusCode(404)
	}
}

var unix1950 = int32(time.Date(1950, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
var unix2005 = int32(time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
var unix2011 = int32(time.Date(2011, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
var unix2018 = int32(time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC).Unix())

func doNew(ctx *fasthttp.RequestCtx) bool {
	var accin AccountIn
	iter := jsonConfig.BorrowIterator(ctx.PostBody())
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
	if accin.Email == "" || len(accin.Email) > 100 {
		logf("email is invalid %s", accin.Email)
		return false
	}
	if len(accin.Phone) > 16 {
		logf("phone is too long %s", accin.Phone)
		return false
	}
	statusix, ok := GetStatusIx(accin.Status)
	if !ok {
		logf("status is not ok %s", statusix)
		return false
	}
	if accin.Birth < unix1950 || accin.Birth > unix2005 {
		logf("birth is not ok %d", accin.Birth)
		return false
	}
	if accin.Sex != "m" && accin.Sex != "f" {
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
	otherMap := &MaleMap
	if accin.Sex == "m" {
		otherMap = &FemaleMap
	}
	for _, like := range accin.Likes {
		if like.Ts < accin.Joined {
			logf("like ts %d less than joined %d", like.Ts, accin.Joined)
			return true
		}
		if !AccountsMap.Has(like.Id) {
			logf("user %d doesn't exists to be liked by %d", like.Id, accin.Id)
		}
		if !otherMap.Has(like.Id) {
			logf("user %d is not of other sex", like.Id, accin.Id)
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

	InsertAccount(&accin)
	ctx.SetStatusCode(201)
	ctx.SetBody([]byte("{}"))
	return true
}

type DoLike struct {
	Liker int32
	Likee int32
	Ts    int32
}

func doLikes(ctx *fasthttp.RequestCtx) bool {
	iter := jsonConfig.BorrowIterator(ctx.PostBody())
	defer jsonConfig.ReturnIterator(iter)
	if attr := iter.ReadObject(); attr != "likes" {
		logf("likes doesn't likes")
		return false
	}
	var likes []DoLike
	var like struct {
		Liker uint32 `json:"liker"`
		Likee uint32 `json:"likee"`
		Ts    int32  `json:"ts"`
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

	for _, like := range likes {
		bitmap2.GetSmall(&HasAccount(like.Liker).Likes).Set(like.Likee)
		SureLikers(like.Likee, func(l *bitmap2.Likes) { l.SetTs(like.Liker, like.Ts) })
	}

	logf("doLikes Looks to be ok")
	ctx.SetStatusCode(202)
	ctx.SetBody([]byte("{}"))
	return true
}
