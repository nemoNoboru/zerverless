package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	db *badger.DB
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	opts := badger.DefaultOptions(filepath.Join(dataDir, "badger"))
	opts.Logger = nil // Disable logging for now

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Get(namespace, key string) ([]byte, error) {
	fullKey := namespace + key
	var value []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fullKey))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = append([]byte{}, val...)
			return nil
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return value, err
}

func (s *Store) Set(namespace, key string, value []byte) error {
	fullKey := namespace + key
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(fullKey), value)
	})
}

func (s *Store) Delete(namespace, key string) error {
	fullKey := namespace + key
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(fullKey))
	})
}

func (s *Store) List(namespace, prefix string, limit int) ([]string, error) {
	var keys []string

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		fullPrefix := namespace + prefix
		it.Seek([]byte(fullPrefix))

		count := 0
		for it.ValidForPrefix([]byte(fullPrefix)) && (limit <= 0 || count < limit) {
			item := it.Item()
			key := string(item.Key())
			// Remove namespace prefix from result
			keys = append(keys, key[len(namespace):])
			count++
			it.Next()
		}

		return nil
	})

	return keys, err
}


