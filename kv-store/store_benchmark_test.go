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

func BenchmarkParallelSet(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 0)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db.Set(1, 1)
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

func BenchmarkParallelSnapshot(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	for key := range 1000 {
		db.Set(key, key)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db.Snapshot()
		}
	})
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

func BenchmarkSwap(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 10)

	for b.Loop() {
		db.Swap(1, 11)
	}
}

func BenchmarkLoadOrStore(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 10)

	for b.Loop() {
		db.LoadOrStore(1, 11)
	}
}

func BenchmarkUpdate(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	db.Set(1, 10)

	update := func(value int) int {
		return value + 1
	}

	for b.Loop() {
		db.Update(1, update)
	}
}

func BenchmarkLoadAndDelete(b *testing.B) {
	b.ReportAllocs()

	const batchSize = 1024

	stores := make([]kvstore.Store[int, int], batchSize)

	populate := func() {
		for i := range stores {
			stores[i].Set(1, 10)
		}
	}

	// Initial construction and insertion are excluded by B.Loop.
	populate()

	next := 0

	for b.Loop() {
		if next == len(stores) {
			b.StopTimer()
			populate()
			next = 0
			b.StartTimer()
		}

		stores[next].LoadAndDelete(1)
		next++
	}
}

func BenchmarkDelete(b *testing.B) {
	b.ReportAllocs()

	const batchSize = 1024

	stores := make([]kvstore.Store[int, int], batchSize)

	populate := func() {
		for i := range stores {
			stores[i].Set(1, 10)
		}
	}

	// Initial construction and insertion are excluded by B.Loop.
	populate()

	next := 0

	for b.Loop() {
		if next == len(stores) {
			b.StopTimer()
			populate()
			next = 0
			b.StartTimer()
		}

		stores[next].Delete(1)
		next++
	}
}

func BenchmarkSequentialLen(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	for key := range 1000 {
		db.Set(key, key)
	}

	for b.Loop() {
		db.Len()
	}
}

func BenchmarkParallelLen(b *testing.B) {
	b.ReportAllocs()

	db := kvstore.New[int, int]()
	for key := range 1000 {
		db.Set(key, key)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db.Len()
		}
	})
}
