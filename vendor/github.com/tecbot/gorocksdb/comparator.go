package gorocksdb

// #include "rocksdb/c.h"
import "C"

// A Comparator object provides a total order across slices that are
// used as keys in an sstable or a database.
type Comparator interface {
	// Three-way comparison. Returns value:
	//   < 0 iff "a" < "b",
	//   == 0 iff "a" == "b",
	//   > 0 iff "a" > "b"
	Compare(a, b []byte) int

	// The name of the comparator.
	Name() string
}

type nativeComparator struct {
	c *C.rocksdb_comparator_t
}

func (c nativeComparator) Compare(a, b []byte) int { return 0 }

func (c nativeComparator) Name() string { return "" }

// NewNativeComparator allocates a Comparator object.
// The Comparator's methods are no-ops, but it is still used correctly by
// RocksDB.
func NewNativeComparator(c *C.rocksdb_comparator_t) Comparator {
	return nativeComparator{c}
}

//export gorocksdb_comparator_compare
func gorocksdb_comparator_compare(handler *Comparator, cKeyA *C.char, cKeyALen C.size_t, cKeyB *C.char, cKeyBLen C.size_t) C.int {
	keyA := charToByte(cKeyA, cKeyALen)
	keyB := charToByte(cKeyB, cKeyBLen)

	compare := (*handler).Compare(keyA, keyB)

	return C.int(compare)
}

//export gorocksdb_comparator_name
func gorocksdb_comparator_name(handler *Comparator) *C.char {
	return stringToChar((*handler).Name())
}
