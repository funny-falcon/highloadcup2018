package bitmap

type Block [8]uint32

const all = ^uint32(0)

var AllBlock = Block{all, all, all, all, all, all, all, all}

func (b *Block) Set(i uint8) bool {
	r := b.Has(i)
	b[i>>5] |= 1 << (i & 31)
	return !r
}

func (b *Block) Unset(i uint8) bool {
	r := b.Has(i)
	b[i>>5] &^= 1 << (i & 31)
	return r
}

func (b *Block) Has(i uint8) bool {
	return b[i>>5]&(1<<(i&31)) != 0
}

func BlockMask(lastbit uint8) Block {
	var bl Block
	var i int
	for i = 0; i < 8 && lastbit >= 31; i++ {
		bl[i] = all
		lastbit -= 32
	}
	if i < 8 {
		bl[i] = all >> (31 - lastbit)
	}
	return bl
}

func (b *Block) Intersect(o Block) {
	b[0] &= o[0]
	b[1] &= o[1]
	b[2] &= o[2]
	b[3] &= o[3]
	b[4] &= o[4]
	b[5] &= o[5]
	b[6] &= o[6]
	b[7] &= o[7]
}

func (b *Block) Union(o Block) {
	b[0] |= o[0]
	b[1] |= o[1]
	b[2] |= o[2]
	b[3] |= o[3]
	b[4] |= o[4]
	b[5] |= o[5]
	b[6] |= o[6]
	b[7] |= o[7]
}

func (b *Block) Unroll(span int32, r *[256]int32) []int32 {
	k := uint8(0)
	span += 256
	for j := 7; j >= 0; j-- {
		v := b[j]
		span -= 32
		if v == 0 {
			continue
		}
		sp := span + 31
		for ; v != 0; v <<= 4 {
			k = unroll2(v>>30, sp, k, r)
			k = unroll2((v>>28)&3, sp-2, k, r)
			sp -= 4
		}
	}
	return r[:k]
}

func unroll2(v uint32, sp int32, k uint8, r *[256]int32) uint8 {
	switch v {
	case 3:
		r[k] = sp
		r[k+1] = sp - 1
		k += 2
	case 2:
		r[k] = sp
		k++
	case 1:
		r[k] = sp - 1
		k++
	case 0:
	}
	return k
}
