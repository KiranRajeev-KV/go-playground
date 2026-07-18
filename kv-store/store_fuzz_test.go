package kvstore_test

import (
	"errors"
	"kvstore"
	"testing"
)

func FuzzStoreSequentialOperations(f *testing.F) {
	const (
		setOperation byte = iota
		getOperation
		deleteOperation
		clearOperation
		loadAndDeleteOperation
		swapOperation
		loadOrStoreOperation
		updateOperation
		snapshotOperation
		lenOperation
	)

	// Set(1, 10), Get(1), Delete(1), Get(1 missing).
	f.Add([]byte{
		setOperation, 1, 10,
		getOperation, 1, 0,
		deleteOperation, 1, 0,
		getOperation, 1, 0,
	})

	// Set two keys, Clear, then exercise missing Get and Delete.
	f.Add([]byte{
		setOperation, 2, 20,
		setOperation, 3, 30,
		clearOperation, 0, 0,
		getOperation, 2, 0,
		deleteOperation, 3, 0,
	})

	// LoadAndDelete a present key, then the same absent key.
	f.Add([]byte{
		setOperation, 7, 70,
		loadAndDeleteOperation, 7, 0,
		loadAndDeleteOperation, 7, 0,
	})

	// Swap a missing key, then the now-existing key.
	f.Add([]byte{
		swapOperation, 8, 80,
		swapOperation, 8, 81,
	})

	// LoadOrStore a missing key, then the now-existing key.
	f.Add([]byte{
		loadOrStoreOperation, 9, 90,
		loadOrStoreOperation, 9, 91,
	})

	// Update an existing key, then attempt to update a missing key.
	f.Add([]byte{
		setOperation, 10, 100,
		updateOperation, 10, 101,
		updateOperation, 11, 110,
	})

	// Snapshot a non-empty store, then continue mutating and checking Len.
	f.Add([]byte{
		setOperation, 12, 120,
		setOperation, 13, 130,
		snapshotOperation, 0, 0,
		updateOperation, 12, 121,
		lenOperation, 0, 0,
		deleteOperation, 13, 0,
		snapshotOperation, 0, 0,
	})

	f.Fuzz(func(t *testing.T, data []byte) {
		const (
			bytesPerOperation = 3
			maxOperations     = 64
			operationCount    = 10
		)

		db := kvstore.New[byte, byte]()
		model := make(map[byte]byte)

		operationTotal := min(
			len(data)/bytesPerOperation,
			maxOperations,
		)

		checkState := func(step int, operation string) {
			t.Helper()

			if got, want := db.Len(), len(model); got != want {
				t.Fatalf(
					"step %d after %s: Len() = %d, want %d",
					step,
					operation,
					got,
					want,
				)
			}

			for candidate := 0; candidate <= 255; candidate++ {
				key := byte(candidate)

				want, exists := model[key]
				got, err := db.Get(key)

				if exists {
					if err != nil {
						t.Fatalf(
							"step %d after %s: Get(%d) returned error %v, want value %d",
							step,
							operation,
							key,
							err,
							want,
						)
					}

					if got != want {
						t.Fatalf(
							"step %d after %s: Get(%d) = %d, want %d",
							step,
							operation,
							key,
							got,
							want,
						)
					}

					continue
				}

				if !errors.Is(err, kvstore.ErrValueNotFound) {
					t.Fatalf(
						"step %d after %s: Get(%d) error = %v, want %v",
						step,
						operation,
						key,
						err,
						kvstore.ErrValueNotFound,
					)
				}
			}
		}

		for step := range operationTotal {
			offset := step * bytesPerOperation

			operation := data[offset] % operationCount
			key := data[offset+1]
			value := data[offset+2]

			switch operation {
			case setOperation:
				db.Set(key, value)
				model[key] = value

				checkState(step, "Set")

			case getOperation:
				got, err := db.Get(key)
				want, exists := model[key]

				if exists {
					if err != nil {
						t.Fatalf(
							"step %d: Get(%d) returned error %v, want value %d",
							step,
							key,
							err,
							want,
						)
					}

					if got != want {
						t.Fatalf(
							"step %d: Get(%d) = %d, want %d",
							step,
							key,
							got,
							want,
						)
					}
				} else if !errors.Is(err, kvstore.ErrValueNotFound) {
					t.Fatalf(
						"step %d: Get(%d) error = %v, want %v",
						step,
						key,
						err,
						kvstore.ErrValueNotFound,
					)
				}

				checkState(step, "Get")

			case deleteOperation:
				_, exists := model[key]

				err := db.Delete(key)

				if exists {
					if err != nil {
						t.Fatalf(
							"step %d: Delete(%d) returned error %v, want nil",
							step,
							key,
							err,
						)
					}

					delete(model, key)
				} else if !errors.Is(err, kvstore.ErrValueNotFound) {
					t.Fatalf(
						"step %d: Delete(%d) error = %v, want %v",
						step,
						key,
						err,
						kvstore.ErrValueNotFound,
					)
				}

				checkState(step, "Delete")

			case clearOperation:
				db.Clear()
				clear(model)

				checkState(step, "Clear")

			case loadAndDeleteOperation:
				want, exists := model[key]

				got, loaded := db.LoadAndDelete(key)

				if loaded != exists {
					t.Fatalf(
						"step %d: LoadAndDelete(%d) loaded = %v, want %v",
						step,
						key,
						loaded,
						exists,
					)
				}

				if exists {
					if got != want {
						t.Fatalf(
							"step %d: LoadAndDelete(%d) value = %d, want %d",
							step,
							key,
							got,
							want,
						)
					}

					delete(model, key)
				} else if got != 0 {
					t.Fatalf(
						"step %d: LoadAndDelete(%d) value = %d, want zero value 0",
						step,
						key,
						got,
					)
				}

				checkState(step, "LoadAndDelete")

			case swapOperation:
				want, exists := model[key]

				got, loaded := db.Swap(key, value)

				if loaded != exists {
					t.Fatalf(
						"step %d: Swap(%d, %d) loaded = %v, want %v",
						step,
						key,
						value,
						loaded,
						exists,
					)
				}

				if exists {
					if got != want {
						t.Fatalf(
							"step %d: Swap(%d, %d) previous value = %d, want %d",
							step,
							key,
							value,
							got,
							want,
						)
					}
				} else if got != 0 {
					t.Fatalf(
						"step %d: Swap(%d, %d) previous value = %d, want zero value 0",
						step,
						key,
						value,
						got,
					)
				}

				model[key] = value

				checkState(step, "Swap")

			case loadOrStoreOperation:
				want, exists := model[key]

				got, loaded := db.LoadOrStore(key, value)

				if loaded != exists {
					t.Fatalf(
						"step %d: LoadOrStore(%d, %d) loaded = %v, want %v",
						step,
						key,
						value,
						loaded,
						exists,
					)
				}

				if exists {
					if got != want {
						t.Fatalf(
							"step %d: LoadOrStore(%d, %d) value = %d, want existing value %d",
							step,
							key,
							value,
							got,
							want,
						)
					}
				} else {
					if got != value {
						t.Fatalf(
							"step %d: LoadOrStore(%d, %d) value = %d, want supplied value %d",
							step,
							key,
							value,
							got,
							value,
						)
					}

					model[key] = value
				}

				checkState(step, "LoadOrStore")

			case updateOperation:
				_, exists := model[key]

				err := db.Update(key, func(byte) byte {
					return value
				})

				if exists {
					if err != nil {
						t.Fatalf(
							"step %d: Update(%d) returned error %v, want nil",
							step,
							key,
							err,
						)
					}

					model[key] = value
				} else if !errors.Is(err, kvstore.ErrValueNotFound) {
					t.Fatalf(
						"step %d: Update(%d) error = %v, want %v",
						step,
						key,
						err,
						kvstore.ErrValueNotFound,
					)
				}

				checkState(step, "Update")

			case snapshotOperation:
				snapshot := db.Snapshot()

				if got, want := len(snapshot), len(model); got != want {
					t.Fatalf(
						"step %d: len(Snapshot()) = %d, want %d",
						step,
						got,
						want,
					)
				}

				for candidate := 0; candidate <= 255; candidate++ {
					key := byte(candidate)

					got, gotExists := snapshot[key]
					want, wantExists := model[key]

					if gotExists != wantExists {
						t.Fatalf(
							"step %d: Snapshot() key %d presence = %v, want %v",
							step,
							key,
							gotExists,
							wantExists,
						)
					}

					if wantExists && got != want {
						t.Fatalf(
							"step %d: Snapshot()[%d] = %d, want %d",
							step,
							key,
							got,
							want,
						)
					}
				}

				checkState(step, "Snapshot")

			case lenOperation:
				if got, want := db.Len(), len(model); got != want {
					t.Fatalf(
						"step %d: Len() = %d, want %d",
						step,
						got,
						want,
					)
				}

				checkState(step, "Len")
			}
		}
	})
}
