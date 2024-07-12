package datastore

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"

	"github.com/e-flux-platform/fluxcd-suspend-notifier/internal/k8s"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *badger.DB
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

func (s *Store) IsSuspended(resource k8s.Resource) (bool, error) {
	var suspended bool
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
		if len(val) != 1 {
			return fmt.Errorf("expected value length of 1, got %d", len(val))
		}
		suspended = val[0] == 1
		return nil
	})
	return suspended, err
}

func (s *Store) SetSuspended(resource k8s.Resource, suspended bool) error {
	return s.db.Update(func(txn *badger.Txn) error {
		var b byte
		if suspended {
			b = 1
		}
		return txn.Set(buildKey(resource), []byte{b})
	})
}

func (s *Store) Close() error {
	return s.db.Close()
}

func buildKey(resource k8s.Resource) []byte {
	return []byte(fmt.Sprintf("%s:%s:%s", resource.Kind, resource.Namespace, resource.Name))
}
