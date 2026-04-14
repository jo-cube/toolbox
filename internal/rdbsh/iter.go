package rdbsh

import (
	"bytes"

	"github.com/jo-cube/toolbox/internal/rdbsh/rocksdb"
)

type iterationResult struct {
	Count   int
	Limited bool
}

func (s *Shell) iterate(prefix []byte, limit int, fn func(key, value []byte) error) (iterationResult, error) {
	var result iterationResult

	it := s.newIterator()
	defer it.Close()

	if len(prefix) > 0 {
		it.Seek(prefix)
	} else {
		it.SeekToFirst()
	}

	for it.Valid() {
		key := it.Key()
		if len(prefix) > 0 && !bytes.HasPrefix(key, prefix) {
			break
		}

		if err := fn(key, it.Value()); err != nil {
			return result, err
		}
		result.Count++

		it.Next()
		if limit > 0 && result.Count >= limit {
			if it.Valid() {
				nextKey := it.Key()
				if len(prefix) == 0 || bytes.HasPrefix(nextKey, prefix) {
					result.Limited = true
				}
			}
			break
		}
	}

	if err := it.Err(); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Shell) newIterator() *rocksdb.Iterator {
	if s.selectedHandle != nil {
		return s.db.NewIteratorCF(s.ro, s.selectedHandle)
	}
	return s.db.NewIterator(s.ro)
}
