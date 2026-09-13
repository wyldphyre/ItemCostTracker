package store

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"itemcosttracker/internal/model"
)

func day(d int) time.Time { return time.Date(2020, 1, d, 0, 0, 0, 0, time.UTC) }

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func names(items []model.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// breakDisk points the store at a directory that no longer exists, so every
// write fails regardless of the user the tests run as.
func breakDisk(t *testing.T, s *Store) {
	t.Helper()
	s.filePath = filepath.Join(t.TempDir(), "gone", "items.json")
}

func TestFailedWriteLeavesStateUnchanged(t *testing.T) {
	seed := []model.Item{
		{ID: "a", Name: "Alpha", PurchaseDate: day(1)},
		{ID: "b", Name: "Bravo", PurchaseDate: day(2)},
	}

	mutations := map[string]func(s *Store) error{
		"Create": func(s *Store) error { return s.Create(model.Item{ID: "c", Name: "Charlie", PurchaseDate: day(3)}) },
		"Update": func(s *Store) error { return s.Update(model.Item{ID: "a", Name: "Renamed", PurchaseDate: day(1)}) },
		"Delete": func(s *Store) error { return s.Delete("a") },
		"ReplaceAll": func(s *Store) error {
			return s.ReplaceAll([]model.Item{{ID: "z", Name: "Zulu", PurchaseDate: day(9)}})
		},
		"Merge": func(s *Store) error {
			_, _, err := s.Merge([]model.Item{{ID: "a", Name: "Merged"}, {ID: "d", Name: "Delta"}})
			return err
		},
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			if err := s.ReplaceAll(seed); err != nil {
				t.Fatal(err)
			}
			goodPath := s.filePath
			before := names(s.All())

			breakDisk(t, s)
			if err := mutate(s); err == nil {
				t.Fatal("expected the write to fail")
			}
			if after := names(s.All()); !equal(before, after) {
				t.Errorf("in-memory items changed after failed write: before %v, after %v", before, after)
			}

			// The file on disk must also still hold the original items.
			reopened, err := New(goodPath)
			if err != nil {
				t.Fatal(err)
			}
			if onDisk := names(reopened.All()); !equal(before, onDisk) {
				t.Errorf("items on disk changed after failed write: %v", onDisk)
			}
		})
	}
}

// An item that cannot be encoded must be rejected without poisoning the store:
// later, valid writes have to keep working.
func TestUnencodableItemDoesNotWedgeStore(t *testing.T) {
	s := newStore(t)
	if err := s.Create(model.Item{ID: "nan", Name: "Poison", PurchasePrice: math.NaN()}); err == nil {
		t.Fatal("expected NaN item to fail to save")
	}
	if err := s.Create(model.Item{ID: "ok", Name: "Fine", PurchaseDate: day(1)}); err != nil {
		t.Fatalf("valid create failed after a rejected item: %v", err)
	}
	if got := names(s.All()); !equal(got, []string{"Fine"}) {
		t.Errorf("items = %v, want [Fine]", got)
	}
}

func TestNotFound(t *testing.T) {
	s := newStore(t)
	if err := s.Update(model.Item{ID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update missing: err = %v, want ErrNotFound", err)
	}
	if err := s.Delete("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete missing: err = %v, want ErrNotFound", err)
	}
}

// A write failure must not be reported as ErrNotFound (handlers map that to 404).
func TestWriteFailureIsNotNotFound(t *testing.T) {
	s := newStore(t)
	if err := s.Create(model.Item{ID: "a", PurchaseDate: day(1)}); err != nil {
		t.Fatal(err)
	}
	breakDisk(t, s)
	if err := s.Delete("a"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("Delete with broken disk: err = %v, want a non-ErrNotFound error", err)
	}
}

func TestAllOrdering(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceAll([]model.Item{
		{ID: "3", Name: "charlie", PurchaseDate: day(5)},
		{ID: "1", Name: "Alpha", PurchaseDate: day(5)},
		{ID: "9", Name: "Newest", PurchaseDate: day(9)},
		{ID: "2", Name: "bravo", PurchaseDate: day(5)},
		{ID: "0", Name: "bravo", PurchaseDate: day(5)},
		{ID: "8", Name: "Oldest", PurchaseDate: day(1)},
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"9", "1", "0", "2", "3", "8"} // date desc, then name (case-insensitive), then ID
	var got []string
	for _, it := range s.All() {
		got = append(got, it.ID)
	}
	if !equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// Rows sharing a purchase date must keep their relative order across
// unrelated creates, updates, and deletes.
func TestAllOrderStableAcrossEdits(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 40; i++ {
		id := string(rune('A' + i))
		if err := s.Create(model.Item{ID: id, Name: "item " + id, PurchaseDate: day(1 + i%5)}); err != nil {
			t.Fatal(err)
		}
	}
	order := func(skip ...string) []string {
		var out []string
	next:
		for _, it := range s.All() {
			for _, sk := range skip {
				if it.ID == sk {
					continue next
				}
			}
			out = append(out, it.ID)
		}
		return out
	}
	before := order("D", "new")

	it, _ := s.Get("R")
	it.Name = "item R" // unchanged sort key, but a fresh write
	it.PurchasePrice = 42
	if err := s.Update(it); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("D"); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(model.Item{ID: "new", Name: "zzz", PurchaseDate: day(3)}); err != nil {
		t.Fatal(err)
	}

	if after := order("D", "new"); !equal(before, after) {
		t.Errorf("relative order changed:\nbefore %v\nafter  %v", before, after)
	}
}

// Leftover temp files from a failed rename should not accumulate.
func TestCommitCleansTempOnSuccess(t *testing.T) {
	s := newStore(t)
	if err := s.Create(model.Item{ID: "a", PurchaseDate: day(1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.filePath + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind after successful write (stat err = %v)", err)
	}
}
