package alloc

import (
	"log"
	"unsafe"

	"github.com/modern-go/reflect2"
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

func (b *Base) Get(ref Ptr, ptr interface{}) {
	chunkn, off := ref>>ChunkSizeShift, ref&(ChunkSize-1)
	//log.Printf("chunks %d ref %d chunkn %d off %d", len(b.Chunks), ref, chunkn, off)
	addr := unsafe.Pointer(&b.Chunks[chunkn][off])
	*(*unsafe.Pointer)(reflect2.PtrOf(ptr)) = addr
}

func (b *Base) GetPtr(ref Ptr) unsafe.Pointer {
	chunkn, off := ref>>ChunkSizeShift, ref&(ChunkSize-1)
	//log.Printf("chunks %d ref %d chunkn %d off %d", len(b.Chunks), ref, chunkn, off)
	return unsafe.Pointer(&b.Chunks[chunkn][off])
}

func (b *Base) ExtendChunks() uint32 {
	b.Chunks = append(b.Chunks, ChunkGenerator.Gen())
	b.CurEnd = ChunkSize * uint32(len(b.Chunks))
	return b.LastChunk()
}

func (b *Base) LastChunk() uint32 {
	return uint32(len(b.Chunks)-1) * ChunkSize
}
