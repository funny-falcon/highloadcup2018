package bitmap3

type Releaser interface {
	Release()
}

type ReleaseHolder struct {
	R []Releaser
}

func (r *ReleaseHolder) Add(rr Releaser) {
	if r == nil {
		return
	}
	r.R = append(r.R, rr)
}

func (r *ReleaseHolder) Release() {
	if r == nil {
		return
	}
	for _, rr := range r.R {
		rr.Release()
	}
}
