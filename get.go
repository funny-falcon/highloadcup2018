package main

import (
	"bytes"

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
	Premium bool
}

func doFilter(ctx *fasthttp.RequestCtx) {
	args := ctx.QueryArgs()
	iterators := make([]bitmap.Iterator, 0, 4)
	filters := []func(*Account) bool{}

	correct := true
	emptyRes := false

	args.VisitAll(func(key []byte, val []byte) {
		if !correct || emptyRes {
			return
		}
		switch string(key) {
		case "sex_eq":
			if len(val) != 1 {
				correct = false
			}
			switch val[0] {
			case 'm':
				iterators = append(iterators, MaleMap.Iterator(MaxId))
			case 'f':
				iterators = append(iterators, FemaleMap.Iterator(MaxId))
			default:
				correct = false
			}
		case "email_domain":
			domain := string(val)
			ix := DomainsStrings.Find(domain)
			if ix == 0 {
				emptyRes = true
				return
			}
			iterator := DomainsStrings.GetIndex(ix).Iterator(MaxId)
			iterators = append(iterators, iterator)
		case "email_gt":
			if len(val) == 0 {
				return // all are greater
			}
			email := string(val)
			emailgt := GetEmailGte(email)
			filters = append(filters, func(acc *Account) bool {
				if acc.EmailStart < emailgt {
					return false
				}
				accEmail := EmailIndex.GetStr(acc.Email)
				return accEmail > email
			})
			chix := int(email[0]) - 25
			if chix < 0 {
				return
			} else if chix > 25 {
				chix = 25
			}
			iterators = append(iterators, EmailGtIndexes[chix].Iterator(MaxId))
		}
	})
}
