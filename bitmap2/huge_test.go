package bitmap2_test

import (
	"math/rand"
	"testing"

	"github.com/funny-falcon/highloadcup2018/bitmap2"
)

func TestHuge(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	gen := func() bitmap2.IMutBitmap { return new(bitmap2.Huge) }
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, gen, 120, func() int32 { return rnd.Int31n(1 << 7) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 9) })
	testIt(t, gen, 1000, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, gen, 60000, func() int32 { return rnd.Int31n(1 << 17) })
	testIt(t, gen, 200, func() int32 { return rnd.Int31n(1 << 8) })
	testIt(t, gen, 800, func() int32 {
		n := rnd.Int31n(1 << 10)
		n = n%256 + n/256*911
		return n
	})
}
