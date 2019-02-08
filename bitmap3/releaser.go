package bitmap3

type Releaser interface {
	Release()
}

type ReleaseHolder struct {
	R []Releaser
}

func (r *ReleaseHolder) Add(rr Releaser) {
	r.R = append(r.R, rr)
}

func (r *ReleaseHolder) Release() {
	for _, rr := range r.R {
		rr.Release()
	}
}
