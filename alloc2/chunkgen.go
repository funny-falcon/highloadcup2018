package alloc2

import (
	"log"
	"syscall"
	"unsafe"
)

const SlabSize = 1 << 24
const ChunkSizeShift = 16
const ChunkSize = 1 << ChunkSizeShift
const ChunkMask = ChunkSize - 1

type Chunk [ChunkSize]byte

type ChunkGen struct {
	CurSlab    []byte
	TotalAlloc int
}

func (g *ChunkGen) Gen() (res *[ChunkSize]byte) {
	if len(g.CurSlab) == 0 {
		var err error
		g.CurSlab, err = mmap(0x80000000, SlabSize, syscall.PROT_READ|syscall.PROT_WRITE,
			syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS, -1, 0)
		if err != nil {
			log.Fatal(err)
		}
		k := uintptr(unsafe.Pointer(&g.CurSlab[0])) % ChunkSize
		if k > 0 {
			if err = munmap(g.CurSlab[: ChunkSize-k : ChunkSize-k]); err != nil {
				log.Fatal(err)
			}
			if err = munmap(g.CurSlab[SlabSize-k:]); err != nil {
				log.Fatal(err)
			}
			g.CurSlab = g.CurSlab[ChunkSize-k : SlabSize-k : SlabSize-k]
		}
		g.TotalAlloc += len(g.CurSlab)
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
