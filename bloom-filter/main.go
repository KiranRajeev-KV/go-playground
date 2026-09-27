package main

import (
	"errors"
	"fmt"
	"hash/maphash"
	"log"
)

type hash struct {
	seed maphash.Seed
}

func (h *hash) hash(s string) uint64 {
	v := maphash.String(h.seed, s)
	return v
}

type storage struct {
	m uint64
	a []uint64
	h hash
}

func New(m uint64) (*storage, error) {
	if m == 0 {
		return nil, errors.New("m cannot be zero")
	}
	return &storage{
		m: m,
		a: make([]uint64, (m+63)/64),
		h: hash{
			seed: maphash.MakeSeed(),
		},
	}, nil
}

func (s *storage) set(i string) {
	h := s.h.hash(i)
	index := h % s.m

	mask := uint64(1) << (index % 64)
	s.a[index/64] |= mask
	// fmt.Printf("%v\n", s.a[i/64])
}

func (s *storage) has(i string) bool {
	h := s.h.hash(i)
	index := h % s.m
	mask := uint64(1) << (index % 64)
	return s.a[index/64]&mask != 0
}

func main() {
	// fmt.Println("Hello, World!")
	// fmt.Println(float64(47950/64))
	// fmt.Println(math.Ceil(float64(47950 / 64)))
	store, err := New(0)
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
