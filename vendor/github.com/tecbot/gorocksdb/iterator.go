package gorocksdb

// #include <stdlib.h>
// #include "rocksdb/c.h"
import "C"

import (
	"bytes"
	"errors"
	"unsafe"
)

// The iterator provides a way to seek to specific keys and iterate through
// the keyspace from that point, as well as access the values of those keys.
//
// For example:
//
//      it := db.NewIterator(readOpts)
//      defer it.Close()
//
//      it.Seek([]byte("foo"))
//		for ; it.Valid(); it.Next() {
//          fmt.Printf("Key: %v Value: %v\n", it.Key().Data(), it.Value().Data())
// 		}
//
//      if err := it.Err(); err != nil {
//          return err
//      }
//
type Iterator struct {
	c *C.rocksdb_iterator_t
}

// NewNativeIterator creates a Iterator object.
func NewNativeIterator(c *C.rocksdb_iterator_t) *Iterator {
	return &Iterator{c}
}

// Valid returns false only when an Iterator has iterated past either the
// first or the last key in the database.
func (self *Iterator) Valid() bool {
	return C.rocksdb_iter_valid(self.c) != 0
}

// ValidForPrefix returns false only when an Iterator has iterated past the
// first or the last key in the database or the specified prefix.
func (self *Iterator) ValidForPrefix(prefix []byte) bool {
	return C.rocksdb_iter_valid(self.c) != 0 && bytes.HasPrefix(self.Key().Data(), prefix)
}

// Key returns the key the iterator currently holds.
func (self *Iterator) Key() *Slice {
	var cLen C.size_t
	cKey := C.rocksdb_iter_key(self.c, &cLen)
	if cKey == nil {
		return nil
	}

	return &Slice{cKey, cLen, true}
}

// Value returns the value in the database the iterator currently holds.
func (self *Iterator) Value() *Slice {
	var cLen C.size_t
	cVal := C.rocksdb_iter_value(self.c, &cLen)
	if cVal == nil {
		return nil
	}

	return &Slice{cVal, cLen, true}
}

// Next moves the iterator to the next sequential key in the database.
func (self *Iterator) Next() {
	C.rocksdb_iter_next(self.c)
}

// Prev moves the iterator to the previous sequential key in the database.
func (self *Iterator) Prev() {
	C.rocksdb_iter_prev(self.c)
}

// SeekToFirst moves the iterator to the first key in the database.
func (self *Iterator) SeekToFirst() {
	C.rocksdb_iter_seek_to_first(self.c)
}

// SeekToLast moves the iterator to the last key in the database.
func (self *Iterator) SeekToLast() {
	C.rocksdb_iter_seek_to_last(self.c)
}

// Seek moves the iterator to the position greater than or equal to the key.
func (self *Iterator) Seek(key []byte) {
	cKey := byteToChar(key)

	C.rocksdb_iter_seek(self.c, cKey, C.size_t(len(key)))
}

// Err returns nil if no errors happened during iteration, or the actual
// error otherwise.
func (self *Iterator) Err() error {
	var cErr *C.char
	C.rocksdb_iter_get_error(self.c, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))

		return errors.New(C.GoString(cErr))
	}

	return nil
}

// Close closes the iterator.
func (self *Iterator) Close() {
	C.rocksdb_iter_destroy(self.c)
	self.c = nil
}
