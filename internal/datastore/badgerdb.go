package datastore

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *badger.DB
}

type Entry struct {
	Resource  k8s.ResourceReference `json:"resource"`
	Suspended bool                  `json:"suspended"`
	UpdatedBy string                `json:"updatedBy"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

func NewBadgerStore(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("badger store path cannot be empty")
	}
	db, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open badger store: %w", err)
	}
	return &Store{
		db: db,
	}, nil
}

func (s *Store) GetEntry(resource k8s.ResourceReference) (Entry, error) {
	var entry Entry
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(buildKey(resource))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("failed to get item: %w", err)
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to get value: %w", err)
		}
		if err = json.Unmarshal(val, &entry); err != nil {
			return fmt.Errorf("failed to unmarshal entry: %w", err)
		}
		return nil
	})
	return entry, err
}

func (s *Store) SaveEntry(entry Entry) error {
	return s.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed ot marshal entry: %w", err)
		}
		return txn.Set(buildKey(entry.Resource), data)
	})
}

func (s *Store) Close() error {
	return s.db.Close()
}

func buildKey(resource k8s.ResourceReference) []byte {
	return []byte(fmt.Sprintf("resource:%s:%s:%s:%s", resource.Type.Group, resource.Type.Kind, resource.Namespace, resource.Name))
}
