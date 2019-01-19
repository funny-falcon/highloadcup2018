package bitmap2

import (
	"math/bits"
	"unsafe"
)

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

func searchSparse16(b []uint16, sid uint16) int {
	i, l := 0, len(b)
	if l > 0 {
		ptr := ptr0_16(b)
		if l > 32 {
			m := 1 << uint8(bits.Len32(uint32(l))-1)
			for ; m > 0; m >>= 1 {
				if n := i + m - 1; n < l {
					if *aref16(ptr, n) > sid {
						i += m
					} else {
						for m >>= 1; m > 0; m >>= 1 {
							if n := i + m - 1; *aref16(ptr, n) > sid {
								i += m
							}
						}
						break
					}
				}
			}
		} else {
			for ; i < l; i++ {
				if *aref16(ptr, i) <= sid {
					break
				}
			}
		}
	}
	return i
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
			if e.UidAndCnt>>8 <= sid {
				return i
			}
		}
	}
	m := 1 << uint8(bits.Len32(uint32(l))-1)
	for ; m > 0; m >>= 1 {
		if n := i + m - 1; n < l {
			if b[n].UidAndCnt>>8 > sid {
				i += m
			} else {
				for m >>= 1; m > 0; m >>= 1 {
					if n := i + m - 1; b[n].UidAndCnt>>8 > sid {
						i += m
					}
				}
				break
			}
		}
	}
	return i
}
