package bitmap3

type SexMap struct {
	Size uint32
	Mask uint32
	L2   [1536]uint32
}

var MaleMask = uint32(0xaaaaaaaa)
var FemaleMask = uint32(0x55555555)

func (s *SexMap) Set(id int32) {
	s.Size++
	Set(s.L2[:], id/32)
}

func (s *SexMap) LoopBlock(f func(int32, uint32) bool) {
	var l2u Unrolled
	for l2ix := int32(len(s.L2) - 1); l2ix >= 0; l2ix-- {
		l2v := s.L2[l2ix]
		if l2v == 0 {
			continue
		}
		l2ixb := l2ix * 32
		for _, l3ix := range Unroll(l2v, l2ixb, &l2u) {
			if !f(l3ix*32, s.Mask) {
				return
			}
		}
	}
}

func (s *SexMap) GetL2() *[1536]uint32 {
	return &s.L2
}

func (s *SexMap) GetBlock(int32) uint32 {
	return s.Mask
}

func (s *SexMap) Has(id int32) bool {
	return (uint32(id)^(s.Mask>>1))&1 == 0
}

func (s *SexMap) Count() uint32 {
	return s.Size
}
