package main

type counter struct {
	u uint32
	s float64
}

func SortGroupLimit(limit int, order int, gr []counter, less func(idi, idj uint32) bool) []counter {
	i := 0
	for _, g := range gr {
		if g.s > 0 {
			gr[i] = g
			i++
		}
	}
	gr = gr[:i]
	if limit > i {
		limit = i
	}
	if order == 1 {
		SortGroupLimitAsc(limit, gr, less)
	} else {
		SortGroupLimitDesc(limit, gr, less)
	}
	if limit < len(gr) {
		gr = gr[:limit]
	}
	return gr
}

func SortGroupLimitAsc(limit int, gr []counter, less func(idi, idj uint32) bool) {
	l := len(gr)
	if l < 10 {
		if l > 6 {
			for i := 0; i+5 < l; i++ {
				gri, grj := gr[i], gr[i+5]
				if gri.s > grj.s || gri.s == grj.s && less(grj.u, gri.u) {
					gr[i], gr[i+5] = grj, gri
				}
			}
		}
		for i := 1; i < l; i++ {
			cur, j := gr[i], i-1
			for ; j >= 0 && (cur.s < gr[j].s || cur.s == gr[j].s && less(cur.u, gr[j].u)); j-- {
				gr[j+1] = gr[j]
			}
			gr[j+1] = cur
		}
		return
	}
	mid := gr[l/2]
	{
		a := gr[0]
		b := gr[l-1]
		if a.s > mid.s || a.s == mid.s && less(mid.u, a.u) {
			a, mid = mid, a
		}
		if b.s < mid.s || b.s == mid.s && less(b.u, mid.u) {
			b, mid = mid, b
			if a.s > mid.s || a.s == mid.s && less(mid.u, a.u) {
				a, mid = mid, a
			}
		}
	}
	i := 0
	for j, cur := range gr {
		if cur.s < mid.s || cur.s == mid.s && !less(mid.u, cur.u) {
			gr[i], gr[j] = cur, gr[i]
			i++
		}
	}
	if limit > i {
		SortGroupLimitAsc(i, gr[:i], less)
		SortGroupLimitAsc(limit-i, gr[i:], less)
	} else {
		SortGroupLimitAsc(limit, gr[:i], less)
	}
}

func SortGroupLimitDesc(limit int, gr []counter, less func(idi, idj uint32) bool) {
	l := len(gr)
	if l < 10 {
		if l > 6 {
			for i := 0; i+5 < l; i++ {
				gri, grj := gr[i], gr[i+5]
				if gri.s < grj.s || gri.s == grj.s && less(gri.u, grj.u) {
					gr[i], gr[i+5] = grj, gri
				}
			}
		}
		for i := 1; i < l; i++ {
			cur, j := gr[i], i-1
			for ; j >= 0 && (cur.s > gr[j].s || cur.s == gr[j].s && !less(cur.u, gr[j].u)); j-- {
				gr[j+1] = gr[j]
			}
			gr[j+1] = cur
		}
		return
	}
	mid := gr[l/2]
	{
		a := gr[0]
		b := gr[l-1]
		if a.s > mid.s || a.s == mid.s && less(mid.u, a.u) {
			a, mid = mid, a
		}
		if b.s < mid.s || b.s == mid.s && less(b.u, mid.u) {
			b, mid = mid, b
			if a.s > mid.s || a.s == mid.s && less(mid.u, a.u) {
				a, mid = mid, a
			}
		}
	}
	i := 0
	for j, cur := range gr {
		if cur.s > mid.s || cur.s == mid.s && !less(cur.u, mid.u) {
			gr[i], gr[j] = cur, gr[i]
			i++
		}
	}
	if limit > i {
		SortGroupLimitAsc(i, gr[:i], less)
		SortGroupLimitAsc(limit-i, gr[i:], less)
	} else {
		SortGroupLimitAsc(limit, gr[:i], less)
	}
}
