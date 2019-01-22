package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type Once struct {
	f    func()
	t    *time.Timer
	tr   time.Time
	m    sync.Mutex
	done uint32
}

func NewOnce(f func()) *Once {
	o := &Once{
		f: f,
	}
	o.t = time.AfterFunc(24*time.Hour, o.Sure)
	return o
}

func (o *Once) Sure() {
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}
	// Slow-path.
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		o.f()
	}
}

func (o *Once) Reset() {
	atomic.StoreUint32(&o.done, 0)
	now := time.Now()
	if now.Sub(o.tr) > time.Second/8 {
		o.tr = now
		o.t.Reset(time.Second / 2)
	}
}
