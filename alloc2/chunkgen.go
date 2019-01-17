package alloc2

import (
	"log"
	"unsafe"

	"golang.org/x/sys/unix"
)

const SlabSize = 1 << 24
const ChunkSizeShift = 18
const ChunkSize = 1 << 18

type ChunkGen struct {
	CurSlab []byte
}

func (g *ChunkGen) Gen() (res *[ChunkSize]byte) {
	if len(g.CurSlab) == 0 {
		var err error
		g.CurSlab, err = unix.Mmap(-1, 0, SlabSize, unix.PROT_READ|unix.PROT_WRITE,
			unix.MAP_PRIVATE|unix.MAP_ANONYMOUS)
		if err != nil {
			log.Fatal(err)
		}
	}
	*(*unsafe.Pointer)(unsafe.Pointer(&res)) = unsafe.Pointer(&g.CurSlab[0])
	g.CurSlab = g.CurSlab[ChunkSize:]
	return res
}

var ChunkGenerator ChunkGen

type Base struct {
	Chunks []*[ChunkSize]byte
	CurEnd uint32
}

func (b *Base) ExtendChunks() *[ChunkSize]byte {
	chunk := ChunkGenerator.Gen()
	b.Chunks = append(b.Chunks, chunk)
	return chunk
}
