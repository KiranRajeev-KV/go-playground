package main

import (
	"fmt"
)

type storage struct {
	m int
	a []uint64
}

func New(m int) *storage {
	return &storage{
		m: m,
		a: make([]uint64, (m+63)/64),
	}
}

func (s *storage) set(i int) {
	mask := uint64(1) << (i % 64)
	s.a[i/64] |= mask
	// fmt.Printf("%v\n", s.a[i/64])
}

func (s *storage) has(i int) bool {
	mask := uint64(1) << (i % 64)
	return s.a[i/64]&mask != 0
}
func main() {
	// fmt.Println("Hello, World!")
	// fmt.Println(float64(47950/64))
	// fmt.Println(math.Ceil(float64(47950 / 64)))
	store := New(130)
	fmt.Println(store.m)
	fmt.Println(len(store.a))

	store.set(70)
	fmt.Println(store.has(70))
	fmt.Println(store.has(69))
	fmt.Println(store.has(71))

}
