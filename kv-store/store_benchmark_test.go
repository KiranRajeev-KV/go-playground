package kvstore_test

import (
	"kvstore"
	"testing"
)

func BenchmarkSequentialGet(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 10)
	for b.Loop() {
		db.Get(1)
	}
}

func BenchmarkSequentialSet(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 10)
	for b.Loop() {
		db.Set(1, 11)
	}
}

func BenchmarkParallelGet(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 10)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db.Get(1)
		}
	})
}

func BenchmarkSnapshot(b *testing.B) {
	cases := []struct {
		name string
		size int
	}{
		{"10", 10},
		{"1000", 1000},
		{"100000", 100000},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			db := kvstore.New[int, int]()

			for i := range tc.size {
				db.Set(i, i)
			}

			for b.Loop() {
				db.Snapshot()
			}
		})
	}
}

func BenchmarkClear(b *testing.B) {
	cases := []struct {
		name string
		size int
	}{
		{"10", 10},
		{"1000", 1000},
		{"100000", 100000},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			db := kvstore.New[int, int]()

			populate := func() {
				for key := range tc.size {
					db.Set(key, key)
				}
			}

			// B.Loop excludes setup before its first invocation.
			populate()

			for b.Loop() {
				db.Clear()

				// Prepare a fully populated store for the next iteration
				// without measuring the setup time or allocations.
				b.StopTimer()
				populate()
				b.StartTimer()
			}
		})
	}
}
