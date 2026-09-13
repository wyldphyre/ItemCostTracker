package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"itemcosttracker/internal/model"
)

// ErrNotFound is returned by Update and Delete when no item has the given ID.
var ErrNotFound = errors.New("item not found")

// Store provides thread-safe JSON persistence for items.
//
// Every mutation builds the new item list on the side and only swaps it in
// once it has been written to disk. A failed write therefore leaves both the
// in-memory state and the file exactly as they were, so the UI never shows a
// change that a restart would silently undo.
type Store struct {
	mu       sync.RWMutex
	filePath string
	items    []model.Item
}

type storeData struct {
	Items []model.Item `json:"items"`
}

// New opens or creates the JSON store at the given path.
// The directory is created if it doesn't exist.
func New(filePath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}
	s := &Store{filePath: filePath}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("loading store: %w", err)
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		s.items = []model.Item{}
		return nil
	}
	if err != nil {
		return err
	}
	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("parsing JSON store: %w", err)
	}
	s.items = sd.Items
	return nil
}

// commit writes items to disk and, only if that succeeds, makes them the
// current state. Callers must hold the write lock and must pass a slice that
// does not share a backing array with s.items.
func (s *Store) commit(items []model.Item) error {
	data, err := json.MarshalIndent(storeData{Items: items}, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: write to temp file then rename
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		os.Remove(tmp)
		return err
	}
	s.items = items
	return nil
}

// Path returns the file the store persists to.
func (s *Store) Path() string { return s.filePath }

// cloneItems returns a copy of the current items with spare capacity, safe to
// modify without affecting s.items.
func (s *Store) cloneItems(extra int) []model.Item {
	next := make([]model.Item, len(s.items), len(s.items)+extra)
	copy(next, s.items)
	return next
}

// All returns a copy of all items, newest purchase first. Items bought on the
// same day are ordered by name and then ID, so the order is fully determined
// by the items themselves and rows don't reshuffle after unrelated edits.
func (s *Store) All() []model.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.Item, len(s.items))
	copy(result, s.items)
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if !a.PurchaseDate.Equal(b.PurchaseDate) {
			return a.PurchaseDate.After(b.PurchaseDate)
		}
		if an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name); an != bn {
			return an < bn
		}
		return a.ID < b.ID
	})
	return result
}

// Get finds an item by ID. Returns the item and true if found.
func (s *Store) Get(id string) (model.Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == id {
			return item, true
		}
	}
	return model.Item{}, false
}

// Create adds a new item. The caller must set ID and timestamps.
func (s *Store) Create(item model.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commit(append(s.cloneItems(1), item))
}

// Update replaces an existing item by ID.
func (s *Store) Update(item model.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.items {
		if existing.ID == item.ID {
			next := s.cloneItems(0)
			next[i] = item
			return s.commit(next)
		}
	}
	return fmt.Errorf("item %s: %w", item.ID, ErrNotFound)
}

// ReplaceAll replaces the entire item list with the provided items and saves.
func (s *Store) ReplaceAll(items []model.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]model.Item, len(items))
	copy(next, items)
	return s.commit(next)
}

// Merge adds items that don't already exist (matched by ID) and updates those that do.
func (s *Store) Merge(incoming []model.Item) (added, updated int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.cloneItems(len(incoming))
	index := make(map[string]int, len(next))
	for i, item := range next {
		index[item.ID] = i
	}

	for _, item := range incoming {
		if i, exists := index[item.ID]; exists {
			next[i] = item
			updated++
		} else {
			next = append(next, item)
			index[item.ID] = len(next) - 1
			added++
		}
	}

	if err := s.commit(next); err != nil {
		return 0, 0, err
	}
	return added, updated, nil
}

// Delete removes an item by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if item.ID == id {
			next := make([]model.Item, 0, len(s.items)-1)
			next = append(next, s.items[:i]...)
			next = append(next, s.items[i+1:]...)
			return s.commit(next)
		}
	}
	return fmt.Errorf("item %s: %w", id, ErrNotFound)
}
