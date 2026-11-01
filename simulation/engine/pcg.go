package engine

import "hash/fnv"

// Each named PCG32 XSH RR stream has a stable FNV-derived sequence selector.
// A draw in one stream cannot change another stream's state.
type rngStream struct {
	state     uint64
	increment uint64
}

func newNamedStream(rootSeed uint64, name string) rngStream {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	selector := h.Sum64()
	s := rngStream{increment: (selector << 1) | 1}
	s.nextUint32()
	s.state += rootSeed ^ selector
	s.nextUint32()
	return s
}

func (s *rngStream) nextUint32() uint32 {
	old := s.state
	s.state = old*6364136223846793005 + s.increment
	xorshifted := uint32(((old >> 18) ^ old) >> 27)
	rotation := uint32(old >> 59)
	return (xorshifted >> rotation) | (xorshifted << ((-rotation) & 31))
}

func (s *rngStream) nextFloat() float64 { return float64(s.nextUint32()) / 4294967296.0 }

func (s *rngStream) nextOpenFloat() float64 { return (float64(s.nextUint32()) + 0.5) / 4294967296.0 }
