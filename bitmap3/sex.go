package bitmap3

type SexMap struct {
	Size uint32
	Mask uint64
	L2   [384]uint64
}

var MaleMask = uint64(0xaaaaaaaaaaaaaaaa)
var FemaleMask = uint64(0x5555555555555555)

func (s *SexMap) Set(id int32) {
	s.Size++
	Set(s.L2[:], id/64)
}

func (s *SexMap) LoopBlock(f func(int32, uint64) bool) {
	var l2u Unrolled
	for l2ix := int32(len(s.L2) - 1); l2ix >= 0; l2ix-- {
		l2v := s.L2[l2ix]
		if l2v == 0 {
			continue
		}
		l2ixb := l2ix * 64
		for _, l3ix := range Unroll(l2v, l2ixb, &l2u) {
			if !f(l3ix*64, s.Mask) {
				return
			}
		}
	}
}

func (s *SexMap) GetL2() *[384]uint64 {
	return &s.L2
}

func (s *SexMap) GetBlock(int32) uint64 {
	return s.Mask
}

func (s *SexMap) Has(id int32) bool {
	return (uint64(id)^(s.Mask>>1))&1 == 0
}

func (s *SexMap) Count() uint32 {
	return s.Size
}
