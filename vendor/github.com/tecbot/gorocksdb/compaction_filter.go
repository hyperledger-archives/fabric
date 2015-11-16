package gorocksdb

// #include "rocksdb/c.h"
import "C"

type CompactionFilter interface {
	// The name of the compaction filter, for logging
	Name() string

	// If the Filter function returns false, it indicates
	// that the kv should be preserved, while a return value of true
	// indicates that this key-value should be removed from the
	// output of the compaction. The application can inspect
	// the existing value of the key and make decision based on it.
	//
	// When the value is to be preserved, the application has the option
	// to modify the existing value and pass it back through a new value.
	// To retain the previous value, simply return nil
	//
	// If multithreaded compaction is being used *and* a single CompactionFilter
	// instance was supplied via SetCompactionFilter, this the Filter function may be
	// called from different threads concurrently. The application must ensure
	// that the call is thread-safe.
	Filter(level int, key, val []byte) (remove bool, newVal []byte)
}

//export gorocksdb_compactionfilter_filter
func gorocksdb_compactionfilter_filter(handle *CompactionFilter, cLevel C.int, cKey *C.char, cKeyLen C.size_t, cVal *C.char, cValLen C.size_t, cNewVal **C.char, cNewValLen *C.size_t, cValChanged *C.uchar) C.int {
	key := charToByte(cKey, cKeyLen)
	val := charToByte(cVal, cValLen)
	remove, newVal := (*handle).Filter(int(cLevel), key, val)

	if remove {
		return C.int(1)
	} else if newVal != nil {
		*cNewVal = byteToChar(newVal)
		*cNewValLen = C.size_t(len(newVal))
		*cValChanged = C.uchar(1)
	}
	return C.int(0)
}

//export gorocksdb_compactionfilter_name
func gorocksdb_compactionfilter_name(handle *CompactionFilter) *C.char {
	return stringToChar((*handle).Name())
}
