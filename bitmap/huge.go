package bitmap

type Huge struct {
	B []Block
}

func (h *Huge) Set(i int32) {
	k := int(i >> 8)
	for k >= len(h.B) {
		h.B = append(h.B, Block{})
	}
	h.B[k].Set(uint8(i))
}

func (h *Huge) Unset(i int32) {
	k := int(i >> 8)
	if k >= len(h.B) {
		return
	}
	h.B[k].Unset(uint8(i))
}

func (h *Huge) LastSpan() int32 {
	return int32(len(h.B)) << 8
}

func (h *Huge) Reset() {}

func (h *Huge) FetchAndNext(span int32) (Block, int32) {
	k := int(span >> 8)
	if k >= len(h.B) {
		return Block{}, h.LastSpan()
	}
	return h.B[k], span - SpanSize
}

func (h *Huge) Stat() (size, compact, count int) {
	for _, bl := range h.B {
		size++
		cnt := bl.Count()
		if cnt <= 4 {
			compact++
		}
		count += int(cnt)
	}
	return
}

func (h *Huge) Iterator(maxId int32) Iterator {
	return h
}
