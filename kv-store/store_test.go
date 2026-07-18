package kvstore_test

import (
	"errors"
	"kvstore"
	"sync"
	"testing"
)

func TestSetAndGetKey(t *testing.T) {
	db := kvstore.New[int, int]()
	db.Set(1, 10)

	const want = 10

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned an error: %v", err)
	}

	if got != want {
		t.Errorf("Get(1) = %d, want %d", got, want)
	}
}

func TestOverwriteExistingKey(t *testing.T) {
	db := kvstore.New[int, int]()
	db.Set(1, 10)
	db.Set(1, 20)

	const want = 20

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) after overwrite returned an error: %v", err)
	}

	if got != want {
		t.Errorf("Get(1) after overwrite = %d, want %d", got, want)
	}
}

func TestMissingKeyBehavior(t *testing.T) {
	db := kvstore.New[int, int]()

	_, err := db.Get(1)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Get(1) error = %v, want %v", err, kvstore.ErrValueNotFound)
	}
}

func TestConcurrentReadersAndWriters(t *testing.T) {
	db := kvstore.New[int, int]()

	const (
		numKeys    = 10
		numReaders = 8
		numWriters = 8
		iterations = 1000
	)

	for k := range numKeys {
		db.Set(k, k)
	}

	var wg sync.WaitGroup

	for w := range numWriters {
		wg.Go(func() {
			for i := range iterations {
				for k := range numKeys {
					db.Set(k, w*iterations+i)
				}
			}
		})
	}

	for range numReaders {
		wg.Go(func() {
			for range iterations {
				for k := range numKeys {
					if _, err := db.Get(k); err != nil {
						t.Errorf("Get(%d) returned an error: %v", k, err)
						return
					}
				}
			}
		})
	}

	wg.Wait()

	for k := range numKeys {
		if _, err := db.Get(k); err != nil {
			t.Errorf("Get(%d) after concurrent access returned an error: %v", k, err)
		}
	}
}

func TestDeleteKey(t *testing.T) {
	db := kvstore.New[int, int]()
	db.Set(1, 10)

	if err := db.Delete(1); err != nil {
		t.Fatalf("Delete(1) returned an error: %v", err)
	}

	_, err := db.Get(1)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Get(1) after deletion error = %v, want %v", err, kvstore.ErrValueNotFound)
	}
}

func TestDeleteMissingKey(t *testing.T) {
	db := kvstore.New[int, int]()

	err := db.Delete(1)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Delete(1) error = %v, want %v", err, kvstore.ErrValueNotFound)
	}
}

func TestZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, int]

	t.Run("Set and Get", func(t *testing.T) {
		const (
			key  = 1
			want = 10
		)

		db.Set(key, want)

		got, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get(%d) returned an error: %v", key, err)
		}

		if got != want {
			t.Errorf("Get(%d) = %d, want %d", key, got, want)
		}
	})

	t.Run("Get missing key", func(t *testing.T) {
		var emptyDb kvstore.Store[int, int]

		_, err := emptyDb.Get(1)
		if !errors.Is(err, kvstore.ErrValueNotFound) {
			t.Errorf("Get(1) error = %v, want %v", err, kvstore.ErrValueNotFound)
		}
	})

	t.Run("Delete missing key", func(t *testing.T) {
		var emptyDb kvstore.Store[int, int]

		err := emptyDb.Delete(1)
		if !errors.Is(err, kvstore.ErrValueNotFound) {
			t.Errorf("Delete(1) error = %v, want %v", err, kvstore.ErrValueNotFound)
		}
	})
}

type User struct {
	ID   int
	Name string
	Age  int
}

func TestStringKeyStructValue(t *testing.T) {
	db := kvstore.New[string, User]()

	want := User{
		ID:   1,
		Name: "Alice",
		Age:  25,
	}

	db.Set("alice", want)

	got, err := db.Get("alice")
	if err != nil {
		t.Fatalf("Get(%q) returned an error: %v", "alice", err)
	}

	if got != want {
		t.Errorf("Get(%q) = %+v, want %+v", "alice", got, want)
	}
}

func TestOverwriteStringKeyStructValue(t *testing.T) {
	db := kvstore.New[string, User]()

	db.Set("alice", User{
		ID:   1,
		Name: "Alice",
		Age:  25,
	})

	want := User{
		ID:   1,
		Name: "Alice Smith",
		Age:  26,
	}

	db.Set("alice", want)

	got, err := db.Get("alice")
	if err != nil {
		t.Fatalf("Get(%q) returned an error: %v", "alice", err)
	}

	if got != want {
		t.Errorf("Get(%q) = %+v, want %+v", "alice", got, want)
	}
}

func TestMissingStringKey(t *testing.T) {
	db := kvstore.New[string, User]()

	_, err := db.Get("missing")
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Get(%q) error = %v, want %v",
			"missing", err, kvstore.ErrValueNotFound)
	}
}

func TestDeleteStringKeyStructValue(t *testing.T) {
	db := kvstore.New[string, User]()

	db.Set("alice", User{
		ID:   1,
		Name: "Alice",
		Age:  25,
	})

	if err := db.Delete("alice"); err != nil {
		t.Fatalf("Delete(%q) returned an error: %v", "alice", err)
	}

	_, err := db.Get("alice")
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Get(%q) error = %v, want %v",
			"alice", err, kvstore.ErrValueNotFound)
	}
}

func TestConcurrentDeleteSameKey(t *testing.T) {
	db := kvstore.New[int, int]()
	db.Set(1, 10)

	const goroutines = 100

	results := make(chan error, goroutines)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			results <- db.Delete(1)
		})
	}

	wg.Wait()
	close(results)

	var (
		successes      int
		notFoundErrors int
	)

	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, kvstore.ErrValueNotFound):
			notFoundErrors++
		default:
			t.Errorf("Delete(1) returned unexpected error: %v", err)
		}
	}

	if successes != 1 {
		t.Errorf("successful Delete(1) calls = %d, want 1", successes)
	}

	if notFoundErrors != goroutines-1 {
		t.Errorf(
			"ErrValueNotFound results = %d, want %d",
			notFoundErrors,
			goroutines-1,
		)
	}

	_, err := db.Get(1)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf(
			"Get(1) after concurrent deletions error = %v, want %v",
			err,
			kvstore.ErrValueNotFound,
		)
	}
}

func TestLoadOrStoreMissingKey(t *testing.T) {
	db := kvstore.New[int, int]()

	const (
		key  = 1
		want = 10
	)

	got, loaded := db.LoadOrStore(key, want)

	if loaded {
		t.Errorf("LoadOrStore(%d, %d) loaded = %v, want false", key, want, loaded)
	}

	if got != want {
		t.Errorf("LoadOrStore(%d, %d) = %d, want %d", key, want, got, want)
	}

	stored, err := db.Get(key)
	if err != nil {
		t.Fatalf("Get(%d) returned an error: %v", key, err)
	}

	if stored != want {
		t.Errorf("Get(%d) = %d, want %d", key, stored, want)
	}
}

func TestLoadOrStoreExistingKey(t *testing.T) {
	db := kvstore.New[int, int]()

	db.Set(1, 10)

	got, loaded := db.LoadOrStore(1, 20)

	if !loaded {
		t.Errorf("LoadOrStore(1, 20) loaded = %v, want true", loaded)
	}

	if got != 10 {
		t.Errorf("LoadOrStore(1, 20) = %d, want %d", got, 10)
	}

	stored, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned an error: %v", err)
	}

	if stored != 10 {
		t.Errorf("Get(1) = %d, want %d", stored, 10)
	}
}

func TestLoadOrStoreConcurrentSameKey(t *testing.T) {
	db := kvstore.New[int, int]()

	const goroutines = 100

	type result struct {
		value  int
		loaded bool
	}

	results := make(chan result, goroutines)

	var wg sync.WaitGroup

	for i := range goroutines {
		value := i

		wg.Go(func() {
			v, loaded := db.LoadOrStore(1, value)
			results <- result{
				value:  v,
				loaded: loaded,
			}
		})
	}

	wg.Wait()
	close(results)

	var (
		winner     int
		haveWinner bool
		storedNew  int
	)

	for r := range results {
		if !haveWinner {
			winner = r.value
			haveWinner = true
		}

		if r.value != winner {
			t.Errorf("observed value = %d, want %d", r.value, winner)
		}

		if !r.loaded {
			storedNew++
		}
	}

	if storedNew != 1 {
		t.Errorf("newly stored count = %d, want 1", storedNew)
	}

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned an error: %v", err)
	}

	if got != winner {
		t.Errorf("Get(1) = %d, want %d", got, winner)
	}
}

func TestLoadOrStoreZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, int]

	const (
		key      = 1
		original = 10
		other    = 20
	)

	got, loaded := db.LoadOrStore(key, original)
	if loaded {
		t.Errorf(
			"first LoadOrStore(%d, %d) loaded = true, want false",
			key,
			original,
		)
	}

	if got != original {
		t.Errorf(
			"first LoadOrStore(%d, %d) = %d, want %d",
			key,
			original,
			got,
			original,
		)
	}

	got, loaded = db.LoadOrStore(key, other)
	if !loaded {
		t.Errorf(
			"second LoadOrStore(%d, %d) loaded = false, want true",
			key,
			other,
		)
	}

	if got != original {
		t.Errorf(
			"second LoadOrStore(%d, %d) = %d, want original value %d",
			key,
			other,
			got,
			original,
		)
	}

	stored, err := db.Get(key)
	if err != nil {
		t.Fatalf("Get(%d) returned an error: %v", key, err)
	}

	if stored != original {
		t.Errorf("Get(%d) = %d, want original value %d", key, stored, original)
	}
}

func TestUpdateExistingKey(t *testing.T) {
	db := kvstore.New[int, int]()

	db.Set(1, 10)

	err := db.Update(1, func(v int) int {
		return v + 5
	})
	if err != nil {
		t.Fatalf("Update(1) returned an error: %v", err)
	}

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned an error: %v", err)
	}

	if got != 15 {
		t.Errorf("Get(1) = %d, want %d", got, 15)
	}
}

func TestUpdateMissingKey(t *testing.T) {
	db := kvstore.New[int, int]()

	called := false
	err := db.Update(1, func(v int) int {
		called = true
		return v + 1
	})

	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf(
			"Update(1) error = %v, want %v",
			err,
			kvstore.ErrValueNotFound,
		)
	}

	if called {
		t.Error("Update(1) called callback for a missing key")
	}
}

func TestUpdateNilCallbackExistingKey(t *testing.T) {
	db := kvstore.New[int, int]()
	db.Set(1, 10)

	err := db.Update(1, nil)
	if !errors.Is(err, kvstore.ErrNilUpdateFunc) {
		t.Fatalf(
			"Update(1, nil) error = %v, want %v",
			err,
			kvstore.ErrNilUpdateFunc,
		)
	}

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned an error: %v", err)
	}

	if got != 10 {
		t.Errorf("Get(1) = %d, want unchanged value %d", got, 10)
	}
}

func TestUpdateNilCallbackMissingKey(t *testing.T) {
	db := kvstore.New[int, int]()

	err := db.Update(1, nil)

	if !errors.Is(err, kvstore.ErrNilUpdateFunc) {
		t.Errorf(
			"Update(1, nil) error = %v, want %v",
			err,
			kvstore.ErrNilUpdateFunc,
		)
	}
}

func TestConcurrentUpdateIncrement(t *testing.T) {
	db := kvstore.New[int, int]()

	db.Set(1, 0)

	const goroutines = 100

	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			if err := db.Update(1, func(v int) int {
				return v + 1
			}); err != nil {
				t.Errorf("Update(1) returned an error: %v", err)
			}
		})
	}

	wg.Wait()

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned an error: %v", err)
	}

	if got != goroutines {
		t.Errorf("Get(1) = %d, want %d", got, goroutines)
	}
}

func TestUpdateZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, int]

	called := false

	err := db.Update(1, func(v int) int {
		called = true
		return v + 1
	})

	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Update(1) error = %v, want %v", err, kvstore.ErrValueNotFound)
	}

	if called {
		t.Error("Update(1) invoked transformation function for a missing key")
	}
}

func TestSnapshotEmptyStore(t *testing.T) {
	db := kvstore.New[int, int]()

	got := db.Snapshot()

	if got == nil {
		t.Fatal("Snapshot() returned nil map")
	}

	if len(got) != 0 {
		t.Errorf("len(Snapshot()) = %d, want 0", len(got))
	}
}

func TestSnapshotZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, int]

	got := db.Snapshot()

	if got == nil {
		t.Fatal("Snapshot() returned nil map")
	}

	if len(got) != 0 {
		t.Errorf("len(Snapshot()) = %d, want 0", len(got))
	}
}

func TestSnapshotReturnsAllEntries(t *testing.T) {
	db := kvstore.New[int, int]()

	want := map[int]int{
		1: 10,
		2: 20,
		3: 30,
	}

	for k, v := range want {
		db.Set(k, v)
	}

	got := db.Snapshot()

	if len(got) != len(want) {
		t.Fatalf("len(Snapshot()) = %d, want %d", len(got), len(want))
	}

	for k, wantValue := range want {
		if gotValue, ok := got[k]; !ok {
			t.Errorf("Snapshot() missing key %d", k)
		} else if gotValue != wantValue {
			t.Errorf("Snapshot()[%d] = %d, want %d", k, gotValue, wantValue)
		}
	}
}

func TestSnapshotIsIndependentOfReturnedMap(t *testing.T) {
	db := kvstore.New[int, int]()

	db.Set(1, 10)

	snapshot := db.Snapshot()

	snapshot[1] = 99
	snapshot[2] = 20
	delete(snapshot, 1)

	if got, err := db.Get(1); err != nil {
		t.Fatalf("Get(1) returned error: %v", err)
	} else if got != 10 {
		t.Errorf("Get(1) = %d, want 10", got)
	}

	if _, err := db.Get(2); !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf("Get(2) error = %v, want %v", err, kvstore.ErrValueNotFound)
	}
}

func TestSnapshotUnaffectedByLaterSet(t *testing.T) {
	db := kvstore.New[int, int]()

	db.Set(1, 10)

	snapshot := db.Snapshot()

	db.Set(1, 20)
	db.Set(2, 30)

	if got := snapshot[1]; got != 10 {
		t.Errorf("snapshot[1] = %d, want 10", got)
	}

	if _, ok := snapshot[2]; ok {
		t.Error("snapshot unexpectedly contains newly added key")
	}
}

func TestSnapshotShallowCopy(t *testing.T) {
	type User struct {
		Name string
	}

	db := kvstore.New[int, *User]()

	user := &User{Name: "Alice"}
	db.Set(1, user)

	snapshot := db.Snapshot()

	snapshot[1].Name = "Bob"

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) returned error: %v", err)
	}

	if got.Name != "Bob" {
		t.Errorf("Get(1).Name = %q, want %q", got.Name, "Bob")
	}
}

func TestConcurrentSnapshotAndWriters(t *testing.T) {
	db := kvstore.New[int, int]()

	const (
		numKeys    = 10
		numWriters = 8
		numReaders = 8
		iterations = 1000
	)

	for k := range numKeys {
		db.Set(k, k)
	}

	var wg sync.WaitGroup

	for w := range numWriters {
		w := w

		wg.Go(func() {
			for i := range iterations {
				for k := range numKeys {
					db.Set(k, w*iterations+i)
				}
			}
		})
	}

	for range numReaders {
		wg.Go(func() {
			for range iterations {
				snapshot := db.Snapshot()

				if len(snapshot) != numKeys {
					t.Errorf("len(Snapshot()) = %d, want %d", len(snapshot), numKeys)
					return
				}

				for k := range numKeys {
					if _, ok := snapshot[k]; !ok {
						t.Errorf("Snapshot() missing key %d", k)
						return
					}
				}
			}
		})
	}

	wg.Wait()
}

func TestReferenceValueAliasing(t *testing.T) {
	db := kvstore.New[string, []int]()

	setValue := []int{1, 2}
	db.Set("set", setValue)

	setValue[0] = 10

	got, err := db.Get("set")
	if err != nil {
		t.Fatalf("Get(%q) returned an error: %v", "set", err)
	}

	if got[0] != 10 {
		t.Fatalf("Get(%q)[0] = %d, want %d", "set", got[0], 10)
	}

	got[1] = 20

	gotAgain, err := db.Get("set")
	if err != nil {
		t.Fatalf("Get(%q) returned an error: %v", "set", err)
	}

	if gotAgain[1] != 20 {
		t.Errorf("Get(%q)[1] = %d, want %d", "set", gotAgain[1], 20)
	}

	loadOrStoreValue := []int{3, 4}
	actual, loaded := db.LoadOrStore("load-or-store", loadOrStoreValue)
	if loaded {
		t.Fatal("LoadOrStore() loaded an existing value, want newly stored value")
	}

	loadOrStoreValue[0] = 30

	if actual[0] != 30 {
		t.Fatalf("LoadOrStore() result[0] = %d, want %d", actual[0], 30)
	}

	actual[1] = 40

	stored, err := db.Get("load-or-store")
	if err != nil {
		t.Fatalf("Get(%q) returned an error: %v", "load-or-store", err)
	}

	if stored[1] != 40 {
		t.Errorf("Get(%q)[1] = %d, want %d", "load-or-store", stored[1], 40)
	}
}

func TestLenEmptyStore(t *testing.T) {
	db := kvstore.New[int, int]()

	got := db.Len()

	if got != 0 {
		t.Fatalf("Len() = %d, want = 0", got)
	}
}

func TestLenZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, int]

	got := db.Len()

	if got != 0 {
		t.Fatalf("Len() = %d, want = 0", got)
	}
}

func TestLenGrowsAfterDistinctInserts(t *testing.T) {
	var db kvstore.Store[int, int]

	db.Set(1, 10)

	if got := db.Len(); got != 1 {
		t.Fatalf("Len() after first insert = %d, want 1", got)
	}

	db.Set(2, 20)

	if got := db.Len(); got != 2 {
		t.Fatalf("Len() after second distinct insert = %d, want 2", got)
	}
}

func TestLenUnchangedAfterOverwrite(t *testing.T) {
	var db kvstore.Store[int, int]

	db.Set(1, 10)
	db.Set(1, 20)

	if got := db.Len(); got != 1 {
		t.Fatalf("Len() after overwrite = %d, want 1", got)
	}
}

func TestLenShrinksAfterDelete(t *testing.T) {
	var db kvstore.Store[int, int]

	db.Set(1, 10)
	db.Set(2, 20)

	if err := db.Delete(1); err != nil {
		t.Fatalf("Delete(1) returned an error: %v", err)
	}

	if got := db.Len(); got != 1 {
		t.Fatalf("Len() after deletion = %d, want 1", got)
	}
}

func TestLenConcurrentWithMutation(t *testing.T) {
	var db kvstore.Store[int, int]

	const (
		mutators   = 8
		readers    = 8
		iterations = 1000
	)

	start := make(chan struct{})
	deleteErrors := make(chan error, mutators)
	invalidLengths := make(chan int, readers)

	var (
		ready sync.WaitGroup
		wg    sync.WaitGroup
	)

	ready.Add(mutators + readers)

	for key := range mutators {
		wg.Go(func() {
			ready.Done()
			<-start

			for value := range iterations {
				db.Set(key, value)

				if err := db.Delete(key); err != nil {
					deleteErrors <- err
					return
				}
			}
		})
	}

	for range readers {
		wg.Go(func() {
			ready.Done()
			<-start

			for range iterations {
				got := db.Len()
				if got < 0 || got > mutators {
					invalidLengths <- got
					return
				}
			}
		})
	}

	ready.Wait()
	close(start)

	wg.Wait()
	close(deleteErrors)
	close(invalidLengths)

	for err := range deleteErrors {
		t.Errorf("Delete() returned an error during concurrent mutation: %v", err)
	}

	for got := range invalidLengths {
		t.Errorf("Len() during concurrent mutation = %d, want between 0 and %d", got, mutators)
	}

	if got := db.Len(); got != 0 {
		t.Errorf("Len() after concurrent mutation = %d, want 0", got)
	}
}

func TestClearPopulatedStore(t *testing.T) {
	db := kvstore.New[int, int]()

	entries := map[int]int{
		1: 10,
		2: 20,
		3: 30,
	}

	for key, value := range entries {
		db.Set(key, value)
	}

	db.Clear()

	if got := db.Len(); got != 0 {
		t.Fatalf("Len() after Clear() = %d, want 0", got)
	}

	for key := range entries {
		_, err := db.Get(key)
		if !errors.Is(err, kvstore.ErrValueNotFound) {
			t.Errorf(
				"Get(%d) after Clear() error = %v, want %v",
				key,
				err,
				kvstore.ErrValueNotFound,
			)
		}
	}
}

func TestClearEmptyStore(t *testing.T) {
	db := kvstore.New[int, int]()

	db.Clear()

	if got := db.Len(); got != 0 {
		t.Fatalf("Len() after clearing empty store = %d, want 0", got)
	}

	db.Set(1, 10)

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) after clearing empty store returned an error: %v", err)
	}

	if got != 10 {
		t.Errorf("Get(1) = %d, want 10", got)
	}
}

func TestClearZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, int]

	db.Clear()

	if got := db.Len(); got != 0 {
		t.Fatalf("Len() after clearing zero-value store = %d, want 0", got)
	}

	db.Set(1, 10)

	got, err := db.Get(1)
	if err != nil {
		t.Fatalf("Get(1) after clearing zero-value store returned an error: %v", err)
	}

	if got != 10 {
		t.Errorf("Get(1) = %d, want 10", got)
	}
}

func TestClearConcurrentWithReaders(t *testing.T) {
	db := kvstore.New[int, int]()

	const (
		numKeys    = 64
		readers    = 8
		clearers   = 4
		iterations = 500
	)

	for key := range numKeys {
		db.Set(key, key)
	}

	type failure struct {
		operation string
		length    int
		key       int
		value     int
		present   bool
	}

	start := make(chan struct{})
	failures := make(chan failure, readers)

	var ready sync.WaitGroup
	var wg sync.WaitGroup

	ready.Add(readers + clearers)

	for range readers {
		wg.Go(func() {
			ready.Done()
			<-start

			for range iterations {
				length := db.Len()
				if length != 0 && length != numKeys {
					failures <- failure{
						operation: "Len",
						length:    length,
					}
					return
				}

				snapshot := db.Snapshot()

				switch len(snapshot) {
				case 0:
					continue

				case numKeys:
					for key := range numKeys {
						value, ok := snapshot[key]
						if !ok || value != key {
							failures <- failure{
								operation: "Snapshot entry",
								length:    len(snapshot),
								key:       key,
								value:     value,
								present:   ok,
							}
							return
						}
					}

				default:
					failures <- failure{
						operation: "Snapshot length",
						length:    len(snapshot),
					}
					return
				}
			}
		})
	}

	for range clearers {
		wg.Go(func() {
			ready.Done()
			<-start

			for range iterations {
				db.Clear()
			}
		})
	}

	ready.Wait()
	close(start)

	wg.Wait()
	close(failures)

	for failure := range failures {
		switch failure.operation {
		case "Len":
			t.Errorf(
				"Len() during concurrent Clear() = %d, want 0 or %d",
				failure.length,
				numKeys,
			)

		case "Snapshot length":
			t.Errorf(
				"len(Snapshot()) during concurrent Clear() = %d, want 0 or %d",
				failure.length,
				numKeys,
			)

		case "Snapshot entry":
			t.Errorf(
				"Snapshot()[%d] during concurrent Clear() = %d, present %v; want %d, true",
				failure.key,
				failure.value,
				failure.present,
				failure.key,
			)
		}
	}

	if got := db.Len(); got != 0 {
		t.Fatalf("Len() after concurrent Clear() calls = %d, want 0", got)
	}

	for key := range numKeys {
		_, err := db.Get(key)
		if !errors.Is(err, kvstore.ErrValueNotFound) {
			t.Errorf(
				"Get(%d) after concurrent Clear() error = %v, want %v",
				key,
				err,
				kvstore.ErrValueNotFound,
			)
		}
	}

	db.Set(numKeys, numKeys)

	got, err := db.Get(numKeys)
	if err != nil {
		t.Fatalf("Get(%d) after concurrent Clear() returned an error: %v", numKeys, err)
	}

	if got != numKeys {
		t.Errorf("Get(%d) = %d, want %d", numKeys, got, numKeys)
	}
}

func TestLoadAndDeletePresentKey(t *testing.T) {
	db := kvstore.New[int, string]()
	db.Set(1, "one")

	got, loaded := db.LoadAndDelete(1)

	if !loaded {
		t.Fatal("LoadAndDelete(1) loaded = false, want true")
	}

	if got != "one" {
		t.Errorf("LoadAndDelete(1) value = %q, want %q", got, "one")
	}

	_, err := db.Get(1)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf(
			"Get(1) after LoadAndDelete error = %v, want %v",
			err,
			kvstore.ErrValueNotFound,
		)
	}

	if got := db.Len(); got != 0 {
		t.Errorf("Len() after LoadAndDelete = %d, want 0", got)
	}
}

func TestLoadAndDeleteMissingKey(t *testing.T) {
	db := kvstore.New[int, string]()

	got, loaded := db.LoadAndDelete(1)

	if loaded {
		t.Fatal("LoadAndDelete(1) loaded = true, want false")
	}

	if got != "" {
		t.Errorf(
			"LoadAndDelete(1) value = %q, want zero value %q",
			got,
			"",
		)
	}

	if got := db.Len(); got != 0 {
		t.Errorf("Len() after missing LoadAndDelete = %d, want 0", got)
	}
}

func TestLoadAndDeleteZeroValueStore(t *testing.T) {
	var db kvstore.Store[int, string]

	got, loaded := db.LoadAndDelete(1)

	if loaded {
		t.Fatal("LoadAndDelete(1) on zero-value store loaded = true, want false")
	}

	if got != "" {
		t.Errorf(
			"LoadAndDelete(1) on zero-value store value = %q, want zero value %q",
			got,
			"",
		)
	}

	// Verify that the zero-value store remains usable.
	db.Set(1, "one")

	got, loaded = db.LoadAndDelete(1)

	if !loaded {
		t.Fatal("LoadAndDelete(1) after Set loaded = false, want true")
	}

	if got != "one" {
		t.Errorf(
			"LoadAndDelete(1) after Set value = %q, want %q",
			got,
			"one",
		)
	}

	_, err := db.Get(1)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf(
			"Get(1) after LoadAndDelete error = %v, want %v",
			err,
			kvstore.ErrValueNotFound,
		)
	}
}

func TestConcurrentLoadAndDeleteSameKey(t *testing.T) {
	db := kvstore.New[int, int]()

	const (
		key        = 1
		stored     = 42
		goroutines = 100
	)

	db.Set(key, stored)

	type result struct {
		value  int
		loaded bool
	}

	results := make(chan result, goroutines)

	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			value, loaded := db.LoadAndDelete(key)
			results <- result{
				value:  value,
				loaded: loaded,
			}
		})
	}

	wg.Wait()
	close(results)

	loadedCount := 0

	for result := range results {
		if result.loaded {
			loadedCount++

			if result.value != stored {
				t.Errorf(
					"successful LoadAndDelete(%d) value = %d, want %d",
					key,
					result.value,
					stored,
				)
			}

			continue
		}

		if result.value != 0 {
			t.Errorf(
				"unsuccessful LoadAndDelete(%d) value = %d, want zero value 0",
				key,
				result.value,
			)
		}
	}

	if loadedCount != 1 {
		t.Errorf(
			"LoadAndDelete(%d) calls with loaded=true = %d, want 1",
			key,
			loadedCount,
		)
	}

	_, err := db.Get(key)
	if !errors.Is(err, kvstore.ErrValueNotFound) {
		t.Errorf(
			"Get(%d) after concurrent LoadAndDelete calls error = %v, want %v",
			key,
			err,
			kvstore.ErrValueNotFound,
		)
	}
}
