package bitmap_test

import (
	"math/rand"
	"testing"

	"github.com/funny-falcon/highloadcup2018/bitmap"
)

func TestLarge(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	testIt(t, bitmap.LargeEmpty, 1000, func() int32 { return rnd.Int31n(1 << 20) })
	testIt(t, bitmap.LargeEmpty, 200, func() int32 { return rnd.Int31n(1 << 8) })
	testIt(t, bitmap.LargeEmpty, 100000, func() int32 { return rnd.Int31n(1 << 17) })
	testIt(t, bitmap.LargeEmpty, 800, func() int32 {
		n := rnd.Int31n(1 << 10)
		n = n%256 + n/256*911
		return n
	})
}
