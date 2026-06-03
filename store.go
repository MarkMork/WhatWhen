package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Item is a single tracked task: a label and the time it was last done.
type Item struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
	LastReset time.Time `json:"lastReset"`
}

type fileShape struct {
	Items []*Item `json:"items"`
}

// ErrNotFound is returned when an item id does not exist.
var ErrNotFound = errors.New("item not found")

// Store keeps items in memory and mirrors them to a JSON file on disk.
// All mutating methods persist before returning.
type Store struct {
	mu    sync.Mutex
	path  string
	items []*Item
}

// NewStore loads (or initializes) the data file at path.
func NewStore(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	s := &Store{path: path, items: []*Item{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fresh install: persist an empty file so the path is valid.
			return s, s.persist()
		}
		return nil, err
	}

	if len(data) > 0 {
		var f fileShape
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		if f.Items != nil {
			s.items = f.Items
		}
	}
	return s, nil
}

// List returns a copy of the current items slice.
func (s *Store) List() []*Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Item, len(s.items))
	copy(out, s.items)
	return out
}

// Add creates a new item with the given label.
func (s *Store) Add(label string) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	item := &Item{
		ID:        newID(),
		Label:     label,
		CreatedAt: now,
		LastReset: now,
	}
	s.items = append(s.items, item)
	if err := s.persist(); err != nil {
		return nil, err
	}
	return item, nil
}

// ItemUpdate holds the fields that can be changed on an existing item. A nil
// field is left unchanged.
type ItemUpdate struct {
	Label     *string
	LastReset *time.Time
}

// Update applies the non-nil fields of u to the item with the given id.
func (s *Store) Update(id string, u ItemUpdate) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.find(id)
	if item == nil {
		return nil, ErrNotFound
	}
	if u.Label != nil {
		item.Label = *u.Label
	}
	if u.LastReset != nil {
		item.LastReset = *u.LastReset
	}
	if err := s.persist(); err != nil {
		return nil, err
	}
	return item, nil
}

// Reset marks an item as just done now.
func (s *Store) Reset(id string) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.find(id)
	if item == nil {
		return nil, ErrNotFound
	}
	item.LastReset = time.Now().UTC()
	if err := s.persist(); err != nil {
		return nil, err
	}
	return item, nil
}

// Delete removes an item.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return s.persist()
		}
	}
	return ErrNotFound
}

// find returns the item with id, or nil. Caller must hold the lock.
func (s *Store) find(id string) *Item {
	for _, item := range s.items {
		if item.ID == id {
			return item
		}
	}
	return nil
}

// persist writes the current items to disk atomically. Caller must hold the lock.
func (s *Store) persist() error {
	data, err := json.MarshalIndent(fileShape{Items: s.items}, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
