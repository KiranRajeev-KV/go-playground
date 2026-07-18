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

	f.Fuzz(func(t *testing.T, data []byte) {
		const (
			bytesPerOperation = 3
			maxOperations     = 64
			operationCount    = 4
		)

		db := kvstore.New[byte, byte]()
		model := make(map[byte]byte)

		operationTotal := min(len(data)/bytesPerOperation, maxOperations)

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

			// The key type is byte, so checking the entire key space
			// is bounded to exactly 256 Get calls.
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
			}
		}
	})
}
