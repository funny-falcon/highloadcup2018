package bitmap2_test

import (
	"math/rand"
	"testing"

	"github.com/funny-falcon/highloadcup2018/bitmap2"
)

func TestSmall(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	gen := func() bitmap2.IMutBitmap {
		return &bitmap2.Small{}
	}
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, gen, 120, func() int32 { return rnd.Int31n(1 << 7) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 9) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 8) })
	testIt(t, gen, 200, func() int32 {
		n := rnd.Int31n(1 << 10)
		n = n%256 + n/256*911
		return n
	})
}

func TestLikes(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	gen := func() bitmap2.IMutBitmap {
		return &bitmap2.Likes{}
	}
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, gen, 120, func() int32 { return rnd.Int31n(1 << 7) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 9) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 8) })
	testIt(t, gen, 200, func() int32 {
		n := rnd.Int31n(1 << 10)
		n = n%256 + n/256*911
		return n
	})
}
