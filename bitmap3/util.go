package bitmap3

import (
	"math/bits"
	"unsafe"
)

func Set(u []uint32, id int32) bool {
	k, b := id/32, uint32(1)<<uint32(id&31)
	v := u[k]
	if v&b == 0 {
		u[k] = v | b
		return true
	}
	return false
}

func Unset(u []uint32, id int32) bool {
	k, b := id/32, uint32(1)<<uint32(id&31)
	v := u[k]
	if v&b != 0 {
		u[k] = v ^ b
		return true
	}
	return false
}

func Has(u []uint32, id int32) bool {
	k, b := id/32, uint32(1)<<uint32(id&31)
	return u[k]&b != 0
}

type Unrolled [32]int32

func Unroll(v uint32, span int32, r *Unrolled) []int32 {
	rp := uintptr(unsafe.Pointer(r))
	p := rp + 31*4
	for i := 0; i < 2; i++ {
		if b := uintptr(v & 0xffff); b != 0 {
			*aref32(p, 0) = span + 0
			p -= uintptr((b << 2) & 4)
			*aref32(p, 0) = span + 1
			p -= uintptr((b << 1) & 4)
			*aref32(p, 0) = span + 2
			p -= uintptr((b << 0) & 4)
			*aref32(p, 0) = span + 3
			p -= uintptr((b >> 1) & 4)
			*aref32(p, 0) = span + 4
			p -= uintptr((b >> 2) & 4)
			*aref32(p, 0) = span + 5
			p -= uintptr((b >> 3) & 4)
			*aref32(p, 0) = span + 6
			p -= uintptr((b >> 4) & 4)
			*aref32(p, 0) = span + 7
			p -= uintptr((b >> 5) & 4)
			*aref32(p, 0) = span + 8
			p -= uintptr((b >> 6) & 4)
			*aref32(p, 0) = span + 9
			p -= uintptr((b >> 7) & 4)
			*aref32(p, 0) = span + 10
			p -= uintptr((b >> 8) & 4)
			*aref32(p, 0) = span + 11
			p -= uintptr((b >> 9) & 4)
			*aref32(p, 0) = span + 12
			p -= uintptr((b >> 10) & 4)
			*aref32(p, 0) = span + 13
			p -= uintptr((b >> 11) & 4)
			*aref32(p, 0) = span + 14
			p -= uintptr((b >> 12) & 4)
			*aref32(p, 0) = span + 15
			p -= uintptr((b >> 13) & 4)
		}
		v >>= 16
		span += 16
	}
	return r[(p+4-rp)/4:]
}

func aref32(ptr uintptr, i int) *int32 {
	return (*int32)(unsafe.Pointer(ptr + uintptr(i)*4))
}
func aref16(ptr uintptr, i int) *uint16 {
	return (*uint16)(unsafe.Pointer(ptr + uintptr(i)*2))
}

func aref8(ptr uintptr, i int) *uint8 {
	return (*uint8)(unsafe.Pointer(ptr + uintptr(i)))
}

func ptr0_32(b []int32) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

func ptr0_16(b []uint16) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

func ptr0_8(b []uint8) uintptr {
	return uintptr(unsafe.Pointer(&b[0]))
}

func searchSparse32(b []int32, sid int32) int {
	i, l := 0, len(b)
	if l > 0 {
		ptr := ptr0_32(b)
		if l > 32 {
			m := 1 << uint8(bits.Len32(uint32(l))-1)
			for ; m > 0; m >>= 1 {
				if n := i + m - 1; n < l {
					if *aref32(ptr, n) > sid {
						i += m
					} else {
						for m >>= 1; m > 0; m >>= 1 {
							if n := i + m - 1; *aref32(ptr, n) > sid {
								i += m
							}
						}
						break
					}
				}
			}
		} else {
			for ; i < l; i++ {
				if *aref32(ptr, i) <= sid {
					break
				}
			}
		}
	}
	return i
}

func searchSparseLikes(b []LikesElem, sid int32) int {
	i, l := 0, len(b)
	if l == 0 {
		return 0
	}
	if l < 32 {
		for i, e := range b {
			if e.Uid <= sid {
				return i
			}
		}
		return len(b)
	}
	m := 1 << uint8(bits.Len32(uint32(l))-1)
	for ; m > 0; m >>= 1 {
		if n := i + m - 1; n < l {
			if b[n].Uid > sid {
				i += m
			} else {
				for m >>= 1; m > 0; m >>= 1 {
					if n := i + m - 1; b[n].Uid > sid {
						i += m
					}
				}
				break
			}
		}
	}
	return i
}
