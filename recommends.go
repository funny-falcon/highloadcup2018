package main

type Recommends struct {
	Accs      []RecElem
	Limit     int
	Birth     int32
	Heapified bool
}

type RecElem struct {
	*Account
	Commons int
}

func (r *Recommends) Add(acc *Account, common int) {
	el := RecElem{acc, common}
	if len(r.Accs) < r.Limit {
		r.Accs = append(r.Accs, el)
		if len(r.Accs) == r.Limit {
			r.Heapify()
		}
	} else if r.LessAcc(r.Accs[0], el) {
		r.Accs[0] = RecElem{acc, common}
		r.SiftUp(0)
	}
}

func (r *Recommends) Heapify() {
	if r.Heapified {
		return
	}
	r.Heapified = true
	for i := len(r.Accs) - 1; i >= 0; i-- {
		r.SiftUp(i)
	}
}

var recStatus = func() [4]uint8 {
	var r [4]uint8
	r[StatusFreeIx] = 2
	r[StatusComplexIx] = 1
	r[StatusMeetingIx] = 0
	return r
}()

func (r *Recommends) LessAcc(acci, accj RecElem) bool {
	if acci.PremiumNow != accj.PremiumNow {
		return accj.PremiumNow
	}
	if recStatus[acci.Status] != recStatus[accj.Status] {
		return recStatus[acci.Status] < recStatus[accj.Status]
	}
	if acci.Commons != accj.Commons {
		return acci.Commons < accj.Commons
	}
	bi := acci.Birth - r.Birth
	if bi < 0 {
		bi = -bi
	}
	ba := accj.Birth - r.Birth
	if ba < 0 {
		ba = -ba
	}
	if bi != ba {
		return bi > ba
	}
	return acci.Uid > accj.Uid
}

func (r *Recommends) Pop() {
	l := len(r.Accs) - 1
	r.Accs[0] = r.Accs[l]
	r.Accs = r.Accs[:l]
	if l > 0 {
		r.SiftUp(0)
	}
}

func (r *Recommends) SiftUp(i int) {
	el := r.Accs[i]
	l := len(r.Accs)
	for i*2+1 < l {
		c1 := i*2 + 1
		c2 := c1 + 1
		if c2 < l && r.LessAcc(r.Accs[c2], r.Accs[c1]) {
			c1 = c2
		}
		if r.LessAcc(el, r.Accs[c1]) {
			break
		}
		r.Accs[i] = r.Accs[c1]
		i = c1
	}
	r.Accs[i] = el
}
