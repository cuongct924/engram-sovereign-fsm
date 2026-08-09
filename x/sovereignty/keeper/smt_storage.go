package keeper

import (
	"context"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStorage struct {
	db *badger.DB
}

func NewBadgerStorage(db *badger.DB) *BadgerStorage {
	return &BadgerStorage{db: db}
}

func (s *BadgerStorage) Put(ctx context.Context, key, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (s *BadgerStorage) Get(ctx context.Context, key []byte) ([]byte, error) {
	var val []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	return val, err
}
