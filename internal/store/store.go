package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"itemcosttracker/internal/model"
)

// Store provides thread-safe JSON persistence for items.
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

func (s *Store) save() error {
	sd := storeData{Items: s.items}
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: write to temp file then rename
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

// All returns a copy of all items sorted by purchase date descending (newest first).
func (s *Store) All() []model.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.Item, len(s.items))
	copy(result, s.items)
	sort.Slice(result, func(i, j int) bool {
		return result[i].PurchaseDate.After(result[j].PurchaseDate)
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
	s.items = append(s.items, item)
	return s.save()
}

// Update replaces an existing item by ID.
func (s *Store) Update(item model.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.items {
		if existing.ID == item.ID {
			s.items[i] = item
			return s.save()
		}
	}
	return fmt.Errorf("item %s not found", item.ID)
}

// ReplaceAll replaces the entire item list with the provided items and saves.
func (s *Store) ReplaceAll(items []model.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = items
	return s.save()
}

// Merge adds items that don't already exist (matched by ID) and updates those that do.
func (s *Store) Merge(incoming []model.Item) (added, updated int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := make(map[string]int, len(s.items))
	for i, item := range s.items {
		index[item.ID] = i
	}

	for _, item := range incoming {
		if i, exists := index[item.ID]; exists {
			s.items[i] = item
			updated++
		} else {
			s.items = append(s.items, item)
			index[item.ID] = len(s.items) - 1
			added++
		}
	}

	return added, updated, s.save()
}

// Delete removes an item by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("item %s not found", id)
}
