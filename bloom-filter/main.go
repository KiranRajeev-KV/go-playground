package main

import (
	"errors"
	"fmt"
	"hash/maphash"
	"log"
)

type hash struct {
	seed1 maphash.Seed
	seed2 maphash.Seed
}

func (h *hash) hash(s string) (uint64, uint64) {
	h1 := maphash.String(h.seed1, s)
	h2 := maphash.String(h.seed2, s)
	return h1, h2
}

type storage struct {
	m uint64
	k int
	a []uint64
	h hash
}

func New(m uint64, k int) (*storage, error) {
	if m == 0 {
		return nil, errors.New("m cannot be zero")
	}
	if k < 0 {
		return nil, errors.New("k cannot be less than zero")
	}
	return &storage{
		m: m,
		k: k,
		a: make([]uint64, (m+63)/64),
		h: hash{
			seed1: maphash.MakeSeed(),
			seed2: maphash.MakeSeed(),
		},
	}, nil
}

func (s *storage) set(i string) {
	h1, h2 := s.h.hash(i)
	pos := h1 % s.m
	step := h2 % s.m

	positions := make([]uint64, s.k)

	for j := range positions {
		positions[j] = pos
		pos = (pos + step) % s.m
	}
	fmt.Printf("inside set: %v\n", positions)

	for _, bitIndex := range positions {
		wordIndex := bitIndex / 64
		mask := uint64(1) << (bitIndex % 64)
		s.a[wordIndex] |= mask
	}
	// fmt.Printf("%v\n", s.a[i/64])
}

func (s *storage) has(i string) bool {
	h1, h2 := s.h.hash(i)
	pos := h1 % s.m
	step := h2 % s.m

	positions := make([]uint64, s.k)

	for j := range positions {
		positions[j] = pos
		pos = (pos + step) % s.m
	}

	fmt.Printf("inside has: %v\n", positions)

	for _, bitIndex := range positions {
		wordIndex := bitIndex / 64
		mask := uint64(1) << (bitIndex % 64)
		if s.a[wordIndex]&mask == 0 {
			return false
		}
	}
	return true
}

func main() {
	// fmt.Println("Hello, World!")
	// fmt.Println(float64(47950/64))
	// fmt.Println(math.Ceil(float64(47950 / 64)))
	store, err := New(130, 7)
	if err != nil {
		log.Fatalf("create storage: %v", err)
	}
	fmt.Println(store.m)
	fmt.Println(len(store.a))

	store.set("kiran")
	fmt.Println(store.has("kiran"))
	fmt.Println(store.has("auco"))
	fmt.Println(store.has("jev"))

}
